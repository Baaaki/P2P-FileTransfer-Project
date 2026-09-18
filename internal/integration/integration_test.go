// Package integration spins up real libp2p hosts on localhost and runs
// the full flow — register, lookup, connect, transfer — end to end.
package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filetransferilla/internal/rendezvous"
	"filetransferilla/internal/transfer"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

func newHost(t *testing.T) host.Host {
	t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

func TestEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The three parties, all in-process on localhost.
	server := newHost(t)
	registry := rendezvous.NewRegistry()
	registry.Serve(server)

	sender := newHost(t)
	receiver := newHost(t)
	serverInfo := host.InfoFromHost(server)
	if err := sender.Connect(ctx, *serverInfo); err != nil {
		t.Fatalf("sender connect to server: %v", err)
	}
	if err := receiver.Connect(ctx, *serverInfo); err != nil {
		t.Fatalf("receiver connect to server: %v", err)
	}

	// Sender side: files, room registration, transfer handler.
	srcDir := t.TempDir()
	want := map[string][]byte{
		"data.bin": bytes.Repeat([]byte{7}, 300_000),
		"note.txt": []byte("istanbul -> izmir"),
	}
	var paths []string
	for name, content := range want {
		p := filepath.Join(srcDir, name)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	room := rendezvous.NewRoomCode()
	if err := rendezvous.Register(ctx, sender, server.ID(), room, sender.Addrs()); err != nil {
		t.Fatalf("register: %v", err)
	}
	sendErr := make(chan error, 1)
	sender.SetStreamHandler(transfer.ProtocolID, func(s network.Stream) {
		sendErr <- transfer.SendPaths(s, paths, transfer.Credentials{
			Code:     room,
			Sender:   sender.ID().String(),
			Receiver: s.Conn().RemotePeer().String(),
		}, transfer.Hooks{})
	})

	// Receiver side: look up the room, connect, approve, receive.
	info, _, err := rendezvous.Lookup(ctx, receiver, server.ID(), room)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if info.ID != sender.ID() {
		t.Fatalf("lookup returned peer %s, want %s", info.ID, sender.ID())
	}
	if err := receiver.Connect(ctx, *info); err != nil {
		t.Fatalf("connect to sender: %v", err)
	}
	s, err := receiver.NewStream(ctx, info.ID, transfer.ProtocolID)
	if err != nil {
		t.Fatalf("open transfer stream: %v", err)
	}

	outDir := t.TempDir()
	confirmed := false
	confirm := func(m transfer.Manifest) bool {
		confirmed = true
		return len(m.Files) == len(want)
	}
	saved, err := transfer.Receive(s, outDir, transfer.Credentials{
		Code:     room,
		Sender:   info.ID.String(),
		Receiver: receiver.ID().String(),
	}, confirm, transfer.Hooks{})
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("send: %v", err)
	}

	if !confirmed {
		t.Error("confirm callback was never called")
	}
	if len(saved) != len(want) {
		t.Fatalf("received %d files, want %d", len(saved), len(want))
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
