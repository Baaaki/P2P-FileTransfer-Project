# Image for the rendezvous + relay server (cmd/server).
#
# This is the ONLY part of the project that runs on your Ubuntu box. The
# client (cmd/client) is a desktop app users download — never run it in a
# container, the extra NAT layer breaks hole punching.

# Stage 1: build
FROM golang:1.26-alpine AS build
WORKDIR /app

# Download dependencies first so this layer is cached across code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server

# Stage 2: runtime — a small image containing only the binary
FROM alpine:3.22
WORKDIR /data
COPY --from=build /server /usr/local/bin/server

# The identity key is written to /data/server.key. Mount a volume here:
# if the peer ID changes, every client already released stops working.
VOLUME /data

# 8080 = libp2p WebSocket listener; this is what Cloudflare Tunnel targets.
# 8081 = /health for the supervisor (OpenShip) to probe.
EXPOSE 8080 8081

ENTRYPOINT ["server"]
CMD ["-ws-port", "8080", "-health-port", "8081", "-key", "/data/server.key"]
