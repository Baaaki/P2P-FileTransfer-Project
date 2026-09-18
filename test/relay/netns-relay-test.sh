#!/usr/bin/env bash
#
# The one scenario a normal test cannot reach: two peers that can both
# talk to the server but cannot reach each other at all, so hole punching
# is not merely hard but impossible, and the relay fallback *has* to carry
# the transfer.
#
# That is the hard version of "mobile data behind CGNAT, home router behind
# a symmetric NAT" — the case this project exists for and the one most
# likely to rot silently, since every other test runs on a loopback where
# the direct path always works.
#
# It needs no root: unprivileged user + network namespaces do the whole
# thing. Run it with `make test-relay`, which builds the binaries first and
# then runs, in effect:
#
#     FT_BIN_DIR=bin unshare -Urnm --map-root-user test/relay/netns-relay-test.sh
#
# The binaries are built *outside* the namespaces on purpose. Inside them
# there is no network at all, so `go build` works only while every module
# happens to be in the cache — which on a fresh CI runner it is not.
set -euo pipefail

# Numbers below are parsed from Prometheus output ("1.6e+07"); under a
# locale that writes decimals with a comma, awk and printf misread them.
export LC_ALL=C

SIZE_MB="${FT_TEST_SIZE_MB:-16}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

if [ "$(id -u)" != "0" ]; then
  echo "this script expects to run inside 'unshare -Urnm --map-root-user'" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# The topology
#
#   netns A (10.0.0.2) ─┐                        ┌─ netns B (10.0.0.3)
#      sender           ├── br0 · 10.0.0.1 ──────┤      receiver
#                       │   server + relay       │
#                       └──── A ✗ B (isolated) ──┘
#
# The bridge has hairpinning off and port isolation on, so A and B can each
# reach the bridge address but never each other.
# ---------------------------------------------------------------------------
log "building the topology"

# `ip netns` keeps its state in /run/netns, which we cannot write to as a
# mapped root. The mount namespace from -m lets us put a tmpfs there
# without touching the real /run.
mount --make-rprivate / 2>/dev/null || true
mount -t tmpfs tmpfs /run
mkdir -p /run/netns

ip link add br0 type bridge
ip addr add 10.0.0.1/24 dev br0
ip link set br0 up
ip link set lo up

for pair in "a:2" "b:3"; do
  name="${pair%%:*}"
  host="${pair##*:}"
  ip netns add "ns$name"
  ip link add "veth-$name" type veth peer name "in-$name"
  ip link set "veth-$name" master br0
  ip link set "veth-$name" type bridge_slave isolated on
  ip link set "veth-$name" up
  ip link set "in-$name" netns "ns$name"
  ip netns exec "ns$name" ip addr add "10.0.0.$host/24" dev "in-$name"
  ip netns exec "ns$name" ip link set "in-$name" up
  ip netns exec "ns$name" ip link set lo up
done

log "checking the isolation actually holds"
check() { # description, netns, target, expectation
  if ip netns exec "$2" ping -c1 -W1 "$3" >/dev/null 2>&1; then got=REACHABLE; else got=BLOCKED; fi
  printf '  %-28s %s' "$1" "$got"
  if [ "$got" != "$4" ]; then printf '   <- expected %s\n' "$4"; exit 1; fi
  printf '\n'
}
check "A -> server (10.0.0.1)" nsa 10.0.0.1 REACHABLE
check "B -> server (10.0.0.1)" nsb 10.0.0.1 REACHABLE
check "A -> B      (10.0.0.3)" nsa 10.0.0.3 BLOCKED
check "B -> A      (10.0.0.2)" nsb 10.0.0.2 BLOCKED

# ---------------------------------------------------------------------------
if [ -n "${FT_BIN_DIR:-}" ]; then
  log "using the binaries in $FT_BIN_DIR"
  cp "$FT_BIN_DIR/puresend-server" "$WORK/server"
  cp "$FT_BIN_DIR/puresend" "$WORK/client"
else
  log "building the binaries (needs every module already in the cache)"
  go build -o "$WORK/server" ./cmd/server
  go build -o "$WORK/client" ./cmd/client
fi

log "starting the server on the bridge"
"$WORK/server" -ws-port 8080 -health-addr 127.0.0.1:8081 -key "$WORK/server.key" \
  -announce /ip4/10.0.0.1/tcp/8080/ws >"$WORK/server.log" 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true; rm -rf "$WORK"' EXIT

for _ in $(seq 1 50); do
  grep -q "Peer ID:" "$WORK/server.log" && break
  sleep 0.2
done
PEER_ID="$(awk '/Peer ID:/ {print $3; exit}' "$WORK/server.log")"
[ -n "$PEER_ID" ] || { cat "$WORK/server.log"; echo "server never started" >&2; exit 1; }
SERVER_ADDR="/ip4/10.0.0.1/tcp/8080/ws/p2p/$PEER_ID"
echo "  $SERVER_ADDR"

# ---------------------------------------------------------------------------
log "sending ${SIZE_MB} MB from A to B, which can only go through the relay"
mkdir -p "$WORK/src" "$WORK/out"
head -c "$((SIZE_MB * 1024 * 1024))" /dev/urandom >"$WORK/src/payload.bin"

ip netns exec nsa "$WORK/client" -server "$SERVER_ADDR" \
  -send "$WORK/src/payload.bin" >"$WORK/room" 2>"$WORK/send.log" &
SEND_PID=$!

for _ in $(seq 1 100); do
  [ -s "$WORK/room" ] && break
  sleep 0.2
done
ROOM="$(tr -d '\r\n' <"$WORK/room")"
[ -n "$ROOM" ] || { cat "$WORK/send.log"; echo "no room code" >&2; exit 1; }
echo "  room code: $ROOM"

ip netns exec nsb "$WORK/client" -server "$SERVER_ADDR" \
  -receive "$ROOM" -out "$WORK/out" -yes 2>"$WORK/recv.log"
wait $SEND_PID

# The whole point: the receiver must report the fallback, not a direct link.
if ! grep -q "fallback relay" "$WORK/recv.log"; then
  echo "the transfer did not use the relay — the isolation is not doing its job" >&2
  cat "$WORK/recv.log" >&2
  exit 1
fi

log "checking the bytes"
a="$(sha256sum "$WORK/src/payload.bin" | cut -d' ' -f1)"
b="$(sha256sum "$WORK/out/payload.bin" | cut -d' ' -f1)"
echo "  sent:     $a"
echo "  received: $b"
[ "$a" = "$b" ] || { echo "digest mismatch" >&2; exit 1; }

log "the room must be gone from the server now"
rooms="$(wget -qO- http://127.0.0.1:8081/health | grep -o '"active_rooms":[0-9]*' | cut -d: -f2)"
echo "  active_rooms: $rooms"
[ "$rooms" = "0" ] || { echo "the room outlived the transfer" >&2; exit 1; }

log "and the relay's own metrics must have seen the bytes go through"
relayed="$(wget -qO- http://127.0.0.1:8081/metrics |
  awk '/^libp2p_relaysvc_data_transferred_bytes_total/ {printf "%.0f", $2}')"
relayed="${relayed:-0}"
echo "  libp2p_relaysvc_data_transferred_bytes_total: $relayed"
[ "$relayed" -ge "$((SIZE_MB * 1024 * 1024))" ] ||
  { echo "the relay metrics do not show the transfer" >&2; exit 1; }

printf '\n\033[1;32mrelay fallback works end to end\033[0m\n'
