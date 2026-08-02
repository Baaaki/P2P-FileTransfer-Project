// Server: the rendezvous point where peers find each other.
//
// It has two jobs:
//  1. Rendezvous: the sender registers a room code, the receiver looks it up.
//  2. Circuit Relay v2: the receiver's *first* connection to the sender is
//     established through this server; libp2p's hole punching mechanism
//     (DCUtR) then uses that bridge to negotiate a direct connection.
//     Once the direct connection is up the files bypass this server
//     entirely.
//
// # Running behind Cloudflare Tunnel
//
// cloudflared only proxies HTTP/WebSocket to the public internet — a raw
// TCP or QUIC port cannot be exposed that way. So the primary listener is
// libp2p's WebSocket transport, which Cloudflare proxies natively:
//
//	cloudflared ingress:  p2p-filetransfer.example.com -> http://localhost:8080
//	server listens on:    /ip4/0.0.0.0/tcp/8080/ws
//	clients dial:         /dns4/p2p-filetransfer.example.com/tcp/443/tls/ws/p2p/<PeerID>
//
// TLS is terminated by Cloudflare, which is why the listener itself is
// plain /ws while clients dial /tls/ws.
//
//	go run ./cmd/server -ws-port 8080 \
//	  -announce /dns4/p2p-filetransfer.example.com/tcp/443/tls/ws
//
// # Running with a plain forwarded port
//
// If you can forward a port on your router instead, -port enables the raw
// TCP+QUIC listeners and no tunnel is needed:
//
//	go run ./cmd/server -port 4001
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"filetransferilla/internal/rendezvous"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	wsPort := flag.Int("ws-port", 8080, "WebSocket port (the Cloudflare Tunnel target); 0 disables it")
	port := flag.Int("port", 0, "raw TCP+QUIC port for setups with a forwarded port; 0 disables it")
	healthPort := flag.Int("health-port", 8081, "port for the /health endpoint; 0 disables it")
	keyPath := flag.String("key", envOr("FT_KEY_PATH", "server.key"), "path of the identity key file")
	announce := flag.String("announce", os.Getenv("FT_ANNOUNCE"), "comma-separated public multiaddrs to advertise, e.g. /dns4/host/tcp/443/tls/ws")
	relayData := flag.Int64("relay-data", 256<<20, "max bytes relayed per connection when hole punching fails")
	relayDuration := flag.Duration("relay-duration", 10*time.Minute, "max lifetime of a relayed connection")
	flag.Parse()

	if *wsPort == 0 && *port == 0 {
		log.Fatal("nothing to listen on: set -ws-port or -port")
	}

	// The server's identity (peer ID) must stay the same across restarts
	// so the address baked into released clients remains valid. We save
	// the private key to disk and reload it on the next start.
	priv, keySource, err := loadOrCreateKey(*keyPath)
	if err != nil {
		log.Fatalf("could not prepare identity key: %v", err)
	}

	announced, err := parseAnnounce(*announce)
	if err != nil {
		log.Fatalf("could not parse -announce: %v", err)
	}

	registry := rendezvous.NewRegistry()

	opts := []libp2p.Option{
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(listenAddrs(*wsPort, *port)...),
		// The relay service normally waits until the node has verified it
		// is publicly reachable. Behind a tunnel that probe cannot
		// succeed (the tunnel is outbound-only), so declare it public.
		libp2p.ForceReachabilityPublic(),
		// Serve AutoNAT v2 so clients can discover whether their own
		// addresses are reachable, which is what makes them advertise a
		// sensible address set for hole punching.
		libp2p.EnableAutoNATv2(),
		// Relay v2. The registry acts as the ACL: only peers with an
		// active room may use the relay, so strangers cannot burn our
		// bandwidth. Limits are bounded rather than infinite — the relay
		// exists to bootstrap hole punching (a few KB of DCUtR
		// coordination), and a bounded fallback keeps a failed hole punch
		// from streaming gigabytes through the tunnel.
		libp2p.EnableRelayService(
			relay.WithResources(relayResources(*relayData, *relayDuration)),
			relay.WithACL(registry),
		),
	}
	if len(announced) > 0 {
		// Behind a tunnel the host's own socket addresses (0.0.0.0, the
		// container's private IP) are useless to the outside world.
		// Advertise only what clients can actually reach.
		opts = append(opts, libp2p.AddrsFactory(func([]multiaddr.Multiaddr) []multiaddr.Multiaddr {
			return announced
		}))
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		log.Fatalf("could not start libp2p host: %v", err)
	}
	defer h.Close()

	h.SetStreamHandler(rendezvous.ProtocolID, registry.Handler)

	stopHealth := startHealthServer(*healthPort, h.ID().String(), registry)
	defer stopHealth()

	fmt.Println("Rendezvous + relay server is running.")
	fmt.Println()
	fmt.Println("  Peer ID:", h.ID())
	fmt.Println("  Identity from:", keySource)
	fmt.Println()

	if len(announced) > 0 {
		fmt.Println("Client address (bake this into the client build):")
		for _, addr := range announced {
			fmt.Printf("  %s/p2p/%s\n", addr, h.ID())
		}
	} else {
		// Without -announce (or FT_ANNOUNCE) the only addresses we know
		// are the ones this process is bound to — behind a tunnel or in a
		// container those are private and useless to a client. Say so,
		// rather than printing them under a heading that invites someone
		// to ship them.
		fmt.Println("No public address configured (-announce / FT_ANNOUNCE).")
		fmt.Println("Clients must dial the hostname your tunnel serves, e.g.")
		fmt.Printf("  /dns4/<your-host>/tcp/443/tls/ws/p2p/%s\n", h.ID())
		fmt.Println()
		fmt.Println("Bound to (private, not for clients):")
		for _, addr := range h.Addrs() {
			fmt.Printf("  %s\n", addr)
		}
	}
	fmt.Println()
	fmt.Printf("Relay fallback limit: %s per connection, %s max lifetime\n",
		formatBytes(*relayData), *relayDuration)
	fmt.Println("Press Ctrl+C to stop.")

	// Keep running until Ctrl+C or SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down.")
}

// listenAddrs builds the listen multiaddrs for the enabled ports.
func listenAddrs(wsPort, rawPort int) []string {
	var addrs []string
	if wsPort > 0 {
		// Plain /ws, not /wss: TLS is terminated by Cloudflare (or any
		// other reverse proxy) in front of us.
		addrs = append(addrs, fmt.Sprintf("/ip4/0.0.0.0/tcp/%d/ws", wsPort))
	}
	if rawPort > 0 {
		addrs = append(addrs,
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", rawPort),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", rawPort),
		)
	}
	return addrs
}

// parseAnnounce parses the comma-separated -announce value.
func parseAnnounce(s string) ([]multiaddr.Multiaddr, error) {
	var out []multiaddr.Multiaddr
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		a, err := multiaddr.NewMultiaddr(part)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", part, err)
		}
		out = append(out, a)
	}
	return out, nil
}

// relayResources bounds what a relayed connection may consume.
func relayResources(data int64, duration time.Duration) relay.Resources {
	rc := relay.DefaultResources()
	rc.Limit = &relay.RelayLimit{Duration: duration, Data: data}
	return rc
}

// startHealthServer exposes a tiny /health endpoint for OpenShip (or any
// other supervisor) to probe. Returns a shutdown function.
func startHealthServer(port int, peerID string, registry *rendezvous.Registry) func() {
	if port <= 0 {
		return func() {}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"status\":\"ok\",\"peer_id\":%q,\"active_rooms\":%d}\n",
			peerID, registry.ActiveRooms())
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("health server stopped: %v", err)
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}
}

// envOr returns the environment variable's value, or fallback when it is
// unset or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadOrCreateKey resolves the server's identity, which decides its peer
// ID and therefore whether the clients already released can still reach
// it. Three sources, in order:
//
//  1. FT_IDENTITY_KEY — the base64 of a marshalled private key. Survives
//     anything that happens to the container's disk, which is what makes
//     it the right choice on a platform that mounts an anonymous volume
//     (or none) and hands you a fresh one on every redeploy.
//  2. the key file, when it exists.
//  3. a freshly generated key, written to the file.
//
// Print an existing key in the form (1) wants with:
//
//	base64 -w0 /data/server.key
func loadOrCreateKey(path string) (crypto.PrivKey, string, error) {
	if encoded := os.Getenv("FT_IDENTITY_KEY"); encoded != "" {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			return nil, "", fmt.Errorf("FT_IDENTITY_KEY is not valid base64: %w", err)
		}
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, "", fmt.Errorf("FT_IDENTITY_KEY is not a valid key: %w", err)
		}
		return priv, "FT_IDENTITY_KEY", nil
	}

	if data, err := os.ReadFile(path); err == nil {
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, "", fmt.Errorf("%s is not a valid key: %w", path, err)
		}
		return priv, path, nil
	}

	priv, _, err := crypto.GenerateEd25519Key(nil) // nil -> uses crypto/rand
	if err != nil {
		return nil, "", err
	}
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, "", err
	}
	// 0600: only the owner may read the key.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, "", err
	}
	return priv, path + " (newly generated)", nil
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
