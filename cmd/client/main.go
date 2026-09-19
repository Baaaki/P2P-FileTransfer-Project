// Client: the app people download and run.
//
// It normally takes no arguments at all — the meeting point's address is
// baked in at build time so a user can double-click the binary:
//
//	go build -ldflags "-X main.defaultServer=/dns4/rendezvous.example.com/tcp/443/tls/ws/p2p/12D3Koo..." ./cmd/client
//
// For local testing, point it somewhere else:
//
//	go run ./cmd/client -server /ip4/127.0.0.1/tcp/4001/p2p/12D3Koo...
//
// Releases also bake in the address of a server list (see
// internal/p2p/serverlist.go): if the meeting point ever moves, copies
// already downloaded find it there instead of dying with the old address.
//
// There is also a headless mode for scripts, servers without a terminal,
// and the end-to-end tests:
//
//	filetransferilla -send holiday.zip        # prints a room code
//	filetransferilla -receive kiraz-liman-42  # downloads into -out
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"filetransferilla/internal/headless"
	"filetransferilla/internal/p2p"
	"filetransferilla/internal/tui"
	"filetransferilla/internal/update"

	"github.com/charmbracelet/x/term"
)

// Build information, filled in at build time with -ldflags -X. Releases
// ship with all of it set; a plain `go build` leaves defaultServer empty
// and the -server flag becomes required.
var (
	defaultServer     = ""
	defaultServerList = ""
	version           = "dev"
	commit            = "unknown"
	date              = "unknown"
)

func main() {
	var (
		server = flag.String("server", envOr("FT_SERVER", defaultServer),
			"comma-separated multiaddrs of the meeting point, tried in order")
		serverList = flag.String("server-list", envOr("FT_SERVER_LIST", defaultServerList),
			"URL of a list of meeting point addresses, used when none of -server answers")
		out = flag.String("out", os.Getenv("FT_OUT"),
			"folder to save incoming files in (default: your downloads folder)")
		send        = flag.String("send", "", "headless: send these files or folders (comma-separated) and print a room code")
		receive     = flag.String("receive", "", "headless: download the given room code and exit")
		yes         = flag.Bool("yes", false, "headless: accept the incoming file list without asking")
		showVersion = flag.Bool("version", false, "print version information and exit")
		doUpdate    = flag.Bool("update", false, "check for updates and update filetransferilla to the latest release")
	)
	flag.Parse()

	if *doUpdate {
		if err := update.Apply(version, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Güncelleme hatası: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *showVersion {
		fmt.Printf("filetransferilla %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  built:   %s\n", date)
		fmt.Printf("  go:      %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		if defaultServer != "" {
			fmt.Printf("  server:  %s\n", defaultServer)
		}
		if defaultServerList != "" {
			fmt.Printf("  list:    %s\n", defaultServerList)
		}
		return
	}

	servers := p2p.SplitServers(*server)
	if len(servers) == 0 && *serverList == "" {
		fmt.Fprintln(os.Stderr, "Buluşma noktası adresi ayarlanmamış.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Bu ikili, sunucu adresi gömülmeden derlenmiş.")
		fmt.Fprintln(os.Stderr, "Adresi elle vererek çalıştırabilirsin:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  filetransferilla -server /dns4/<alan-adi>/tcp/443/tls/ws/p2p/<PeerID>")
		os.Exit(1)
	}

	if *send != "" && *receive != "" {
		fmt.Fprintln(os.Stderr, "-send ve -receive aynı anda kullanılamaz.")
		os.Exit(1)
	}

	outDir := *out
	if outDir == "" {
		outDir = tui.DefaultOutDir()
	}

	list := p2p.WithServerList(*serverList)
	switch {
	case *send != "":
		run(headless.Send(servers, splitList(*send), list))
	case *receive != "":
		run(headless.Receive(servers, *receive, outDir, *yes, list))
	default:
		maybeSpawnTerminal()
		run(tui.Run(tui.Config{Servers: servers, ServerList: *serverList, OutDir: *out, Version: version}))
	}
}

// maybeSpawnTerminal launches a terminal window if the user double-clicked
// the binary from a graphical desktop (Linux Mint Nemo, Ubuntu Nautilus, etc.).
func maybeSpawnTerminal() {
	if runtime.GOOS != "linux" {
		return
	}
	if os.Getenv("FT_IN_TERMINAL") != "" {
		return
	}
	// If DISPLAY and WAYLAND_DISPLAY are empty, we are on a purely headless server without desktop.
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}
	// If already running in a terminal emulator, do nothing.
	if term.IsTerminal(os.Stdin.Fd()) || term.IsTerminal(os.Stdout.Fd()) {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	// Supported terminal emulators on Linux desktop environments
	type termChoice struct {
		bin  string
		args []string
	}
	choices := []termChoice{
		{"x-terminal-emulator", []string{"-e"}},
		{"gnome-terminal", []string{"--"}},
		{"mate-terminal", []string{"-e"}},
		{"xfce4-terminal", []string{"-e"}},
		{"konsole", []string{"-e"}},
		{"alacritty", []string{"-e"}},
		{"kitty", nil},
		{"xterm", []string{"-e"}},
	}

	for _, c := range choices {
		path, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		var cmdArgs []string
		cmdArgs = append(cmdArgs, c.args...)
		cmdArgs = append(cmdArgs, exe)
		cmdArgs = append(cmdArgs, os.Args[1:]...)
		cmd := exec.Command(path, cmdArgs...)
		cmd.Env = append(os.Environ(), "FT_IN_TERMINAL=1")
		if err := cmd.Start(); err == nil {
			os.Exit(0)
		}
	}
}

func run(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hata:", err)
		os.Exit(1)
	}
}

// splitList parses a comma-separated list of paths.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
