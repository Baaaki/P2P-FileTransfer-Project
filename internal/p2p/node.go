// Package p2p wraps the libp2p plumbing behind an API the user interface
// can drive: every long-running step reports progress as an Event instead
// of printing to the screen, so the same logic works under a TUI, a CLI
// or a test.
package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"puresend/internal/rendezvous"
	"puresend/internal/transfer"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

const (
	// directWait is the longest a receiver is kept waiting while DCUtR
	// tries to replace the relayed connection with a direct one. It is a
	// backstop, not the normal case: the hole puncher says when it has
	// given up and we stop right there, which on a hopeless pair takes a
	// few seconds. Because of that, this can afford to be generous for the
	// slow mobile link where hole punching does eventually work.
	directWait = 30 * time.Second

	// holePunchAttempts mirrors how many tries libp2p's DCUtR makes before
	// giving up. Counting them is what lets us stop early; if the number
	// ever changes upstream we simply fall back to waiting out directWait.
	holePunchAttempts = 3

	// Progress events are throttled to this interval: a 1 GB file would
	// otherwise produce tens of thousands of screen updates.
	progressInterval = 80 * time.Millisecond

	// handshakeSlots is how many receivers a sender will be proving the
	// code to at once. Only one of them can ever get the files, so this
	// only needs to be enough that a stranger dawdling in a handshake
	// cannot keep the real friend from starting theirs.
	handshakeSlots = 4

	// connectBudget is how long reaching a meeting point may take in all.
	connectBudget = 30 * time.Second

	// maxBackoff caps the pause between two attempts to get back to the
	// meeting point after losing it.
	maxBackoff = 30 * time.Second
)

// errRoomExpired ends a room that could not be put back in time.
var errRoomExpired = errors.New("the room code expired")

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

// Event is something worth telling the user about. The concrete types
// below are the full set.
type Event interface{ isEvent() }

// StatusEvent is a plain-language description of the current step.
type StatusEvent struct{ Text string }

// PreparingEvent fires while files are being read to compute or check
// their digests. On the sending side this happens in the background right
// after the code is shown; on the receiving side, while checking what an
// earlier attempt left behind.
type PreparingEvent struct {
	Name         string
	Index, Files int
}

// PreparedEvent fires on the sending side once every file has been read
// and the offer is ready for whoever connects.
type PreparedEvent struct{}

// RemotePreparingEvent fires on the receiving side while the sender is
// still reading its files: Done of Total so far.
type RemotePreparingEvent struct{ Done, Total int }

// ConnectedEvent fires once the two peers can talk. Direct is false when
// the connection still runs through the server's relay, in which case
// RelayLimit says how many bytes that relay will carry before cutting the
// connection off (0 if the server did not say).
type ConnectedEvent struct {
	Direct     bool
	RelayLimit int64
}

// ManifestEvent asks the user to approve an incoming file list. The
// transfer blocks until exactly one value is sent on Reply.
type ManifestEvent struct {
	Manifest transfer.Manifest
	Reply    chan<- bool
}

// ProgressEvent reports transfer progress.
type ProgressEvent struct{ transfer.Progress }

// DoneEvent is the final event of a transfer. Err is nil on success;
// Paths is populated on the receiving side. On the sending side a failed
// transfer leaves the room open, so another attempt may follow.
type DoneEvent struct {
	Paths []string
	Err   error
}

// RejectedEvent fires on the sending side when someone reached the room
// but could not prove they hold the code. Nothing was revealed to them;
// the room stays open for the real receiver.
type RejectedEvent struct{}

// ServerLostEvent fires on the sending side when the connection to the
// meeting point drops while a room is open. The node is already working
// on getting it back.
type ServerLostEvent struct{}

// ServerBackEvent fires when the room is reachable again after a
// ServerLostEvent.
type ServerBackEvent struct{}

// RoomLostEvent ends a hosting session for good: the code no longer works
// and Err says why — the files could not be read, or the room could not
// be put back after the connection to the meeting point was lost.
type RoomLostEvent struct{ Err error }

func (StatusEvent) isEvent()          {}
func (PreparingEvent) isEvent()       {}
func (PreparedEvent) isEvent()        {}
func (RemotePreparingEvent) isEvent() {}
func (ConnectedEvent) isEvent()       {}
func (ManifestEvent) isEvent()        {}
func (ProgressEvent) isEvent()        {}
func (DoneEvent) isEvent()            {}
func (RejectedEvent) isEvent()        {}
func (ServerLostEvent) isEvent()      {}
func (ServerBackEvent) isEvent()      {}
func (RoomLostEvent) isEvent()        {}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

// Node is a running libp2p host with an open connection to the
// rendezvous server.
type Node struct {
	host    host.Host
	servers []peer.AddrInfo // every meeting point known, in order of preference
	server  atomic.Pointer[peer.AddrInfo]
	punch   *punchWatcher
	events  chan Event

	// publicIP holds the external WAN IPv4 address discovered via STUN.
	publicIP atomic.Pointer[net.IP]

	// claimed guards the one-transfer-per-room rule: it is set when a
	// receiver that proved the code takes the room, and cleared again if
	// that attempt fails.
	claimed    atomic.Bool
	handshakes chan struct{}

	// lost is signalled when the connection to the meeting point drops.
	lost chan struct{}

	mu       sync.Mutex
	room     string // the room being hosted, empty once it is closed
	hostedAt time.Time

	ctx       context.Context // cancelled by Close; ends background work
	cancel    context.CancelFunc
	closeOnce sync.Once
	done      chan struct{}
}

// Option adjusts how New finds a meeting point or discovers its network.
type Option func(*options)

type options struct {
	listURL     string
	stunServers []string
}

// WithServerList names a URL listing more meeting point addresses, one per
// line, which New fetches only when none of the addresses it was given
// answers. See fetchServerList.
func WithServerList(url string) Option {
	return func(o *options) { o.listURL = strings.TrimSpace(url) }
}

// WithSTUNServers configures custom STUN servers in order of priority.
// If not specified, DefaultSTUNServers is used.
func WithSTUNServers(servers []string) Option {
	return func(o *options) {
		var list []string
		for _, s := range servers {
			if s = strings.TrimSpace(s); s != "" {
				list = append(list, s)
			}
		}
		o.stunServers = list
	}
}

// SplitServers parses a comma-separated list of meeting point addresses.
// More than one is a cheap insurance policy: the address is baked into
// every released client, so a single server going away would otherwise
// take every copy of the program down with it.
func SplitServers(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parseServers turns multiaddrs into peer infos.
func parseServers(addrs []string) ([]peer.AddrInfo, error) {
	infos := make([]peer.AddrInfo, 0, len(addrs))
	for _, addr := range addrs {
		info, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, fmt.Errorf("meeting point address is not valid: %w", err)
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// New starts a libp2p host and connects it to the first meeting point it
// can reach, trying the given multiaddrs in order (e.g.
// /dns4/host/tcp/443/tls/ws/p2p/12D3Koo...). If none answers and a server
// list is configured, the addresses it names are tried next.
func New(ctx context.Context, serverAddrs []string, opts ...Option) (*Node, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if len(serverAddrs) == 0 && o.listURL == "" {
		return nil, fmt.Errorf("no meeting point address configured")
	}
	infos, err := parseServers(serverAddrs)
	if err != nil {
		return nil, err
	}

	var pubIP *net.IP
	stunServers := o.stunServers
	if len(stunServers) == 0 {
		stunServers = DefaultSTUNServers
	}
	stunCtx, stunCancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	if ip, err := ResolvePublicIP(stunCtx, stunServers); err == nil && ip != nil {
		pubIP = &ip
	}
	stunCancel()

	addrsFactory := func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
		var result []multiaddr.Multiaddr
		for _, a := range addrs {
			// Skip Docker container bridge networks (172.16.0.0/12) which
			// bloat the multiaddr list and push useful addrs past MaxAddrs.
			if isDockerAddr(a) {
				continue
			}
			result = append(result, a)

			// If we have a STUN-discovered public IP, synthesize public multiaddrs
			// from local LAN / interface bindings.
			if pubIP != nil {
				if pubMA, ok := injectPublicIP(a, *pubIP); ok {
					result = append(result, pubMA)
				}
			}
		}
		return multiaddr.Unique(result)
	}

	punch := newPunchWatcher()
	h, err := libp2p.New(
		// DCUtR (hole punching): upgrades a relayed connection to a
		// direct one. This is what keeps files off the server.
		libp2p.EnableHolePunching(holepunch.WithTracer(punch)),
		// AutoNAT v2: learn whether our own addresses are reachable.
		libp2p.EnableAutoNATv2(),
		// Ask the router to forward a port if it speaks UPnP/NAT-PMP.
		libp2p.NATPortMap(),
		// Filter out useless virtual docker addresses and inject STUN-discovered public IP
		libp2p.AddrsFactory(addrsFactory),
	)
	if err != nil {
		return nil, fmt.Errorf("could not start the network layer: %w", err)
	}

	nodeCtx, cancel := context.WithCancel(context.Background())
	n := &Node{
		host:       h,
		servers:    infos,
		punch:      punch,
		events:     make(chan Event, 64),
		handshakes: make(chan struct{}, handshakeSlots),
		lost:       make(chan struct{}, 1),
		ctx:        nodeCtx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	if pubIP != nil {
		n.publicIP.Store(pubIP)
	}

	server, err := n.connectAny(ctx, infos)
	if err != nil && o.listURL != "" {
		// The addresses baked into this copy of the program are all dead —
		// most likely the server moved. Ask the published list where it
		// went, rather than dying with every other copy.
		listed, lerr := fetchServerList(ctx, o.listURL)
		if lerr == nil {
			extra, perr := parseServers(listed)
			if perr == nil {
				n.servers = append(n.servers, extra...)
				server, err = n.connectAny(ctx, extra)
			}
		}
	}
	if err != nil {
		_ = n.Close()
		return nil, err
	}
	n.server.Store(&server)

	h.Network().Notify(&network.NotifyBundle{
		DisconnectedF: func(net network.Network, c network.Conn) {
			if c.RemotePeer() != n.currentServer().ID || net.Connectedness(c.RemotePeer()) == network.Connected {
				return
			}
			select {
			case n.lost <- struct{}{}:
			default:
			}
		},
	})
	return n, nil
}

// connectAny tries the meeting points in order and returns the first that
// answers. Each gets its own slice of the budget so a black-holed address
// cannot eat the whole timeout and starve a healthy one further down.
func (n *Node) connectAny(ctx context.Context, infos []peer.AddrInfo) (peer.AddrInfo, error) {
	if len(infos) == 0 {
		return peer.AddrInfo{}, fmt.Errorf("could not reach the meeting point: no address to try")
	}
	// Forcing a direct dial skips libp2p's dial backoff, which after a
	// couple of failures would otherwise refuse to even try for minutes —
	// exactly when a meeting point that restarted is coming back.
	ctx = network.WithForceDirectDial(ctx, "reaching the meeting point")
	perServer := max(connectBudget/time.Duration(len(infos)), 10*time.Second)
	var errs []error
	for _, info := range infos {
		dialCtx, cancel := context.WithTimeout(ctx, perServer)
		n.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		err := n.host.Connect(dialCtx, info)
		cancel()
		if err == nil {
			return info, nil
		}
		errs = append(errs, err)
	}
	return peer.AddrInfo{}, fmt.Errorf("could not reach the meeting point: %w", errors.Join(errs...))
}

// currentServer is the meeting point in use.
func (n *Node) currentServer() peer.AddrInfo {
	if s := n.server.Load(); s != nil {
		return *s
	}
	return peer.AddrInfo{}
}

// Events returns the channel carrying transfer events. The channel is
// never closed — a transfer may still be winding down and about to emit —
// so a consumer selects on Done to learn when to stop reading.
func (n *Node) Events() <-chan Event { return n.events }

// Done is closed when the node is closed.
func (n *Node) Done() <-chan struct{} { return n.done }

// ID is this node's libp2p peer ID.
func (n *Node) ID() peer.ID { return n.host.ID() }

// PublicIP returns this node's external WAN IP discovered via STUN, or nil
// if discovery failed or was not possible.
func (n *Node) PublicIP() net.IP {
	if ip := n.publicIP.Load(); ip != nil {
		return *ip
	}
	return nil
}

// Addrs returns the multiaddrs this node is listening on and advertising.
func (n *Node) Addrs() []multiaddr.Multiaddr {
	return n.host.Addrs()
}

// Close shuts the host down. It is safe to call more than once.
func (n *Node) Close() error {
	n.closeOnce.Do(func() {
		n.cancel()
		close(n.done)
	})
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

// hooks returns the transfer callbacks, with progress throttled.
func (n *Node) hooks() transfer.Hooks {
	var last time.Time
	return transfer.Hooks{
		Prepare: func(name string, index, total int) {
			n.emit(PreparingEvent{Name: name, Index: index, Files: total})
		},
		Waiting: func(done, total int) {
			n.emit(RemotePreparingEvent{Done: done, Total: total})
		},
		Progress: func(p transfer.Progress) {
			// Always report the final chunk so the bar reaches 100%.
			if p.Done < p.Total && time.Since(last) < progressInterval {
				return
			}
			last = time.Now()
			n.emit(ProgressEvent{Progress: p})
		},
	}
}

// ---------------------------------------------------------------------------
// Sending
// ---------------------------------------------------------------------------

// Host claims a room code for the given files and folders and starts
// waiting for a receiver. It returns as soon as the room is live; reading
// the files, the transfer itself and keeping the room reachable all run
// in the background and report through Events.
//
// A failed attempt does not end the session: the room stays registered, so
// the receiver can simply try the same code again. A *successful* one
// does — see closeRoom.
func (n *Node) Host(ctx context.Context, paths []string) (string, error) {
	// Fail on an unreadable selection now, while there is still a screen to
	// report it on, rather than when a receiver is already connected.
	offer, err := transfer.NewOffer(paths)
	if err != nil {
		return "", err
	}

	// A fresh code is almost never taken, but when it is, another one is
	// as good — the user has not seen it yet.
	var room string
	for attempt := 0; ; attempt++ {
		room = rendezvous.NewRoomCode()
		err = n.announce(ctx, room)
		if errors.Is(err, rendezvous.ErrInUse) && attempt < 3 {
			continue
		}
		if err != nil {
			return "", err
		}
		break
	}

	n.mu.Lock()
	n.room, n.hostedAt = room, time.Now()
	n.mu.Unlock()

	// Read the files while the code is being read out, so whoever
	// connects gets the manifest without a wait.
	offer.Start(n.ctx, n.hooks())
	go n.watchOffer(offer)
	n.host.SetStreamHandler(transfer.ProtocolID, n.serve(room, offer))
	go n.keepRoom()
	return room, nil
}

// announce registers the room on the current meeting point and reserves
// a relay slot there — everything a receiver needs to reach us.
func (n *Node) announce(ctx context.Context, room string) error {
	server := n.currentServer()

	// Prioritize addresses to advertise:
	// 1. Direct public addresses (STUN IPv4 + Global IPv6)
	// 2. Relay circuit addresses (fallback if both peers are behind NAT)
	// 3. Local LAN addresses (win when both peers are on the same local network)
	// 4. Loopback (for local testing)
	var (
		publicAddrs   []multiaddr.Multiaddr
		circuitAddrs  []multiaddr.Multiaddr
		privateAddrs  []multiaddr.Multiaddr
		loopbackAddrs []multiaddr.Multiaddr
	)

	for _, sa := range server.Addrs {
		circuit, err := multiaddr.NewMultiaddr(
			fmt.Sprintf("%s/p2p/%s/p2p-circuit", sa, server.ID))
		if err == nil {
			circuitAddrs = append(circuitAddrs, circuit)
		}
	}

	for _, a := range n.host.Addrs() {
		if isDockerAddr(a) {
			continue
		}
		switch {
		case manet.IsPublicAddr(a):
			publicAddrs = append(publicAddrs, a)
		case manet.IsPrivateAddr(a):
			privateAddrs = append(privateAddrs, a)
		case manet.IsIPLoopback(a):
			loopbackAddrs = append(loopbackAddrs, a)
		default:
			privateAddrs = append(privateAddrs, a)
		}
	}

	var addrs []multiaddr.Multiaddr
	addrs = append(addrs, publicAddrs...)
	addrs = append(addrs, circuitAddrs...)
	addrs = append(addrs, privateAddrs...)
	addrs = append(addrs, loopbackAddrs...)
	addrs = multiaddr.Unique(addrs)
	if len(addrs) > rendezvous.MaxAddrs {
		addrs = addrs[:rendezvous.MaxAddrs]
	}

	// Register the room before reserving the relay slot: the server's
	// relay only serves peers with an active room, so the reservation
	// would otherwise be refused by its ACL.
	if err := rendezvous.Register(ctx, n.host, server.ID, room, addrs); err != nil {
		return err
	}
	if _, err := relayclient.Reserve(ctx, n.host, server); err != nil {
		// Do not leave behind a room nobody can reach.
		_ = rendezvous.Unregister(ctx, n.host, server.ID, room)
		return fmt.Errorf("the meeting point refused to hold a slot for us: %w", err)
	}
	return nil
}

// serve is the transfer protocol handler for a room.
func (n *Node) serve(room string, offer *transfer.Offer) network.StreamHandler {
	return func(s network.Stream) {
		// A handshake is cheap to start and anyone who knows our address
		// can start one; only so many run at a time.
		select {
		case n.handshakes <- struct{}{}:
		default:
			_ = s.Reset()
			return
		}
		held := true
		release := func() {
			if held {
				held = false
				<-n.handshakes
			}
		}
		defer release()

		claimed := false
		err := transfer.Send(s, offer, transfer.Credentials{
			Code:     room,
			Sender:   n.host.ID().String(),
			Receiver: s.Conn().RemotePeer().String(),
		}, transfer.SendOptions{
			Hooks: n.hooks(),
			// One room, one transfer — but the room is only taken by a
			// receiver that proved the code. Anyone else arriving while a
			// transfer runs is told the room is busy.
			Claim: func() bool {
				release()
				if !n.claimed.CompareAndSwap(false, true) {
					return false
				}
				claimed = true
				n.emit(ConnectedEvent{Direct: !s.Conn().Stat().Limited})
				return true
			},
		})

		if !claimed {
			// Nobody's transfer started, so there is nothing to finish.
			// A wrong code is worth mentioning; a dropped handshake is not.
			if errors.Is(err, transfer.ErrWrongCode) {
				n.emit(RejectedEvent{})
			}
			return
		}
		switch {
		case err == nil:
			// The files are delivered, so the code has done its job. Retire
			// it now rather than leaving it usable — by whoever else was
			// told it — until the user gets back to the keyboard.
			n.closeRoom()
			n.emit(DoneEvent{})
		case transfer.IsSourceError(err):
			// Our own files cannot be read: no retry will fix that, and
			// the person who chose them needs to hear it.
			n.abandonRoom(err)
		default:
			// A failed attempt leaves the room open on purpose: the usual
			// cause is a flaky link, and the friend should be able to try
			// the same code again without asking for a new one.
			n.claimed.Store(false)
			n.emit(DoneEvent{Err: err})
		}
	}
}

// watchOffer ends the room if the files could not be read.
func (n *Node) watchOffer(offer *transfer.Offer) {
	select {
	case <-offer.Ready():
	case <-n.done:
		return
	}
	if err := offer.Err(); err != nil {
		if n.ctx.Err() == nil {
			n.abandonRoom(err)
		}
		return
	}
	n.emit(PreparedEvent{})
}

// keepRoom puts the room back whenever the connection to the meeting point
// drops. A tunnel restarting or a server being redeployed is an ordinary
// event, not an emergency: the relay slot and the room go with the
// connection, and without this the sender would sit on "waiting" forever
// while its friend is told the code does not exist.
func (n *Node) keepRoom() {
	for {
		select {
		case <-n.done:
			return
		case <-n.lost:
		}
		if n.ctx.Err() != nil || n.currentRoom() == "" {
			return
		}
		if n.host.Network().Connectedness(n.currentServer().ID) == network.Connected {
			continue // already back, by some other path
		}

		_ = n.host.Network().ClosePeer(n.currentServer().ID)
		n.emit(ServerLostEvent{})
		if err := n.restore(); err != nil {
			if n.ctx.Err() == nil && n.currentRoom() != "" {
				n.abandonRoom(err)
			}
			return
		}
		if n.currentRoom() != "" {
			n.emit(ServerBackEvent{})
		}
	}
}

// restore reconnects to a meeting point and puts the room back, trying
// again with growing pauses until it works or the room's hour is up.
func (n *Node) restore() error {
	backoff := time.Second
	for {
		n.mu.Lock()
		room, hostedAt := n.room, n.hostedAt
		n.mu.Unlock()
		if room == "" {
			return nil // closed meanwhile; nothing to put back
		}
		if time.Since(hostedAt) >= rendezvous.RoomTTL {
			return errRoomExpired
		}

		ctx, cancel := context.WithTimeout(n.ctx, connectBudget)
		server, err := n.connectAny(ctx, n.reconnectOrder())
		if err == nil {
			n.server.Store(&server)
			err = n.announce(ctx, room)
		}
		cancel()
		switch {
		case err == nil:
			return nil
		case errors.Is(err, rendezvous.ErrInUse):
			return err // someone else holds our code now; it is gone
		}

		select {
		case <-n.done:
			return n.ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maxBackoff)
	}
}

// reconnectOrder is the meeting point we were using, then every other
// one we know of: the room lives on the first, but any will do.
func (n *Node) reconnectOrder() []peer.AddrInfo {
	cur := n.currentServer()
	order := []peer.AddrInfo{cur}
	for _, s := range n.servers {
		if s.ID != cur.ID {
			order = append(order, s)
		}
	}
	return order
}

func (n *Node) currentRoom() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.room
}

// takeRoom forgets the room and returns what it was, so exactly one caller
// gets to close it.
func (n *Node) takeRoom() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	room := n.room
	n.room = ""
	return room
}

// closeRoom retires a room that has served its purpose: the handler goes
// first so nothing new can start, then the server is told to forget the
// code. Both halves matter — dropping only the registration would still
// leave the sender answering anyone who already knows the code.
func (n *Node) closeRoom() {
	room := n.takeRoom()
	if room == "" {
		return
	}
	n.host.RemoveStreamHandler(transfer.ProtocolID)

	// Best effort, and on its own deadline: the transfer is already done,
	// and a meeting point that has gone away must not turn a successful
	// send into a failure. The room's one-hour expiry is the backstop.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = rendezvous.Unregister(ctx, n.host, n.currentServer().ID, room)
}

// abandonRoom ends hosting for good and says why.
func (n *Node) abandonRoom(err error) {
	if n.currentRoom() == "" {
		return
	}
	n.claimed.Store(true) // nobody new gets in while it winds down
	n.closeRoom()
	n.emit(RoomLostEvent{Err: err})
}

// ---------------------------------------------------------------------------
// Receiving
// ---------------------------------------------------------------------------

// CodeError says what is wrong with a typed room code.
type CodeError struct {
	Code string // the code as normalized
	Word string // the word that is not in the list, if that is the problem
}

func (e *CodeError) Error() string {
	if e.Word != "" {
		return fmt.Sprintf("the word %q is not used in room codes", e.Word)
	}
	return fmt.Sprintf("%q is not a room code", e.Code)
}

// CheckCode normalizes what a person typed and says what is wrong with it,
// if anything. Checking before asking the server means a typo never costs
// one of the few tries the server allows.
func CheckCode(typed string) (string, error) {
	code := rendezvous.NormalizeCode(typed)
	if !rendezvous.ValidCode(code) {
		return code, &CodeError{Code: code}
	}
	if w := rendezvous.UnknownWord(code); w != "" {
		return code, &CodeError{Code: code, Word: w}
	}
	return code, nil
}

// Fetch looks up a room code, connects to the sender and downloads the
// files into outDir. It blocks until the transfer finishes and reports
// every step through Events — including ManifestEvent, which waits for
// the user's approval before anything touches the disk.
func (n *Node) Fetch(ctx context.Context, room, outDir string) {
	paths, err := n.fetch(ctx, room, outDir)
	n.emit(DoneEvent{Paths: paths, Err: err})
}

func (n *Node) fetch(ctx context.Context, typed, outDir string) ([]string, error) {
	room, err := CheckCode(typed)
	if err != nil {
		return nil, err
	}

	n.emit(StatusEvent{Text: "looking up the code"})
	sender, relayLimit, err := rendezvous.Lookup(ctx, n.host, n.currentServer().ID, room)
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
	direct := n.waitForDirect(ctx, sender.ID, directWait)
	n.emit(ConnectedEvent{Direct: direct, RelayLimit: relayLimit})

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
	return transfer.Receive(s, outDir, transfer.Credentials{
		Code:     room,
		Sender:   sender.ID.String(),
		Receiver: n.host.ID().String(),
	}, confirm, n.hooks())
}

// waitForDirect blocks until a non-relayed connection to the peer exists,
// the hole puncher reports it has run out of ideas, or the timeout
// expires. Waiting out the full timeout on a pair that was never going to
// work is the single most visible delay in the program, so the give-up
// signal matters as much as the success one.
func (n *Node) waitForDirect(ctx context.Context, p peer.ID, timeout time.Duration) bool {
	gaveUp := n.punch.watch(p)
	defer n.punch.forget(p)

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(250 * time.Millisecond)
	defer poll.Stop()

	for {
		if n.isDirect(p) {
			return true
		}
		select {
		case <-poll.C:
		case <-gaveUp:
			// One last look: the final attempt may have succeeded in the
			// moment between the event and this check.
			return n.isDirect(p)
		case <-deadline.C:
			return n.isDirect(p)
		case <-ctx.Done():
			return false
		case <-n.done:
			return false
		}
	}
}

// isDirect reports whether any connection to the peer bypasses the relay.
func (n *Node) isDirect(p peer.ID) bool {
	for _, c := range n.host.Network().ConnsToPeer(p) {
		// Limited==true means the connection goes through a relay.
		if !c.Stat().Limited {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Hole punch watching
// ---------------------------------------------------------------------------

// punchWatcher listens to DCUtR's own account of how it is getting on. It
// implements holepunch.EventTracer.
type punchWatcher struct {
	mu       sync.Mutex
	failures map[peer.ID]int
	watching map[peer.ID]chan struct{}
}

func newPunchWatcher() *punchWatcher {
	return &punchWatcher{
		failures: make(map[peer.ID]int),
		watching: make(map[peer.ID]chan struct{}),
	}
}

// watch returns a channel that is closed once hole punching towards p has
// definitively failed.
func (w *punchWatcher) watch(p peer.ID) <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan struct{})
	w.failures[p] = 0
	w.watching[p] = ch
	return ch
}

func (w *punchWatcher) forget(p peer.ID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.failures, p)
	delete(w.watching, p)
}

// Trace is called by libp2p for every step of a hole punch.
func (w *punchWatcher) Trace(evt *holepunch.Event) {
	switch e := evt.Evt.(type) {
	case *holepunch.EndHolePunchEvt:
		if !e.Success {
			w.countFailure(evt.Remote, 1)
		}
	case *holepunch.ProtocolErrorEvt:
		// The other side cannot even coordinate a hole punch: there will
		// be no retries, so there is nothing left to wait for.
		w.countFailure(evt.Remote, holePunchAttempts)
	}
}

func (w *punchWatcher) countFailure(p peer.ID, n int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch, ok := w.watching[p]
	if !ok {
		return
	}
	w.failures[p] += n
	if w.failures[p] >= holePunchAttempts {
		delete(w.watching, p)
		close(ch)
	}
}
