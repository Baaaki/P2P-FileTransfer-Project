package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRoundTrip sends real files through an in-memory pipe and checks
// they arrive byte-identical, including a 0-byte file.
func TestRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	want := map[string][]byte{
		"big.bin":   bytes.Repeat([]byte("filetransferilla"), 20000), // ~320 KB, spans many chunks
		"note.txt":  []byte("merhaba"),
		"empty.dat": {},
	}
	var paths []string
	for name, data := range want {
		paths = append(paths, writeTempFile(t, srcDir, name, data))
	}

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, paths, nil) }()

	saved, err := Receive(receiver, outDir, nil, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(saved) != len(want) {
		t.Fatalf("saved %d files, want %d", len(saved), len(want))
	}
	for _, p := range saved {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want[filepath.Base(p)]) {
			t.Errorf("%s: content differs from the original", filepath.Base(p))
		}
	}
}

// TestNameCollision checks that an existing file is never overwritten:
// the incoming file must be saved under a new name like "note (1).txt".
func TestNameCollision(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	existing := writeTempFile(t, outDir, "note.txt", []byte("old content"))
	src := writeTempFile(t, srcDir, "note.txt", []byte("new content"))

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, []string{src}, nil) }()

	saved, err := Receive(receiver, outDir, nil, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("Send: %v", err)
	}

	if got := filepath.Base(saved[0]); got != "note (1).txt" {
		t.Errorf("saved as %q, want %q", got, "note (1).txt")
	}
	if data, _ := os.ReadFile(existing); string(data) != "old content" {
		t.Error("existing file was overwritten")
	}
}

// fakeSend plays the sender side of the protocol with a hand-crafted
// manifest, so tests can exercise the receiver against malicious input.
func fakeSend(t *testing.T, s net.Conn, m Manifest, payload []byte) {
	t.Helper()
	defer s.Close()
	if err := json.NewEncoder(s).Encode(m); err != nil {
		return // receiver aborted; the test asserts on Receive's error
	}
	dec := json.NewDecoder(s)
	var goAhead ack
	if err := dec.Decode(&goAhead); err != nil || !goAhead.OK {
		return
	}
	if len(payload) > 0 {
		if _, err := s.Write(payload); err != nil {
			return
		}
	}
	var a ack
	dec.Decode(&a)
}

// TestDeclinedTransfer checks that a declined manifest stops the
// transfer before anything reaches the disk, on both sides.
func TestDeclinedTransfer(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	src := writeTempFile(t, srcDir, "note.txt", []byte("data"))

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, []string{src}, nil) }()

	decline := func(Manifest) bool { return false }
	if _, err := Receive(receiver, outDir, decline, nil); err == nil {
		t.Fatal("Receive reported success for a declined transfer")
	}
	if err := <-sendErr; err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("sender did not observe the decline: %v", err)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Error("output directory was created for a declined transfer")
	}
}

// TestPathTraversalName checks that a malicious file name like
// "../evil.txt" cannot escape the output directory.
func TestPathTraversalName(t *testing.T) {
	outDir := t.TempDir()
	payload := []byte("gotcha")
	sum := sha256.Sum256(payload)
	m := Manifest{Files: []FileInfo{{
		Name:   "../evil.txt",
		Size:   int64(len(payload)),
		SHA256: hex.EncodeToString(sum[:]),
	}}}

	sender, receiver := net.Pipe()
	go fakeSend(t, sender, m, payload)

	saved, err := Receive(receiver, outDir, nil, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if dir := filepath.Dir(saved[0]); dir != outDir {
		t.Errorf("file saved outside the output directory: %s", saved[0])
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(outDir), "evil.txt")); !os.IsNotExist(err) {
		t.Error("file escaped the output directory")
	}
}

// TestManifestSizeLimit checks that an absurdly large manifest is
// rejected instead of being buffered into memory.
func TestManifestSizeLimit(t *testing.T) {
	outDir := t.TempDir()
	m := Manifest{Files: []FileInfo{{
		Name: strings.Repeat("a", 2<<20), // 2 MB name → manifest over the 1 MB cap
		Size: 1,
	}}}

	sender, receiver := net.Pipe()
	go fakeSend(t, sender, m, nil)

	if _, err := Receive(receiver, outDir, nil, nil); err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("expected a manifest size error, got: %v", err)
	}
}

// eagerEOFReader returns io.EOF together with the final chunk of data,
// which the io.Reader contract explicitly allows.
type eagerEOFReader struct{ data []byte }

func (r *eagerEOFReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

// TestReceiveFileDataWithEOF checks that a complete file is accepted
// even when the last Read reports io.EOF alongside the data.
func TestReceiveFileDataWithEOF(t *testing.T) {
	outDir := t.TempDir()
	payload := []byte("last read carries EOF")
	sum := sha256.Sum256(payload)
	info := FileInfo{Name: "f.txt", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}

	path, err := receiveFile(&eagerEOFReader{data: payload}, outDir, info, nil)
	if err != nil {
		t.Fatalf("receiveFile: %v", err)
	}
	if data, _ := os.ReadFile(path); !bytes.Equal(data, payload) {
		t.Error("received content differs from the original")
	}
}

// TestChecksumMismatch checks that a wrong digest is detected and
// reported as an error.
func TestChecksumMismatch(t *testing.T) {
	outDir := t.TempDir()
	payload := []byte("data")
	m := Manifest{Files: []FileInfo{{
		Name:   "f.bin",
		Size:   int64(len(payload)),
		SHA256: strings.Repeat("00", 32), // deliberately wrong
	}}}

	sender, receiver := net.Pipe()
	go fakeSend(t, sender, m, payload)

	if _, err := Receive(receiver, outDir, nil, nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected a checksum error, got: %v", err)
	}

	// A failed transfer must not leave anything behind — neither the
	// final file nor a .part temp file.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("failed transfer left a file behind: %s", e.Name())
	}
}
