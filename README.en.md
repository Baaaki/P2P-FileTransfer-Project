# FileTransferilla 📦

> **End-to-end encrypted, direct peer-to-peer file and folder transfer tool.**  
> Transfer files directly between devices without uploading to any intermediate server — even behind home routers (NAT).

[![CI Status](https://github.com/Baaaki/P2P-FileTransfer-Project/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/P2P-FileTransfer-Project/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/P2P-FileTransfer-Project)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/P2P-FileTransfer-Project)](https://github.com/Baaaki/P2P-FileTransfer-Project/releases/latest)

🇹🇷 **[Türkçe Dokümantasyon](README.md)** | 🌐 **[Website & Live Demo](https://p2p-filetransfer.madebybaki.com)** | 📋 **[Roadmap](docs/ROADMAP.md)** | 🛡️ **[Security Policy](docs/SECURITY.md)**

---

## ✨ Features

- 🚀 **Direct P2P Transfer:** Files never touch the cloud or rendezvous server; bytes flow directly between the two computers.
- 🔑 **Zero Config, 3-Word Code:** No port forwarding, IP configuration, or user accounts. Read the generated code out to a friend.
- 🛡️ **Cryptographic Security (PAKE/SPAKE2):** The room code acts as a password. The rendezvous server is untrusted and cannot decrypt or eavesdrop.
- ⚡ **NAT Hole Punching & Bounded Relay:** Upgrades connections to direct routes via libp2p DCUtR and UPnP; bounded relay fallback if hole punching fails.
- 📂 **Folders & Multi-File Support:** Retains directory trees seamlessly without archiving.
- 🔄 **Resume Interrupted Transfers:** Broken transfers resume where they left off using SHA-256 verified `.part` files.
- 💻 **TUI & Headless CLI:** Beautiful terminal interface (Bubble Tea) plus clean flags (`-send`, `-receive`) for scripts and CI.
- 🐧 **Desktop Integration:** Debian/Ubuntu `.deb` packages, desktop launcher (`.desktop`), SVG icon, and auto-spawning terminal on Linux desktops.

---

## 🚀 Quick Start

> [!TIP]
> **🟢 Live and Ready to Use:**  
> FileTransferilla is **live right now**! You do not need to host or configure any servers. Our official shared rendezvous server (`rendezvous.madebybaki.com`) is running 24/7. Simply download the binary for your platform from the links below and start transferring immediately.

### Download & Run

Precompiled binaries are available directly in the repository's [`bin/`](bin/) folder. Click below to download:

| Platform | Download Link (`bin/`) | How to Run |
|---|---|---|
| **Ubuntu / Debian / Mint** | [📥 `filetransferilla_0.2.0_amd64.deb`](https://github.com/Baaaki/P2P-FileTransfer-Project/raw/main/bin/filetransferilla_0.2.0_amd64.deb) | `sudo apt install ./filetransferilla_0.2.0_amd64.deb` *(Adds to app menu)* |
| **Windows** | [📥 `filetransferilla.exe`](https://github.com/Baaaki/P2P-FileTransfer-Project/raw/main/bin/filetransferilla.exe) | Download and double-click |
| **Linux (Portable)** | [📥 `filetransferilla`](https://github.com/Baaaki/P2P-FileTransfer-Project/raw/main/bin/filetransferilla) | `chmod +x filetransferilla && ./filetransferilla` *(Auto-spawns terminal)* |
| **macOS (Apple Silicon)** | [📥 `filetransferilla_mac_arm64`](https://github.com/Baaaki/P2P-FileTransfer-Project/raw/main/bin/filetransferilla_mac_arm64) | `chmod +x filetransferilla_mac_arm64 && ./filetransferilla_mac_arm64` |
| **macOS (Intel)** | [📥 `filetransferilla_mac_amd64`](https://github.com/Baaaki/P2P-FileTransfer-Project/raw/main/bin/filetransferilla_mac_amd64) | `chmod +x filetransferilla_mac_amd64 && ./filetransferilla_mac_amd64` |

> 📦 Release archives and full release assets are also available on [**GitHub Releases**](https://github.com/Baaaki/P2P-FileTransfer-Project/releases/latest).

#### First-Launch Warnings
Binaries are open-source and not code-signed:
- **macOS:** **Right-click → Open** (or run `xattr -d com.apple.quarantine filetransferilla`).
- **Windows:** Click **More info → Run anyway** on the SmartScreen prompt.
- **Linux:** Mark executable with `chmod +x filetransferilla` if needed.

---

## 💡 Usage

### 1. Interactive Terminal UI (TUI)

Simply run:
```bash
filetransferilla
```
* **Sender:** Choose *"I want to send files"* → Select files/folders (`Enter` to pick, `f` for whole directory, `Backspace` for parent) → Press `s` → Share the 3-word code (`kiraz-liman-42`) with your friend.
* **Receiver:** Choose *"Someone is sending me files"* → Enter the 3-word code → Confirm. Files are saved into your `Downloads/FileTransferilla` folder (customizable via UI or `-out`).

### 2. Headless CLI Mode

For headless servers, scripts, or automation:

```bash
# Send file or directory (prints code to stdout, waits for peer)
filetransferilla -send holiday/

# Receive code directly into a target folder
filetransferilla -receive kiraz-liman-42 -out /mnt/storage -yes

# Version information
filetransferilla -version
```

---

## 🔍 How It Works

```
Sender (Istanbul)                Rendezvous Server                     Receiver (Izmir)
      │                                  │                                    │
      │ 1. Open room "kiraz-liman-42"    │                                    │
      │─────────────────────────────────►│◄───────────────────────────────────│ 2. Who has "kiraz-liman-42"?
      │                                  │                                    │
      │◄══════════ 3. SPAKE2 Key Exchange & NAT Hole Punching (DCUtR) ════════►│
      │                                                                       │
      │═══════════ 4. Files Stream DIRECTLY Peer-to-Peer (SHA-256) ═══════════►│
```

1. **Rendezvous:** Sender registers a random code with its addresses; receiver looks it up.
2. **Authentication (PAKE):** Both ends derive a shared key via **SPAKE2** and bind their peer IDs. The code never crosses the wire in plaintext; a rogue server cannot intercept the transfer.
3. **Hole Punching:** Circuit Relay v2 facilitates initial discovery; DCUtR upgrades to a direct connection.
4. **Transfer:** Data streams block by block with per-file **SHA-256 verification**. Resumes automatically if interrupted.

---

## 🛠️ Self-Hosting

Released clients connect to the community meeting point out of the box. To run your own rendezvous server:

```bash
PUBLIC_HOST=rendezvous.example.com docker compose up -d
```

> For comprehensive Cloudflare Tunnel configuration, Prometheus queries, rate limiting, and operational guidance:  
> 👉 **[Server Deployment Guide (docs/DEPLOYMENT.md)](docs/DEPLOYMENT.md)**

---

## 💻 Development

```bash
make            # List all make targets
make build      # Build client and server into bin/
make test       # Run tests with race detector enabled
make test-relay # Test relay fallback in isolated network namespaces
make lint       # Run golangci-lint
make vuln       # Run govulncheck for reachable vulnerabilities
make deb        # Build Debian/Ubuntu .deb package
```

---

## 📂 Project Architecture

| Directory / Component | Description |
|---|---|
| **`cmd/client/`** | Desktop client (TUI, headless CLI, terminal auto-spawner) |
| **`cmd/server/`** | Rendezvous and Circuit Relay v2 server |
| **`internal/p2p/`** | libp2p host lifecycle, multi-server dialing, dynamic fallback |
| **`internal/transfer/`** | File transfer protocol, SPAKE2 auth, partial resume engine |
| **`internal/tui/`** | Bubble Tea & Lipgloss terminal user interface |
| **`internal/safetext/`** | ANSI escape sequence & Unicode bidi sanitization filter |
| **`LandingPage/`** | React 19 + Vite + Tailwind v4 showcase & browser TUI demo |
| **`packaging/`** | Desktop launcher (`.desktop`), SVG app icon, packaging configs |
| **`deploy/`** | Cloudflare Tunnel configuration templates |

---

## 📚 Documentation

- 🛡️ [Security Policy & Architecture](docs/SECURITY.md)
- 🚀 [Server Deployment & Operations Guide](docs/DEPLOYMENT.md)
- 📋 [Product Roadmap](docs/ROADMAP.md)
- 📝 [Changelog](docs/CHANGELOG.md)
- 🤝 [Contributing Guidelines](docs/CONTRIBUTING.md)

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
