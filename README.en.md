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

## ⚡ Performance & System Efficiency

PureSend removes artificial speed and file-size throttling imposed by cloud providers. Its stream-based architecture fully saturates available physical bandwidth:

| Metric / Domain | Performance & Characteristics | Technical Details |
| :--- | :---: | :--- |
| **Memory Footprint (RAM)** | **Constant ~35–45 MB ($O(1)$)** | 32 KB chunk streaming; memory usage remains flat regardless of 100 MB or 50 GB payloads. |
| **Dynamic Compression** | **~800 MB/s** | Adaptive DEFLATE (`flate.HuffmanOnly`) compression accelerates text and code beyond raw wire speed; data that does not shrink is sent as is. |
| **Zero-Allocation Digest** | **25 ns / 0 allocs** | Validation of incoming SHA-256 digests runs with zero heap allocations. |
| **LAN Line-Rate** | **Full Interface Saturation** | Saturated at **~112 MB/s** on Gigabit Ethernet and **~280 MB/s** on 2.5G interfaces. |
| **WAN (Internet) Transfer** | **100% Raw Bandwidth** | Serverless P2P via DCUtR hole punching; throughput is bounded solely by ISP uplink/downlink. |
| **Cryptographic Handshake**| **< 5 ms** | Zero-knowledge SPAKE2 (P-256) mutual key exchange completes almost instantaneously. |

> 📊 For full micro-benchmark outputs, memory profiles, and reproducibility steps: **[Performance Guide (docs/BENCHMARK.md)](docs/BENCHMARK.md)**

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

Developed adhering to strict Go engineering standards with complete race-condition safety and continuous linting:

```bash
# Run unit tests with Go race detector
make test
# or
go test -v -race ./...

# Static analysis
golangci-lint run

# Vulnerability scan
govulncheck ./...
```

---

## 📄 License & Contact

Distributed under the [GNU General Public License v3.0](LICENSE).

* **Website:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
* **Contact:** [contact@madebybaki.com](mailto:contact@madebybaki.com)
* **Security Policy:** [SECURITY.md](.github/SECURITY.md)
* **Contributing Guide:** [CONTRIBUTING.md](.github/CONTRIBUTING.md)
