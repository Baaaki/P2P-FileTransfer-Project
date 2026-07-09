// Package rendezvous implements a simple "room" protocol that lets two
// peers find each other through a shared server.
//
// The flow:
//  1. The sender connects to the server and registers its addresses
//     under a room code.
//  2. The receiver asks the server for the same room code and gets the
//     sender's addresses back.
//  3. From that point on the server is out of the picture; the receiver
//     connects to the sender directly.
//
// The protocol exchanges single-line JSON messages: the client sends one
// Request, the server replies with one Response, and the stream closes.
package rendezvous

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// ProtocolID identifies the rendezvous protocol on libp2p.
const ProtocolID = "/puresend/rendezvous/1.0.0"

// Room registrations expire after this duration.
const roomTTL = 1 * time.Hour

// Maximum number of rooms the server keeps at once (a simple safeguard).
const maxRooms = 1000

// Request is the message sent from client to server.
type Request struct {
	Type  string   `json:"type"`            // "register" or "lookup"
	Room  string   `json:"room"`            // room code, e.g. "cherry-harbor-42"
	Addrs []string `json:"addrs,omitempty"` // register: the sender's multiaddrs
}

// Response is the server's reply.
type Response struct {
	Type   string   `json:"type"` // "ok", "found", "not_found" or "error"
	PeerID string   `json:"peer_id,omitempty"`
	Addrs  []string `json:"addrs,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Client side
// ---------------------------------------------------------------------------

// Register stores the given room code and addresses on the server.
// The sending side calls this so the receiver can find it.
func Register(ctx context.Context, h host.Host, server peer.ID, room string, addrs []multiaddr.Multiaddr) error {
	addrStrs := make([]string, len(addrs))
	for i, a := range addrs {
		addrStrs[i] = a.String()
	}

	resp, err := roundTrip(ctx, h, server, Request{Type: "register", Room: room, Addrs: addrStrs})
	if err != nil {
		return err
	}
	if resp.Type != "ok" {
		return fmt.Errorf("server rejected registration: %s", resp.Error)
	}
	return nil
}

// Lookup asks the server for a room code and returns the registered
// peer's connection info. The receiving side calls this.
func Lookup(ctx context.Context, h host.Host, server peer.ID, room string) (*peer.AddrInfo, error) {
	resp, err := roundTrip(ctx, h, server, Request{Type: "lookup", Room: room})
	if err != nil {
		return nil, err
	}
	switch resp.Type {
	case "found":
		id, err := peer.Decode(resp.PeerID)
		if err != nil {
			return nil, fmt.Errorf("invalid peer ID from server: %w", err)
		}
		info := &peer.AddrInfo{ID: id}
		for _, s := range resp.Addrs {
			a, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				continue // skip a malformed address; the rest may suffice
			}
			info.Addrs = append(info.Addrs, a)
		}
		if len(info.Addrs) == 0 {
			return nil, fmt.Errorf("no valid addresses registered in the room")
		}
		return info, nil
	case "not_found":
		return nil, fmt.Errorf("room %q not found — the code may be wrong or expired", room)
	default:
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}
}

// roundTrip sends one request to the server and reads one response.
func roundTrip(ctx context.Context, h host.Host, server peer.ID, req Request) (*Response, error) {
	s, err := h.NewStream(ctx, server, ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("could not open stream to server: %w", err)
	}
	defer s.Close()
	s.SetDeadline(time.Now().Add(30 * time.Second))

	if err := json.NewEncoder(s).Encode(req); err != nil {
		return nil, fmt.Errorf("could not send request: %w", err)
	}
	var resp Response
	if err := json.NewDecoder(bufio.NewReader(s)).Decode(&resp); err != nil {
		return nil, fmt.Errorf("could not read server response: %w", err)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Server side
// ---------------------------------------------------------------------------

type roomEntry struct {
	info      peer.AddrInfo
	createdAt time.Time
}

// Registry is a simple in-memory ledger of active rooms.
type Registry struct {
	mu    sync.Mutex
	rooms map[string]roomEntry
}

func NewRegistry() *Registry {
	return &Registry{rooms: make(map[string]roomEntry)}
}

// Handler is registered as the server's stream handler.
func (r *Registry) Handler(s network.Stream) {
	defer s.Close()
	s.SetDeadline(time.Now().Add(30 * time.Second))

	var req Request
	if err := json.NewDecoder(bufio.NewReader(s)).Decode(&req); err != nil {
		return
	}

	resp := r.handle(s.Conn().RemotePeer(), req)
	json.NewEncoder(s).Encode(resp)
}

func (r *Registry) handle(from peer.ID, req Request) Response {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropExpired()

	switch req.Type {
	case "register":
		if req.Room == "" || len(req.Addrs) == 0 {
			return Response{Type: "error", Error: "room code and address list are required"}
		}
		// A room may only be re-registered by the peer that owns it.
		// Otherwise anyone who learns the code could take the room
		// over and serve their own files to the receiver.
		if existing, ok := r.rooms[req.Room]; ok && existing.info.ID != from {
			return Response{Type: "error", Error: "room code is already in use"}
		}
		if len(r.rooms) >= maxRooms {
			return Response{Type: "error", Error: "server is full, try again later"}
		}
		info := peer.AddrInfo{ID: from}
		for _, s := range req.Addrs {
			if a, err := multiaddr.NewMultiaddr(s); err == nil {
				info.Addrs = append(info.Addrs, a)
			}
		}
		r.rooms[req.Room] = roomEntry{info: info, createdAt: time.Now()}
		return Response{Type: "ok"}

	case "lookup":
		entry, ok := r.rooms[req.Room]
		if !ok {
			return Response{Type: "not_found"}
		}
		addrs := make([]string, len(entry.info.Addrs))
		for i, a := range entry.info.Addrs {
			addrs[i] = a.String()
		}
		return Response{Type: "found", PeerID: entry.info.ID.String(), Addrs: addrs}

	default:
		return Response{Type: "error", Error: "unknown request type: " + req.Type}
	}
}

// dropExpired removes rooms past their TTL. Called with the lock held.
func (r *Registry) dropExpired() {
	now := time.Now()
	for room, e := range r.rooms {
		if now.Sub(e.createdAt) > roomTTL {
			delete(r.rooms, room)
		}
	}
}

// ---------------------------------------------------------------------------
// Room code generation
// ---------------------------------------------------------------------------

// Short, common words that are easy to say over the phone and easy to
// type on any keyboard.
var words = []string{
	"apple", "pear", "cherry", "melon", "olive", "almond",
	"mint", "pepper", "lemon", "walnut", "river", "harbor",
	"forest", "cloud", "drop", "meadow", "leaf", "cedar",
	"pencil", "book", "lamp", "ferry", "balloon", "violin",
}

// NewRoomCode returns an easy-to-read room code like "cherry-harbor-42".
func NewRoomCode() string {
	return fmt.Sprintf("%s-%s-%d", pickWord(), pickWord(), pickNumber())
}

func pickWord() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	return words[n.Int64()]
}

func pickNumber() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(90))
	return n.Int64() + 10 // range 10-99
}
