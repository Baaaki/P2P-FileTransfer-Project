# Security Policy

Security and user privacy are foundational to PureSend. Reports and contributions from security researchers and the open-source community are always welcome.

---

## Reporting a Vulnerability

To protect PureSend users, please report security vulnerabilities **privately** rather than opening a public issue or pull request.

The primary and recommended channel is **GitHub Private Vulnerability Reporting**:  
👉 **[Report a Security Vulnerability](https://github.com/Baaaki/PureSend/security/advisories/new)**

This feature works just like submitting an issue, but keeps all reproduction steps, logs, and discussions completely private between you and the repository maintainer until a patch is released.

Alternatively, you can email:  
📧 **<contact@madebybaki.com>**

You do not need a complete or weaponized proof-of-concept to reach out. An early heads-up or rough observation is always welcome.

---

## What to Include

Providing structured information helps validate and remediate findings rapidly:

- **Description:** A clear summary of the issue and its realistic security impact.
- **Reproduction:** Step-by-step instructions, including a proof-of-concept (PoC) script, multi-peer setup, or network capture where applicable.
- **Affected Component(s):** Affected submodules (`cmd/client`, `cmd/server`, `internal/transfer`, `internal/rendezvous`, etc.), release version, commit hash, operating system, and architecture.
- **Preconditions & Environment:** Required execution context (e.g., direct P2P connection vs. Circuit Relay v2 fallback, interactive Bubble Tea TUI vs. headless CLI mode, default public server vs. custom self-hosted rendezvous server, symmetric NAT vs. open firewall).
- **Remediation:** Any proposed code fix, patch, or configuration adjustment if you have developed one.

---

## Safe Harbor

Security research conducted in good faith under this policy is considered **authorized**. For research adhering to these terms:

- No legal action will be pursued against you regarding your research activities.
- You will be publicly credited for valid findings in release notes and GitHub Security Advisories (unless you request anonymity).

**Good-faith research guidelines:**

- **Authorized scope only:** Test exclusively against systems and environments you control — your own client machines, test transfers between your own endpoints, and your own self-hosted rendezvous/relay instances.
- **Protect user privacy & service reliability:** Do not attempt to intercept, eavesdrop on, or alter files belonging to other users. Do not exhaust or degrade shared relay bandwidth or server resources.
- **Zero data exfiltration:** Never retain, copy, or distribute data belonging to others. If incidental data is encountered during testing, halt testing, report the incident immediately, and securely purge all local copies once reported.
- **Coordinated disclosure:** Allow reasonable time for remediation and release before disclosing findings publicly.

If you are uncertain whether a particular testing method falls within scope, reach out to <contact@madebybaki.com> before proceeding.

---

## Scope

### In Scope — PureSend Components

- **PureSend Client (`cmd/client`):**
  - Interactive Terminal User Interface (`internal/tui`) built on Charmbracelet Bubble Tea.
  - Headless and automated transfer CLI modes (`internal/headless`).
- **Transfer Engine & Cryptographic Core (`internal/transfer`):**
  - SPAKE2 password-authenticated key exchange (`internal/transfer/auth.go`).
  - Session key derivation with mutual libp2p Peer ID binding.
  - End-to-end stream encryption (AES-GCM, Noise, TLS).
  - Role-separated HMAC confirmation tags (`confirmReceiverLabel`, `confirmSenderLabel`).
  - File manifest parser, path validation, and directory traversal protections (`internal/transfer/names.go`).
  - Symlink safety and destination boundary containment (`safeJoin`).
  - Filename sanitization, control character stripping, and bidirectional (Bidi) override mitigation (`internal/safetext`).
  - Windows reserved device name filtering (`CON`, `PRN`, `AUX`, `NUL`, `COM1-9`, `LPT1-9`, etc.).
  - SHA-256 block-level verification and chunked resume engine (`.part` staging).
- **Peer-to-Peer & Networking Stack (`internal/p2p`):**
  - libp2p node configuration, multiaddr discovery, and transport negotiation (WebSockets, TLS).
  - Direct Connection Upgrade through Relay (DCUtR) hole punching and STUN traversal.
  - Circuit Relay v2 protocol handling and fallback routing.
- **Rendezvous & Relay Server (`cmd/server`, `internal/rendezvous`):**
  - Ephemeral room registry and room lifecycle management.
  - Room code lookup rate limiting, per-peer miss budgets, and global brute-force throttling.
  - Relay access control lists (ACL), per-reservation bandwidth bounds, and session duration limits.
  - Local diagnostic services: Prometheus metrics (`/metrics`) and health check endpoints (`/health`).
- **Packaging, Deployment & Updates:**
  - Docker container configuration (`Dockerfile`, `docker-compose.yml`).
  - Cloudflare Tunnel routing configuration (`deploy/cloudflared-config.yml`).
  - One-line installation scripts (`install.sh`, `install.ps1`).
  - Binary self-update engine (`internal/update`).

### Out of Scope

- **Volumetric Denial-of-Service (DoS):** Flooding public rendezvous servers, saturating relay network pipes, or resource-exhaustion attacks against public infrastructure without an exploitable application flaw.
- **Social Engineering:** Phishing, spear-phishing, or social engineering targeting the maintainer, server operators, or users.
- **Local Machine / Host Compromise:** Vulnerabilities that require prior physical access, malware execution, or root/administrator privileges on the user's host operating system.
- **Probabilistic Room Code Guessing:** Exploiting the baseline entropy of the room code (22.5 bits) within expected probabilistic parameters, without bypassing server rate limiting or cryptographic controls.
- **Harmless Content Receipt:** Transferring files containing malware that the receiving user explicitly approved and accepted. (PureSend guarantees data integrity in transit via SHA-256, but does not perform content inspection or antivirus analysis).
- **Unexploited Upstream Dependencies:** Vulnerabilities in third-party Go modules or upstream libraries without a demonstrated, reachable exploit vector within PureSend.
- **Automated Scanner Dumps:** Unvalidated outputs from automated security scanners lacking an actionable proof-of-concept.

---

## Supported Versions

| Version | Supported | Notes |
| :--- | :---: | :--- |
| **Latest Release (v1.x)** | ✅ | Active support. Security patches are prioritized and released promptly. |
| **Pre-release / Beta / Release Candidates** | ⚠️ | Evaluated on a best-effort basis; fixes merge into the upcoming release. |
| **Older Releases (< v1.0.0)** | ❌ | Not supported. Users must upgrade using `puresend -update` or installer scripts. |

PureSend distributes single static binaries (`CGO_ENABLED=0`). Security fixes are deployed as new tagged releases rather than backported point releases. Self-hosted server operators and end users should keep their installations up to date.

---

## Response & Triage

PureSend is maintained by an independent open-source developer.

- **Triage & Remediation:** Reports are reviewed and addressed on a **best-effort basis** around personal, professional, and military service commitments. Critical vulnerabilities affecting transfer confidentiality, data integrity, or remote code execution are prioritized for resolution.
- **Coordinated Disclosure:** Once a fix is verified, a patched release is published and credited in the GitHub Security Advisory and release notes.

---

## Recognition

With your permission, contributors are credited in:
- The corresponding GitHub Security Advisory.
- The project release notes ([CHANGELOG.md](CHANGELOG.md)).

PureSend is an open-source, community-driven project and does not currently operate a paid bug-bounty program.

---

## Threat Model & Security Guarantees

PureSend operates under a **Zero-Trust** security architecture. Below is a detailed breakdown of what the system protects and what falls outside its security boundary.

### What PureSend Protects

1. **File Contents (Zero-Trust Confidentiality):**
   Every byte moves inside a libp2p connection encrypted with Noise or TLS directly between the two communicating peers. The rendezvous server is cryptographically untrusted:
   - On the direct P2P path (hole punching via DCUtR), the rendezvous server is entirely absent from the data stream.
   - On the Circuit Relay v2 fallback path, the relay merely proxies encrypted ciphertext frames that it cannot inspect, decrypt, or tamper with.

2. **Data Integrity & Atomic Finalization:**
   Every offered file carries an authoritatively computed SHA-256 digest in its manifest. The receiver streams incoming chunks through a streaming SHA-256 hasher while writing to a staging file (`.part`). The file is renamed to its destination path **only after** the complete byte stream matches the sender's SHA-256 checksum. An interrupted or corrupted transfer never leaves a poisoned or incomplete file posing as legitimate.

3. **Peer Authentication & Impersonation Defense:**
   The ephemeral 3-word room code serves as a shared secret in a Password-Authenticated Key Exchange (PAKE). Both peers cryptographically prove knowledge of the code before any file list or metadata is exchanged. The handshake explicitly binds both libp2p Peer IDs into the derived session key. As a result:
   - A compromised or rogue rendezvous server cannot inject an impostor peer: if the server supplies a rogue Peer ID, the cryptographic handshake fails immediately.
   - A network eavesdropper cannot relay handshake messages between two legitimate peers to perform a Man-in-the-Middle (MitM) attack.

4. **Offline Attack Resistance (Room Code Safety):**
   The room code is never transmitted in plaintext across the wire. Nothing derived from the code exposed during the handshake can be subjected to offline dictionary attacks by network listeners or rendezvous operators.

5. **Filesystem Boundary Protection (`safeJoin`):**
   Manifest data received from the remote peer is treated as untrusted input. The manifest parser (`internal/transfer/names.go`) enforces strict sanitization before the user is presented with an approval prompt or any disk write occurs:
   - Directory traversal sequences (`..`, absolute paths, leading slashes, Windows drive letters `C:\`, backslashes) are rejected immediately.
   - Digest strings must strictly match 64 lowercase hexadecimal characters (preventing path traversal via staging file names).
   - Negative file sizes, manifest file count overflows (`maxFiles`), and cumulative integer overflows are rejected.
   - Windows reserved device names (`CON`, `PRN`, `AUX`, `NUL`, `COM1-9`, `LPT1-9`, and names ending with trailing dots or spaces) are blocked on Windows destinations.
   - Existing destination files are never silently overwritten; collisions trigger safe unique naming or user confirmation.

6. **Terminal & UI Command Injection Defense:**
   All text originating from the remote peer or the server (file names, error messages, cancellation reasons) is sanitized through `internal/safetext` before display. ANSI escape sequences that could reprogram the terminal, reposition cursors, or spoof UI approval prompts are stripped. Unicode Bidirectional (Bidi) override characters (which could visually camouflage `.exe` files as `.jpg`) are removed.

7. **Transfer Deadlines & State Isolation:**
   Every protocol phase is strictly bounded by deterministic timeouts:
   - Handshake authentication timeout: 30 seconds.
   - Receiver approval wait timeout: 5 minutes.
   - In-transit idle silence timeout: 2 minutes.
   Rooms are claimed exclusively by receivers that successfully complete the cryptographic handshake. Unauthenticated connections or peers guessing incorrect codes are dropped without blocking legitimate receivers.

---

### What PureSend Does Not Protect

1. **Metadata Confidentiality:**
   The rendezvous server observes when peers connect, their IP addresses, and their public libp2p Peer IDs. While file names, directory structures, and file payloads remain completely hidden, PureSend is not designed for metadata-resistant traffic anonymity.

2. **Network IP Privacy on Direct Connections:**
   In direct P2P mode (the primary performance target), peers inherently discover each other's public/local IP addresses to establish socket connections. On the Circuit Relay v2 fallback path, peer IP addresses remain hidden behind the relay, but relaying is an operational fallback, not a guaranteed privacy mechanism.

3. **Room Codes Shared Over Insecure Channels:**
   The 3-word room code represents complete authorization for that transfer session. If a user transmits the code over an unencrypted or compromised communications channel (e.g., public chat, compromised email), anyone with the code can attempt to connect and claim the room.

4. **Brute-Force Guessing Entropy Limits:**
   A room code consists of two words chosen from a 256-word dictionary and a two-digit integer (10–99), yielding approximately 5.9 million combinations (~22.5 bits of entropy). Protection against online guessing relies on server-side rate limiting:
   - **Per-peer limit:** A peer is allowed up to 5 failed lookups per minute; exceeding this locks the peer out from further lookups.
   - **Server-wide global pressure limit:** If 200 failed lookups accumulate across all clients within a 60-second window, the server enters pressure mode, permitting only one failed attempt per peer. Legitimate users entering the correct code on their first attempt are never locked out.
   - **Decoupled decision ordering:** Rejections are evaluated before database/registry lookups to prevent side-channel timing disclosures between valid and invalid codes.
   - **Reverse Proxy / Cloudflare Tunnel Layer:** When deployed behind reverse proxies where all incoming connections share a gateway IP, external IP rate limiting rules must be configured at the proxy layer (see [DEPLOYMENT.md](DEPLOYMENT.md)).

5. **Sender-Side Selection Mistakes:**
   PureSend recursively walks and packages selected directories. Symbolic links targeting locations outside the chosen directory tree are explicitly ignored to prevent unintentional leakage, but all non-symlink contents within selected folders will be transmitted.

6. **Safety of Approved Downloaded Files:**
   A SHA-256 match verifies that the received file is identical to what the sender transmitted. It does not certify that the file is safe to execute, benign, or free of malicious code. PureSend does not sandbox or execute transferred files.

7. **Binary Code Signing:**
   Binary releases are not currently signed with commercial OS code-signing certificates. Users and system administrators should verify downloaded packages against published `checksums.txt` SHA-256 digests.

---

## Cryptographic Architecture

PureSend's authentication and key exchange pipeline is implemented in `internal/transfer/auth.go`:

```
Sender (Peer A)                                           Receiver (Peer B)
      │                                                          │
      │ ◄────────── 1. libp2p Connection (Noise / TLS) ────────► │
      │                                                          │
      │ 2. Initialize SPAKE2 Party                               │ 2. Initialize SPAKE2 Party
      │    Role: Sender (Role 1)                                 │    Role: Receiver (Role 0)
      │    Curve: P-256 (Constant-time)                         │    Curve: P-256 (Constant-time)
      │                                                          │
      │ ◄────────── 3. Exchange PAKE Public Messages ──────────► │
      │                                                          │
      │ 4. Compute Shared Key S                                  │ 4. Compute Shared Key S
      │ 5. Session Key = KDF(S || PeerID_A || PeerID_B)          │ 5. Session Key = KDF(S || PeerID_A || PeerID_B)
      │                                                          │
      │ ◄────────── 6. Mutual HMAC Confirmation Tags ──────────► │
      │    "puresend/pake/confirm/receiver/v1"                   │
      │    "puresend/pake/confirm/sender/v1"                     │
      │                                                          │
      │ ═══════════ 7. Authenticated Stream Established ═════════│
```

- **PAKE Primitive:** SPAKE2 implementation via `github.com/schollz/pake/v3` using the NIST P-256 elliptic curve (`pakeCurve = "p256"`), backed by Go standard library constant-time scalar arithmetic.
- **Identity Binding:** The derived session key binds both the sender and receiver's cryptographic `peer.ID`, preventing cross-session message splicing and man-in-the-middle relay substitution.
- **Role-Separated Confirmation:** Before manifest or file payloads are accepted, both parties exchange mutual HMAC-SHA256 authentication tags using domain-separated protocol labels:
  - Receiver tag: `confirmReceiverLabel = "puresend/pake/confirm/receiver/v1"`
  - Sender tag: `confirmSenderLabel = "puresend/pake/confirm/sender/v1"`

---

## Server Hardening & Operational Security Guide

For operators running self-hosted rendezvous and relay nodes (`cmd/server`):

1. **Protect the Server Identity Key (`server.key`):**
   - The cryptographic identity of the server defines its libp2p `Peer ID`. Client binaries may pin or discover this server address.
   - Store `server.key` on a secured, non-root readable volume (`chmod 600`).
   - Maintain an offline backup of the key (`base64 -w0 server.key`), restorable via the `FT_IDENTITY_KEY` environment variable.
2. **Isolate Diagnostic & Metrics Ports:**
   - The server exposes `/health` and `/metrics` on port `8081`.
   - Ensure port `8081` binds strictly to `127.0.0.1` and is never exposed to the public internet. Use reverse-proxy authentication or SSH port forwarding for monitoring.
3. **Configure Edge Rate Limiting:**
   - When placing the server behind Cloudflare Tunnel or an OpenResty/Nginx reverse proxy, client multiaddrs will appear to originate from the proxy IP.
   - Configure `-trusted-proxies` to prevent libp2p from throttling the tunnel, and enforce strict IP-based rate limiting on WebSocket upgrades at the edge gateway.
4. **Enforce Relay Quotas:**
   - Retain bounded relay constraints: `-relay-data` (default: 512 MB per reservation) and `-relay-duration` (default: 5 minutes) prevent rogue peers from abusing relay bandwidth when direct hole punching fails.
5. **Enforce Single-Room Concurrency:**
   - Maintain `-rooms-per-peer 1`. A legitimate sender only requires one active room per transfer session. Allowing arbitrary rooms per peer enables state-exhaustion attacks.

---

## Verified Safeguards & Testing

PureSend maintains a comprehensive automated security regression suite in `internal/transfer/hardening_test.go` and `internal/rendezvous/rendezvous_test.go`, verifying:

| Test Case | Defensive Guarantee | Test Verification |
| :--- | :--- | :--- |
| **Path Traversal Defenses** | Rejects `../`, absolute paths, leading slashes, and Windows drive roots. | `TestUnsafePaths` |
| **Manifest Sanitization** | Blocks path-traversal digests, invalid checksum formats, negative file sizes, and arithmetic overflows. | `TestManifestFieldsAreValidated`, `TestManifestTotalCannotOverflow` |
| **Windows Namespace Isolation** | Blocks illegal DOS device names (`CON`, `NUL`, `AUX`, `LPT1-9`, `COM1-9`, trailing dots/spaces). | `TestWindowsNames` |
| **Handshake Impersonation** | Rejects unauthenticated connections, mismatched peer identities, and wrong room codes without leaking secret state. | `TestWrongCodeRejected`, `TestIdentityMismatchRejected` |
| **Denial-of-State & Hijacking** | Prevents unauthenticated receivers from holding or claiming rooms; turns away conflicting claims with `ErrBusy`. | `TestWrongCodeNeverClaims`, `TestSilentReceiverTimesOut`, `TestBusyRoomTurnsAway` |
| **Terminal Control Sanitization** | Strips ANSI escape sequences and Unicode Bidirectional control markers from peer text. | `TestRemoteErrorTextIsCleaned`, `safetext.Clean` |
| **Data Integrity Verification** | Detects transmission bit flips and chunk tampering, terminating transfers without final file rename. | `TestChecksumMismatch` |

---

## Contact & Questions

If you have questions regarding PureSend's security architecture, deployment hardening, or this policy, please reach out via:
- **Email:** <contact@madebybaki.com>
- **Website:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
