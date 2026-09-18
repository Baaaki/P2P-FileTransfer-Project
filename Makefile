# FileTransferilla — everything you need to build, test and run it.
#
#   make            list the targets
#   make test       the whole suite, with the race detector
#   make dev        a server plus two clients on localhost, ready to try
#
# FT_SERVER is the meeting point address baked into a client build. Leave
# it empty for local work and pass -server on the command line instead.
# FT_SERVER_LIST is where a client looks if FT_SERVER stops answering.

SHELL := /bin/bash
GO ?= go

# The linters run through `go run`, so they are always built by the Go on
# this machine. A golangci-lint or govulncheck binary built by an older Go
# fails on newer standard library code in ways that look like bugs in the
# project ("undefined: rand", "file requires newer Go version"). Pinned to
# the versions CI uses.
GOLANGCI_LINT_VERSION ?= v2.13.2
GOVULNCHECK_VERSION   ?= v1.8.0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
FT_SERVER ?=
FT_SERVER_LIST ?=

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)
CLIENT_LDFLAGS := $(LDFLAGS) -X main.defaultServer=$(FT_SERVER) \
	-X main.defaultServerList=$(FT_SERVER_LIST)

BIN := bin

.DEFAULT_GOAL := help

## help: list the targets
help:
	@echo "FileTransferilla $(VERSION)"
	@echo
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | awk -F': ' '{printf "  \033[1m%-16s\033[0m %s\n", $$1, $$2}'

## build: build the client and the server into ./bin
build: $(BIN)/filetransferilla $(BIN)/filetransferilla-server

$(BIN)/filetransferilla: $(shell find . -name '*.go' -not -path './LandingPage/*')
	@mkdir -p $(BIN)
	$(GO) build -trimpath -ldflags "$(CLIENT_LDFLAGS)" -o $@ ./cmd/client

$(BIN)/filetransferilla-server: $(shell find . -name '*.go' -not -path './LandingPage/*')
	@mkdir -p $(BIN)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/server

## test: vet and run the whole suite with the race detector
test: vet
	$(GO) test -race -timeout 10m ./...

## test-short: the fast tests only — no binaries built, no network
test-short:
	$(GO) test -short -timeout 2m ./...

## test-relay: prove the relay fallback still works when hole punching cannot
# The binaries are built here, with the network, and handed to the script:
# inside its namespaces there is none, and a build there fails the moment a
# module is missing from the cache.
test-relay: $(BIN)/filetransferilla $(BIN)/filetransferilla-server
	FT_BIN_DIR=$(CURDIR)/$(BIN) unshare -Urnm --map-root-user ./test/relay/netns-relay-test.sh

## cover: run the suite and open a coverage report
cover:
	$(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out | tail -1
	@echo "html report: go tool cover -html=coverage.out"

## vet: go vet
vet:
	$(GO) vet ./...

## lint: golangci-lint, the same version CI runs
lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

## vuln: known vulnerabilities the code can actually reach
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

## fmt: gofmt the tree
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './LandingPage/*')

## tidy: go mod tidy
tidy:
	$(GO) mod tidy

## dev: run a local server and print the address to point clients at
dev: build
	@echo "starting a local meeting point; Ctrl+C to stop"
	@echo "point clients at the address it prints:"
	@echo "  ./$(BIN)/filetransferilla -server <address>"
	@echo
	./$(BIN)/filetransferilla-server -port 4001 -ws-port 0 -key .dev-server.key \
		-announce /ip4/127.0.0.1/tcp/4001

## docker: build the server image
docker:
	docker build -t filetransferilla-server:$(VERSION) .

## deb: build a .deb package for Debian/Ubuntu/Mint
deb:
	./scripts/build-deb.sh

## clean: remove build output
clean:
	rm -rf $(BIN) coverage.out

.PHONY: help build test test-short test-relay cover vet lint vuln fmt tidy dev docker deb clean

