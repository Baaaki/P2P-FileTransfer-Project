package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"puresend/internal/p2p"
	"puresend/internal/rendezvous"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

// newWSServer starts a rendezvous server configured the way it runs in
// production behind Cloudflare Tunnel: a plain WebSocket listener (TLS is
// terminated by the tunnel), relay enabled with the registry as its ACL.
func newWSServer(t *testing.T) (host.Host, *rendezvous.Registry, string) {
	t.Helper()
	return startWSServer(t, nil, "/ip4/127.0.0.1/tcp/0/ws")
}

// startWSServer is newWSServer with a chosen identity and listen address,
// for the tests that restart "the same" server.
func startWSServer(t *testing.T, priv crypto.PrivKey, listen string) (host.Host, *rendezvous.Registry, string) {
	t.Helper()

	registry := rendezvous.NewRegistry()
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(listen),
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableRelayService(
			relay.WithResources(relay.DefaultResources()),
			relay.WithACL(registry),
		),
	}
	if priv != nil {
		opts = append(opts, libp2p.Identity(priv))
	}
	var (
		h   host.Host
		err error
	)
	for i := 0; i < 20; i++ {
		h, err = libp2p.New(opts...)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "address already in use") {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	registry.Serve(h)

	addr := fmt.Sprintf("%s/p2p/%s", h.Addrs()[0], h.ID())
	return h, registry, addr
}

// TestWebSocketRendezvous runs the whole product flow over the WebSocket
// transport: two clients reach the server over /ws, the sender claims a
// room and reserves a relay slot, and the receiver pulls the files.
func TestWebSocketRendezvous(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, _, serverAddr := newWSServer(t)

	sender, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatalf("sender could not reach the ws server: %v", err)
	}
	defer sender.Close()

	receiver, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatalf("receiver could not reach the ws server: %v", err)
	}
	defer receiver.Close()

	// A file big enough that progress events actually fire.
	srcDir := t.TempDir()
	want := bytes.Repeat([]byte("puresend"), 40_000)
	srcPath := filepath.Join(srcDir, "tatil.jpg")
	if err := os.WriteFile(srcPath, want, 0o644); err != nil {
		t.Fatal(err)
	}

	// Drain the sender's events; the transfer blocks on delivery.
	sendDone := make(chan error, 1)
	go func() {
		for ev := range sender.Events() {
			if d, ok := ev.(p2p.DoneEvent); ok {
				sendDone <- d.Err
				return
			}
		}
	}()

	room, err := sender.Host(ctx, []string{srcPath})
	if err != nil {
		t.Fatalf("host room (relay reservation over ws): %v", err)
	}
	if room == "" {
		t.Fatal("empty room code")
	}

	// Drain the receiver's events, approving the manifest when asked.
	outDir := t.TempDir()
	recvDone := make(chan error, 1)
	sawManifest := make(chan struct{}, 1)
	sawProgress := make(chan struct{}, 1)
	go func() {
		for ev := range receiver.Events() {
			switch e := ev.(type) {
			case p2p.ManifestEvent:
				select {
				case sawManifest <- struct{}{}:
				default:
				}
				e.Reply <- true
			case p2p.ProgressEvent:
				select {
				case sawProgress <- struct{}{}:
				default:
				}
			case p2p.DoneEvent:
				recvDone <- e.Err
				return
			}
		}
	}()

	go receiver.Fetch(ctx, room, outDir)

	if err := <-recvDone; err != nil {
		t.Fatalf("receive over ws: %v", err)
	}
	if err := <-sendDone; err != nil {
		t.Fatalf("send over ws: %v", err)
	}

	select {
	case <-sawManifest:
	default:
		t.Error("receiver was never asked to approve the manifest")
	}
	select {
	case <-sawProgress:
	default:
		t.Error("no progress was ever reported")
	}

	got, err := os.ReadFile(filepath.Join(outDir, "tatil.jpg"))
	if err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("received content differs from the original")
	}
}

// TestWebSocketWrongCode checks the error a user gets after a typo — the
// TUI turns this into "no room was opened with this code".
func TestWebSocketWrongCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _, serverAddr := newWSServer(t)

	receiver, err := p2p.New(ctx, []string{serverAddr})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	done := make(chan error, 1)
	go func() {
		for ev := range receiver.Events() {
			if d, ok := ev.(p2p.DoneEvent); ok {
				done <- d.Err
				return
			}
		}
	}()

	go receiver.Fetch(ctx, "kiraz-liman-99", t.TempDir())

	err = <-done
	if err == nil {
		t.Fatal("expected an error for an unknown room code")
	}
}
