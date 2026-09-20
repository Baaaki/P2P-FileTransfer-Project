package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/pion/stun/v3"
)

// DefaultSTUNServers lists the public STUN servers used to discover the
// host's external WAN IPv4 address. Cloudflare Anycast provides ultra-low
// latency in Turkey (~3.5 ms via Istanbul POP), while Google STUN serves as a
// reliable global backup.
var DefaultSTUNServers = []string{
	"stun.cloudflare.com:3478",
	"stun.l.google.com:19302",
}

// ResolvePublicIP queries STUN servers in order of priority to determine this
// node's external WAN IPv4 address. It returns the first valid public IP
// discovered, or an error if all servers fail or ctx expires.
func ResolvePublicIP(ctx context.Context, servers []string) (net.IP, error) {
	if len(servers) == 0 {
		servers = DefaultSTUNServers
	}

	var errs []error
	for i, server := range servers {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		timeout := 1500 * time.Millisecond
		if i > 0 {
			timeout = 2000 * time.Millisecond
		}

		serverCtx, cancel := context.WithTimeout(ctx, timeout)
		ip, err := querySTUNServer(serverCtx, server)
		cancel()

		if err == nil && ip != nil && !ip.IsLoopback() && !ip.IsPrivate() {
			return ip, nil
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", server, err))
		} else if ip != nil {
			errs = append(errs, fmt.Errorf("%s: returned non-public IP %s", server, ip))
		}
	}

	return nil, fmt.Errorf("could not discover public IP via STUN: %w", errors.Join(errs...))
}

// querySTUNServer performs a standard RFC 5389 Binding Request over UDP to
// the specified STUN server.
func querySTUNServer(ctx context.Context, server string) (net.IP, error) {
	c, err := stun.Dial("udp4", server)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()

	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	var (
		resIP  net.IP
		resErr error
		done   = make(chan struct{})
	)

	err = c.Start(message, func(e stun.Event) {
		defer close(done)
		if e.Error != nil {
			resErr = e.Error
			return
		}

		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(e.Message); err == nil && xorAddr.IP != nil {
			resIP = xorAddr.IP
			return
		}

		var mappedAddr stun.MappedAddress
		if err := mappedAddr.GetFrom(e.Message); err == nil && mappedAddr.IP != nil {
			resIP = mappedAddr.IP
			return
		}

		resErr = errors.New("STUN response contains neither XOR-MAPPED-ADDRESS nor MAPPED-ADDRESS")
	})
	if err != nil {
		return nil, err
	}

	select {
	case <-done:
		return resIP, resErr
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// isDockerAddr reports whether a multiaddr belongs to a Docker container
// bridge network (RFC 1918 172.16.0.0/12: 172.16.0.0 to 172.31.255.255).
// Developer machines often have 10+ Docker networks, which otherwise pollute
// the advertised multiaddr list and push useful addresses past MaxAddrs.
func isDockerAddr(a multiaddr.Multiaddr) bool {
	ipStr, err := a.ValueForProtocol(multiaddr.P_IP4)
	if err != nil || ipStr == "" {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31
}

// isLoopbackAddr reports whether a multiaddr points to a loopback interface
// (127.0.0.0/8 or ::1).
func isLoopbackAddr(a multiaddr.Multiaddr) bool {
	if ipStr, err := a.ValueForProtocol(multiaddr.P_IP4); err == nil && ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil && ip.IsLoopback() {
			return true
		}
	}
	if ipStr, err := a.ValueForProtocol(multiaddr.P_IP6); err == nil && ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil && ip.IsLoopback() {
			return true
		}
	}
	return false
}

// injectPublicIP replaces the IPv4 portion of a local multiaddr with the
// public IP, keeping ports and transport protocols intact.
func injectPublicIP(a multiaddr.Multiaddr, publicIP net.IP) (multiaddr.Multiaddr, bool) {
	if publicIP == nil {
		return nil, false
	}
	ipStr, err := a.ValueForProtocol(multiaddr.P_IP4)
	if err != nil || ipStr == "" {
		return nil, false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil || ip.IsLoopback() || ip.Equal(publicIP) {
		return nil, false
	}

	oldPrefix := "/ip4/" + ipStr
	newPrefix := "/ip4/" + publicIP.String()
	newStr := strings.Replace(a.String(), oldPrefix, newPrefix, 1)
	newMA, err := multiaddr.NewMultiaddr(newStr)
	if err != nil {
		return nil, false
	}
	return newMA, true
}
