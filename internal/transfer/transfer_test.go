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

const testCode = "kiraz-liman-42"

// testCreds is what both honest sides of a test transfer agree on.
func testCreds() Credentials {
	return Credentials{Code: testCode, Sender: "sender-peer", Receiver: "receiver-peer"}
}

func writeTempFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
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
		"big.bin":   bytes.Repeat([]byte("puresend"), 20000), // ~320 KB, spans many chunks
		"note.txt":  []byte("merhaba"),
		"empty.dat": {},
	}
	var paths []string
	for name, data := range want {
		paths = append(paths, writeTempFile(t, srcDir, name, data))
	}

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- SendPaths(sender, paths, testCreds(), Hooks{}) }()

	saved, err := Receive(receiver, outDir, testCreds(), nil, Hooks{})
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

// TestFolderRoundTrip sends a whole directory and checks the tree is
// rebuilt on the other side, nesting and all.
func TestFolderRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	tree := map[string][]byte{
		"tatil/foto.jpg":          []byte("jpeg"),
		"tatil/2024/deniz.png":    []byte("png"),
		"tatil/2024/ek/notlar.md": []byte("# notlar"),
	}
	for rel, data := range tree {
		writeTempFile(t, srcDir, rel, data)
	}

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	root := filepath.Join(srcDir, "tatil")
	go func() { sendErr <- SendPaths(sender, []string{root}, testCreds(), Hooks{}) }()

	saved, err := Receive(receiver, outDir, testCreds(), nil, Hooks{})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(saved) != len(tree) {
		t.Fatalf("saved %d files, want %d", len(saved), len(tree))
	}
	for rel, data := range tree {
		got, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s was not rebuilt: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, data) {
			t.Errorf("%s: content differs from the original", rel)
		}
	}
}

// TestCollectSkipsSymlinks checks that a link inside a chosen folder is
// left alone: following it would send files the user never picked.
func TestCollectSkipsSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	writeTempFile(t, srcDir, "tree/real.txt", []byte("real"))
	secret := writeTempFile(t, srcDir, "secret.txt", []byte("secret"))
	if err := os.Symlink(secret, filepath.Join(srcDir, "tree", "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	entries, err := Collect([]string{filepath.Join(srcDir, "tree")})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Rel != "tree/real.txt" {
		t.Fatalf("Collect walked into a symlink: %+v", entries)
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
	go func() { sendErr <- SendPaths(sender, []string{src}, testCreds(), Hooks{}) }()

	saved, err := Receive(receiver, outDir, testCreds(), nil, Hooks{})
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

// ---------------------------------------------------------------------------
// The room-code handshake
// ---------------------------------------------------------------------------

// TestWrongCodeRejected checks that a receiver with the wrong code learns
// nothing: not the file list, not even how many files there are.
func TestWrongCodeRejected(t *testing.T) {
	srcDir := t.TempDir()
	src := writeTempFile(t, srcDir, "gizli.txt", []byte("secret"))

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- SendPaths(sender, []string{src}, testCreds(), Hooks{}) }()

	wrong := testCreds()
	wrong.Code = "zebra-zebra-99"

	var seen *Manifest
	confirm := func(m Manifest) bool { seen = &m; return true }
	if _, err := Receive(receiver, t.TempDir(), wrong, confirm, Hooks{}); err == nil {
		t.Fatal("Receive accepted a wrong room code")
	}
	if seen != nil {
		t.Error("the file list leaked to a receiver with the wrong code")
	}
	if err := <-sendErr; err == nil {
		t.Error("Send accepted a receiver with the wrong room code")
	}
}

// TestIdentityMismatchRejected is the attack the handshake exists for: a
// rendezvous server that hands the receiver an impostor's peer ID, or a
// peer that relays the handshake between the two honest ends. Both sides
// hold the right code, but they disagree about who is on the other end, so
// the session keys differ and confirmation fails.
func TestIdentityMismatchRejected(t *testing.T) {
	srcDir := t.TempDir()
	src := writeTempFile(t, srcDir, "note.txt", []byte("data"))

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- SendPaths(sender, []string{src}, testCreds(), Hooks{}) }()

	lied := testCreds()
	lied.Sender = "someone-elses-peer-id"

	if _, err := Receive(receiver, t.TempDir(), lied, nil, Hooks{}); err == nil {
		t.Fatal("Receive accepted a sender it was misinformed about")
	}
	if err := <-sendErr; err == nil {
		t.Error("Send completed against a mismatched identity")
	}
}

// TestHandshakeErrorIsVague checks that a failed handshake does not report
// which half went wrong — that detail would only ever help a guesser.
func TestHandshakeErrorIsVague(t *testing.T) {
	a, b := net.Pipe()
	go func() {
		enc := json.NewEncoder(a)
		dec := json.NewDecoder(a)
		creds := testCreds()
		creds.Code = "baska-bir-kod-11"
		authenticate(roleSender, creds,
			func(m *authMsg) error { return dec.Decode(m) },
			func(m authMsg) error { return enc.Encode(m) })
		a.Close()
	}()

	enc := json.NewEncoder(b)
	dec := json.NewDecoder(b)
	_, err := authenticate(roleReceiver, testCreds(),
		func(m *authMsg) error { return dec.Decode(m) },
		func(m authMsg) error { return enc.Encode(m) })
	if err == nil {
		t.Fatal("handshake succeeded with mismatched codes")
	}
	if !strings.Contains(err.Error(), "room code") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Resume
// ---------------------------------------------------------------------------

// cutAfter is a connection that stops working once n bytes have been
// written through it — a link that drops in the middle of a transfer.
type cutAfter struct {
	net.Conn
	left int64
}

func (c *cutAfter) Write(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, io.ErrClosedPipe
	}
	if int64(len(p)) > c.left {
		n, _ := c.Conn.Write(p[:c.left])
		c.left = 0
		return n, io.ErrClosedPipe
	}
	c.left -= int64(len(p))
	return c.Conn.Write(p)
}

// TestResumeAfterInterruption cuts a transfer in half, retries it, and
// checks the second attempt continues from where the first stopped instead
// of downloading everything again.
func TestResumeAfterInterruption(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	payload := bytes.Repeat([]byte("puresend"), 64*1024) // 1 MB
	src := writeTempFile(t, srcDir, "video.bin", payload)

	// First attempt: dies partway through.
	sender, receiver := net.Pipe()
	go SendPaths(&cutAfter{Conn: sender, left: 200 << 10}, []string{src}, testCreds(), Hooks{})
	if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); err == nil {
		t.Fatal("the interrupted transfer reported success")
	}

	// The leftovers are kept, under the digest of the file they belong to.
	sum := sha256.Sum256(payload)
	partPath := filepath.Join(outDir, partialDir, hex.EncodeToString(sum[:])+".part")
	st, err := os.Stat(partPath)
	if err != nil {
		t.Fatalf("nothing was kept to resume from: %v", err)
	}
	if st.Size() == 0 || st.Size() >= int64(len(payload)) {
		t.Fatalf("partial download is %d bytes, expected a fraction of %d", st.Size(), len(payload))
	}

	// Second attempt: must pick up where the first stopped.
	var firstReport Progress
	seen := false
	hooks := Hooks{Progress: func(p Progress) {
		if !seen {
			firstReport, seen = p, true
		}
	}}

	sender, receiver = net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- SendPaths(sender, []string{src}, testCreds(), Hooks{}) }()

	saved, err := Receive(receiver, outDir, testCreds(), nil, hooks)
	if err != nil {
		t.Fatalf("resumed Receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("resumed Send: %v", err)
	}
	if firstReport.Done != st.Size() {
		t.Errorf("resumed from byte %d, want %d", firstReport.Done, st.Size())
	}
	if data, _ := os.ReadFile(saved[0]); !bytes.Equal(data, payload) {
		t.Error("the resumed file does not match the original")
	}
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Error("the partial download was not cleaned up after success")
	}
}

// TestRejectsImpossibleResumeOffset checks the sender does not trust the
// receiver's offsets: one past the end of the file must be refused rather
// than turned into a seek into nowhere.
func TestRejectsImpossibleResumeOffset(t *testing.T) {
	m := Manifest{Files: []FileInfo{{Path: "a.bin", Size: 10}}}
	if _, err := checkOffsets([]int64{11}, m); err == nil {
		t.Error("an offset past the end of the file was accepted")
	}
	if _, err := checkOffsets([]int64{-1}, m); err == nil {
		t.Error("a negative offset was accepted")
	}
	if _, err := checkOffsets([]int64{0, 0}, m); err == nil {
		t.Error("a mismatched number of offsets was accepted")
	}
	got, err := checkOffsets(nil, m)
	if err != nil || len(got) != 1 || got[0] != 0 {
		t.Errorf("a sender talking to an offset-less receiver got %v, %v", got, err)
	}
}

// ---------------------------------------------------------------------------
// Hostile input
// ---------------------------------------------------------------------------

// fakeSend plays the sender side of the protocol with a hand-crafted
// manifest, so tests can exercise the receiver against malicious input.
func fakeSend(t *testing.T, s net.Conn, m Manifest, payload []byte) {
	t.Helper()
	defer s.Close()

	enc := json.NewEncoder(s)
	dec := json.NewDecoder(s)
	if _, err := authenticate(roleSender, testCreds(),
		func(m *authMsg) error { return dec.Decode(m) },
		func(m authMsg) error { return enc.Encode(m) },
	); err != nil {
		return
	}

	if err := enc.Encode(offerMsg{Manifest: &m}); err != nil {
		return // receiver aborted; the test asserts on Receive's error
	}
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
	go func() { sendErr <- SendPaths(sender, []string{src}, testCreds(), Hooks{}) }()

	decline := func(Manifest) bool { return false }
	if _, err := Receive(receiver, outDir, testCreds(), decline, Hooks{}); err == nil {
		t.Fatal("Receive reported success for a declined transfer")
	}
	if err := <-sendErr; err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("sender did not observe the decline: %v", err)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Error("output directory was created for a declined transfer")
	}
}

// TestUnsafePaths checks the names a hostile sender might put in a
// manifest. All of them have to be refused before anything is written —
// and refused up front, not halfway through the transfer.
func TestUnsafePaths(t *testing.T) {
	for _, name := range []string{
		"../evil.txt",
		"a/../../evil.txt",
		"/etc/passwd",
		`..\evil.txt`,
		`C:\Windows\evil.txt`,
		"",
		".",
		partialDir + "/evil.txt",
	} {
		t.Run(name, func(t *testing.T) {
			outDir := t.TempDir()
			payload := []byte("gotcha")
			sum := sha256.Sum256(payload)
			m := Manifest{Files: []FileInfo{{
				Path:   name,
				Size:   int64(len(payload)),
				SHA256: hex.EncodeToString(sum[:]),
			}}}

			sender, receiver := net.Pipe()
			go fakeSend(t, sender, m, payload)

			if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); err == nil {
				t.Fatalf("Receive accepted the path %q", name)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(outDir), "evil.txt")); !os.IsNotExist(err) {
				t.Error("a file escaped the output directory")
			}
		})
	}
}

// TestSafeJoinAcceptsOrdinaryPaths guards the other half of safeJoin: the
// names it must let through unchanged.
func TestSafeJoinAcceptsOrdinaryPaths(t *testing.T) {
	outDir := filepath.Join("tmp", "out")
	for rel, want := range map[string]string{
		"foto.jpg":              filepath.Join(outDir, "foto.jpg"),
		"tatil/foto.jpg":        filepath.Join(outDir, "tatil", "foto.jpg"),
		"tatil/./2024/foto.jpg": filepath.Join(outDir, "tatil", "2024", "foto.jpg"),
		"tatil//2024/foto.jpg":  filepath.Join(outDir, "tatil", "2024", "foto.jpg"),
		"boşluklu ad.txt":       filepath.Join(outDir, "boşluklu ad.txt"),
	} {
		got, err := safeJoin(outDir, rel)
		if err != nil {
			t.Errorf("safeJoin(%q): %v", rel, err)
			continue
		}
		if got != want {
			t.Errorf("safeJoin(%q) = %q, want %q", rel, got, want)
		}
	}
}

// TestManifestSizeLimit checks that an absurdly large manifest is
// rejected instead of being buffered into memory.
func TestManifestSizeLimit(t *testing.T) {
	outDir := t.TempDir()
	m := Manifest{Files: []FileInfo{{
		Path: strings.Repeat("a", 2<<20), // 2 MB name → manifest over the 1 MB cap
		Size: 1,
	}}}

	sender, receiver := net.Pipe()
	go fakeSend(t, sender, m, nil)

	if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); err == nil || !strings.Contains(err.Error(), "manifest") {
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
	info := FileInfo{Path: "f.txt", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}

	p := partial{path: filepath.Join(outDir, "f.txt.part"), hasher: sha256.New()}
	path, err := receiveFile(&eagerEOFReader{data: payload}, deadlines{}, filepath.Join(outDir, "f.txt"), info, p, nil)
	if err != nil {
		t.Fatalf("receiveFile: %v", err)
	}
	if data, _ := os.ReadFile(path); !bytes.Equal(data, payload) {
		t.Error("received content differs from the original")
	}
}

// TestChecksumMismatch checks that a wrong digest is detected and reported,
// and that the poisoned leftovers are thrown away rather than kept for a
// resume that would fail exactly the same way.
func TestChecksumMismatch(t *testing.T) {
	outDir := t.TempDir()
	payload := []byte("data")
	m := Manifest{Files: []FileInfo{{
		Path:   "f.bin",
		Size:   int64(len(payload)),
		SHA256: strings.Repeat("00", 32), // deliberately wrong
	}}}

	sender, receiver := net.Pipe()
	go fakeSend(t, sender, m, payload)

	if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected a checksum error, got: %v", err)
	}

	// A failed transfer must not leave a finished-looking file behind, and
	// the discredited partial must be gone too.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != partialDir {
			t.Errorf("failed transfer left a file behind: %s", e.Name())
		}
	}
	leftovers, err := os.ReadDir(filepath.Join(outDir, partialDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range leftovers {
		t.Errorf("a partial download with a bad digest was kept: %s", e.Name())
	}
}
