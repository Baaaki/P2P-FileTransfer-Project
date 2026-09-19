# PureSend 📦

End-to-end P2P file transfer built on [libp2p](https://libp2p.io). Two
computers find each other through a small rendezvous server, then the
files move **directly between them, never touching the server** — even
when both sides sit behind home routers (NAT).

The user never sees anything technical: open the app, pick a file, read
the **three-word code** out to a friend. That's it.

🇹🇷 Türkçe dokümantasyon: [README.md](README.md)  
🌐 Website & Live Simulation: [puresend.madebybaki.com](https://puresend.madebybaki.com)  
📋 Productisation plan: [ROADMAP.md](docs/ROADMAP.md)

```
Sender (Istanbul)          Rendezvous server           Receiver (Izmir)
      │                            │                          │
      │ 1. open room               │                          │
      │    "kiraz-liman-42" ──────►│◄─── 2. who is in          │
      │                            │        "kiraz-liman-42"?  │
      │                            │                          │
      │◄═══ 3. Direct P2P connection (hole punching) ════════►│
      │              4. Files flow directly                   │
```

## Download and run

No installer, no configuration, no account. Grab the file for your system
from [**Releases**](https://github.com/Baaaki/PureSend/releases/latest),
unpack it, run it.

| Your system | File to download |
|---|---|
| Ubuntu / Debian / Mint (`.deb`) | `puresend_<version>_amd64.deb` (ARM: `arm64.deb`) |
| Linux (Portable binary) | `puresend_<version>_linux_x86_64.tar.gz` (ARM: `arm64.tar.gz`) |
| Windows | `puresend_<version>_windows_x86_64.zip` (ARM: `arm64.zip`) |
| macOS (M1 / M2 / M3 / M4) | `puresend_<version>_macOS_arm64.tar.gz` |
| macOS (pre-2020, Intel) | `puresend_<version>_macOS_x86_64.tar.gz` |

- **Ubuntu / Debian / Mint (`.deb`):** The easiest setup. Install via double-click or terminal:
  ```bash
  sudo apt install ./puresend_<version>_amd64.deb
  ```
  Adds PureSend to your application menu with its launcher icon, and installs `/usr/bin/puresend` to your system path.
- **Windows:** double-click the executable inside the `.zip`.
- **macOS:** double-click the binary inside `.tar.gz` — it opens in a Terminal window.
- **Linux (Portable):** double-clicking the binary from your file manager automatically launches a terminal window (GNOME Terminal, Konsole, XFCE Terminal, etc.). Alternatively, run it from a shell:
  `chmod +x puresend && ./puresend`

### If your system warns you on first launch

The binaries are **not code-signed** (signing certificates cost money), so
the OS asks once. It is not an error, and it does not recur after you
allow it.

**macOS** — if you get *"cannot be opened because the developer cannot be
verified"*, **right-click → Open**, then **Open** again in the dialog.
Plain double-clicking will not offer that choice; the right-click is what
matters. Or, from a terminal:

```bash
xattr -d com.apple.quarantine puresend
```

**Windows** — on the *"Windows protected your PC"* screen, click
**More info** → **Run anyway**.

**Linux** — you may need to mark it executable:

```bash
chmod +x puresend
```

To verify the download, every release ships a `checksums.txt`; put it
next to the archive you downloaded:

```bash
# Linux
sha256sum -c checksums.txt --ignore-missing

# macOS — there is no sha256sum; shasum does the same job
grep macOS checksums.txt | shasum -a 256 -c

# Windows (PowerShell) — compare with the line in checksums.txt
Get-FileHash .\puresend_*_windows_x86_64.zip -Algorithm SHA256
```

## Which directory runs where?

| Directory | Runs on | Shipped as |
|---|---|---|
| **`cmd/server/`** | 🖥️ your Ubuntu box, 24/7 | Docker behind Cloudflare Tunnel |
| **`cmd/client/`** | 💻 the user's desktop | GitHub Releases (single binary or `.deb`) |
| `internal/rendezvous/` | both | shared protocol (`/puresend/rendezvous/1.1.0`) |
| `internal/transfer/` | client only | file transfer, PAKE auth, resume |
| `internal/p2p/`, `internal/tui/` | client only | network layer + Bubble Tea interface |
| `internal/safetext/` | client only | ANSI escape sequence & bidi injection sanitizer |
| `internal/headless/` | client only | headless CLI mode (`-send`, `-receive`) |
| **`LandingPage/`** | 🌐 web (Vercel/CDN) | React + Vite + Tailwind v4 (showcase & live TUI) |
| `packaging/` | Linux desktop | `.desktop` launcher, SVG icon, `.deb` package specs |
| `deploy/` | 🖥️ server | cloudflared config (compose lives at the root) |
| `scripts/` | build / packaging | local `.deb` builder script (`build-deb.sh`) |

## What the user actually does

**Sending:** open the app → *"I want to send files"* → pick files → `s` →
a code appears (`kiraz-liman-42`) → send it over WhatsApp → done.

**Receiving:** open the app → *"Someone is sending me files"* → type the
code → review the incoming list → *"Yes, download"*. Case, Turkish
letters and spaces instead of hyphens do not matter (`KİRAZ liman 42`
works), and a code of the wrong shape, or with a word codes never use, is
caught before the server is asked.

Codes are two words from a list of 256 plain Turkish words and a number
from 10 to 99 — about 5.9 million combinations. The interface is Turkish
and so are the codes: they are read out over the phone, and "kiraz" is
easier to spell to a Turkish speaker than "glacier".

No IP addresses, no ports, no config files at any point.

## How it works

1. **Rendezvous** — the sender registers a random room code together with
   its network addresses. The receiver enters the same code and gets them.
2. **Hole punching** — the first connection is bridged by the server's
   **Circuit Relay v2**; libp2p's **DCUtR** then upgrades it to a direct
   connection. The relay only serves peers holding an active room (ACL).
3. **Transfer** — the receiver reviews and **approves** the incoming file
   list, then the bytes flow over the direct connection with a **SHA-256
   check per file**. If hole punching fails, the relay carries the
   transfer as a bounded fallback. An interrupted transfer resumes where
   it stopped, without fetching finished files again.
4. **Staying reachable** — the sender reads its files in the background
   while the code is on screen, and if the connection to the meeting
   point drops (the tunnel restarted, the server was redeployed) it
   reconnects and puts the same code back.

### Why WebSocket behind Cloudflare Tunnel?

`cloudflared` only exposes **HTTP/WebSocket** publicly. A raw TCP ingress
requires `cloudflared access` on the other end too, and UDP/QUIC cannot be
exposed at all. So the server speaks libp2p over **WebSocket transport**,
which passes through a normal HTTP ingress:

```
client ──wss://...:443──► Cloudflare ──ws://localhost:8080──► server
                          (TLS terminates here)
```

The tunnel only carries rendezvous and hole-punch coordination (a few KB).
Once the direct connection is up, Cloudflare never sees the files.

## Running the server

```bash
PUBLIC_HOST=rendezvous.example.com \
  docker compose up -d

docker compose logs rendezvous   # grab the Peer ID
curl localhost:8081/health
curl -s localhost:8081/metrics | grep -E '^(puresend|libp2p_relaysvc)_'
```

> ⚠️ Never delete the `rendezvous-key` volume. If the peer ID changes,
> every client you already released stops working. Keep a copy of
> `base64 -w0 /data/server.key` off the box; `FT_IDENTITY_KEY` restores
> the same identity anywhere.

The health and metrics endpoint binds to `127.0.0.1:8081` by default
(`-health-addr`); the image opens it inside the container, and the
compose file maps it to the host's loopback only.

`rate(libp2p_relaysvc_data_transferred_bytes_total[1h])` is the number to
watch: hole punch coordination moves a few kilobytes through the relay,
so anything measured in megabytes is transfers that fell back to it.

**Behind a tunnel, every client has the same address.** libp2p allows an
address 8 connections, 8 relay reservations and a trickle of new
connections; behind cloudflared or Docker's port proxy that would be 8
for the whole world. Connections from `-trusted-proxies` (loopback and
private networks by default) are not limited per address, and the relay
holds as many reservations as there can be rooms. Per-client limits
belong to the tunnel, which can see real addresses: a Cloudflare rate
limiting rule on the hostname — IP, 20 requests per 10 seconds, block —
is available on the free plan and caps how fast anyone can guess codes.

Cloudflare Tunnel setup and the reasoning behind it:
[deploy/cloudflared-config.yml](deploy/cloudflared-config.yml)

## Releasing the client

The server address is baked in at build time, so the binary needs no
configuration. Set the `FT_SERVER` repository variable to
`/dns4/<host>/tcp/443/tls/ws/p2p/<PeerID>`, then:

```bash
git tag v0.1.0 && git push --tags
```

The release workflow runs vet and the whole test suite first, and
installs `syft` for the SBOMs.

Released clients also carry the address of a server list — by default
the landing page's [`server.txt`](LandingPage/public/server.txt), or
`FT_SERVER_LIST` if set. A client reads it only when none of its
built-in addresses answers, so if the server ever moves or changes
identity, adding the new address there keeps every copy already
downloaded working. Add the server's address to it once it is deployed.

[GoReleaser](.goreleaser.yaml) produces six binaries (Linux / macOS /
Windows × x86_64 / arm64).

## Development

```bash
make            # list targets
make test       # full test suite with race detector
make lint       # golangci-lint (same version as CI)
make vuln       # govulncheck for reachable vulnerabilities
make cover      # coverage report
make test-relay # test relay fallback between isolated namespaces
make deb        # build Debian/Ubuntu .deb package
```

Three terminals on one machine:

```bash
go run ./cmd/server -ws-port 8080
go run ./cmd/client -server /ip4/127.0.0.1/tcp/8080/ws/p2p/<PeerID>
```

### Headless mode

For scripts, machines without a terminal, and CI. The room code goes to
stdout on its own line; everything else goes to stderr.

```bash
puresend -send holiday/            # prints a code, waits
puresend -receive kiraz-liman-42 -out /mnt/disk -yes
puresend -version
```

## Technology and Mechanisms

- **Go 1.26+**, **go-libp2p v0.49** — TCP + QUIC + WebSocket transports,
  Circuit Relay v2 (ACL and quota protected), DCUtR hole punching (NAT traversal),
  AutoNAT v2, UPnP port mapping, Noise/TLS encryption, and Resource Manager
- **Bubble Tea + Lipgloss + Bubbles** — Terminal user interface (TUI) with smooth transfer speed and ETA calculation
- **schollz/pake/v3** — Room code key exchange (SPAKE2 over P-256), HMAC proof and mutual Peer ID binding (MITM and rogue server immunity)
- **Prometheus client_golang v1.24.1** — Server `/metrics` (relay bytes, active rooms, rate limits) and `/health` JSON endpoint
- **Landing Page** — React 19, Vite, Tailwind CSS v4, TypeScript, Oxlint; in-browser live TUI simulation and GitHub Release API integration
- **Packaging & Desktop Integration** — GoReleaser v2, Syft (SBOM), Debian/Ubuntu `.deb` generation (`dpkg-deb` / `nfpm`), FreeDesktop desktop launcher (`.desktop`), and SVG application icon
- **Sanitization & Security (`internal/safetext`)** — Neutralising terminal injection attacks (ANSI escape sequences, control characters, and Unicode bidi overrides), plus strict Path Traversal and illegal Windows filename enforcement
- **Partial Resumption (Resume)** — Seamless recovery of interrupted downloads via `.part` files verified by per-file SHA-256 digests
- Two custom protocols: `/puresend/rendezvous/1.1.0` and
  `/puresend/transfer/2.0.0`

## Security model in one paragraph

The room code is a password, not just a lookup key: both ends derive a
shared key from it and prove they hold it before a file list is exchanged.
The code never crosses the wire, and both peer IDs are bound into the
exchange. So the **rendezvous server is not a trusted party** — it is what
tells the receiver who the sender is, and a peer of its own would fail the
handshake. A code works exactly once: the sender retires it the moment the
transfer completes. See [SECURITY.md](docs/SECURITY.md).

## Limitations

- **The relay fallback is capped at 256 MB per connection**
  (`-relay-data`). A larger transfer between two peers that cannot open a
  direct route will break off — the receiver is warned before it starts,
  and a retry resumes where it stopped.
- **Empty folders do not survive the trip**: the protocol moves files, and
  the tree is rebuilt from their relative paths.
- **Symbolic links are skipped**, not followed — one pointing outside a
  chosen folder would quietly widen what you agreed to send.
- **At most 5000 files per transfer.**
- **Names Windows cannot store are refused** on a Windows receiver
  (`? * < > | "`, device names like `CON` or `NUL`), before the transfer
  starts. Names with control characters are refused everywhere.
- **The server sees metadata**: who meets whom, and when. Not the files, and
  not the room code.
- **Binaries are not code-signed**; your OS may warn on first launch.
