package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"puresend/internal/p2p"
	"puresend/internal/transfer"
)

// TestFolderOverTheNetwork sends a nested folder through two real libp2p
// hosts and checks the tree arrives intact. The unit tests cover the
// protocol over a pipe; this covers it over the transport people use.
func TestFolderOverTheNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, _, serverAddr := newWSServer(t)

	sender, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	receiver, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	root := filepath.Join(t.TempDir(), "tatil")
	tree := map[string][]byte{
		"foto.jpg":          bytes.Repeat([]byte("j"), 40_000),
		"2024/deniz.png":    bytes.Repeat([]byte("p"), 90_000),
		"2024/ek/notlar.md": []byte("# notlar"),
		"2024/ek/bos.txt":   {},
	}
	for rel, data := range tree {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sendDone := drain(t, sender)
	room, err := sender.Host(ctx, []string{root})
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	recvDone := drain(t, receiver)
	go receiver.Fetch(ctx, room, outDir)

	if d := <-recvDone; d.Err != nil {
		t.Fatalf("receive: %v", d.Err)
	}
	if d := <-sendDone; d.Err != nil {
		t.Fatalf("send: %v", d.Err)
	}

	for rel, want := range tree {
		got, err := os.ReadFile(filepath.Join(outDir, "tatil", filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("tatil/%s was not rebuilt: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("tatil/%s: content differs from the original", rel)
		}
	}
}

// TestProgressReportsOverallTotals checks the numbers the interface builds
// its bar, speed and remaining time from. Per-file counters alone made a
// folder of a thousand files look like a thousand separate transfers.
func TestProgressReportsOverallTotals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, _, serverAddr := newWSServer(t)

	sender, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	receiver, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	srcDir := t.TempDir()
	var paths []string
	var want int64
	for i, n := range []int{120_000, 60_000, 200_000} {
		p := filepath.Join(srcDir, fmt.Sprintf("part-%d.bin", i))
		if err := os.WriteFile(p, bytes.Repeat([]byte("x"), n), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
		want += int64(n)
	}

	go func() {
		for {
			select {
			case <-sender.Done():
				return
			case <-sender.Events():
			}
		}
	}()
	room, err := sender.Host(ctx, paths)
	if err != nil {
		t.Fatal(err)
	}

	var last transfer.Progress
	done := make(chan error, 1)
	go func() {
		for {
			select {
			case <-receiver.Done():
				return
			case ev := <-receiver.Events():
				switch e := ev.(type) {
				case p2p.ManifestEvent:
					e.Reply <- true
				case p2p.ProgressEvent:
					last = e.Progress
				case p2p.DoneEvent:
					done <- e.Err
					return
				}
			}
		}
	}()
	go receiver.Fetch(ctx, room, t.TempDir())

	if err := <-done; err != nil {
		t.Fatalf("receive: %v", err)
	}
	if last.OverallTotal != want {
		t.Errorf("overall total = %d, want %d", last.OverallTotal, want)
	}
	if last.OverallDone != want {
		t.Errorf("overall done = %d at the end, want %d", last.OverallDone, want)
	}
	if last.Files != len(paths) {
		t.Errorf("file count = %d, want %d", last.Files, len(paths))
	}
}

// TestResumeOverTheNetwork checks that a half-finished download is picked
// up rather than started over — the feature that matters most on a mobile
// link, and the one most likely to break quietly, since a resume that
// silently restarts still produces a correct file.
//
// The interruption is staged rather than performed: on localhost a
// transfer finishes long before a test could cut it, so the leftovers of
// an earlier attempt are written to disk directly. Everything after that
// is the real thing — the offsets travel over the wire, and the sender
// seeks. (The interruption itself is covered in the transfer package,
// where the connection can be cut at an exact byte.)
func TestResumeOverTheNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, _, serverAddr := newWSServer(t)

	sender, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	receiver, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	payload := bytes.Repeat([]byte("puresend"), 200_000) // ~3 MB
	srcPath := filepath.Join(t.TempDir(), "video.bin")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	// What an interrupted attempt would have left behind: the first third
	// of the file, named after the digest of the whole.
	const kept = 1 << 20
	outDir := t.TempDir()
	partDir := filepath.Join(outDir, ".puresend-partial")
	if err := os.MkdirAll(partDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	partPath := filepath.Join(partDir, hex.EncodeToString(sum[:])+".part")
	if err := os.WriteFile(partPath, payload[:kept], 0o600); err != nil {
		t.Fatal(err)
	}

	sendDone := drain(t, sender)
	room, err := sender.Host(ctx, []string{srcPath})
	if err != nil {
		t.Fatal(err)
	}

	var firstReport transfer.Progress
	seen := false
	done := make(chan error, 1)
	go func() {
		for {
			select {
			case <-receiver.Done():
				return
			case ev := <-receiver.Events():
				switch e := ev.(type) {
				case p2p.ManifestEvent:
					e.Reply <- true
				case p2p.ProgressEvent:
					if !seen {
						firstReport, seen = e.Progress, true
					}
				case p2p.DoneEvent:
					done <- e.Err
					return
				}
			}
		}
	}()
	go receiver.Fetch(ctx, room, outDir)

	if err := <-done; err != nil {
		t.Fatalf("resumed receive: %v", err)
	}
	if d := <-sendDone; d.Err != nil {
		t.Fatalf("resumed send: %v", d.Err)
	}

	if firstReport.Done != kept {
		t.Errorf("resumed from byte %d, want %d — the leftovers were ignored", firstReport.Done, kept)
	}
	got, err := os.ReadFile(filepath.Join(outDir, "video.bin"))
	if err != nil {
		t.Fatalf("the resumed file is missing: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("the resumed file does not match the original")
	}
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Error("the partial download was not cleaned up after success")
	}
}
