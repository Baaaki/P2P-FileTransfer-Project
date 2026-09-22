package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"puresend/internal/p2p"
	"puresend/internal/rendezvous"

	"github.com/libp2p/go-libp2p/core/network"
)

// spyStream records every byte a client sends the meeting point.
type spyStream struct {
	network.Stream
	mu   *sync.Mutex
	seen *bytes.Buffer
}

func (s spyStream) Read(p []byte) (int, error) {
	n, err := s.Stream.Read(p)
	s.mu.Lock()
	s.seen.Write(p[:n])
	s.mu.Unlock()
	return n, err
}

// TestServerNeverSeesTheWords is the property the nameplate exists for. A
// meeting point that is told the whole code can run the handshake with
// both ends itself and sit in the middle of the transfer — reading every
// file, or swapping them. So across a whole real transfer, register to
// unregister, nothing a client sends the server may contain the code's
// secret words.
func TestServerNeverSeesTheWords(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	server, registry, serverAddr := newWSServer(t)
	var mu sync.Mutex
	var seen bytes.Buffer
	server.SetStreamHandler(rendezvous.ProtocolID, func(s network.Stream) {
		registry.Handler(spyStream{Stream: s, mu: &mu, seen: &seen})
	})

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

	srcPath := filepath.Join(t.TempDir(), "belge.txt")
	if err := os.WriteFile(srcPath, []byte("gizli belge"), 0o644); err != nil {
		t.Fatal(err)
	}
	sendDone := drain(t, sender)
	room, err := sender.Host(ctx, []string{srcPath})
	if err != nil {
		t.Fatal(err)
	}
	recvDone := drain(t, receiver)
	go receiver.Fetch(ctx, room, t.TempDir())
	if d := <-recvDone; d.Err != nil {
		t.Fatalf("the transfer failed: %v", d.Err)
	}
	if d := <-sendDone; d.Err != nil {
		t.Fatalf("the send failed: %v", d.Err)
	}

	// The sender unregisters when it is done; wait until the server has
	// heard that too, so every request is in the record.
	deadline := time.Now().Add(10 * time.Second)
	for registry.ActiveRooms() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	record := seen.String()
	mu.Unlock()
	nameplate := rendezvous.Nameplate(room)
	if !strings.Contains(record, `"`+nameplate+`"`) {
		t.Fatalf("the spy saw no rendezvous traffic for %s: %q", nameplate, record)
	}
	// The forms the secret would travel in. A bare word is not searched
	// for: "nar" or "fil" turn up inside a base58 peer ID by chance.
	secret := strings.TrimSuffix(room, "-"+nameplate)
	leaks := []string{room, secret}
	for _, word := range strings.Split(secret, "-") {
		leaks = append(leaks, `"`+word+`"`, word+"-", "-"+word)
	}
	for _, leak := range leaks {
		if strings.Contains(record, leak) {
			t.Errorf("the server was sent %q, part of the secret of %q:\n%s", leak, room, record)
		}
	}
}
