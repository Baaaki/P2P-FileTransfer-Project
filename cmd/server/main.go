// Server: the rendezvous point where peers find each other.
//
// It has two jobs:
//  1. Rendezvous: the sender registers a room code, the receiver looks it up.
//  2. Circuit Relay v2: when both sides are behind NAT, the first
//     connection is established through this server; libp2p's hole
//     punching mechanism (DCUtR) then uses that bridge to upgrade to a
//     direct connection.
//
// To work on the public internet, run it on any machine with an open
// port (e.g. a cheap VPS):
//
//	go run ./cmd/server -port 4001
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"puresend/internal/rendezvous"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

func main() {
	port := flag.Int("port", 4001, "TCP/UDP port to listen on")
	keyPath := flag.String("key", "server.key", "path of the identity key file")
	flag.Parse()

	// The server's identity (peer ID) must stay the same across restarts
	// so the address we hand to clients remains valid. We therefore save
	// the private key to disk and reload it on the next start.
	priv, err := loadOrCreateKey(*keyPath)
	if err != nil {
		log.Fatalf("could not prepare identity key: %v", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(priv),
		// Listen on both TCP and QUIC; clients use whichever works.
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", *port),
		),
		// The relay service normally waits until the node has verified
		// it is publicly reachable. The server runs on a public machine
		// anyway, so skip that wait.
		libp2p.ForceReachabilityPublic(),
		// Enable the Relay v2 service. The default limits (2 minutes /
		// 128 KB) are lifted so a transfer can still complete through
		// the relay when a direct connection cannot be established.
		libp2p.EnableRelayService(relay.WithInfiniteLimits()),
	)
	if err != nil {
		log.Fatalf("could not start libp2p host: %v", err)
	}
	defer h.Close()

	registry := rendezvous.NewRegistry()
	h.SetStreamHandler(rendezvous.ProtocolID, registry.Handler)

	fmt.Println("Rendezvous + relay server is running.")
	fmt.Println("Peer ID:", h.ID())
	fmt.Println("\nAddresses for clients (pass one to the -server flag):")
	for _, addr := range h.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, h.ID())
	}
	fmt.Println("\nPress Ctrl+C to stop.")

	// Keep running until Ctrl+C or SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down.")
}

// loadOrCreateKey loads the identity key from disk, or generates and
// saves a new Ed25519 key if none exists.
func loadOrCreateKey(path string) (crypto.PrivKey, error) {
	if data, err := os.ReadFile(path); err == nil {
		return crypto.UnmarshalPrivateKey(data)
	}

	priv, _, err := crypto.GenerateEd25519Key(nil) // nil -> uses crypto/rand
	if err != nil {
		return nil, err
	}
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	// 0600: only the owner may read the key.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	return priv, nil
}
