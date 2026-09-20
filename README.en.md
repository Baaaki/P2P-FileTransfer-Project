# PureSend 📦 · [Türkçe](README.md)

> **A production-grade, end-to-end encrypted (Zero-Trust) peer-to-peer (P2P) file transfer tool written in Go.**  
> Stream files directly between devices across the internet without cloud storage intermediaries, accounts, or complex network configurations — even behind home routers (NAT) and strict firewalls.

[![CI Pipeline](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

---

## 🎯 Key Features

* 🔒 **Zero-Trust Security:** **SPAKE2** key exchange; the rendezvous server cannot inspect, listen to, or impersonate peers.
* ⚡ **Intelligent NAT Traversal (P2P):** **libp2p (DCUtR)** direct device-to-device streaming without open ports (Relay v2 fallback).
* 🔄 **Resilience & Resumability:** Interrupted transfers automatically resume from the last byte via SHA-256 chunk verification.
* 💻 **TUI & CLI Automation:** Interactive bilingual terminal UI (`Bubble Tea`) or headless automation flags (`-send`, `-receive`).

---

## 🛠️ Tech Stack

| Area | Technologies |
| :--- | :--- |
| **Language & Runtime** | Go (Golang 1.27) — `CGO_ENABLED=0` (standalone static binary, zero runtime dependencies) |
| **Networking & Protocols** | libp2p (v0.49), WebSockets, TLS, DCUtR (Hole Punching), Circuit Relay v2, STUN (pion/stun v3.1.7), UPnP |
| **Cryptography** | SPAKE2 (PAKE / pake v3), AES-GCM, SHA-256 block verification, Govulncheck |
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
      │ 1. Register room "apple-port-42" │                                    │
      │─────────────────────────────────►│◄───────────────────────────────────│ 2. Query room "apple-port-42"
      │                                  │                                    │
      │◄══════════ 3. SPAKE2 Key Exchange & NAT Hole Punching (DCUtR) ════════►│
      │                                                                       │
      │═══════════ 4. Files Stream DIRECTLY Peer-to-Peer (SHA-256) ═══════════►│
```

1. **Discovery:** Sender registers a short 3-word disposable code with the rendezvous server.
2. **Key Exchange:** Both peers perform a **SPAKE2** handshake over an untrusted channel to prove shared knowledge of the room code and establish encrypted communications.
3. **Direct P2P Upgrade:** **DCUtR** punches through NAT boundaries on both endpoints to establish a direct, low-latency socket.
4. **Verified Transfer:** Chunks stream directly peer-to-peer, with SHA-256 validation applied to ensure end-to-end data integrity.

---

## 💻 Usage

### 1. Interactive Terminal UI (TUI)
```bash
puresend
```
* **Send:** Select files or directories $\rightarrow$ Share the generated 3-word code.
* **Receive:** Enter the code $\rightarrow$ Confirm transfer (files save into `Downloads/PureSend`).
* **Language:** Press `[L]` to switch between English and Turkish at any time.

### 2. Headless CLI Mode
```bash
# Send directory in the background
puresend -send ./backups/

# Receive code into a target directory non-interactively
puresend -receive apple-port-42 -out /var/data -yes

# In-place self-update to latest release
puresend -update
```

---

## ⚡ Performance & System Efficiency

PureSend removes artificial speed and file-size throttling imposed by cloud providers. Its stream-based architecture fully saturates available physical bandwidth:

| Metric / Domain | Performance & Characteristics | Technical Details |
| :--- | :---: | :--- |
| **Memory Footprint (RAM)** | **Constant ~35–45 MB ($O(1)$)** | 32 KB chunk streaming; memory usage remains flat regardless of 100 MB or 50 GB payloads. |
| **Dynamic Compression** | **~800 MB/s** | Adaptive Snappy compression accelerates transfer of text and code archives beyond raw network wire speeds. |
| **Zero-Allocation Digest** | **25 ns / 0 allocs** | SHA-256 integrity verification runs with zero heap allocations in the Go runtime. |
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
