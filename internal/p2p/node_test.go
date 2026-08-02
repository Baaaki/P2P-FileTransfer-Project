package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
)

// newTestNode builds a Node around a host that listens nowhere. Enough to
// exercise the shutdown path without touching the network.
func newTestNode(t *testing.T) *Node {
	t.Helper()
	h, err := libp2p.New(libp2p.NoListenAddrs)
	if err != nil {
		t.Fatalf("could not start a test host: %v", err)
	}
	// events is deliberately tiny so a single value fills it.
	return &Node{host: h, events: make(chan Event, 1), done: make(chan struct{})}
}

// TestCloseIsIdempotent guards the two paths that both close the node:
// the user quitting mid-session and the program tearing down on the way
// out. A second Close must not panic on an already-closed channel.
func TestCloseIsIdempotent(t *testing.T) {
	n := newTestNode(t)

	if err := n.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := n.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	select {
	case <-n.Done():
	default:
		t.Error("Done was not closed by Close")
	}
}

// TestEmitDoesNotBlockAfterClose covers the shutdown race: when the user
// quits, the interface stops reading events while a transfer may still be
// unwinding. Without the done case in emit, that transfer's goroutine
// would park forever on a send nobody will ever receive.
func TestEmitDoesNotBlockAfterClose(t *testing.T) {
	n := newTestNode(t)
	defer n.Close()

	// Fill the buffer so the send in emit cannot possibly proceed.
	n.events <- StatusEvent{Text: "filler"}
	n.Close()

	emitted := make(chan struct{})
	go func() {
		n.emit(DoneEvent{}) // blocks forever if emit ignores done
		close(emitted)
	}()

	select {
	case <-emitted:
	case <-time.After(5 * time.Second):
		t.Fatal("emit blocked after the node was closed")
	}
}

// TestProgressEventsNeverBlock checks the throttling escape hatch: a slow
// consumer must not be able to stall the transfer itself, because a
// progress event is superseded by the next one anyway.
func TestProgressEventsNeverBlock(t *testing.T) {
	n := newTestNode(t)
	defer n.Close()

	emitted := make(chan struct{})
	go func() {
		// One more than the buffer holds; the overflow is dropped.
		for i := range 3 {
			n.emit(ProgressEvent{Name: "tatil.jpg", Done: int64(i), Total: 2})
		}
		close(emitted)
	}()

	select {
	case <-emitted:
	case <-time.After(5 * time.Second):
		t.Fatal("a progress event blocked on a full channel")
	}
}
