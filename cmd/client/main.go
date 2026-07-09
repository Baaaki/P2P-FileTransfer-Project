// Client: the interactive terminal app that sends and receives files.
//
// Usage:
//
//	go run ./cmd/client -server /ip4/1.2.3.4/tcp/4001/p2p/12D3Koo...
//
// The sending side picks files and gets a room code printed on screen.
// The receiving side enters the same code; the two machines find each
// other through the server, connect directly (via hole punching when
// possible), and the files are transferred without touching the server.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"puresend/internal/rendezvous"
	"puresend/internal/transfer"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"
)

var stdin = bufio.NewReader(os.Stdin)

func main() {
	serverAddr := flag.String("server", "", "multiaddr of the rendezvous server")
	flag.Parse()

	// Keep libp2p's log output terse (optional, keeps the screen clean).
	log.SetFlags(0)

	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║  PureSend — P2P File Transfer ║")
	fmt.Println("╚═══════════════════════════════════════╝")

	addr := *serverAddr
	if addr == "" {
		addr = prompt("Server address (multiaddr)")
	}
	serverInfo, err := peer.AddrInfoFromString(addr)
	if err != nil {
		fatal("could not parse server address: %v", err)
	}

	h, err := newHost()
	if err != nil {
		fatal("could not start libp2p host: %v", err)
	}
	defer h.Close()

	ctx := context.Background()
	fmt.Print("\nConnecting to server... ")
	if err := connectToServer(ctx, h, *serverInfo); err != nil {
		fatal("connection failed: %v", err)
	}
	fmt.Println("connected ✓")

	for {
		fmt.Println("\n1) Send files")
		fmt.Println("2) Receive files")
		fmt.Println("q) Quit")
		switch prompt("Choice") {
		case "1":
			if err := sendFlow(ctx, h, *serverInfo); err != nil {
				fmt.Println("\n✗ Sending failed:", err)
			}
		case "2":
			if err := receiveFlow(ctx, h, *serverInfo); err != nil {
				fmt.Println("\n✗ Receiving failed:", err)
			}
		case "q", "Q":
			return
		default:
			fmt.Println("Invalid choice.")
		}
	}
}

// newHost creates a libp2p node that can operate behind NAT.
func newHost() (host.Host, error) {
	return libp2p.New(
		// DCUtR (hole punching): tries to upgrade a relayed connection
		// to a direct one. This is the heart of the project.
		libp2p.EnableHolePunching(),
		// AutoNAT v2: lets us learn whether our addresses are reachable
		// from the outside.
		libp2p.EnableAutoNATv2(),
		// Try automatic port forwarding if the router supports UPnP.
		libp2p.NATPortMap(),
	)
}

// connectToServer dials the server and remembers its address permanently.
func connectToServer(ctx context.Context, h host.Host, server peer.AddrInfo) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	h.Peerstore().AddAddrs(server.ID, server.Addrs, peerstore.PermanentAddrTTL)
	return h.Connect(ctx, server)
}

// ---------------------------------------------------------------------------
// Sending flow
// ---------------------------------------------------------------------------

func sendFlow(ctx context.Context, h host.Host, server peer.AddrInfo) error {
	paths, err := askFilePaths()
	if err != nil {
		return err
	}

	// Reserve a relay slot on the server: if both sides are behind NAT,
	// the receiver establishes its first connection to us through this
	// bridge. A reservation is valid for 1 hour by default.
	if _, err := relayclient.Reserve(ctx, h, server); err != nil {
		return fmt.Errorf("relay reservation failed: %w", err)
	}

	// Addresses to advertise to the receiver: our own addresses (useful
	// when on the same network) plus our relay addresses via the server.
	addrs := append([]multiaddr.Multiaddr{}, h.Addrs()...)
	for _, sa := range server.Addrs {
		circuit, err := multiaddr.NewMultiaddr(
			fmt.Sprintf("%s/p2p/%s/p2p-circuit", sa, server.ID))
		if err == nil {
			addrs = append(addrs, circuit)
		}
	}

	// Install the handler that sends the files once the receiver opens
	// a stream.
	done := make(chan error, 1)
	h.SetStreamHandler(transfer.ProtocolID, func(s network.Stream) {
		fmt.Printf("\nReceiver connected: %s\n", connType(s.Conn()))
		done <- transfer.Send(s, paths, progressBar())
	})
	defer h.RemoveStreamHandler(transfer.ProtocolID)

	// Register the room on the server and show the code to the user.
	room := rendezvous.NewRoomCode()
	if err := rendezvous.Register(ctx, h, server.ID, room, addrs); err != nil {
		return err
	}

	fmt.Println("\n┌────────────────────────────────┐")
	fmt.Printf("│  Room code:  %-17s │\n", room)
	fmt.Println("└────────────────────────────────┘")
	fmt.Println("Share this code with the receiver. It is valid for 1 hour.")
	fmt.Println("Waiting for the receiver... (Ctrl+C to cancel)")

	if err := <-done; err != nil {
		return err
	}
	fmt.Printf("\n✓ %d file(s) sent successfully.\n", len(paths))
	return nil
}

// askFilePaths asks the user for the files to send, one at a time.
func askFilePaths() ([]string, error) {
	fmt.Println("\nEnter the paths of the files to send (leave empty to finish):")
	var paths []string
	var total int64
	for {
		p := prompt(fmt.Sprintf("File %d", len(paths)+1))
		if p == "" {
			break
		}
		info, err := os.Stat(p)
		if err != nil {
			fmt.Println("  File not found, try again:", p)
			continue
		}
		if info.IsDir() {
			fmt.Println("  That is a directory; please enter a file.")
			continue
		}
		paths = append(paths, p)
		total += info.Size()
		fmt.Printf("  + %s (%s)\n", info.Name(), formatBytes(info.Size()))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no files selected")
	}
	fmt.Printf("Total: %d file(s), %s\n", len(paths), formatBytes(total))
	return paths, nil
}

// ---------------------------------------------------------------------------
// Receiving flow
// ---------------------------------------------------------------------------

func receiveFlow(ctx context.Context, h host.Host, server peer.AddrInfo) error {
	room := prompt("\nRoom code")
	if room == "" {
		return fmt.Errorf("room code cannot be empty")
	}
	outDir := prompt("Save directory [received]")
	if outDir == "" {
		outDir = "received"
	}

	// Ask the server: who is in this room?
	sender, err := rendezvous.Lookup(ctx, h, server.ID, room)
	if err != nil {
		return err
	}
	fmt.Println("Sender found:", sender.ID)

	// Connect to the sender. The address list contains both direct
	// addresses and the relay address via the server; libp2p picks
	// whatever works.
	connectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := h.Connect(connectCtx, *sender); err != nil {
		return fmt.Errorf("could not connect to sender: %w", err)
	}

	// If the connection came up through the relay, DCUtR tries to
	// upgrade it to a direct one in the background; give it some time.
	streamCtx := ctx
	if waitForDirect(h, sender.ID, 30*time.Second) {
		fmt.Println("✓ Direct P2P connection established — files will bypass the server.")
	} else {
		fmt.Println("! Could not establish a direct connection; transferring through the relay (server).")
		// Relayed connections are considered "limited"; opening a
		// stream on one requires explicit permission.
		streamCtx = network.WithAllowLimitedConn(ctx, "file transfer")
	}

	s, err := h.NewStream(streamCtx, sender.ID, transfer.ProtocolID)
	if err != nil {
		return fmt.Errorf("could not open transfer stream: %w", err)
	}

	saved, err := transfer.Receive(s, outDir, progressBar())
	if err != nil {
		return err
	}
	fmt.Printf("\n✓ %d file(s) received and verified:\n", len(saved))
	for _, p := range saved {
		fmt.Println("  ", p)
	}
	return nil
}

// waitForDirect blocks until a non-relayed (direct) connection to the
// given peer exists. Returns false if the timeout expires first.
func waitForDirect(h host.Host, p peer.ID, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, c := range h.Network().ConnsToPeer(p) {
			// Limited=true means the connection goes through a relay;
			// false means a direct connection is up.
			if !c.Stat().Limited {
				return true
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// connType reports whether a connection is direct or relayed.
func connType(c network.Conn) string {
	if c.Stat().Limited {
		return "via relay (attempting direct connection...)"
	}
	return "direct connection ✓"
}

// ---------------------------------------------------------------------------
// Small UI helpers
// ---------------------------------------------------------------------------

// progressBar returns a single-line, self-updating progress bar.
func progressBar() transfer.ProgressFunc {
	lastFile := ""
	return func(name string, done, total int64) {
		const width = 24
		filled := int(float64(done) / float64(total) * width)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
		fmt.Printf("\r  %-20s [%s] %3d%%  %s / %s   ",
			truncate(name, 20), bar, done*100/total,
			formatBytes(done), formatBytes(total))
		if done == total && name != lastFile {
			fmt.Println() // file finished, move to the next line
			lastFile = name
		}
	}
}

func prompt(label string) string {
	fmt.Printf("%s: ", label)
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		// Ctrl+D (EOF): exit gracefully.
		fmt.Println("\nBye!")
		os.Exit(0)
	}
	return strings.TrimSpace(line)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func fatal(format string, args ...any) {
	fmt.Printf("✗ "+format+"\n", args...)
	os.Exit(1)
}
