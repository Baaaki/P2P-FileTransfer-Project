# FileTransferilla 📦

End-to-end P2P file transfer built on [libp2p](https://libp2p.io). Two
computers find each other through a small rendezvous server, then the
files move **directly between them, never touching the server** — even
when both sides sit behind home routers (NAT).

The user never sees anything technical: open the app, pick a file, read
the **three-word code** out to a friend. That's it.

🇹🇷 Türkçe dokümantasyon: [README.md](README.md)
📋 Productisation plan: [ROADMAP.md](ROADMAP.md)

```
Sender (Istanbul)          Rendezvous server           Receiver (Izmir)
      │                            │                          │
      │ 1. open room               │                          │
      │    "cherry-harbor-42" ────►│◄─── 2. who is in          │
      │                            │        "cherry-harbor-42"?│
      │                            │                          │
      │◄═══ 3. Direct P2P connection (hole punching) ════════►│
      │              4. Files flow directly                   │
```

## Download and run

No installer, no configuration, no account. Grab the file for your system
from [**Releases**](https://github.com/Baaaki/P2P-FileTransfer-Project/releases/latest),
unpack it, run it.

| Your system | File to download |
|---|---|
| Windows | `filetransferilla_<version>_windows_x86_64.zip` |
| macOS (M1 / M2 / M3 / M4) | `filetransferilla_<version>_macOS_arm64.tar.gz` |
| macOS (pre-2020, Intel) | `filetransferilla_<version>_macOS_x86_64.tar.gz` |
| Linux | `filetransferilla_<version>_linux_x86_64.tar.gz` |

The archive holds a **single file** called `filetransferilla`. That is
the whole program.

- **Windows:** double-click it.
- **macOS:** double-click it — it opens in a Terminal window.
- **Linux:** desktop environments often refuse to double-click a terminal
  program, so run it from a shell:
  `chmod +x filetransferilla && ./filetransferilla`

### If your system warns you on first launch

The binaries are **not code-signed** (signing certificates cost money), so
the OS asks once. It is not an error, and it does not recur after you
allow it.

**macOS** — if you get *"cannot be opened because the developer cannot be
verified"*, **right-click → Open**, then **Open** again in the dialog.
Plain double-clicking will not offer that choice; the right-click is what
matters. Or, from a terminal:

```bash
xattr -d com.apple.quarantine filetransferilla
```

**Windows** — on the *"Windows protected your PC"* screen, click
**More info** → **Run anyway**.

**Linux** — you may need to mark it executable:

```bash
chmod +x filetransferilla
```

To verify the download, every release ships a `checksums.txt`:

```bash
sha256sum -c checksums.txt --ignore-missing
```

## Which directory runs where?

| Directory | Runs on | Shipped as |
|---|---|---|
| **`cmd/server/`** | 🖥️ your Ubuntu box, 24/7 | Docker behind Cloudflare Tunnel |
| **`cmd/client/`** | 💻 the user's desktop | single binary from GitHub Releases |
| `internal/rendezvous/` | both | shared protocol |
| `internal/transfer/` | client only | the server never runs this code |
| `internal/p2p/`, `internal/tui/` | client only | network layer + interface |
| `deploy/` | 🖥️ server | compose + cloudflared config |

## What the user actually does

**Sending:** open the app → *"I want to send files"* → pick files → `s` →
a code appears (`cherry-harbor-42`) → send it over WhatsApp → done.

**Receiving:** open the app → *"Someone is sending me files"* → type the
code → review the incoming list → *"Yes, download"*.

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
   transfer as a bounded fallback.

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
PUBLIC_HOST=p2p-filetransfer.example.com \
  docker compose -f deploy/docker-compose.yml up -d

docker logs filetransferilla   # grab the Peer ID
curl localhost:8081/health
```

> ⚠️ Never delete the `rendezvous-key` volume. If the peer ID changes,
> every client you already released stops working.

Cloudflare Tunnel setup and the reasoning behind it:
[deploy/cloudflared-config.yml](deploy/cloudflared-config.yml)

## Releasing the client

The server address is baked in at build time, so the binary needs no
configuration. Set the `FT_SERVER` repository variable to
`/dns4/<host>/tcp/443/tls/ws/p2p/<PeerID>`, then:

```bash
git tag v0.1.0 && git push --tags
```

[GoReleaser](.goreleaser.yaml) produces six binaries (Linux / macOS /
Windows × x86_64 / arm64).

## Development

```bash
go build ./...
go test ./...   # unit tests + real end-to-end tests over libp2p
```

Three terminals on one machine:

```bash
go run ./cmd/server -ws-port 8080 -health-port 8081
go run ./cmd/client -server /ip4/127.0.0.1/tcp/8080/ws/p2p/<PeerID>
```

## Technology

- **Go 1.25+**, **go-libp2p v0.48** — TCP + QUIC + WebSocket transports,
  Circuit Relay v2, DCUtR hole punching, AutoNAT v2, UPnP, Noise/TLS
- **Bubble Tea + Lipgloss** — terminal interface
- Two custom protocols: `/filetransferilla/rendezvous/1.0.0` and
  `/filetransferilla/transfer/1.1.0`

## Limitations

- Whoever enters the code first gets the files. ~1.7 million combinations,
  a per-peer guess limit and room-ownership protection guard it, but PAKE
  would be a real improvement — as it stands **the server is trusted**
  (it is what tells the receiver who the sender is).
- No resume; an interrupted transfer restarts. Partial downloads are
  cleaned up, so a corrupt file never looks complete.
- Files only, no directories.
- Relay fallback is capped at 256 MB per connection (`-relay-data`).
