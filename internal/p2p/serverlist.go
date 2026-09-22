package p2p

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The meeting point's address is baked into every released copy of the
// program, peer ID and all. If that ever changes — the identity key is
// lost, the server moves to a new host — every copy already downloaded
// stops working at once, and a new release only helps the people who
// fetch it.
//
// The server list is the escape hatch: a small text file at a fixed HTTPS
// address (the landing page serves it), listing the current addresses.
// A client consults it only when none of its built-in addresses answers,
// so it costs nothing in the normal case and keeps old copies alive when
// the server has moved.
//
// The list is not trusted any more than the server is. Whoever controls it
// can point clients at a meeting point of their own, which learns who
// meets whom — but cannot read or alter a transfer: a meeting point is
// only ever told a code's nameplate, and both ends prove the whole code,
// secret words included, to each other.
const (
	serverListTimeout  = 10 * time.Second
	maxServerListBytes = 16 << 10
	maxListedServers   = 16
)

// fetchServerList downloads the list: one multiaddr per line, blank lines
// and lines starting with # ignored.
func fetchServerList(ctx context.Context, url string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, serverListTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("server list: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server list: %s", resp.Status)
	}

	var out []string
	sc := bufio.NewScanner(io.LimitReader(resp.Body, maxServerListBytes))
	for sc.Scan() && len(out) < maxListedServers {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("server list: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("server list: no addresses")
	}
	return out, nil
}
