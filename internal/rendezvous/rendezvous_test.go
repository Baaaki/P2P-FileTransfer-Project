package rendezvous

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	sender   = peer.ID("sender")
	receiver = peer.ID("receiver")
)

func register(r *Registry, from peer.ID, room string) Response {
	return r.handle(from, Request{
		Type:  "register",
		Room:  room,
		Addrs: []string{"/ip4/127.0.0.1/tcp/4001"},
	})
}

func TestRegisterAndLookup(t *testing.T) {
	r := NewRegistry()

	if resp := register(r, sender, "apple-river-42"); resp.Type != "ok" {
		t.Fatalf("register failed: %+v", resp)
	}

	resp := r.handle(receiver, Request{Type: "lookup", Room: "apple-river-42"})
	if resp.Type != "found" {
		t.Fatalf("lookup failed: %+v", resp)
	}
	if resp.PeerID != sender.String() {
		t.Errorf("lookup returned peer %q, want %q", resp.PeerID, sender.String())
	}
	if len(resp.Addrs) != 1 {
		t.Errorf("lookup returned %d addrs, want 1", len(resp.Addrs))
	}
}

func TestLookupUnknownRoom(t *testing.T) {
	r := NewRegistry()
	if resp := r.handle(receiver, Request{Type: "lookup", Room: "no-such-room"}); resp.Type != "not_found" {
		t.Fatalf("got %+v, want not_found", resp)
	}
}

func TestRegisterValidation(t *testing.T) {
	r := NewRegistry()
	if resp := r.handle(sender, Request{Type: "register", Room: ""}); resp.Type != "error" {
		t.Errorf("empty room accepted: %+v", resp)
	}
	if resp := r.handle(sender, Request{Type: "register", Room: "a-b-1"}); resp.Type != "error" {
		t.Errorf("register without addresses accepted: %+v", resp)
	}
	if resp := r.handle(sender, Request{Type: "bogus"}); resp.Type != "error" {
		t.Errorf("unknown request type accepted: %+v", resp)
	}
}

func TestRoomExpiry(t *testing.T) {
	r := NewRegistry()
	register(r, sender, "old-room-10")

	// Age the entry past its TTL by editing it directly.
	r.rooms["old-room-10"] = roomEntry{
		info:      r.rooms["old-room-10"].info,
		createdAt: time.Now().Add(-roomTTL - time.Minute),
	}

	if resp := r.handle(receiver, Request{Type: "lookup", Room: "old-room-10"}); resp.Type != "not_found" {
		t.Fatalf("expired room still found: %+v", resp)
	}
}

func TestServerFull(t *testing.T) {
	r := NewRegistry()
	for i := range maxRooms {
		if resp := register(r, sender, fmt.Sprintf("room-%d", i)); resp.Type != "ok" {
			t.Fatalf("register %d failed: %+v", i, resp)
		}
	}
	if resp := register(r, sender, "one-too-many"); resp.Type != "error" {
		t.Fatalf("register beyond maxRooms accepted: %+v", resp)
	}
}

func TestNewRoomCodeFormat(t *testing.T) {
	format := regexp.MustCompile(`^[a-z]+-[a-z]+-[1-9][0-9]$`)
	for range 100 {
		if code := NewRoomCode(); !format.MatchString(code) {
			t.Fatalf("unexpected room code format: %q", code)
		}
	}
}

func TestWordListHasNoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(words))
	for _, w := range words {
		if seen[w] {
			t.Errorf("duplicate word in the code list: %q", w)
		}
		seen[w] = true
	}
}
