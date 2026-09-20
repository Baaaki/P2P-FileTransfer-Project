package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// shortTimeouts makes every deadline expire quickly, so the tests that
// wait for one to fire do not wait for minutes.
func shortTimeouts(t *testing.T) {
	t.Helper()
	saved := timeouts
	timeouts.auth = 300 * time.Millisecond
	timeouts.offerGap = 2 * time.Second
	timeouts.heartbeat = 50 * time.Millisecond
	timeouts.approval = 300 * time.Millisecond
	timeouts.idle = 300 * time.Millisecond
	timeouts.linger = 300 * time.Millisecond
	t.Cleanup(func() { timeouts = saved })
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestManifestFieldsAreValidated covers the fields safeJoin never saw. The
// digest names the partial download a resume continues from, so a digest
// that is really a path — "../../../../home/ali/x" — used to create,
// truncate or delete a file outside the target folder.
func TestManifestFieldsAreValidated(t *testing.T) {
	payload := []byte("gotcha")
	good := digestOf(payload)

	for name, f := range map[string]FileInfo{
		"digest that is a path":  {Path: "a.txt", Size: 6, SHA256: "../../../../x"},
		"digest too short":       {Path: "a.txt", Size: 6, SHA256: good[:63]},
		"digest in upper case":   {Path: "a.txt", Size: 6, SHA256: strings.ToUpper(good)},
		"digest with a slash":    {Path: "a.txt", Size: 6, SHA256: good[:62] + "/x"},
		"empty digest":           {Path: "a.txt", Size: 6},
		"negative size":          {Path: "a.txt", Size: -1, SHA256: good},
		"control character name": {Path: "\x1b[2Jtemiz.txt", Size: 6, SHA256: good},
		"bidi override name":     {Path: "foto\u202Egpj.exe", Size: 6, SHA256: good},
		"name too long":          {Path: strings.Repeat("a", maxNameBytes+1), Size: 6, SHA256: good},
	} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			outDir := filepath.Join(parent, "out")
			m := Manifest{Files: []FileInfo{f}}

			sender, receiver := net.Pipe()
			go fakeSend(t, sender, m, payload)

			confirmed := false
			confirm := func(Manifest) bool { confirmed = true; return true }
			if _, err := Receive(receiver, outDir, testCreds(), confirm, Hooks{}); err == nil {
				t.Fatal("Receive accepted the manifest")
			}
			if confirmed {
				t.Error("an invalid manifest reached the user's approval screen")
			}
			// Nothing at all may have been written, inside or outside.
			entries, _ := os.ReadDir(parent)
			for _, e := range entries {
				t.Errorf("something was written for a refused manifest: %s", e.Name())
			}
		})
	}
}

// TestManifestTotalCannotOverflow guards the sum the progress bar and the
// relay warning are computed from.
func TestManifestTotalCannotOverflow(t *testing.T) {
	d := digestOf(nil)
	m := Manifest{Files: []FileInfo{
		{Path: "a", Size: math.MaxInt64, SHA256: d},
		{Path: "b", Size: 1, SHA256: d},
	}}
	if _, err := checkManifest(t.TempDir(), m); err == nil {
		t.Error("a manifest whose total overflows was accepted")
	}
}

// TestWindowsNames covers the names a Mac or Linux sender can produce and
// Windows cannot store. Without the check they failed at the very last
// step, the rename, with an error nobody could read.
func TestWindowsNames(t *testing.T) {
	for name, want := range map[string]bool{
		"foto.jpg":        true,
		"boşluklu ad.txt": true,
		"con.txt.bak":     false,
		"CON":             false,
		"nul.txt":         false,
		"Com1.log":        false,
		"LPT9":            false,
		"COM¹":            false,
		"console.txt":     true, // only the exact device names are taken
		"aux-notes.md":    true,
		"a<b.txt":         false,
		"soru?.txt":       false,
		"yıldız*.txt":     false,
		`tırnak".txt`:     false,
		"boru|.txt":       false,
		"sonunda nokta.":  false,
		"sonunda boşluk ": false,
		"NUL .txt":        false,
	} {
		if got := windowsCanStore(name); got != want {
			t.Errorf("windowsCanStore(%q) = %v, want %v", name, got, want)
		}
	}

	// And through the real receiving path, as it runs on Windows.
	checkWindowsNames = true
	t.Cleanup(func() { checkWindowsNames = false })

	m := Manifest{Files: []FileInfo{{Path: "klasor/CON.txt", Size: 1, SHA256: digestOf([]byte("x"))}}}
	sender, receiver := net.Pipe()
	go fakeSend(t, sender, m, []byte("x"))
	_, err := Receive(receiver, t.TempDir(), testCreds(), nil, Hooks{})
	if err == nil || !strings.Contains(err.Error(), "cannot store") {
		t.Fatalf("a reserved Windows name got through: %v", err)
	}
}

// TestSilentReceiverTimesOut covers the stall where a peer that
// connects and says nothing used to hold the sender's handler — and with
// it the room — for as long as it liked.
func TestSilentReceiverTimesOut(t *testing.T) {
	shortTimeouts(t)
	src := writeTempFile(t, t.TempDir(), "a.txt", []byte("data"))
	offer, err := NewOffer([]string{src})
	if err != nil {
		t.Fatal(err)
	}

	sender, receiver := net.Pipe()
	defer receiver.Close()

	var claimed atomic.Bool
	done := make(chan error, 1)
	go func() {
		done <- Send(sender, offer, testCreds(), SendOptions{
			Claim: func() bool { claimed.Store(true); return true },
		})
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stopped responding") {
			t.Errorf("a silent receiver ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the sender waited for a silent receiver without end")
	}
	if claimed.Load() {
		t.Error("the room was claimed by someone who never proved the code")
	}
}

// TestWrongCodeNeverClaims checks the other half: the room is taken only
// after the handshake, so a stranger without the code cannot keep the
// friend it was meant for out, not even for a moment.
func TestWrongCodeNeverClaims(t *testing.T) {
	src := writeTempFile(t, t.TempDir(), "a.txt", []byte("data"))
	offer, _ := NewOffer([]string{src})

	sender, receiver := net.Pipe()
	var claimed atomic.Bool
	sendErr := make(chan error, 1)
	go func() {
		sendErr <- Send(sender, offer, testCreds(), SendOptions{
			Claim: func() bool { claimed.Store(true); return true },
		})
	}()

	wrong := testCreds()
	wrong.Code = "zebra-zebra-99"
	Receive(receiver, t.TempDir(), wrong, nil, Hooks{})

	if err := <-sendErr; !errors.Is(err, ErrWrongCode) {
		t.Errorf("sender reported %v, want the wrong-code error", err)
	}
	if claimed.Load() {
		t.Error("a receiver with the wrong code claimed the room")
	}
}

// TestBusyRoomTurnsAway checks what a second receiver with the right code
// hears while the room is taken: a clear "busy", not a reset connection
// that reads like a network fault.
func TestBusyRoomTurnsAway(t *testing.T) {
	src := writeTempFile(t, t.TempDir(), "a.txt", []byte("data"))
	offer, _ := NewOffer([]string{src})

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() {
		sendErr <- Send(sender, offer, testCreds(), SendOptions{Claim: func() bool { return false }})
	}()

	outDir := filepath.Join(t.TempDir(), "out")
	if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); !errors.Is(err, ErrBusy) {
		t.Errorf("receiver got %v, want ErrBusy", err)
	}
	if err := <-sendErr; !errors.Is(err, ErrBusy) {
		t.Errorf("sender got %v, want ErrBusy", err)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Error("a turned-away receiver created its download folder")
	}
}

// TestApprovalWaitIsBounded checks that a receiver who never answers the
// manifest releases the sender after a while.
func TestApprovalWaitIsBounded(t *testing.T) {
	shortTimeouts(t)
	src := writeTempFile(t, t.TempDir(), "a.txt", []byte("data"))
	offer, _ := NewOffer([]string{src})

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, offer, testCreds(), SendOptions{}) }()

	release := make(chan struct{})
	received := make(chan struct{})
	outDir := t.TempDir()
	go func() {
		defer close(received)
		Receive(receiver, outDir, testCreds(), func(Manifest) bool {
			<-release // a person who walked away from the screen
			return false
		}, Hooks{})
	}()
	// Cleanups run last-in first-out: this one finishes the receiver before
	// shortTimeouts puts the real deadlines back.
	t.Cleanup(func() { close(release); <-received })

	select {
	case err := <-sendErr:
		if err == nil || !strings.Contains(err.Error(), "no answer from receiver") {
			t.Errorf("sender ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the sender waited for an answer without end")
	}
}

// TestReceiverHearsFromAPreparingSender checks the notes a sender sends
// while it is still reading a large folder: the receiver learns it is
// alive and how far along it is, instead of timing out or staring at
// "preparing" with nothing to go on.
func TestReceiverHearsFromAPreparingSender(t *testing.T) {
	shortTimeouts(t)
	data := []byte("data")
	src := writeTempFile(t, t.TempDir(), "a.txt", data)
	offer, _ := NewOffer([]string{src})
	// Mark the offer as being read without reading it yet, so the test
	// decides when the read finishes.
	offer.startOnce.Do(func() {})

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, offer, testCreds(), SendOptions{}) }()

	var notes atomic.Int32
	hooks := Hooks{Waiting: func(done, total int) {
		if notes.Add(1) == 3 {
			go offer.read(context.Background(), Hooks{})
		}
	}}
	outDir := t.TempDir()
	saved, err := Receive(receiver, outDir, testCreds(), nil, hooks)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("Send: %v", err)
	}
	if notes.Load() < 3 {
		t.Errorf("the receiver heard %d progress notes, want at least 3", notes.Load())
	}
	if got, _ := os.ReadFile(saved[0]); !bytes.Equal(got, data) {
		t.Error("the file did not arrive intact")
	}
}

// TestOfferNoticesChangedFiles checks that reading the files ahead of time
// cannot hand out a stale digest: a file edited after it was read is read
// again before it is offered.
func TestOfferNoticesChangedFiles(t *testing.T) {
	src := writeTempFile(t, t.TempDir(), "notlar.txt", []byte("first draft"))
	offer, _ := NewOffer([]string{src})
	offer.Start(context.Background(), Hooks{})
	<-offer.Ready()

	// Same size, new content, a different modification time.
	edited := []byte("final draft")
	if err := os.WriteFile(src, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(src, later, later); err != nil {
		t.Fatal(err)
	}

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, offer, testCreds(), SendOptions{}) }()
	saved, err := Receive(receiver, t.TempDir(), testCreds(), nil, Hooks{})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got, _ := os.ReadFile(saved[0]); !bytes.Equal(got, edited) {
		t.Errorf("received %q, want the edited %q", got, edited)
	}
}

// TestVanishedFileIsASourceError checks that a file deleted after it was
// chosen is reported as the sender's problem — something retrying cannot
// fix — and that the receiver is told why instead of seeing a dead link.
func TestVanishedFileIsASourceError(t *testing.T) {
	src := writeTempFile(t, t.TempDir(), "gidecek.txt", []byte("data"))
	offer, _ := NewOffer([]string{src})
	offer.Start(context.Background(), Hooks{})
	<-offer.Ready()
	os.Remove(src)

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, offer, testCreds(), SendOptions{}) }()

	_, err := Receive(receiver, t.TempDir(), testCreds(), nil, Hooks{})
	if err == nil || !strings.Contains(err.Error(), "could not prepare") {
		t.Errorf("receiver got %v", err)
	}
	if err := <-sendErr; !IsSourceError(err) {
		t.Errorf("sender got %v, want a source error", err)
	}
}

// TestRemoteErrorTextIsCleaned checks that a message the other side
// chose cannot carry escape sequences into the sender's terminal.
func TestRemoteErrorTextIsCleaned(t *testing.T) {
	src := writeTempFile(t, t.TempDir(), "a.txt", []byte("data"))
	offer, _ := NewOffer([]string{src})

	sender, receiver := net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- Send(sender, offer, testCreds(), SendOptions{}) }()

	// A hostile receiver: proves the code, then declines with a message
	// built to clear the screen.
	enc := json.NewEncoder(receiver)
	dec := json.NewDecoder(receiver)
	if _, err := authenticate(roleReceiver, testCreds(),
		func(m *authMsg) error { return dec.Decode(m) },
		func(m authMsg) error { return enc.Encode(m) },
	); err != nil {
		t.Fatal(err)
	}
	var msg offerMsg
	for msg.Manifest == nil {
		if err := dec.Decode(&msg); err != nil {
			t.Fatal(err)
		}
	}
	enc.Encode(ack{OK: false, Error: "\x1b[2J\x1b[Hher şey yolunda"})
	receiver.Close()

	err := <-sendErr
	if err == nil || strings.ContainsRune(err.Error(), 0x1b) {
		t.Errorf("the sender's error carries the escape sequence: %q", err)
	}
}

// TestResumeWithIdenticalFiles is the folder holding the same photo twice.
// Both copies used to share one partial download: the first finished and
// took it away, and the second then continued "from where it stopped" in
// a file that no longer existed — padded with zeroes, failing its checksum
// on every retry.
func TestResumeWithIdenticalFiles(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	photo := bytes.Repeat([]byte("aynı fotoğraf "), 20_000) // ~300 KB
	writeTempFile(t, srcDir, "album/bir.jpg", photo)
	writeTempFile(t, srcDir, "album/iki.jpg", photo)
	root := filepath.Join(srcDir, "album")

	// First attempt dies partway through the first copy.
	sender, receiver := net.Pipe()
	go SendPaths(&cutAfter{Conn: sender, left: 100 << 10}, []string{root}, testCreds(), Hooks{})
	if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); err == nil {
		t.Fatal("the interrupted transfer reported success")
	}

	sender, receiver = net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- SendPaths(sender, []string{root}, testCreds(), Hooks{}) }()
	if _, err := Receive(receiver, outDir, testCreds(), nil, Hooks{}); err != nil {
		t.Fatalf("the resumed transfer failed: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("resumed Send: %v", err)
	}
	for _, name := range []string{"bir.jpg", "iki.jpg"} {
		got, err := os.ReadFile(filepath.Join(outDir, "album", name))
		if err != nil || !bytes.Equal(got, photo) {
			t.Errorf("%s did not arrive intact: %v", name, err)
		}
	}
}

// TestResumeKeepsFinishedFiles checks that a retry does not fetch again
// what the earlier attempt already finished — which used to arrive a
// second time as "bir (1).jpg" next to the first.
func TestResumeKeepsFinishedFiles(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	first := bytes.Repeat([]byte("1"), 50_000)
	second := bytes.Repeat([]byte("2"), 300_000)
	writeTempFile(t, srcDir, "album/a.bin", first)
	writeTempFile(t, srcDir, "album/b.bin", second)
	root := filepath.Join(srcDir, "album")

	// Cut after the first file is complete but well before the second is.
	sender, receiver := net.Pipe()
	go SendPaths(&cutAfter{Conn: sender, left: 150 << 10}, []string{root}, testCreds(), Hooks{})
	Receive(receiver, outDir, testCreds(), nil, Hooks{})
	if got, _ := os.ReadFile(filepath.Join(outDir, "album", "a.bin")); !bytes.Equal(got, first) {
		t.Fatal("setup: the first file should have been finished by the first attempt")
	}

	var resumedFrom []int64
	hooks := Hooks{Progress: func(p Progress) {
		if p.Index > len(resumedFrom) {
			resumedFrom = append(resumedFrom, p.Done)
		}
	}}
	sender, receiver = net.Pipe()
	sendErr := make(chan error, 1)
	go func() { sendErr <- SendPaths(sender, []string{root}, testCreds(), Hooks{}) }()
	saved, err := Receive(receiver, outDir, testCreds(), nil, hooks)
	if err != nil {
		t.Fatalf("the resumed transfer failed: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("resumed Send: %v", err)
	}

	if len(resumedFrom) == 0 || resumedFrom[0] != int64(len(first)) {
		t.Errorf("the finished file was fetched again (started at %v)", resumedFrom)
	}
	entries, _ := os.ReadDir(filepath.Join(outDir, "album"))
	if len(entries) != 2 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("album holds %v, want just a.bin and b.bin", names)
	}
	if got, _ := os.ReadFile(saved[1]); !bytes.Equal(got, second) {
		t.Error("the second file did not arrive intact")
	}
}

func FuzzSafeJoin(f *testing.F) {
	seeds := []string{
		"belge.txt",
		"klasor/resim.png",
		"a/b/c/d/e.txt",
		"../../etc/passwd",
		"../../../.bashrc",
		"/root/secret",
		"C:\\Windows\\explorer.exe",
		"CON",
		"NUL.txt",
		"AUX",
		"COM1",
		"a/./b",
		"a//b",
		"dosya.",
		"dosya ",
		"foto\u202Egpj.exe",
		"\x1b[2Jtemiz.txt",
		".puresend-partial/sinsi.bin",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	outDir := filepath.Join(os.TempDir(), "fuzz-out")
	f.Fuzz(func(t *testing.T, rel string) {
		target, err := safeJoin(outDir, rel)
		if err != nil {
			return
		}

		// Invariant 1: Target must strictly be inside outDir and not escape it.
		cleanOutDir := filepath.Clean(outDir)
		cleanTarget := filepath.Clean(target)
		if cleanTarget != cleanOutDir && !strings.HasPrefix(cleanTarget, cleanOutDir+string(filepath.Separator)) {
			t.Fatalf("safeJoin(%q) produced target outside outDir: target=%q, outDir=%q", rel, cleanTarget, cleanOutDir)
		}

		diff, relErr := filepath.Rel(outDir, target)
		if relErr != nil || diff == ".." || strings.HasPrefix(diff, ".."+string(filepath.Separator)) {
			t.Fatalf("safeJoin(%q) escaped outDir: target=%q, diff=%q", rel, target, diff)
		}

		// Invariant 2: Target must never begin with reserved partial download directory.
		cleanRel := filepath.ToSlash(diff)
		if strings.HasPrefix(cleanRel, partialDir+"/") || cleanRel == partialDir {
			t.Fatalf("safeJoin(%q) allowed reserved partialDir in target=%q", rel, target)
		}
	})
}

func FuzzValidDigest(f *testing.F) {
	seeds := []string{
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",  // sha256 of empty string
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",  // sha256 of "abc"
		"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855",  // uppercase
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85",   // 63 chars
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8555", // 65 chars
		"",
		"../../../../x",
		"g3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // 'g' is invalid hex
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		got := validDigest(s)
		if got {
			if len(s) != 64 {
				t.Fatalf("validDigest(%q) = true, but length is %d (want 64)", s, len(s))
			}
			for i := 0; i < len(s); i++ {
				c := s[i]
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Fatalf("validDigest(%q) = true, but contains non-hex char %c", s, c)
				}
			}
		}
	})
}
