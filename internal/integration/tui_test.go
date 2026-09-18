package integration

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"filetransferilla/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// ansi matches the styling lipgloss adds, so assertions can be made
// against the words a user actually reads.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func screenText(m tea.Model) string {
	return ansi.ReplaceAllString(m.View(), "")
}

// runCmd executes a Bubble Tea command and returns the message it emits.
func runCmd(t *testing.T, cmd tea.Cmd, timeout time.Duration) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	out := make(chan tea.Msg, 1)
	go func() { out <- cmd() }()
	select {
	case msg := <-out:
		return msg
	case <-time.After(timeout):
		t.Fatal("command did not finish in time")
		return nil
	}
}

// TestTUIReachesCodeEntryOverWebSocket drives the real interface against a
// real server over the transport used in production. It covers the seam
// the other tests leave out: the unit tests never touch the network, and
// the end-to-end tests never go through the interface. What is verified
// here is that choosing a mode actually dials the meeting point and lands
// the user on the screen that asks for the code.
func TestTUIReachesCodeEntryOverWebSocket(t *testing.T) {
	_, _, serverAddr := newWSServer(t)

	var m tea.Model = tui.New(tui.Config{Servers: []string{serverAddr}})
	if !strings.Contains(screenText(m), "Ne yapmak istiyorsun") {
		t.Fatalf("first screen is not the welcome menu:\n%s", screenText(m))
	}

	// "Bana dosya gönderilecek", then confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !strings.Contains(screenText(m), "Buluşma noktasına bağlanılıyor") {
		t.Errorf("expected the connecting screen, got:\n%s", screenText(m))
	}

	// Actually dial the server: the step that breaks if the WebSocket
	// address or the handshake is wrong.
	m, _ = m.Update(runCmd(t, cmd, 40*time.Second))

	text := screenText(m)
	if strings.Contains(text, "Bir sorun çıktı") {
		t.Fatalf("connecting to the meeting point failed:\n%s", text)
	}
	if !strings.Contains(text, "kodu yaz") {
		t.Fatalf("expected the code entry screen, got:\n%s", text)
	}
}

// TestTUIReportsAnUnreachableServerInPlainLanguage checks the first thing
// that goes wrong for a user whose download points at a dead server: it
// must surface as an explanation, not a stack of libp2p vocabulary.
func TestTUIReportsAnUnreachableServerInPlainLanguage(t *testing.T) {
	// A syntactically valid address with nothing listening behind it.
	dead := "/ip4/127.0.0.1/tcp/1/ws/p2p/12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn"

	var m tea.Model = tui.New(tui.Config{Servers: []string{dead}})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(runCmd(t, cmd, 60*time.Second))

	text := screenText(m)
	if !strings.Contains(text, "Bir sorun çıktı") {
		t.Fatalf("expected an error screen, got:\n%s", text)
	}
	for _, jargon := range []string{"multiaddr", "peer", "libp2p", "dial"} {
		if strings.Contains(strings.ToLower(text), jargon) {
			t.Errorf("error screen leaked the term %q:\n%s", jargon, text)
		}
	}
}
