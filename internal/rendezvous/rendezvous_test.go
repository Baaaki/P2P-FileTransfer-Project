package rendezvous

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

// The Registry doubles as the relay service's access control filter.
var _ relay.ACLFilter = (*Registry)(nil)

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

	tooMany := make([]string, MaxAddrs+1)
	for i := range tooMany {
		tooMany[i] = "/ip4/127.0.0.1/tcp/4001"
	}
	if resp := r.handle(sender, Request{Type: "register", Room: "a-b-1", Addrs: tooMany}); resp.Type != "error" {
		t.Errorf("register with too many addresses accepted: %+v", resp)
	}
	if resp := r.handle(sender, Request{Type: "register", Room: "a-b-1", Addrs: []string{"garbage"}}); resp.Type != "error" {
		t.Errorf("register without a single valid address accepted: %+v", resp)
	}
}

func TestRoomTakeoverRejected(t *testing.T) {
	r := NewRegistry()
	register(r, sender, "apple-river-42")

	if resp := register(r, peer.ID("attacker"), "apple-river-42"); resp.Type != "error" {
		t.Fatalf("takeover by another peer accepted: %+v", resp)
	}
	// The owner itself may refresh its own room.
	if resp := register(r, sender, "apple-river-42"); resp.Type != "ok" {
		t.Fatalf("owner re-register rejected: %+v", resp)
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

func TestWordList(t *testing.T) {
	// Fewer words would shrink the code space and make guessing easier.
	if len(words) < 128 {
		t.Errorf("word list has %d entries, want at least 128", len(words))
	}
	seen := make(map[string]bool, len(words))
	for _, w := range words {
		if seen[w] {
			t.Errorf("duplicate word in the code list: %q", w)
		}
		seen[w] = true
	}
}

func TestRelayACL(t *testing.T) {
	r := NewRegistry()

	if r.AllowReserve(sender, nil) {
		t.Error("reservation allowed for a peer without a room")
	}
	register(r, sender, "apple-river-42")
	if !r.AllowReserve(sender, nil) {
		t.Error("reservation refused for a room owner")
	}
	if !r.AllowConnect(receiver, nil, sender) {
		t.Error("relayed connection towards a room owner refused")
	}
	if r.AllowConnect(sender, nil, receiver) {
		t.Error("relayed connection allowed towards a peer without a room")
	}
}

func TestLookupRateLimit(t *testing.T) {
	r := NewRegistry()
	register(r, sender, "apple-river-42")

	for range maxFailedLookups {
		if resp := r.handle(receiver, Request{Type: "lookup", Room: "wrong-guess"}); resp.Type != "not_found" {
			t.Fatalf("miss not reported: %+v", resp)
		}
	}
	// Once over the limit, even a valid code is rejected for this peer.
	if resp := r.handle(receiver, Request{Type: "lookup", Room: "apple-river-42"}); resp.Type != "error" {
		t.Fatalf("rate limit not applied: %+v", resp)
	}
	// Other peers are unaffected.
	if resp := r.handle(peer.ID("other"), Request{Type: "lookup", Room: "apple-river-42"}); resp.Type != "found" {
		t.Fatalf("rate limit leaked to another peer: %+v", resp)
	}
	// The block lifts once the window expires.
	r.fails[receiver] = failCounter{count: maxFailedLookups, windowStart: time.Now().Add(-2 * lookupFailWindow)}
	if resp := r.handle(receiver, Request{Type: "lookup", Room: "apple-river-42"}); resp.Type != "found" {
		t.Fatalf("rate limit did not expire: %+v", resp)
	}
}
