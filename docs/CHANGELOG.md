# Changelog

Notable changes to FileTransferilla. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

This release works through the findings of the end-to-end test report
(`TEST-RAPORU.md`, 7 August 2026) — every issue it raised and every
improvement it suggested — and then through a pre-release security and
operations review of the result.

### Security

- **The room code is now a password, not just a lookup key.** Both ends run
  a password-authenticated key exchange (PAKE2 over P-256) over the code and
  prove the derived key to each other before a manifest is sent. The code
  never crosses the wire, a wrong one fails before the file list is
  revealed, and guessing costs a full connection each time — there is no
  offline attack to speed the search up.
- **The rendezvous server is no longer a trusted party.** It is the server
  that tells the receiver who the sender is; if it names a peer of its own,
  that peer cannot complete the handshake. Both peer IDs are bound into the
  exchange, so a peer in the middle cannot relay the two ends' messages to
  each other either.
- **Room codes are single-use.** When a transfer completes, the sender drops
  the transfer protocol handler and unregisters the room. Previously the
  room stayed live until the user pressed a key, so anyone else who had been
  told the code could download the files again — silently, with the sender's
  screen still saying "sent". A *failed* attempt still leaves the code
  usable, so a flaky link does not cost a phone call.
- **Rooms are released as soon as they are done with**, rather than sitting
  in the server's table until their one-hour expiry.
- **Keeping the room table for real users.** One room per sender
  (`-rooms-per-peer`, default 1 — the program never opens more), a
  server-wide budget of new rooms per minute (`-register-budget`), room
  codes held to their exact shape (a room name used to be up to 16 KB of
  anything), and a room whose owner disconnects is dropped after a minute
  instead of holding its slot for the rest of the hour.
- **A server-wide limit on failed lookups** closes the hole in the per-peer
  one: a client mints a fresh identity on every start, so an attacker
  willing to reconnect could reset its own counter. It does not refuse
  everyone once spent — a first draft did, which let a few random guesses a
  second lock every real user out. Under pressure a peer gets one guess;
  someone typing the right code first time is never refused. Every refusal
  is decided before the room is looked at, so it never reveals whether a
  guess hit.
- **Manifests are validated in full before anything is used.** The digest
  names the partial download a resume continues from, and was not checked:
  a sender could put a path in it and have the receiver create, truncate
  or delete a `.part` file outside the target folder. Digests must now be
  64 lowercase hex characters, sizes non-negative, totals must not
  overflow.
- **File names with control characters or bidirectional marks are
  refused.** They were printed on the approval screen, where an escape
  sequence could repaint the list being approved. On Windows, names
  Windows cannot store (`? * < > | "`, `CON`, `NUL`, a trailing dot) are
  refused before the transfer instead of failing at the final rename.
- **Text from the other side is cleaned before it is shown.** Error
  messages from the other peer and the server could carry escape
  sequences into the terminal.
- **Every wait has an end.** The handshake has 30 seconds, the answer to
  the file list five minutes, a stalled transfer two minutes. And the room
  is claimed only *after* the handshake: someone who connected and went
  quiet used to hold the sender's one slot indefinitely, turning the real
  receiver away with a reset connection.
- **The health and metrics endpoint listens on 127.0.0.1** unless told
  otherwise (`-health-addr`, replacing `-health-port`). A binary started
  by hand used to publish `/metrics` to the whole LAN.
- **Dependencies with reachable vulnerabilities upgraded**: go-libp2p
  v0.49.0 (quic-go v0.60.0, webtransport-go v0.11.1) and pion/dtls v3.1.4.
  `govulncheck` now reports none the code can reach.
- **Manifest paths are validated before anything is written**, and anything
  that could escape the target folder is refused outright rather than
  quietly repaired.
- The server image now runs as a non-root user.

### Added

- **The sender survives the meeting point going away.** A cloudflared
  restart or a redeploy used to take the relay slot and the room with it
  while the sender sat on "waiting" forever and its friend was told the
  code did not exist. The sender now notices, says so on screen,
  reconnects with backoff and puts the same code back — for as long as the
  code's hour lasts.
- **A server list for when the server moves.** Released clients carry the
  address of a small text file (the landing page's `server.txt`, or
  `-server-list` / `FT_SERVER_LIST`) that they read only when none of their
  built-in addresses answers. It is the insurance for every copy already
  downloaded if the server's address or identity ever changes.
- **Files are read while the code is on screen.** The sender used to
  compute every digest after the receiver connected — on every attempt —
  while the receiver stared at "preparing". Now it starts the moment the
  code is shown, re-reads only files that changed since, and keeps an
  early receiver informed of its progress.
- **A clear "busy".** A second receiver with the right code, arriving while
  a transfer runs, is told the room is taken instead of getting a reset
  connection.
- **Room codes in Turkish.** 256 plain-letter Turkish words, none one letter
  from another or the start of another, so "kiraz-liman-42" is what the
  code looks like — as the interface always claimed. 5.9 million codes,
  up from 1.7 million.
- **Codes are forgiving to type.** Case, Turkish letters and spaces are
  normalized; a malformed code or an unknown word is pointed out on the
  code screen, before the server is asked and a miss is counted.
- **libp2p's own metrics on `/metrics`** — the relay service, connection
  limits and transports. They were recorded all along, into a registry
  nobody served. `libp2p_relaysvc_data_transferred_bytes_total` answers
  how much traffic falls back to the relay; a comment used to promise a
  `filetransferilla_relayed_transfers` metric that never existed. New
  counters for abandoned rooms and throttled registrations.
- **A `HEALTHCHECK` in the image**, and a hardened compose file:
  read-only root filesystem, all capabilities dropped,
  `no-new-privileges`, memory and process limits, log rotation.
- **`make vuln`**, and a vulnerability check in CI.
- **A CI workflow for the landing page** (lint, build, check that
  `server.txt` ships), with an optional Vercel deploy job.

- **Folder transfer.** A chosen folder is walked and its structure rebuilt
  on the other side. Symbolic links are skipped rather than followed.
- **Resume.** An interrupted download is kept, keyed by the digest of the
  file it belongs to, and the next attempt continues from where it stopped
  instead of starting over. Abandoned partials are swept after a week.
- **Transfer speed and remaining time**, smoothed over a short window so the
  numbers are steady enough to read, and correct when a transfer resumes
  partway through.
- **A choosable download folder** — from the main menu, from the code-entry
  screen with `Ctrl+O`, or with `-out`.
- **Headless mode**: `-send`, `-receive`, `-yes`. The room code goes to
  stdout on its own line so a script can read it. This is also what lets the
  end-to-end tests drive the real binary instead of typing into a
  pseudo-terminal and scraping ANSI output.
- **`-version`** on both binaries, stamped at build time, and the server's
  version in `/health`.
- **Prometheus metrics** at `/metrics`: active rooms, rooms opened, closed
  and expired, lookups found, missed and throttled, plus the Go runtime and
  process collectors.
- **Several meeting point addresses** may be given, comma-separated, and are
  tried in order — insurance against the single address baked into every
  released client going away.
- **A relay size warning.** When the direct route could not be opened and
  the transfer is larger than the relay will carry, the receiver is told
  before it starts rather than after it breaks.
- **A CI job for the relay fallback.** Two peers in isolated network
  namespaces that can each reach the server but never each other, so hole
  punching is impossible and the relay path *must* work. It needs no
  privileges, so it runs on every commit.
- Linting (`golangci-lint`), race-detector test runs, coverage reporting,
  and a Docker image smoke test in CI. A `Makefile` for all of it.

### Changed

- **The server is sized for running behind a tunnel.** libp2p allows an
  address 8 connections, 8 relay reservations and a trickle of new
  connections, and exempts only loopback. Behind cloudflared — and through
  Docker's bridge, which is not loopback — every client shares one
  address, so the ninth concurrent sender was refused a relay slot and the
  ninth connection refused outright. Connections from `-trusted-proxies`
  are no longer limited per address, the relay holds a reservation for
  every room the server can hold, and the server-wide connection ceiling
  is raised to match.
- **The headless sender keeps the room open after a failed attempt**, as
  the interface does. It used to exit, taking the code with it.
- **A retried transfer does not fetch finished files again.** They used to
  arrive a second time as `name (1).ext`.
- **One Go version throughout**: `go.mod` (now `go 1.26`; Go 1.25 is out of
  support), the Dockerfile, CI and the release build.
- **`make lint` works on any Go.** The linters run through `go run` at the
  versions CI pins; a `golangci-lint` built by an older Go fails on a newer
  standard library with errors that look like the project's.

- **The wait for a direct connection adapts.** It used to be a flat 20
  seconds whatever happened; now DCUtR's own report that it has given up
  ends the wait early, which leaves room for a longer backstop (30s) for the
  slow mobile link where hole punching does eventually succeed.
- **Progress carries the whole transfer's totals**, not just the current
  file's, so a folder of a thousand files shows one honest bar.
- **The health endpoint is fatal when its port is taken.** It used to log a
  line and carry on, leaving a server that looked alive to its supervisor
  and dead to its health check, forever.
- **In the file picker**, backspace goes back up a folder again; `x` removes
  the last pick. Intercepting backspace left the user stuck in a directory
  as soon as they had chosen anything.
- Protocol versions: `/filetransferilla/rendezvous/1.1.0` (adds
  `unregister` and the relay limit in a lookup response) and
  `/filetransferilla/transfer/2.0.0` (adds the handshake, folder paths and
  resume). No compatibility is kept with the earlier versions; none were
  ever released.

### Fixed

- **Resuming a folder that holds the same file twice silently corrupted
  the second copy.** Both copies shared one partial download; the first
  finished and took it away, and the second continued "from where it
  stopped" in a fresh file padded with zeroes — and passed its checksum,
  because the digest state had been computed from the old leftovers. Each
  copy now has a partial of its own.
- **The first release would have failed**: GoReleaser needs `syft` for the
  SBOMs and the release workflow did not install it. It also published
  without running a single test; now it runs vet and the whole suite
  first.
- **The relay CI job depended on a warm module cache**: it built the
  binaries inside a network namespace with no network. They are built
  before entering it now, and the job lifts Ubuntu 24.04's AppArmor
  restriction on unprivileged user namespaces where it applies. The test
  also checks the relay's metrics saw the transfer.
- `.dockerignore` now excludes the landing page and `node_modules`, which
  bloated every build context after an `npm install`.

- **Technical error text no longer reaches the screen.** A dropped
  connection used to surface as `stream reset: stream reset: connection
  closed: unexp…`, against the project's own rule. `explain()` now covers
  dropped connections, mismatched codes, throttling, unsafe names, full
  servers, unreadable files and a full disk — each with something the user
  can actually do about it.
- The Prometheus registration would have panicked at startup: the Go
  collector already publishes a `version` label on `go_info`, and wrapping
  it with a second one is fatal. Caught by the new relay test.
