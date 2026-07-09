# Image for the rendezvous + relay server.
# Note: running the client in Docker is not recommended — the container's
# own NAT layer makes hole punching harder. Run the client natively.

# Stage 1: build
FROM golang:1.26-alpine AS build
WORKDIR /app

# Download dependencies first so this layer is cached across code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 2: runtime — a small image containing only the binary
FROM alpine:3.22
WORKDIR /data
COPY --from=build /server /usr/local/bin/server

# The identity key is written to /data/server.key; mount a volume here so
# the server's peer ID survives container re-creation.
VOLUME /data
EXPOSE 4001/tcp 4001/udp

ENTRYPOINT ["server"]
CMD ["-port", "4001", "-key", "/data/server.key"]
