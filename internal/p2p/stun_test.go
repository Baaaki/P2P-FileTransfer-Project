package p2p

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/pion/stun/v3"
)

// startMockSTUNServer starts a local UDP server that answers RFC 5389
// Binding Requests with an XOR-MAPPED-ADDRESS attribute pointing to targetIP.
func startMockSTUNServer(t *testing.T, targetIP net.IP) (string, func()) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("could not start mock STUN server: %v", err)
	}

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			msg := stun.New()
			if err := stun.Decode(buf[:n], msg); err != nil {
				continue
			}

			// Respond with a binding success
			res := stun.MustBuild(
				stun.NewTransactionIDSetter(msg.TransactionID),
				stun.BindingSuccess,
				&stun.XORMappedAddress{
					IP:   targetIP,
					Port: addr.(*net.UDPAddr).Port,
				},
				stun.Fingerprint,
			)
			_, _ = conn.WriteTo(res.Raw, addr)
		}
	}()

	closeFn := func() {
		_ = conn.Close()
		close(done)
	}
	return conn.LocalAddr().String(), closeFn
}

func TestResolvePublicIP_MockServer(t *testing.T) {
	expectedIP := net.ParseIP("203.0.113.50") // RFC 5737 documentation public IP
	addr, cleanup := startMockSTUNServer(t, expectedIP)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ip, err := ResolvePublicIP(ctx, []string{addr})
	if err != nil {
		t.Fatalf("ResolvePublicIP failed: %v", err)
	}
	if !ip.Equal(expectedIP) {
		t.Errorf("got IP %s, want %s", ip, expectedIP)
	}
}

func TestResolvePublicIP_Fallback(t *testing.T) {
	expectedIP := net.ParseIP("198.51.100.25")
	validAddr, cleanup := startMockSTUNServer(t, expectedIP)
	defer cleanup()

	// Dead primary server that drops packets
	deadAddr := "127.0.0.1:1"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ip, err := ResolvePublicIP(ctx, []string{deadAddr, validAddr})
	if err != nil {
		t.Fatalf("fallback ResolvePublicIP failed: %v", err)
	}
	if !ip.Equal(expectedIP) {
		t.Errorf("got IP %s, want %s", ip, expectedIP)
	}
}

func TestIsDockerAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"/ip4/172.17.0.1/tcp/4001", true},
		{"/ip4/172.28.0.1/udp/5000/quic-v1", true},
		{"/ip4/172.16.0.1/tcp/80", true},
		{"/ip4/172.31.255.254/tcp/80", true},
		{"/ip4/172.15.0.1/tcp/80", false}, // outside 172.16-31
		{"/ip4/172.32.0.1/tcp/80", false}, // outside 172.16-31
		{"/ip4/192.168.1.104/tcp/4001", false},
		{"/ip4/10.0.0.5/tcp/4001", false},
		{"/ip4/127.0.0.1/tcp/4001", false},
		{"/ip4/188.119.40.165/tcp/4001", false},
		{"/ip6/::1/tcp/4001", false},
	}

	for _, tc := range cases {
		ma, err := multiaddr.NewMultiaddr(tc.addr)
		if err != nil {
			t.Fatalf("invalid multiaddr %s: %v", tc.addr, err)
		}
		got := isDockerAddr(ma)
		if got != tc.want {
			t.Errorf("isDockerAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"/ip4/127.0.0.1/tcp/4001", true},
		{"/ip4/127.0.0.2/udp/5000/quic-v1", true},
		{"/ip6/::1/tcp/4001", true},
		{"/ip4/192.168.1.104/tcp/4001", false},
		{"/ip4/172.17.0.1/tcp/4001", false},
		{"/ip4/188.119.40.165/tcp/4001", false},
		{"/ip6/2a02:ff0::1/tcp/4001", false},
	}

	for _, tc := range cases {
		ma, err := multiaddr.NewMultiaddr(tc.addr)
		if err != nil {
			t.Fatalf("invalid multiaddr %s: %v", tc.addr, err)
		}
		got := isLoopbackAddr(ma)
		if got != tc.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestInjectPublicIP(t *testing.T) {
	pubIP := net.ParseIP("188.119.40.165")

	// LAN addr should be converted to public IP
	lanMA := multiaddr.StringCast("/ip4/192.168.1.104/udp/45000/quic-v1")
	injected, ok := injectPublicIP(lanMA, pubIP)
	if !ok {
		t.Fatal("injectPublicIP should succeed for LAN addr")
	}
	want := "/ip4/188.119.40.165/udp/45000/quic-v1"
	if injected.String() != want {
		t.Errorf("got %s, want %s", injected.String(), want)
	}

	// Loopback should NOT be converted
	loopMA := multiaddr.StringCast("/ip4/127.0.0.1/tcp/4001")
	if _, ok := injectPublicIP(loopMA, pubIP); ok {
		t.Error("injectPublicIP should not convert loopback")
	}

	// Already public IP should not be duplicated
	alreadyPub := multiaddr.StringCast("/ip4/188.119.40.165/tcp/4001")
	if _, ok := injectPublicIP(alreadyPub, pubIP); ok {
		t.Error("injectPublicIP should not convert already identical public IP")
	}

	// IPv6 should be ignored by injectPublicIP
	ipv6MA := multiaddr.StringCast("/ip6/2a02::1/tcp/4001")
	if _, ok := injectPublicIP(ipv6MA, pubIP); ok {
		t.Error("injectPublicIP should not convert IPv6 addr")
	}
}

func TestResolvePublicIP_LiveServer(t *testing.T) {
	// Skip if running in short/sandbox mode without external network
	if testing.Short() {
		t.Skip("skipping live STUN test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ip, err := ResolvePublicIP(ctx, DefaultSTUNServers)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host") {
			t.Skipf("skipping live test due to network isolation: %v", err)
		}
		t.Fatalf("live ResolvePublicIP failed: %v", err)
	}
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() {
		t.Errorf("got unexpected public IP: %v", ip)
	}
}

func TestLiveNodeDiscoversPublicIPAndSynthesizesAddrs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	serverAddr := "/dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooWJdXaT1FN4UGLCrrTpdqvpo7cqrJZK6tHvbUPQbJ6APtK"
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	node, err := New(ctx, []string{serverAddr})
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host") {
			t.Skipf("skipping due to network isolation: %v", err)
		}
		t.Fatalf("New failed: %v", err)
	}
	defer node.Close()

	pubIP := node.PublicIP()
	if pubIP == nil {
		t.Fatal("node.PublicIP() is nil; STUN did not discover public IP")
	}
	t.Logf("Discovered public IP via STUN: %s", pubIP)

	hasPublicAddr := false
	for _, a := range node.Addrs() {
		if strings.Contains(a.String(), pubIP.String()) {
			hasPublicAddr = true
			t.Logf("Found synthesized public addr: %s", a)
		}
	}
	if !hasPublicAddr {
		t.Errorf("did not find any advertised multiaddr with discovered public IP %s", pubIP)
	}
}
