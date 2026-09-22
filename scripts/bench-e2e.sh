#!/usr/bin/env bash
#
# End-to-end throughput and memory of the shipped binaries, on loopback.
#
# The micro-benchmarks in the Go packages time one function each. This
# times the whole program the way a person runs it: a real server, a
# sender and a receiver as separate processes, libp2p's encrypted
# transport, SHA-256 on both ends and the file landing on disk.
#
# Loopback has no wire speed of its own, so what this measures is the
# ceiling the software puts on a transfer — the number to hold against a
# network link. If it is well above 125 MB/s, gigabit Ethernet is the
# limit on a LAN, not PureSend. It says nothing about hole punching or
# the relay; test/relay covers the relay.
#
#   make bench-e2e                     # 2 GiB of random data
#   FT_BENCH_SIZE_MB=512 make bench-e2e
set -euo pipefail
export LC_ALL=C

SIZE_MB="${FT_BENCH_SIZE_MB:-2048}"
PORT="${FT_BENCH_PORT:-4190}"
BIN_DIR="${FT_BIN_DIR:-bin}"
WORK="$(mktemp -d)"
PIDS=()
cleanup() {
  for p in "${PIDS[@]}"; do kill "$p" 2>/dev/null || true; done
  rm -rf "$WORK"
}
trap cleanup EXIT

log() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

[ -x /usr/bin/time ] || { echo "needs GNU time (/usr/bin/time) for peak memory" >&2; exit 1; }
for b in puresend puresend-server; do
  [ -x "$BIN_DIR/$b" ] || { echo "missing $BIN_DIR/$b, run make build first" >&2; exit 1; }
done

log "machine"
echo "  cpu:    $(awk -F': ' '/model name/ {print $2; exit}' /proc/cpuinfo)"
echo "  kernel: $(uname -r)"
echo "  go:     $(go version 2>/dev/null | awk '{print $3}')"
echo "  commit: $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

log "starting a local server on port $PORT"
"$BIN_DIR/puresend-server" -port "$PORT" -ws-port 0 -health-addr "" \
  -key "$WORK/server.key" -announce "/ip4/127.0.0.1/tcp/$PORT" >"$WORK/server.log" 2>&1 &
PIDS+=($!)
for _ in $(seq 1 50); do
  grep -q "Peer ID:" "$WORK/server.log" && break
  sleep 0.2
done
PEER_ID="$(awk '/Peer ID:/ {print $3; exit}' "$WORK/server.log")"
[ -n "$PEER_ID" ] || { cat "$WORK/server.log"; echo "server never started" >&2; exit 1; }
SERVER_ADDR="/ip4/127.0.0.1/tcp/$PORT/p2p/$PEER_ID"

log "writing $SIZE_MB MiB of random data"
# Random bytes do not compress, so every byte crosses the connection.
mkdir -p "$WORK/src" "$WORK/out"
head -c "$((SIZE_MB * 1024 * 1024))" /dev/urandom >"$WORK/src/payload.bin"

log "sending"
/usr/bin/time -f '%M %e' -o "$WORK/send.time" \
  "$BIN_DIR/puresend" -server "$SERVER_ADDR" -send "$WORK/src/payload.bin" \
  >"$WORK/room" 2>"$WORK/send.log" &
SEND_PID=$!
PIDS+=($SEND_PID)

# The sender hashes its files before it will offer them. Wait for that to
# finish so the clock below covers the transfer, not the preparation.
for _ in $(seq 1 600); do
  grep -q "files ready" "$WORK/send.log" && break
  sleep 0.1
done
grep -q "files ready" "$WORK/send.log" || { cat "$WORK/send.log"; echo "sender never got ready" >&2; exit 1; }
ROOM="$(tr -d '\r\n' <"$WORK/room")"

start=$(date +%s.%N)
/usr/bin/time -f '%M %e' -o "$WORK/recv.time" \
  "$BIN_DIR/puresend" -server "$SERVER_ADDR" -receive "$ROOM" -out "$WORK/out" -yes \
  2>"$WORK/recv.log"
end=$(date +%s.%N)
wait "$SEND_PID"

grep -q "connected directly" "$WORK/recv.log" ||
  { cat "$WORK/recv.log"; echo "expected a direct connection on loopback" >&2; exit 1; }

a="$(sha256sum "$WORK/src/payload.bin" | cut -d' ' -f1)"
b="$(sha256sum "$WORK/out/payload.bin" | cut -d' ' -f1)"
[ "$a" = "$b" ] || { echo "digest mismatch" >&2; exit 1; }

read -r send_kb _ <"$WORK/send.time"
read -r recv_kb _ <"$WORK/recv.time"
secs="$(echo "$end - $start" | bc -l)"

log "result"
printf '  payload:            %d MiB, random, digest verified\n' "$SIZE_MB"
printf '  receiver wall time: %.2f s  (start, lookup, handshake, transfer, verify)\n' "$secs"
printf '  throughput:         %.0f MB/s\n' "$(echo "$SIZE_MB * 1048576 / $secs / 1000000" | bc -l)"
printf '  peak RSS, sender:   %d MB\n' "$((send_kb / 1024))"
printf '  peak RSS, receiver: %d MB\n' "$((recv_kb / 1024))"
