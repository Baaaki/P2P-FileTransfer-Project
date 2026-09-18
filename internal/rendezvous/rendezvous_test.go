package rendezvous

import (
	"fmt"
	"strings"
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

const room = "kiraz-liman-42"

func register(r *Registry, from peer.ID, room string) Response {
	return r.handle(from, Request{
		Type:  "register",
		Room:  room,
		Addrs: []string{"/ip4/127.0.0.1/tcp/4001"},
	})
}

func lookup(r *Registry, from peer.ID, room string) Response {
	return r.handle(from, Request{Type: "lookup", Room: room})
}

// code returns the i-th of an endless supply of distinct, well-formed
// codes, for tests that need more rooms than the word list suggests.
func code(i int) string {
	return fmt.Sprintf("oda%c-test%c-%02d", 'a'+rune(i/100%26), 'a'+rune(i/2600%26), i%100)
}

func TestRegisterAndLookup(t *testing.T) {
	r := NewRegistry()

	if resp := register(r, sender, room); resp.Type != "ok" {
		t.Fatalf("register failed: %+v", resp)
	}

	resp := lookup(r, receiver, room)
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
	if resp := lookup(r, receiver, "elma-deniz-11"); resp.Type != "not_found" {
		t.Fatalf("got %+v, want not_found", resp)
	}
}

func TestRegisterValidation(t *testing.T) {
	r := NewRegistry()
	if resp := r.handle(sender, Request{Type: "register", Room: room}); resp.Type != "error" {
		t.Errorf("register without addresses accepted: %+v", resp)
	}
	if resp := r.handle(sender, Request{Type: "bogus"}); resp.Type != "error" {
		t.Errorf("unknown request type accepted: %+v", resp)
	}

	tooMany := make([]string, MaxAddrs+1)
	for i := range tooMany {
		tooMany[i] = "/ip4/127.0.0.1/tcp/4001"
	}
	if resp := r.handle(sender, Request{Type: "register", Room: room, Addrs: tooMany}); resp.Type != "error" {
		t.Errorf("register with too many addresses accepted: %+v", resp)
	}
	if resp := r.handle(sender, Request{Type: "register", Room: room, Addrs: []string{"garbage"}}); resp.Type != "error" {
		t.Errorf("register without a single valid address accepted: %+v", resp)
	}
}

// TestRegisterRejectsMalformedCodes: a room name used to be up to 16 KB of
// anything at all. Now it has exactly the shape a real code has.
func TestRegisterRejectsMalformedCodes(t *testing.T) {
	for _, bad := range []string{
		"",
		"a-b-1",
		"kiraz-liman-4",
		"kiraz-liman-420",
		"Kiraz-liman-42",
		"kiraz liman 42",
		"kiraz-liman-42\n",
		"kıraz-liman-42",
		"kiraz-liman-kule-42",
		strings.Repeat("a", 30) + "-" + strings.Repeat("b", 30) + "-42",
		strings.Repeat("x", 16<<10),
	} {
		r := NewRegistry()
		if resp := register(r, sender, bad); resp.Type != "error" {
			t.Errorf("register accepted the code %q", bad)
		}
		if r.ActiveRooms() != 0 {
			t.Errorf("a room was created for %q", bad)
		}
	}
}

func TestRoomTakeoverRejected(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)

	if resp := register(r, peer.ID("attacker"), room); resp.Type != "error" {
		t.Fatalf("takeover by another peer accepted: %+v", resp)
	}
	// The owner itself may refresh its own room.
	if resp := register(r, sender, room); resp.Type != "ok" {
		t.Fatalf("owner re-register rejected: %+v", resp)
	}
}

func TestRoomExpiry(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)

	// Age the entry past its TTL by editing it directly.
	e := r.rooms[room]
	e.createdAt = time.Now().Add(-RoomTTL - time.Minute)
	r.rooms[room] = e

	if resp := lookup(r, receiver, room); resp.Type != "not_found" {
		t.Fatalf("expired room still found: %+v", resp)
	}
	if got := r.Stats().Expired; got != 1 {
		t.Errorf("Expired = %d, want 1", got)
	}
}

// TestRefreshKeepsTheClock: re-registering — which the client does after
// every reconnect — must not stretch a code past its hour.
func TestRefreshKeepsTheClock(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)
	e := r.rooms[room]
	e.createdAt = time.Now().Add(-RoomTTL + time.Second)
	r.rooms[room] = e

	if resp := register(r, sender, room); resp.Type != "ok" {
		t.Fatalf("refresh failed: %+v", resp)
	}
	time.Sleep(1100 * time.Millisecond)
	if resp := lookup(r, receiver, room); resp.Type != "not_found" {
		t.Fatalf("a refresh extended the room past its hour: %+v", resp)
	}
}

func TestServerFull(t *testing.T) {
	r := NewRegistry(WithMaxRoomsPerPeer(MaxRooms), WithRegisterBudget(MaxRooms+10))
	for i := range MaxRooms {
		if resp := register(r, sender, code(i)); resp.Type != "ok" {
			t.Fatalf("register %d failed: %+v", i, resp)
		}
	}
	if resp := register(r, sender, code(MaxRooms)); resp.Type != "error" {
		t.Fatalf("register beyond MaxRooms accepted: %+v", resp)
	}
}

// TestRoomsPerPeerLimit checks that one sender cannot claim the whole
// table and leave every other user with "server is full".
func TestRoomsPerPeerLimit(t *testing.T) {
	r := NewRegistry(WithMaxRoomsPerPeer(2))
	for i := range 2 {
		if resp := register(r, sender, code(i)); resp.Type != "ok" {
			t.Fatalf("register %d failed: %+v", i, resp)
		}
	}
	if resp := register(r, sender, code(2)); resp.Type != "error" {
		t.Fatalf("register beyond the per-peer limit accepted: %+v", resp)
	}
	// Another sender is unaffected, and so is refreshing a room already held.
	if resp := register(r, receiver, code(3)); resp.Type != "ok" {
		t.Errorf("the limit leaked to another peer: %+v", resp)
	}
	if resp := register(r, sender, code(0)); resp.Type != "ok" {
		t.Errorf("refreshing an owned room was refused: %+v", resp)
	}
	// Closing one frees the slot again.
	if resp := r.handle(sender, Request{Type: "unregister", Room: code(0)}); resp.Type != "ok" {
		t.Fatalf("unregister failed: %+v", resp)
	}
	if resp := register(r, sender, code(4)); resp.Type != "ok" {
		t.Errorf("a freed slot was not reusable: %+v", resp)
	}
}

// TestOneRoomPerPeerByDefault: the program opens one room per session, so
// that is all a peer gets unless the operator says otherwise.
func TestOneRoomPerPeerByDefault(t *testing.T) {
	r := NewRegistry()
	register(r, sender, code(0))
	if resp := register(r, sender, code(1)); resp.Type != "error" {
		t.Errorf("a second room for the same peer was accepted: %+v", resp)
	}
}

// TestRegisterBudget covers the other half of keeping the table from being
// filled: identities are free, so the number of new rooms per minute is
// bounded server-wide. A refresh is not a new room and is never refused.
func TestRegisterBudget(t *testing.T) {
	r := NewRegistry(WithRegisterBudget(3))
	for i := range 3 {
		if resp := register(r, peer.ID(fmt.Sprintf("p%d", i)), code(i)); resp.Type != "ok" {
			t.Fatalf("register %d failed: %+v", i, resp)
		}
	}
	resp := register(r, peer.ID("one-too-many"), code(3))
	if resp.Type != "error" || !strings.Contains(resp.Error, "busy") {
		t.Fatalf("register beyond the budget got %+v", resp)
	}
	if resp := register(r, peer.ID("p0"), code(0)); resp.Type != "ok" {
		t.Errorf("a refresh was charged to the budget: %+v", resp)
	}
	if got := r.Stats().RegisterThrottled; got != 1 {
		t.Errorf("RegisterThrottled = %d, want 1", got)
	}

	r.newRooms.start = time.Now().Add(-2 * time.Minute)
	if resp := register(r, peer.ID("next-minute"), code(3)); resp.Type != "ok" {
		t.Errorf("the budget did not refill: %+v", resp)
	}
}

// TestOwnerGone: a room is only reachable while its owner is connected to
// the server. One whose owner has vanished is kept for a short grace
// period — long enough to reconnect — and then dropped, instead of taking
// a slot for the rest of its hour.
func TestOwnerGone(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)

	r.ownerGone(sender)
	resp := lookup(r, receiver, room)
	if resp.Type != "error" || !strings.Contains(resp.Error, "reconnecting") {
		t.Fatalf("lookup while the owner is away got %+v", resp)
	}

	r.ownerBack(sender)
	if resp := lookup(r, receiver, room); resp.Type != "found" {
		t.Fatalf("the room did not come back with its owner: %+v", resp)
	}

	r.ownerGone(sender)
	e := r.rooms[room]
	e.ownerGone = time.Now().Add(-ownerGrace - time.Second)
	r.rooms[room] = e
	if got := r.ActiveRooms(); got != 0 {
		t.Fatalf("an abandoned room outlived its grace period: %d rooms", got)
	}
	if got := r.Stats().Abandoned; got != 1 {
		t.Errorf("Abandoned = %d, want 1", got)
	}
	// And the owner, back later, can simply register it again.
	if resp := register(r, sender, room); resp.Type != "ok" {
		t.Errorf("re-registering after a long absence failed: %+v", resp)
	}
}

// TestUnregister checks that a finished sender can retire its code
// immediately, and that nobody else can retire it for them.
func TestUnregister(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)

	if resp := r.handle(peer.ID("attacker"), Request{Type: "unregister", Room: room}); resp.Type != "ok" {
		t.Fatalf("unregister by a stranger returned %+v, want a plain ok", resp)
	}
	if resp := lookup(r, receiver, room); resp.Type != "found" {
		t.Fatal("a stranger managed to close someone else's room")
	}

	if resp := r.handle(sender, Request{Type: "unregister", Room: room}); resp.Type != "ok" {
		t.Fatalf("owner unregister failed: %+v", resp)
	}
	if resp := lookup(r, receiver, room); resp.Type != "not_found" {
		t.Fatalf("the room survived its owner closing it: %+v", resp)
	}
	if got := r.ActiveRooms(); got != 0 {
		t.Errorf("ActiveRooms is %d after the only room closed, want 0", got)
	}
}

// TestLookupReportsRelayLimit checks that a receiver is told how much the
// server will relay, which is what lets it warn before a transfer that
// cannot fit through the fallback route.
func TestLookupReportsRelayLimit(t *testing.T) {
	r := NewRegistry(WithRelayLimit(256 << 20))
	register(r, sender, room)

	resp := lookup(r, receiver, room)
	if resp.RelayLimit != 256<<20 {
		t.Errorf("lookup reported a relay limit of %d, want %d", resp.RelayLimit, 256<<20)
	}
}

// TestStats checks the counters the health endpoint and the metrics
// exporter are built on.
func TestStats(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)
	lookup(r, receiver, room)
	lookup(r, receiver, "elma-deniz-11")
	r.handle(sender, Request{Type: "unregister", Room: room})

	got := r.Stats()
	want := Stats{Registered: 1, Unregistered: 1, LookupsFound: 1, LookupsNotFound: 1}
	if got != want {
		t.Errorf("Stats() = %+v, want %+v", got, want)
	}
}

func TestRelayACL(t *testing.T) {
	r := NewRegistry()

	if r.AllowReserve(sender, nil) {
		t.Error("reservation allowed for a peer without a room")
	}
	register(r, sender, room)
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
	register(r, sender, room)

	for range maxFailedLookups {
		if resp := lookup(r, receiver, "yanlis-tahmin-11"); resp.Type != "not_found" {
			t.Fatalf("miss not reported: %+v", resp)
		}
	}
	// Once over the limit, even a valid code is rejected for this peer —
	// otherwise the refusal would tell a guesser which guesses hit.
	if resp := lookup(r, receiver, room); resp.Type != "error" {
		t.Fatalf("rate limit not applied: %+v", resp)
	}
	// Other peers are unaffected.
	if resp := lookup(r, peer.ID("other"), room); resp.Type != "found" {
		t.Fatalf("rate limit leaked to another peer: %+v", resp)
	}
	// The block lifts once the window expires.
	r.fails[receiver] = window{count: maxFailedLookups, start: time.Now().Add(-2 * lookupFailWindow)}
	if resp := lookup(r, receiver, room); resp.Type != "found" {
		t.Fatalf("rate limit did not expire: %+v", resp)
	}
}

// TestMalformedLookupIsNotAMiss: a code that cannot exist teaches nothing,
// and a typo in its shape should not use up one of a user's tries.
func TestMalformedLookupIsNotAMiss(t *testing.T) {
	r := NewRegistry()
	for range maxFailedLookups + 2 {
		lookup(r, receiver, "not a code")
	}
	register(r, sender, room)
	if resp := lookup(r, receiver, room); resp.Type != "found" {
		t.Fatalf("malformed lookups counted against the peer: %+v", resp)
	}
}

// fillGlobalBudget spends the server-wide miss budget with fresh
// identities — the attacker who reconnects between guesses.
func fillGlobalBudget(t *testing.T, r *Registry) {
	t.Helper()
	for i := range maxGlobalFailedLookups {
		guesser := peer.ID(fmt.Sprintf("fresh-identity-%d", i))
		if resp := lookup(r, guesser, "yanlis-tahmin-11"); resp.Type != "not_found" {
			t.Fatalf("miss %d not reported: %+v", i, resp)
		}
	}
}

// TestGlobalLimitIsNotAKillSwitch is the finding that reshaped this
// limit. It used to refuse every lookup once the budget was spent, so a
// few random guesses a second closed the door on every real user. Someone
// typing the code they were given must still get through.
func TestGlobalLimitIsNotAKillSwitch(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)
	fillGlobalBudget(t, r)

	if resp := lookup(r, peer.ID("the-real-friend"), room); resp.Type != "found" {
		t.Fatalf("under a guessing flood, a first-try lookup was refused: %+v", resp)
	}
}

// TestGlobalLimitTakesAwaySecondChances covers what the limit still does
// under pressure: a peer gets one guess, answered honestly, and nothing
// after it — hit or miss, so the refusal gives nothing away.
func TestGlobalLimitTakesAwaySecondChances(t *testing.T) {
	r := NewRegistry()
	register(r, sender, room)
	fillGlobalBudget(t, r)

	guesser := peer.ID("guesser")
	if resp := lookup(r, guesser, "baska-tahmin-22"); resp.Type != "not_found" {
		t.Fatalf("a first guess under pressure got %+v, want an honest not_found", resp)
	}
	if resp := lookup(r, guesser, room); resp.Type != "error" {
		t.Fatalf("a second guess under pressure was answered: %+v", resp)
	}
	if resp := lookup(r, guesser, "ucuncu-tahmin-33"); resp.Type != "error" {
		t.Fatalf("a third guess under pressure was answered: %+v", resp)
	}

	// The pressure lifts with the window.
	r.globalFail = window{count: maxGlobalFailedLookups, start: time.Now().Add(-2 * globalFailWindow)}
	r.fails[guesser] = window{count: 1, start: time.Now()}
	if resp := lookup(r, guesser, room); resp.Type != "found" {
		t.Fatalf("the server-wide limit did not expire: %+v", resp)
	}
}
