# Security

## Reporting a vulnerability

Please report security issues privately, through GitHub's
[private vulnerability reporting](https://github.com/Baaaki/P2P-FileTransfer-Project/security/advisories/new)
rather than a public issue. Include what you did, what happened, and what
you expected — a proof of concept helps but is not required.

Expect an acknowledgement within a week. If a fix is warranted, we will
agree a disclosure date with you, and credit you in the release notes
unless you would rather we did not.

## What this program protects, and what it does not

### Protected

**File contents.** Every byte moves inside a libp2p connection secured with
Noise or TLS between the two peers. The rendezvous server terminates
nothing: on the relay path it forwards ciphertext it cannot read, and on the
direct path it is not in the conversation at all.

**Integrity.** Every file carries a SHA-256 digest computed by the sender.
The receiver checks it while writing, and a file only takes its final name
once the digest matches — an interrupted transfer never leaves something
that looks complete but is not.

**Who you are talking to.** The room code is a shared password, and both
ends prove they hold it before a file list is exchanged. The exchange binds
both peer IDs, so a peer that intercepts the connection cannot pass the two
honest ends' messages through to each other. In particular the **rendezvous
server is not trusted**: it is the party that tells the receiver who the
sender is, and if it names a peer of its own, that peer fails the handshake.

**The code itself.** It never crosses the wire, and nothing derived from it
can be attacked offline — not by the server, not by anyone listening on
either link.

**Where files land.** Everything in a manifest is checked before the user
sees it or anything touches the disk: a path that could escape the chosen
folder, a digest that is not 64 lowercase hex characters (it names the
partial download, so an unchecked one would be a path too), a negative
size or a total that overflows. Names with control characters or
bidirectional marks are refused, since they are about to be printed on the
approval screen; on Windows, names Windows cannot store are refused too. An
existing file is never overwritten.

**The terminal.** Anything the other peer or the server writes — file
names, error messages, reasons for refusing — is stripped of control
characters before it is shown.

**Your time and your room.** Every wait on the other side has an end: 30
seconds for the handshake, five minutes for a person to answer the file
list, two minutes of silence while files move. The room is taken only by a
receiver that has proven the code, so someone who connects and says
nothing, or guesses wrong, cannot keep the real receiver out.

### Not protected

**Metadata.** The server sees which peers meet, when, and from what
addresses. It does not see file names, sizes or contents — but if who is
talking to whom is itself sensitive, this is not the tool for it.

**Anonymity.** The two peers learn each other's IP addresses; that is what a
direct connection means. On the relay path the addresses stay hidden from
each other, but that path is the fallback, not the goal.

**A code you hand to the wrong person.** The code is the whole
authentication. Send it over a channel you trust, and remember it works
exactly once.

**A code someone guesses.** A code is two words from a list of 256 and a
number from 10 to 99: about 5.9 million combinations, 22.5 bits. The code
is also the key the server looks rooms up by, so a lookup that finds a
room *is* a correct guess — the handshake protects against a lying server
and eavesdroppers, not against that. What limits guessing is how fast the
server answers:

- a peer gets 5 misses a minute, then nothing — hit or miss — until the
  minute is over;
- once 200 misses a minute have piled up server-wide, a peer that has
  missed once gets nothing more. Someone typing the code they were given,
  first time, is never refused, so flooding the server with guesses cannot
  lock real users out;
- a refusal is always decided before the room is looked at. Answering hits
  while refusing misses would tell a guesser exactly which guesses hit.

Identities cost nothing, so under a flood the real limit is how fast one
can open connections. Behind a tunnel the server cannot see addresses; the
tunnel can, and a rate limiting rule there (see the README) is what caps
guesses per address. With N rooms open, each guess succeeds with
probability N / 5.9 million. A code in the hands of a stranger is exactly
as good as one in the hands of your friend — the receiver still has to
approve the file list, but the sender does not see who the receiver is.

**The sending machine.** Anything you select is sent. Folders are walked in
full, so check what is in one before choosing it. Symbolic links inside a
folder are skipped precisely so a link cannot widen the selection past what
you agreed to.

**Downloads themselves.** A digest proves a file arrived intact, not that it
is safe to open. Nothing that arrives is executed or opened for you.

**Binary authenticity.** Releases are not code-signed. Check downloads
against the published `checksums.txt`.

## Cryptography

The handshake is a password-authenticated key exchange from
[schollz/pake](https://github.com/schollz/pake) (v3, P-256). It is a
SPAKE2-style construction of that library's own design — not RFC 9382
SPAKE2 and not CPace — and has had far less review than either. It is
what croc has shipped for years. The properties this project relies on
are built around it rather than inside it: both peer IDs are bound into
the session key, and both sides prove the key to each other with
role-separated HMAC tags before anything else is said. Moving to a
standard construction once a maintained Go implementation exists means
changing `internal/transfer/auth.go` and the protocol version; nothing
else depends on the choice.

## Running the server safely

- **Guard `server.key`.** It is the server's identity, and every released
  client has the corresponding peer ID compiled in. Losing it breaks every
  copy of the program in the world; leaking it lets someone impersonate the
  meeting point — which the room-code handshake makes far less useful than
  it once was, but is still worth avoiding.
- **Keep a copy of the key off the machine** (`base64 -w0 server.key`,
  restored with `FT_IDENTITY_KEY`), and keep the landing page's
  `server.txt` listing the server's current address. Released clients
  read that list when their built-in address stops answering; it is what
  keeps them alive if the key is lost anyway.
- **Keep the health port off the internet.** `8081` serves `/health` and
  `/metrics`. The binary listens on `127.0.0.1` unless `-health-addr` says
  otherwise; the image listens inside the container, and the shipped
  compose file maps it to the host's `127.0.0.1` only.
- **Rate-limit at the tunnel.** Behind cloudflared every client arrives
  from the same address, so the server exempts the proxy networks
  (`-trusted-proxies`) from libp2p's per-address limits — which would
  otherwise let 8 connections in for the whole world. Put a per-IP rate
  limiting rule on the hostname at Cloudflare; it is on the free plan.
- **Leave the relay limits in place.** `-relay-data` and `-relay-duration`
  bound what a failed hole punch can push through your connection. The
  relay's ACL already restricts it to peers with an active room.
- **Keep `-rooms-per-peer` at 1.** The program opens one room per session.
  Together with `-register-budget` (new rooms per minute) and rooms being
  dropped a minute after their owner disconnects, it is what stops one
  client filling the room table.

## Supported versions

The latest release is supported. Given the project's size, security fixes
are shipped as a new release rather than backported.
