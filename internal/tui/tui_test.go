package tui

import (
	"strings"
	"testing"

	"puresend/internal/transfer"

	tea "github.com/charmbracelet/bubbletea"
)

// press feeds one keypress to the model and returns the new state.
func press(t *testing.T, m Model, key string) Model {
	t.Helper()
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, _ := m.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", next)
	}
	return got
}

// TestWelcomeChoosesMode checks the very first decision a user makes.
func TestWelcomeChoosesMode(t *testing.T) {
	m := New("/ip4/127.0.0.1/tcp/1/ws/p2p/x")
	if m.screen != screenWelcome {
		t.Fatalf("start screen = %v, want welcome", m.screen)
	}

	// Default highlight is "send"; Enter commits it.
	send := press(t, m, "enter")
	if send.mode != modeSend {
		t.Errorf("mode after Enter on first item = %v, want send", send.mode)
	}
	if send.screen != screenConnecting {
		t.Errorf("screen = %v, want connecting", send.screen)
	}

	// Arrow down then Enter picks "receive".
	recv := press(t, press(t, m, "down"), "enter")
	if recv.mode != modeReceive {
		t.Errorf("mode after Down+Enter = %v, want receive", recv.mode)
	}
}

// TestRoomCodeButtonAdvances covers the "I told my friend" button: the
// user reads the code out, presses Enter, and lands on the waiting screen.
func TestRoomCodeButtonAdvances(t *testing.T) {
	m := New("x")
	m.mode = modeSend
	m.screen = screenRoomCode
	m.room = "kiraz-liman-42"

	next := press(t, m, "enter")
	if next.screen != screenWaiting {
		t.Fatalf("screen after confirming = %v, want waiting", next.screen)
	}

	// The code must stay on screen while waiting — the user often has to
	// read it out a second time.
	if !strings.Contains(next.View(), "kiraz-liman-42") {
		t.Error("waiting screen does not show the room code")
	}
}

// TestConfirmRepliesToTransfer checks that approving an incoming manifest
// actually unblocks the transfer waiting on the reply channel.
func TestConfirmRepliesToTransfer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		key    string
		want   bool
		screen screen
	}{
		{"accept with Enter", "enter", true, screenTransfer},
		{"accept with y", "y", true, screenTransfer},
		{"decline with n", "n", false, screenWelcome},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reply := make(chan bool, 1)
			m := New("x")
			m.mode = modeReceive
			m.screen = screenConfirm
			m.reply = reply
			m.manifest = transfer.Manifest{
				Files: []transfer.FileInfo{{Name: "tatil.jpg", Size: 2 << 20}},
			}

			next := press(t, m, tc.key)
			select {
			case got := <-reply:
				if got != tc.want {
					t.Errorf("replied %v, want %v", got, tc.want)
				}
			default:
				t.Fatal("no answer was sent to the waiting transfer")
			}
			if next.screen != tc.screen {
				t.Errorf("screen = %v, want %v", next.screen, tc.screen)
			}
		})
	}
}

// TestPickFilesNeedsAFile guards the "s" shortcut: it must do nothing
// until at least one file is chosen.
func TestPickFilesNeedsAFile(t *testing.T) {
	m := New("x")
	m.mode = modeSend
	m.screen = screenPickFiles

	if got := press(t, m, "s"); got.screen != screenPickFiles {
		t.Errorf("screen = %v, want to stay on the picker with no files chosen", got.screen)
	}

	m.picked = []pickedFile{{path: "/tmp/a.txt", name: "a.txt", size: 10}}
	if got := press(t, m, "s"); got.screen != screenConnecting {
		t.Errorf("screen = %v, want connecting once a file is chosen", got.screen)
	}
}

// TestBackspaceRemovesLastPick covers undo in the file picker.
func TestBackspaceRemovesLastPick(t *testing.T) {
	m := New("x")
	m.screen = screenPickFiles
	m.picked = []pickedFile{
		{path: "/tmp/a.txt", name: "a.txt", size: 10},
		{path: "/tmp/b.txt", name: "b.txt", size: 20},
	}
	next := press(t, m, "backspace")
	if len(next.picked) != 1 || next.picked[0].name != "a.txt" {
		t.Errorf("picked = %v, want only a.txt left", next.picked)
	}
}

// TestEveryScreenRenders is a guard against a panic or an empty screen in
// any state — a TUI that crashes mid-transfer is worse than a CLI.
func TestEveryScreenRenders(t *testing.T) {
	base := New("/ip4/127.0.0.1/tcp/1/ws/p2p/x")
	base.room = "kiraz-liman-42"
	base.outDir = "/home/user/Downloads"
	base.picked = []pickedFile{{path: "/tmp/a.txt", name: "a.txt", size: 1234}}
	base.manifest = transfer.Manifest{
		Files: []transfer.FileInfo{{Name: "tatil.jpg", Size: 2 << 20}},
	}
	base.savedPaths = []string{"/home/user/Downloads/tatil.jpg"}
	base.progName, base.progDone, base.progTotal = "tatil.jpg", 1<<20, 2<<20
	base.haveConn = true

	all := []struct {
		name string
		s    screen
	}{
		{"welcome", screenWelcome},
		{"connecting", screenConnecting},
		{"pickFiles", screenPickFiles},
		{"roomCode", screenRoomCode},
		{"waiting", screenWaiting},
		{"enterCode", screenEnterCode},
		{"finding", screenFinding},
		{"confirm", screenConfirm},
		{"transfer", screenTransfer},
		{"done", screenDone},
		{"error", screenError},
	}

	for _, mode := range []mode{modeSend, modeReceive} {
		for _, sc := range all {
			m := base
			m.screen = sc.s
			m.mode = mode
			// Exercise both branches of the connection-quality notice.
			for _, direct := range []bool{true, false} {
				m.direct = direct
				out := m.View()
				if strings.TrimSpace(out) == "" {
					t.Errorf("%s (mode %v, direct %v) rendered nothing", sc.name, mode, direct)
				}
			}
		}
	}
}

// TestErrorsAreExplainedInPlainLanguage checks that the messages a user
// is most likely to hit never surface raw technical text.
func TestErrorsAreExplainedInPlainLanguage(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{errString(`room "kiraz-liman-42" not found — the code may be wrong or expired`), "bulunamadı"},
		{errString("could not reach the meeting point: dial timeout"), "ulaşılamadı"},
		{errString("could not connect to the other computer: no good addresses"), "bağlanılamadı"},
		{errString("checksum mismatch for tatil.jpg"), "bozuk"},
	} {
		headline, hints := explain(tc.err)
		if !strings.Contains(strings.ToLower(headline), tc.want) {
			t.Errorf("explain(%q) headline = %q, want it to mention %q", tc.err, headline, tc.want)
		}
		if strings.Contains(headline, "multiaddr") || strings.Contains(headline, "peer") {
			t.Errorf("explain(%q) leaked a technical term: %q", tc.err, headline)
		}
		_ = hints
	}
}

type errString string

func (e errString) Error() string { return string(e) }
