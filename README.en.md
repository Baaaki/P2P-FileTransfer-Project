# PureSend 📦 · [Türkçe](README.md)

> **A production-grade, end-to-end encrypted peer-to-peer (P2P) file transfer tool written in Go.**  
> Stream files directly between devices across the internet without cloud storage intermediaries, accounts, or complex network configurations — even behind home routers (NAT) and strict firewalls.

[![CI Pipeline](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

---

## 🎯 Key Features

* 🔒 **No Need to Trust the Server:** The secret words of a code never reach the rendezvous server, so with the **SPAKE2** handshake it can neither read transfers nor put itself in the middle.
* ⚡ **Intelligent NAT Traversal (P2P):** **libp2p (DCUtR)** direct device-to-device streaming without open ports (Relay v2 fallback).
* 🔄 **Resilience & Resumability:** Interrupted transfers resume from the last byte; every file is verified with SHA-256 once complete.
* 💻 **TUI & CLI Automation:** Interactive bilingual terminal UI (`Bubble Tea`) or headless automation flags (`-send`, `-receive`).

---

## 🛠️ Tech Stack

| Area | Technologies |
| :--- | :--- |
| **Language & Runtime** | Go (Golang 1.27) — `CGO_ENABLED=0` (standalone static binary, zero runtime dependencies) |
| **Networking & Protocols** | libp2p (v0.49), WebSockets, TLS, DCUtR (Hole Punching), Circuit Relay v2, STUN (pion/stun v3.1.7), UPnP |
| **Cryptography** | SPAKE2 (PAKE / pake v3), Noise / TLS 1.3, per-file SHA-256, minisign-signed releases, Govulncheck |
| **Interface (TUI)** | Charmbracelet Bubble Tea (v1.3 - Elm Architecture), Lipgloss (v1.1) |
| **DevOps & Packaging** | GoReleaser (v2), GitHub Actions CI/CD, Debian (`.deb`), Arch Linux (`PKGBUILD`), One-Line Installer (`sh`/`ps1`) |

---

## ⚡ Quick Installation

Run the one-line installer for your platform to install and integrate PureSend into your PATH:

```bash
# Linux & macOS (Arch, Ubuntu, Fedora, Debian, macOS, etc.)
curl -fsSL https://raw.githubusercontent.com/Baaaki/PureSend/main/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/Baaaki/PureSend/main/install.ps1 | iex
```

> **Portable Binaries:** Prebuilt standalone executables (`.exe`, `mac_arm64`, and `.deb`) are available directly from [GitHub Releases](https://github.com/Baaaki/PureSend/releases/latest) and the [Project Website](https://puresend.madebybaki.com/#indir).

---

## 🧩 How It Works (Protocol Sequence)

```
Sender (Peer A)                  Rendezvous Server                     Receiver (Peer B)
      │                                  │                                    │
      │ 1. Open room → server gives "42" │                                    │
      │─────────────────────────────────►│◄───────────────────────────────────│ 2. Look up "42" only
      │                                  │                                    │
      │◄══════════ 3. SPAKE2 Key Exchange & NAT Hole Punching (DCUtR) ════════►│
      │                                                                       │
      │═══════════ 4. Files Stream DIRECTLY Peer-to-Peer (SHA-256) ═══════════►│
```

1. **Discovery:** The sender gets a room number from the server (`42`) and puts two secret words of its own in front of it: `kiraz-liman-42`. The server only ever knows the number.
2. **Key Exchange:** Both peers run a **SPAKE2** handshake with the whole code as the password, proving they hold it; a server that never saw the words cannot sit in between. Three wrong codes close the room.
3. **Direct P2P Upgrade:** **DCUtR** punches through NAT boundaries on both endpoints to establish a direct, low-latency socket.
4. **Verified Transfer:** Chunks stream directly peer-to-peer, with SHA-256 validation applied to ensure end-to-end data integrity.

---

## 💻 Usage

### 1. Interactive Terminal UI (TUI)
```bash
puresend
```
* **Send:** Select files or directories $\rightarrow$ Share the generated code (e.g. `kiraz-liman-42`).
* **Receive:** Enter the code $\rightarrow$ Confirm transfer (files save into `Downloads/PureSend`).
* **Language:** Press `[L]` to switch between English and Turkish at any time.

### 2. Headless CLI Mode
```bash
# Send directory in the background
puresend -send ./backups/

# Receive code into a target directory non-interactively
puresend -receive kiraz-liman-42 -out /var/data -yes

# In-place self-update to latest release
puresend -update
```

---

## ⚡ Performance

Every number below was measured; the method and every individual run are in [docs/BENCHMARK.md](docs/BENCHMARK.md). Hardware: AMD Ryzen 7 5700X, NVMe disk, Linux 6.8, Go 1.27.

| Measurement | Result | How |
| :--- | :---: | :--- |
| **End-to-end throughput** | **360–390 MB/s** | Real server, sender and receiver processes on loopback; 2–8 GiB of random data over the encrypted libp2p connection, SHA-256 on both ends, written to disk (`make bench-e2e`) |
| **Peak memory (RSS)** | **35–41 MB** | Same runs; flat from 256 MiB to 8 GiB |
| **Handshake (CPU)** | **~0.6 ms** | Both sides' PAKE and confirmation steps together (`BenchmarkHandshake`); on a real connection, network round trips dominate |
| **Compression** | **~800 MB/s** | Text-like 32 KB chunks, DEFLATE `HuffmanOnly`; a chunk that does not shrink is sent as is |

**What it means:** loopback has no wire speed of its own, so this measures the ceiling the software sets. 390 MB/s is about three times gigabit Ethernet (~118 MB/s): on this hardware the LAN, not PureSend, is the bottleneck. A slower CPU or disk lowers the ceiling. Over the internet, speed is bounded by the two ends' connections, and on the relay by the server's limits. Hole-punching success rates across real network pairs have not been measured yet.

---

## 📂 Codebase Architecture

```
├── cmd/
│   ├── client/          # Terminal application (TUI & Headless CLI entry point)
│   └── server/          # Rendezvous & Circuit Relay v2 server daemon
├── internal/
│   ├── p2p/             # libp2p host lifecycle, multi-address listener, dynamic relay fallback
│   ├── transfer/        # SPAKE2 handshake engine, chunk streaming & resumable transfers
│   ├── tui/             # Bubble Tea models, formatters, and keyboard navigation
│   ├── i18n/            # OS locale detection & localization dictionary (TR/EN)
│   ├── update/          # In-place self-updater querying GitHub Releases
│   └── safetext/        # Terminal escape sequence and bidirectional override sanitization
├── packaging/           # Arch Linux PKGBUILD, .desktop files, and SVG branding
└── scripts/             # Native .deb packaging and build automation scripts
```

---

## 🧪 Testing & Code Quality

CI runs on every commit: a `gofmt` check, `go vet`, the whole suite under the race detector, golangci-lint, govulncheck, a health check of the server's Docker image, and a relay-fallback test over isolated network namespaces where the two peers cannot see each other at all. Coverage is ~77%, counting the compiled client binary the integration tests drive.

```bash
make test        # go vet + the whole suite with the race detector
make cover       # merged coverage report, binary included
make lint        # golangci-lint, the same version CI runs
make vuln        # reachable known vulnerabilities (govulncheck)
make test-relay  # relay fallback over isolated networks (no root needed)
make bench-e2e   # end-to-end throughput and memory
```

---

## 📄 License & Contact

Distributed under the [GNU General Public License v3.0](LICENSE).

* **Website:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
* **Contact:** [contact@madebybaki.com](mailto:contact@madebybaki.com)
* **Security Policy:** [SECURITY.md](.github/SECURITY.md)
* **Contributing Guide:** [CONTRIBUTING.md](.github/CONTRIBUTING.md)
