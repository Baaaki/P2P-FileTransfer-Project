# Contributing to PureSend

Thanks for your interest in contributing! PureSend is a secure, serverless-feel peer-to-peer file transfer tool designed for people who are not technical and should never have to be. Most architectural rules and design decisions follow directly from that goal.

This guide covers how to report issues, run the project locally, and submit clean, well-tested pull requests.

---

## Before you open an issue

Please categorize your issue using the standard prefixes below so it is clear and easy to triage:

| Category      | Title prefix    | Label           | Description |
| ------------- | --------------- | --------------- | ----------- |
| Bug           | `[Bug]`         | `bug`           | Something is broken or behaving unexpectedly |
| Feature       | `[Feature]`     | `enhancement`   | A proposal for a new capability |
| Improvement   | `[Improvement]` | `enhancement`   | Refining an existing feature, UI flow, or performance |
| Documentation | `[Docs]`        | `documentation` | Fixes or improvements to guides, docstrings, or specs |
| Security      | `[Security]`    | `security`      | Non-critical security inquiries (see note below) |
| Question      | `[Question]`    | `question`      | Inquiries about usage, architecture, or design choices |

> [!CAUTION]
> **Reporting Security Vulnerabilities:** Do **not** open public issues for sensitive security bugs. Please report them privately via [GitHub Security Advisories](https://github.com/Baaaki/PureSend/security/advisories/new) as described in [SECURITY.md](SECURITY.md).

---

## Before you open a pull request

**Bug fixes, documentation improvements, test additions, and small self-contained fixes** are always welcome as direct pull requests — no need to ask beforehand.

**New features, protocol changes (`/puresend/...`), new third-party dependencies, UI flow alterations, or architectural shifts** should start as an **issue** first. Because PureSend is maintained by an individual developer, discussing substantial changes beforehand ensures effort isn't wasted on architectural directions that might be difficult to merge or maintain.

The recommended workflow:
1. Open an issue describing the problem, proposed solution, and architectural approach.
2. Discuss and align on direction.
3. Implement the changes and link the issue in your pull request.

### Pull request quality bar

To keep code review manageable for a solo maintainer and keep the codebase reliable, please follow these guidelines:

- **One change per PR:** One focused bug fix, or one agreed feature. Do not bundle unrelated changes.
- **Scope the diff:** Touch only the files your change strictly requires. Do **not** reformat unrelated files or fix unrelated linter warnings across lines you did not modify. Run `make fmt` and inspect `git diff` before opening.
- **Explain the why:** Describe what was broken or what feature was added, why the particular approach was chosen, and how you verified it (including commands run and before/after behavior).
- **Prove it with tests:** Bug fixes should include a test that reproduces the problem and verifies the fix.
- **Quality over test volume:** One deterministic test that catches a real regression is worth twenty that cannot fail. Avoid tests that simply assert obvious language features.
- **Verify before opening:** Ensure `make fmt`, `make lint`, `make vuln`, and `make test` pass locally before submitting.

### Using AI assistants

AI assistance is completely welcome, but please ensure **you** understand and stand behind every line of submitted code:

- **Understand your diff:** Ensure you can explain why each line exists and its concurrency or error-handling implications.
- **Verify against a real peer:** Actually compile and test changes locally with `make dev` or `make test`. Do not submit untested abstractions.
- **Respect project conventions:** Keep error messages friendly and free of raw technical jargon, keep network deadlines bounded, and preserve streaming `.part` file writing.
- **Keep diffs minimal:** Avoid unsolicited mass reformats or speculative abstractions.

---

## Prerequisites

To build and test PureSend, you need:
- **Go**: Version `1.27` or newer (as declared in `go.mod`).
- **Linux / macOS / Windows**: Go code compiles natively on all three.
- **Docker** (optional): Only needed if you are building the production server container image.
- **Linux `unshare` utility** (optional): Needed if you wish to run `make test-relay` (isolated network namespace tests).

All linters (`golangci-lint`) and vulnerability checkers (`govulncheck`) are pinned and invoked through `go run` inside the `Makefile`, so you do not need to install them globally.

---

## Development Setup

Clone the repository and inspect the available Make targets:

```bash
git clone https://github.com/Baaaki/PureSend.git
cd PureSend
make help           # Lists all available targets and descriptions
make build          # Builds both client and server binaries into ./bin/
```

### Trying it locally (One Machine, Three Terminals)

PureSend can easily be tested locally using its built-in development server:

**Terminal 1 — Start the local rendezvous & relay server:**
```bash
make dev
```
This builds and runs `cmd/server` on `127.0.0.1:4001` with a temporary identity key and prints the client address, for example:
```
/ip4/127.0.0.1/tcp/4001/p2p/12D3KooW...
```

**Terminal 2 — Sender client:**
```bash
./bin/puresend -server /ip4/127.0.0.1/tcp/4001/p2p/<ServerPeerID>
```
Select "Send", pick a file, and note the generated 3-word room code (e.g. `kiraz-liman-42`).

**Terminal 3 — Receiver client:**
```bash
./bin/puresend -server /ip4/127.0.0.1/tcp/4001/p2p/<ServerPeerID>
```
Select "Receive", enter the code, approve the transfer manifest, and watch the transfer complete.

---

## Project Structure

```
cmd/
  client/         → Desktop client entry point (starts TUI or headless mode)
  server/         → Rendezvous & Circuit Relay v2 daemon

internal/
  headless/       → Non-interactive CLI runner (-send, -receive, -yes) for scripts & CI
  i18n/           → Locale detection and localized UI strings
  integration/    → End-to-end integration tests (headless mode, single-use rooms, resilience)
  p2p/            → libp2p node orchestration, DCUtR hole punching, STUN, server list failover
  rendezvous/     → Meeting point wire protocol, room registry, rate limiting, and PAKE lookup
  safetext/       → Terminal sanitizer stripping dangerous ANSI escape sequences
  transfer/       → Transfer protocol (/puresend/transfer/2.0.0), PAKE2 auth, chunking, resume, SHA-256
  tui/            → Terminal User Interface (Bubble Tea, Lip Gloss, Bubbles), rate calculation
  update/         → Automatic update checker against GitHub Releases

deploy/           → Cloudflare Tunnel configuration templates
packaging/        → Desktop entries, application icons, and Arch PKGBUILD
scripts/          → Helper build scripts (Debian .deb packager)
test/
  relay/          → Network-namespace script testing relay fallback across isolated networks
```

---

## The Non-Negotiable Rules

PureSend has a small set of foundational principles that must never be compromised:

### 1. No technical jargon on screen
The end user should never see words like `"multiaddr"`, `"peer"`, `"NAT"`, `"relay"`, `"goroutine"`, or raw Go runtime errors on screen.
- All user-facing errors must pass through `explain()` in `internal/tui/tui.go`.
- If a new error case can reach the user, add a descriptive branch explaining what happened in plain, friendly language and offering clear, actionable next steps.
- The `default` branch returning raw errors is an absolute last resort.

### 2. Nothing on disk that looks finished but is not
Files must never be written directly to their final destination name.
- Incoming data is streamed into a `.part` file.
- The SHA-256 hash is computed in-flight.
- The file is only renamed to its final target path once the full digest matches the manifest.
- Interrupted downloads remain marked as partial until resumed or swept.

### 3. The other peer is not trusted
The receiving client must assume the sending peer might be compromised, buggy, or malicious:
- Validate every file name, path, digest, and size in the manifest before creating files or showing them on the approval screen.
- Path traversal sequences (`../`, absolute paths, forbidden characters, control characters) must be rejected outright, not quietly sanitized.
- Every incoming text that reaches the terminal must pass through `internal/safetext`.
- Every network wait must have a strict deadline or timeout.

### 4. The rendezvous server is not trusted either
The meeting point server coordinates connections and relays traffic if hole punching fails, but it must never be trusted with secrets:
- Peer authentication is handled end-to-end via PAKE2 over the room code.
- The server never learns the room code or session keys.
- Both peer IDs are bound into the key exchange; a malicious server cannot impersonate either party or intercept file payloads.

---

## Coding Style & Conventions

- **Standard Go:** Write idiomatic Go formatted strictly with `gofmt`.
- **Comments explain the *why*, not the *what*:** Explain non-obvious design choices, protocol quirks, or reasons why a simpler approach failed (e.g., buffering nuances in stream decoders).
- **Truthful comments:** Keep documentation and comments in sync with code modifications. A misleading comment is worse than no comment.
- **Git commits:** Follow [Conventional Commits](https://www.conventionalcommits.org/):
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation updates
  - `test:` for test additions or refactoring
  - `refactor:` for code restructuring without behavior changes
  - `ci:` for CI/CD workflow adjustments
  - `chore:` for maintenance or dependency bumps
- Include a concise subject line, and provide a descriptive body whenever the rationale behind the diff is not self-evident.

---

## Testing

PureSend maintains a rigorous testing regime with race detection enabled across all suites:

```bash
make test         # Runs all unit and integration tests with -race
make test-short   # Fast test suite (skips heavy integration tests and network operations)
make cover        # Runs tests with coverage profiling and prints package coverage
```

### Isolated Relay Fallback Test (`make test-relay`)

Standard loopback tests always succeed in direct communication. The only way to prove that the Circuit Relay v2 fallback works is to simulate two machines that are mathematically incapable of reaching each other directly:

```bash
make test-relay
```
This runs `./test/relay/netns-relay-test.sh` inside unprivileged Linux network namespaces (`unshare -Urnm`). It configures isolated virtual network interfaces where clients can only talk to the server and never directly to each other, verifying that relayed transfers complete with identical SHA-256 digests.

### Headless Mode Testing

For automated end-to-end testing, use the headless client flags instead of driving pseudo-terminals:
```bash
./bin/puresend -server <addr> -send <file-or-folder>
./bin/puresend -server <addr> -receive <code> -yes -out <destination-dir>
```

### Prove the test can fail

A test that cannot fail proves nothing. When fixing a bug, first write a test reproducing the failure, verify that it fails on clean `main`, apply your fix, and verify that it passes. Mention this verification explicitly in your pull request.

---

## Pre-PR Verification Checklist

Before opening a pull request, run the full verification pipeline locally:

```bash
make fmt       # Enforces standard gofmt formatting
make vet       # Runs go vet static analysis
make lint      # Runs golangci-lint (matching CI version)
make vuln      # Scans for reachable vulnerabilities via govulncheck
make test      # Runs full test suite with race detector enabled
```

If your changes affect `internal/p2p`, `internal/rendezvous`, or `cmd/server`, also run:
```bash
make test-relay
```

---

## Need Help?

If you have questions, run into setup issues, or want to discuss an architectural proposal before opening an issue, feel free to open a [GitHub Discussion](https://github.com/Baaaki/PureSend/discussions) or submit a `[Question]` issue. Feedback and ideas are always welcome!
