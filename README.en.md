# PureSend 📦

> **A production-grade, end-to-end encrypted (Zero-Trust) peer-to-peer (P2P) file transfer tool written in Go.**  
> Stream files directly between devices across the internet without cloud storage intermediaries, accounts, or complex network configurations — even behind home routers (NAT) and strict firewalls.

[![CI Pipeline](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

🌐 **[Live Website & Web Terminal](https://puresend.madebybaki.com)** | 🇹🇷 **[Türkçe Dokümantasyon (README.md)](README.md)** | 🛡️ **[Security Policy](docs/SECURITY.md)** | ✉️ **[Contact](mailto:contact@madebybaki.com)**

---

## 🎯 Executive Summary

PureSend is built to solve the privacy, speed, and size-limit bottlenecks of modern file sharing. Instead of uploading sensitive archives to centralized third-party servers, peers establish an authenticated direct P2P data stream.

* **Zero-Trust Security:** Uses Password-Authenticated Key Exchange (**SPAKE2 / PAKE**). The rendezvous server coordinates peer discovery but is cryptographically untrusted — it cannot inspect, tamper with, or decrypt transfers.
* **Intelligent NAT Traversal:** Leverages **libp2p**, **DCUtR (Direct Connection Upgrade through Relay)**, and **UPnP** to punch holes through home and office firewalls; gracefully falls back to an encrypted, bounded relay if direct traversal fails.
* **Resilience & Resumability:** Interrupted connections automatically resume via SHA-256 verified `.part` chunks without re-transmitting completed files.
* **Bilingual Reactive TUI:** Powered by **Bubble Tea** (Elm Architecture), supporting automatic OS locale detection (Turkish/English) and runtime toggling via the `[L]` key.
* **Headless & Automation Ready:** Native CLI flags (`-send`, `-receive`, `-yes`) facilitate scripted deployment on headless servers and CI/CD runners.

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

## 📊 Real-World Field Benchmarks

Thanks to direct peer-to-peer hole punching and local ISP peering, PureSend bypasses typical cloud storage bandwidth throttles. Verified real-world transfer results:

| Route | Distance | File Size | Average Throughput | Transfer Time | Status / Notes |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Istanbul ➔ Tekirdağ** | ~140 km | **1.5 GB** (Video) | **~20 MB/s** | **~1 min** | **Verified** (Nominal ISP upload package was 48 Mbps, but direct P2P socket achieved ~160–200 Mbps effective throughput) |
| **Istanbul ➔ Istanbul** (Cross-District) | ~35 km | *1.5 GB+* | *Measuring* | *—* | ⏳ *In progress (Coming soon)* |
| **Istanbul ➔ Izmir** | ~480 km | *1.5 GB+* | *Measuring* | *—* | ⏳ *In progress (Coming soon)* |

> 💡 **Note:** Standard asymmetric upload restrictions enforced by cloud providers do not constrain direct P2P streaming, allowing peers to leverage optimal regional peering and full line capacity.

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

* **Email:** [contact@madebybaki.com](mailto:contact@madebybaki.com)
* **Website:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
