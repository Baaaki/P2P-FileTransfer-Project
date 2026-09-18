// Package headless drives a transfer without a terminal interface.
//
// It exists for three reasons. Scripts and servers with no terminal need a
// way in. End-to-end tests need one too — before this, the only way to
// exercise the real program was to drive the TUI through a pseudo-terminal
// and read room codes off the screen, which is as brittle as it sounds.
// And it keeps the promise the p2p package makes in its own doc comment:
// that the same logic works under a TUI, a CLI or a test.
//
// Output is deliberately plain and line-oriented, so a shell script can
// read it: the room code goes to stdout on its own line, everything else
// to stderr.
package headless

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"filetransferilla/internal/p2p"
	"filetransferilla/internal/transfer"
)

// Send publishes the given files and folders under a fresh room code,
// prints the code, and blocks until one transfer has completed.
//
// Like the interface, it keeps the room open when an attempt fails: a
// dropped connection is the receiver's cue to try the same code again,
// and a script that exited at the first hiccup would throw the code away
// with it. It gives up only when the code itself stops working.
func Send(servers []string, paths []string, opts ...p2p.Option) error {
	ctx, stop := signalContext()
	defer stop()

	node, err := p2p.New(ctx, servers, opts...)
	if err != nil {
		return err
	}
	defer node.Close() //nolint:errcheck // shutting down; nothing to recover

	room, err := node.Host(ctx, paths)
	if err != nil {
		return err
	}
	// stdout, alone on its line: this is the one piece of output another
	// program is meant to parse.
	fmt.Println(room)
	logf("waiting for the receiver, code %s", room)

	return pump(ctx, node, nil, true)
}

// Receive downloads the given room code into outDir. Unless autoAccept is
// set, the file list is printed and confirmed on the terminal first.
func Receive(servers []string, room, outDir string, autoAccept bool, opts ...p2p.Option) error {
	// A malformed code fails here, before anything touches the network.
	room, err := p2p.CheckCode(room)
	if err != nil {
		return err
	}

	ctx, stop := signalContext()
	defer stop()

	node, err := p2p.New(ctx, servers, opts...)
	if err != nil {
		return err
	}
	defer node.Close() //nolint:errcheck // shutting down; nothing to recover

	go node.Fetch(ctx, room, outDir)
	return pump(ctx, node, func(m transfer.Manifest) bool {
		logf("incoming: %d file(s), %s", len(m.Files), formatBytes(m.TotalSize()))
		for _, f := range m.Files {
			logf("  %s (%s)", f.Path, formatBytes(f.Size))
		}
		if autoAccept {
			return true
		}
		return askYesNo()
	}, false)
}

// pump consumes the node's events until the transfer ends, reporting
// progress on stderr. confirm answers the manifest question; a nil confirm
// declines, which is what a sender should do if it is ever asked. A host
// keeps going after a failed attempt, since its room is still open.
func pump(ctx context.Context, node *p2p.Node, confirm func(transfer.Manifest) bool, host bool) error {
	var lastPrinted, lastPrep time.Time

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-node.Done():
			return fmt.Errorf("stopped before the transfer finished")
		case ev := <-node.Events():
			switch e := ev.(type) {
			case p2p.StatusEvent:
				logf("%s", e.Text)

			case p2p.ConnectedEvent:
				if e.Direct {
					logf("connected directly")
				} else {
					logf("connected through the fallback relay (limit %s per connection)",
						formatBytes(e.RelayLimit))
				}

			case p2p.PreparingEvent:
				// A folder of thousands would otherwise print thousands of
				// lines; the first, the last and one a second is plenty.
				if e.Index == 1 || e.Index == e.Files || time.Since(lastPrep) >= time.Second {
					lastPrep = time.Now()
					logf("reading %s (%d/%d)", e.Name, e.Index, e.Files)
				}

			case p2p.PreparedEvent:
				logf("files ready")

			case p2p.RemotePreparingEvent:
				if time.Since(lastPrep) >= time.Second {
					lastPrep = time.Now()
					logf("the sender is still reading its files (%d/%d)", e.Done, e.Total)
				}

			case p2p.RejectedEvent:
				logf("someone tried a wrong code; nothing was shown to them")

			case p2p.ServerLostEvent:
				logf("lost the meeting point, reconnecting; the code stays the same")

			case p2p.ServerBackEvent:
				logf("reconnected, the code works again")

			case p2p.RoomLostEvent:
				return e.Err

			case p2p.ManifestEvent:
				ok := confirm != nil && confirm(e.Manifest)
				e.Reply <- ok
				if !ok {
					return fmt.Errorf("transfer declined")
				}

			case p2p.ProgressEvent:
				// One line per second is plenty for a log file.
				if time.Since(lastPrinted) < time.Second && e.OverallDone < e.OverallTotal {
					continue
				}
				lastPrinted = time.Now()
				logf("%s  %s / %s", e.Name,
					formatBytes(e.OverallDone), formatBytes(e.OverallTotal))

			case p2p.DoneEvent:
				if e.Err != nil && host {
					logf("attempt failed: %v", e.Err)
					logf("still waiting, the same code works again")
					continue
				}
				if e.Err != nil {
					return e.Err
				}
				for _, p := range e.Paths {
					logf("saved %s", p)
				}
				logf("done")
				return nil
			}
		}
	}
}

// signalContext returns a context cancelled by Ctrl+C or SIGTERM, so an
// interrupted transfer shuts the node down instead of being killed
// mid-write.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

func askYesNo() bool {
	fmt.Fprint(os.Stderr, "accept? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes", "e", "evet":
		return true
	}
	return false
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
