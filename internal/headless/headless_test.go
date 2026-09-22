package headless

import (
	"strings"
	"testing"

	"puresend/internal/p2p"
)

// TestDescribeConnection: the sender's side never learns the relay's
// limit, and a relayed sender used to be told its relay carried "0 B".
func TestDescribeConnection(t *testing.T) {
	for _, tc := range []struct {
		event p2p.ConnectedEvent
		want  string
	}{
		{p2p.ConnectedEvent{Direct: true, RelayLimit: 256 << 20}, "connected directly"},
		{p2p.ConnectedEvent{RelayLimit: 256 << 20}, "connected through the fallback relay (limit 256.0 MB per connection)"},
		{p2p.ConnectedEvent{}, "connected through the fallback relay"},
	} {
		got := describeConnection(tc.event)
		if got != tc.want {
			t.Errorf("describeConnection(%+v) = %q, want %q", tc.event, got, tc.want)
		}
		if strings.Contains(got, "0 B") {
			t.Errorf("describeConnection(%+v) = %q mentions a limit of nothing", tc.event, got)
		}
	}
}
