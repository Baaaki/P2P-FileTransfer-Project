// Client: the app people download and run.
//
// It normally takes no arguments at all — the meeting point's address is
// baked in at build time so a user can double-click the binary:
//
//	go build -ldflags "-X main.defaultServer=/dns4/p2p-filetransfer.example.com/tcp/443/tls/ws/p2p/12D3Koo..." ./cmd/client
//
// For local testing, point it somewhere else:
//
//	go run ./cmd/client -server /ip4/127.0.0.1/tcp/4001/p2p/12D3Koo...
package main

import (
	"flag"
	"fmt"
	"os"

	"puresend/internal/tui"
)

// defaultServer is filled in at build time with -ldflags -X. Releases
// ship with it set; a plain `go build` leaves it empty and the -server
// flag becomes required.
var defaultServer = ""

func main() {
	server := flag.String("server", defaultServer,
		"multiaddr of the meeting point (rendezvous server)")
	flag.Parse()

	if *server == "" {
		fmt.Fprintln(os.Stderr, "Buluşma noktası adresi ayarlanmamış.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Bu ikili, sunucu adresi gömülmeden derlenmiş.")
		fmt.Fprintln(os.Stderr, "Adresi elle vererek çalıştırabilirsin:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  puresend -server /dns4/<alan-adi>/tcp/443/tls/ws/p2p/<PeerID>")
		os.Exit(1)
	}

	if err := tui.Run(*server); err != nil {
		fmt.Fprintln(os.Stderr, "Beklenmedik bir hata oldu:", err)
		os.Exit(1)
	}
}
