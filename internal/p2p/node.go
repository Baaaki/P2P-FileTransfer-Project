// Package p2p wraps the libp2p plumbing behind an API the user interface
// can drive: every long-running step reports progress as an Event instead
// of printing to the screen, so the same logic works under a TUI, a CLI
// or a test.
package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"filetransferilla/internal/rendezvous"
	"filetransferilla/internal/transfer"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"
)

// How long to wait for DCUtR to upgrade a relayed connection to a direct
// one before giving up and transferring through the relay.
const directWait = 20 * time.Second

// Progress events are throttled to this interval: a 1 GB file would
// otherwise produce tens of thousands of screen updates.
const progressInterval = 80 * time.Millisecond

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

// Event is something worth telling the user about. The concrete types
// below are the full set.
type Event interface{ isEvent() }

// StatusEvent is a plain-language description of the current step.
type StatusEvent struct{ Text string }

// ConnectedEvent fires once the two peers can talk. Direct is false when
// the connection still runs through the server's relay.
type ConnectedEvent struct{ Direct bool }

// ManifestEvent asks the user to approve an incoming file list. The
// transfer blocks until exactly one value is sent on Reply.
type ManifestEvent struct {
	Manifest transfer.Manifest
	Reply    chan<- bool
}

// ProgressEvent reports transfer progress for a single file.
type ProgressEvent struct {
	Name        string
	Done, Total int64
}

// DoneEvent is the final event of a transfer. Err is nil on success;
// Paths is populated on the receiving side.
type DoneEvent struct {
	Paths []string
	Err   error
}

func (StatusEvent) isEvent()    {}
func (ConnectedEvent) isEvent() {}
func (ManifestEvent) isEvent()  {}
func (ProgressEvent) isEvent()  {}
func (DoneEvent) isEvent()      {}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

// Node is a running libp2p host with an open connection to the
// rendezvous server.
type Node struct {
	host   host.Host
	server peer.AddrInfo
	events chan Event

	closeOnce sync.Once
	done      chan struct{}
}

// New starts a libp2p host and connects it to the rendezvous server at
// the given multiaddr (e.g. /dns4/host/tcp/443/tls/ws/p2p/12D3Koo...).
func New(ctx context.Context, serverAddr string) (*Node, error) {
	info, err := peer.AddrInfoFromString(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("meeting point address is not valid: %w", err)
	}

	h, err := libp2p.New(
		// DCUtR (hole punching): upgrades a relayed connection to a
		// direct one. This is what keeps files off the server.
		libp2p.EnableHolePunching(),
		// AutoNAT v2: learn whether our own addresses are reachable.
		libp2p.EnableAutoNATv2(),
		// Ask the router to forward a port if it speaks UPnP/NAT-PMP.
		libp2p.NATPortMap(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not start the network layer: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	if err := h.Connect(dialCtx, *info); err != nil {
		h.Close()
		return nil, fmt.Errorf("could not reach the meeting point: %w", err)
	}

	return &Node{
		host:   h,
		server: *info,
		events: make(chan Event, 64),
		done:   make(chan struct{}),
	}, nil
}

// Events returns the channel carrying transfer events. The channel is
// never closed — a transfer may still be winding down and about to emit —
// so a consumer selects on Done to learn when to stop reading.
func (n *Node) Events() <-chan Event { return n.events }

// Done is closed when the node is closed.
func (n *Node) Done() <-chan struct{} { return n.done }

// Close shuts the host down. It is safe to call more than once.
func (n *Node) Close() error {
	n.closeOnce.Do(func() { close(n.done) })
	return n.host.Close()
}

// emit delivers an event. Progress events are dropped when the consumer
// is behind — they are superseded by the next one anyway — while every
// other event blocks until delivered. Nothing blocks once the node is
// closed: a transfer that is still unwinding must not leave a goroutine
// waiting on a reader that will never come back.
func (n *Node) emit(e Event) {
	if _, isProgress := e.(ProgressEvent); isProgress {
		select {
		case n.events <- e:
		default:
		}
		return
	}
	select {
	case n.events <- e:
	case <-n.done:
	}
}

// progressFunc returns a throttled transfer.ProgressFunc.
func (n *Node) progressFunc() transfer.ProgressFunc {
	var last time.Time
	return func(name string, done, total int64) {
		// Always report the final chunk so the bar reaches 100%.
		if done < total && time.Since(last) < progressInterval {
			return
		}
		last = time.Now()
		n.emit(ProgressEvent{Name: name, Done: done, Total: total})
	}
}

// ---------------------------------------------------------------------------
// Sending
// ---------------------------------------------------------------------------

// Host claims a room code for the given files and starts waiting for a
// receiver. It returns as soon as the room is live; the transfer itself
// runs in the background and reports through Events.
//
// A failed attempt does not end the session: the room stays registered,
// so the receiver can simply try the same code again.
func (n *Node) Host(ctx context.Context, paths []string) (string, error) {
	// Addresses to advertise: relay circuit addresses through the server
	// first — those are what matter when both sides are behind NAT —
	// then our own, which win when both peers are on the same network.
	var addrs []multiaddr.Multiaddr
	for _, sa := range n.server.Addrs {
		circuit, err := multiaddr.NewMultiaddr(
			fmt.Sprintf("%s/p2p/%s/p2p-circuit", sa, n.server.ID))
		if err == nil {
			addrs = append(addrs, circuit)
		}
	}
	addrs = append(addrs, n.host.Addrs()...)
	if len(addrs) > rendezvous.MaxAddrs {
		addrs = addrs[:rendezvous.MaxAddrs]
	}

	// Register the room before reserving the relay slot: the server's
	// relay only serves peers with an active room, so the reservation
	// would otherwise be refused by its ACL.
	room := rendezvous.NewRoomCode()
	if err := rendezvous.Register(ctx, n.host, n.server.ID, room, addrs); err != nil {
		return "", err
	}
	if _, err := relayclient.Reserve(ctx, n.host, n.server); err != nil {
		return "", fmt.Errorf("the meeting point refused to hold a slot for us: %w", err)
	}

	n.host.SetStreamHandler(transfer.ProtocolID, func(s network.Stream) {
		n.emit(ConnectedEvent{Direct: !s.Conn().Stat().Limited})
		err := transfer.Send(s, paths, n.progressFunc())
		n.emit(DoneEvent{Err: err})
	})

	return room, nil
}

// ---------------------------------------------------------------------------
// Receiving
// ---------------------------------------------------------------------------

// Fetch looks up a room code, connects to the sender and downloads the
// files into outDir. It blocks until the transfer finishes and reports
// every step through Events — including ManifestEvent, which waits for
// the user's approval before anything touches the disk.
func (n *Node) Fetch(ctx context.Context, room, outDir string) {
	paths, err := n.fetch(ctx, room, outDir)
	n.emit(DoneEvent{Paths: paths, Err: err})
}

func (n *Node) fetch(ctx context.Context, room, outDir string) ([]string, error) {
	n.emit(StatusEvent{Text: "looking up the code"})
	sender, err := rendezvous.Lookup(ctx, n.host, n.server.ID, room)
	if err != nil {
		return nil, err
	}

	n.emit(StatusEvent{Text: "connecting to the other computer"})
	dialCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := n.host.Connect(dialCtx, *sender); err != nil {
		return nil, fmt.Errorf("could not connect to the other computer: %w", err)
	}

	// The first connection usually arrives through the relay; DCUtR then
	// tries to replace it with a direct one in the background.
	n.emit(StatusEvent{Text: "opening a direct route"})
	direct := n.waitForDirect(sender.ID, directWait)
	n.emit(ConnectedEvent{Direct: direct})

	streamCtx := ctx
	if !direct {
		// Relayed connections are "limited"; opening a stream on one
		// requires explicit consent.
		streamCtx = network.WithAllowLimitedConn(ctx, "file transfer")
	}
	s, err := n.host.NewStream(streamCtx, sender.ID, transfer.ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("could not start the transfer: %w", err)
	}

	confirm := func(m transfer.Manifest) bool {
		reply := make(chan bool, 1)
		n.emit(ManifestEvent{Manifest: m, Reply: reply})
		select {
		case ok := <-reply:
			return ok
		case <-ctx.Done():
			return false
		}
	}
	return transfer.Receive(s, outDir, confirm, n.progressFunc())
}

// waitForDirect blocks until a non-relayed connection to the peer exists,
// or the timeout expires.
func (n *Node) waitForDirect(p peer.ID, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, c := range n.host.Network().ConnsToPeer(p) {
			// Limited==true means the connection goes through a relay.
			if !c.Stat().Limited {
				return true
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}
