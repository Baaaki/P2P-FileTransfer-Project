# FileTransferilla 📦

Peer-to-peer file transfer built on [libp2p](https://libp2p.io). Two machines
find each other through a small rendezvous server, then the files are
transferred **directly between the two peers — never touching the server** —
even across different networks, with both sides behind home NAT.

🇹🇷 Türkçe dokümantasyon: [README.md](README.md)

```
Sender (Istanbul)                  Server (VPS)                Receiver (Izmir)
       │                                │                             │
       │ 1. register room               │                             │
       │    "cherry-harbor-42" ────────►│                             │
       │    with my addresses           │◄─── 2. who is in room       │
       │                                │        "cherry-harbor-42"?  │
       │                                │                             │
       │◄══════ 3. direct P2P connection (hole punching) ════════════►│
       │                4. files flow directly                        │
```

## How it works

1. **Rendezvous** — the sender registers a random room code (e.g.
   `cherry-harbor-42`) together with its network addresses on the server.
   The receiver enters the same code and gets those addresses back.
2. **NAT traversal** — when both peers are behind NAT, the first connection
   is established through the server's **Circuit Relay v2** bridge, and
   libp2p's **DCUtR hole punching** upgrades it to a direct connection.
   The relay only serves peers with an active room (ACL) — the server is
   not a free bridge for unrelated nodes.
3. **Transfer** — the receiver first sees the incoming file list (names +
   sizes) and **approves it**; only then do the files stream over the
   direct connection with per-file **SHA-256 verification**. If hole
   punching fails (e.g. symmetric NAT), the transfer still completes
   through the relay as a fallback.

## Tech stack

- **Go 1.25+** — single external dependency: **go-libp2p v0.48**
- libp2p features: TCP + QUIC transports, Circuit Relay v2, DCUtR hole
  punching, AutoNAT v2, UPnP port mapping, Noise/TLS encryption (always on)
- Two small custom protocols: `/filetransferilla/rendezvous/1.0.0`
  (room register/lookup) and `/filetransferilla/transfer/1.1.0`
  (manifest + accept/decline + file bytes + acknowledgement)

```
cmd/server      rendezvous + relay server (runs on a VPS)
cmd/client      interactive terminal app (send / receive)
internal/       the two protocol implementations
```

## Setup & run

```bash
git clone <repo> && cd FileTransferilla
go build ./...
go test ./...   # unit tests + an end-to-end test over libp2p
```

### 1. Start the server — any machine with an open port (e.g. a cheap VPS)

```bash
go run ./cmd/server -port 4001
```

Open port 4001 (**TCP and UDP**) in the firewall. The server prints the
address clients need — copy the line containing the public IP:

```
/ip4/<VPS-IP>/tcp/4001/p2p/<PeerID>
```

The identity key is saved to `server.key`, so the address survives restarts.

**Or with Docker** (the container prints internal IPs — build the address
yourself from the public IP plus the Peer ID shown in the logs):

```bash
docker build -t filetransferilla-server .
docker run -d -p 4001:4001 -p 4001:4001/udp -v ft-data:/data \
  --restart unless-stopped filetransferilla-server
```

> Don't run the *client* in Docker — the container's extra NAT layer breaks
> hole punching. The client is a single static binary; run it natively.

### 2. Sender

```bash
go run ./cmd/client -server /ip4/<VPS-IP>/tcp/4001/p2p/<PeerID>
```

Pick **1) Send files**, enter the file paths one per line (empty line to
finish). Share the printed room code with the receiver — it is valid for
1 hour.

### 3. Receiver

Run the same command on the other machine, pick **2) Receive files** and
enter the room code. The incoming file list is shown for approval; once
accepted, files land in `received/` by default, each verified against
its SHA-256 digest:

```
Sender found: 12D3KooWHxxef3pj...
✓ Direct P2P connection established — files will bypass the server.

Incoming files:
  photo1.jpg (2.1 MB)
Total: 1 file(s), 2.1 MB
Accept? [y/N]: y
  photo1.jpg  [████████████████████████] 100%  2.1 MB / 2.1 MB
✓ 1 file(s) received and verified
```

> **Trying it locally:** run all three programs on one machine in three
> terminals, using the `127.0.0.1` address the server prints.

## Limitations

- The first receiver to enter the room code gets the files
  (single-transfer, 1-hour design). Guessing is impractical — ~1.7
  million code combinations, a per-peer lookup limit on the server and
  room-ownership protection — but PAKE-based password auth would still
  be a nice addition.
- Interrupted transfers restart from scratch — no resume yet. A partial
  download is cleaned up together with its temporary `.part` file, so no
  corrupt file is ever left behind looking complete.
