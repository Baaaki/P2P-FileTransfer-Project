# Contributing

Thanks for taking an interest. This is a small project with a specific
audience — people who are not technical and should never have to be — and
most of its rules follow from that.

## Getting set up

You need Go (the version in `go.mod`, or newer) and nothing else. Docker is
only for the server image. `make lint` and `make vuln` fetch and build
their tools themselves, with your Go, at the versions CI uses.

```bash
git clone https://github.com/Baaaki/P2P-FileTransfer-Project
cd P2P-FileTransfer-Project
make            # lists the targets
make test       # the whole suite, race detector on
```

To try it on one machine, three terminals:

```bash
go run ./cmd/server -ws-port 8080
# take the Peer ID from its output, then twice:
go run ./cmd/client -server /ip4/127.0.0.1/tcp/8080/ws/p2p/<PeerID>
```

## Before you open a pull request

```bash
make fmt lint vuln test
```

`make test-relay` is worth running too if you touched anything in
`internal/p2p`, `internal/rendezvous`, or the server. It is the only test
that exercises the relay fallback, because every other test runs on
loopback where the direct path always works.

## The rules that are not negotiable

**No technical terms on screen.** Not "multiaddr", not "peer", not "NAT",
not "relay", not a raw Go error. If a new failure can reach the user, add a
branch to `explain()` in `internal/tui/tui.go` saying what happened and what
they can do about it. The `default` branch printing the raw error is a
last resort, not a working fallback — if you see one in practice, that is a
missing case.

**Nothing on disk that looks finished but is not.** Files are written to a
`.part` file and only renamed once their digest matches.

**The other side is not trusted.** File names, sizes, digests, resume
offsets, error messages — all of it arrives from a peer you have no reason
to believe. Validate it, and prefer refusing to quietly repairing. Anything
from it that reaches the screen goes through `internal/safetext` first.
Every wait on it needs a deadline.

**The server is not trusted either.** It should never be in a position where
lying gets it anything. If a change would make the clients depend on it
being honest, that is worth a conversation before the code.

## Style

Match what is already there. The house style is ordinary Go, with comments
that explain *why* rather than restating the code — particularly where
something looks odd, because it usually looks odd for a reason someone
learned the hard way (see the note on `json.Decoder` in
`internal/transfer/transfer.go`).

Keep comments truthful as the code changes. A stale comment is worse than
none.

## Tests

New behaviour needs a test. Bug fixes need one that fails before the fix —
several tests here exist because a real bug got through, and their comments
say so.

- `internal/*/..._test.go` — unit tests, no network
- `internal/integration/` — real libp2p hosts on localhost, and the real
  client binary through its headless mode
- `test/relay/` — the isolated-networks relay test

Prefer a deterministic setup to a timing-dependent one. The resume
integration test writes the leftovers of an interrupted attempt directly,
because on loopback a transfer finishes long before a test could cut it.

## Commits and pull requests

Conventional-commit prefixes (`feat:`, `fix:`, `docs:`, `test:`, `ci:`,
`refactor:`), a short subject line, and a body when the *why* is not obvious
from the diff.

In the pull request, say what a user would notice. If nothing, say that too.
