package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"puresend/internal/i18n"
	"puresend/internal/p2p"
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
	m := New(Config{Servers: []string{"/ip4/127.0.0.1/tcp/1/ws/p2p/x"}})
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
	m := New(Config{Servers: []string{"x"}})
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
			m := New(Config{Servers: []string{"x"}})
			m.mode = modeReceive
			m.screen = screenConfirm
			m.reply = reply
			m.manifest = transfer.Manifest{
				Files: []transfer.FileInfo{{Path: "tatil.jpg", Size: 2 << 20}},
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

// TestDeclineEndsTheSession checks that saying "no" does more than answer
// the transfer: it clears the session with it. The refused transfer
// reports itself as failed a moment later, and that must not land the
// user on an error screen about a refusal they chose themselves.
func TestDeclineEndsTheSession(t *testing.T) {
	reply := make(chan bool, 1)
	m := New(Config{Servers: []string{"x"}})
	m.mode = modeReceive
	m.screen = screenConfirm
	m.reply = reply
	m.room = "kiraz-liman-42"
	m.manifest = transfer.Manifest{
		Files: []transfer.FileInfo{{Path: "tatil.jpg", Size: 2 << 20}},
	}

	next := press(t, m, "n")

	if got := <-reply; got {
		t.Error("declining sent an approval to the transfer")
	}
	if next.screen != screenWelcome || next.mode != modeNone {
		t.Errorf("screen/mode = %v/%v, want welcome/none", next.screen, next.mode)
	}
	if next.reply != nil {
		t.Error("the reply channel was left dangling after answering")
	}
	if next.room != "" || len(next.manifest.Files) != 0 {
		t.Error("session state survived the decline")
	}
}

// TestPickFilesNeedsAFile guards the "s" shortcut: it must do nothing
// until at least one file is chosen.
func TestPickFilesNeedsAFile(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
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

// TestRemoveLastPick covers undo in the file picker. Undo is "x" rather
// than backspace on purpose: backspace is how the browser walks back up a
// folder, and taking it away left the user stuck in a directory as soon as
// they had picked anything.
func TestRemoveLastPick(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	m.screen = screenPickFiles
	m.picked = []pickedFile{
		{path: "/tmp/a.txt", name: "a.txt", size: 10},
		{path: "/tmp/b.txt", name: "b.txt", size: 20},
	}
	next := press(t, m, "x")
	if len(next.picked) != 1 || next.picked[0].name != "a.txt" {
		t.Errorf("picked = %v, want only a.txt left", next.picked)
	}
	// Backspace belongs to the folder browser now, not to the pick list.
	if kept := press(t, m, "backspace"); len(kept.picked) != 2 {
		t.Errorf("backspace removed a pick; it should navigate instead")
	}
}

// TestPickFolder checks that a whole folder can be chosen with "f", and
// that its size is reported as what would actually be sent.
func TestPickFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alt", "b.txt"), []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(Config{Servers: []string{"x"}})
	m.screen = screenPickFiles
	m.picker.CurrentDirectory = dir

	next := press(t, m, "f")
	if len(next.picked) != 1 {
		t.Fatalf("picked %d items, want the one folder", len(next.picked))
	}
	got := next.picked[0]
	if !got.isDir || got.files != 2 || got.size != 8 {
		t.Errorf("picked folder = %+v, want 2 files totalling 8 bytes", got)
	}
	if !strings.Contains(next.View(), "2 dosya") {
		t.Error("the folder's file count is not shown")
	}
}

// TestEmptyFolderIsNotPicked guards against a selection that would send
// nothing: the protocol carries files, so an empty folder would arrive as
// an error the user cannot act on.
func TestEmptyFolderIsNotPicked(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	m.screen = screenPickFiles
	m.picker.CurrentDirectory = t.TempDir()

	if next := press(t, m, "f"); len(next.picked) != 0 {
		t.Errorf("an empty folder was accepted: %+v", next.picked)
	}
}

// TestChooseOutDir covers picking where downloads land — previously a
// fixed folder with no way to change it.
func TestChooseOutDir(t *testing.T) {
	target := t.TempDir()

	m := New(Config{Servers: []string{"x"}})
	m.menuIndex = 2
	m = press(t, m, "enter")
	if m.screen != screenOutDir {
		t.Fatalf("screen = %v, want the folder chooser", m.screen)
	}

	m.dirPicker.CurrentDirectory = target
	m = press(t, m, "s")
	if m.outDir != target {
		t.Errorf("outDir = %q, want %q", m.outDir, target)
	}
	if m.screen != screenWelcome {
		t.Errorf("screen = %v, want to be back on the menu", m.screen)
	}
}

// TestOutDirFromConfig checks the -out flag reaches the model.
func TestOutDirFromConfig(t *testing.T) {
	m := New(Config{Servers: []string{"x"}, OutDir: "/mnt/disk"})
	if m.outDir != "/mnt/disk" {
		t.Errorf("outDir = %q, want the configured one", m.outDir)
	}
	if New(Config{Servers: []string{"x"}}).outDir == "" {
		t.Error("an unset -out left the download folder empty")
	}
}

// TestRelayLimitWarning covers the warning a user cannot work out for
// themselves: on the fallback route there is a size limit, and a transfer
// over it will break off. It must appear only when it is actually true.
func TestRelayLimitWarning(t *testing.T) {
	base := New(Config{Servers: []string{"x"}})
	base.screen = screenConfirm
	base.haveConn = true
	base.relayLimit = 256 << 20
	base.manifest = transfer.Manifest{
		Files: []transfer.FileInfo{{Path: "film.mkv", Size: 400 << 20}},
	}

	relayed := base
	relayed.direct = false
	if !strings.Contains(relayed.View(), "yedek yoldan geçemez") {
		t.Error("no warning for a transfer too big for the fallback route")
	}

	direct := base
	direct.direct = true
	if strings.Contains(direct.View(), "yedek yoldan geçemez") {
		t.Error("warned about the fallback limit on a direct connection")
	}

	small := relayed
	small.manifest = transfer.Manifest{
		Files: []transfer.FileInfo{{Path: "not.txt", Size: 1 << 10}},
	}
	if strings.Contains(small.View(), "yedek yoldan geçemez") {
		t.Error("warned about a transfer that fits comfortably")
	}
}

// TestEveryScreenRenders is a guard against a panic or an empty screen in
// any state — a TUI that crashes mid-transfer is worse than a CLI.
func TestEveryScreenRenders(t *testing.T) {
	base := New(Config{Servers: []string{"/ip4/127.0.0.1/tcp/1/ws/p2p/x"}})
	base.room = "kiraz-liman-42"
	base.outDir = "/home/user/Downloads"
	base.picked = []pickedFile{{path: "/tmp/a.txt", name: "a.txt", size: 1234}}
	base.manifest = transfer.Manifest{
		Files: []transfer.FileInfo{{Path: "tatil.jpg", Size: 2 << 20}},
	}
	base.savedPaths = []string{"/home/user/Downloads/tatil.jpg"}
	base.prog = transfer.Progress{
		Name: "tatil.jpg", Index: 1, Files: 1,
		Done: 1 << 20, Total: 2 << 20,
		OverallDone: 1 << 20, OverallTotal: 2 << 20,
	}
	base.meter.reset(0)
	base.meter.observe(1 << 20)
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
		{"outDir", screenOutDir},
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
		{errString("connection lost while receiving tatil.bin: stream reset: stream reset: connection closed"), "koptu"},
		{errString("the room code does not match the other side"), "eşleşmedi"},
		{errString("no acknowledgement from receiver: EOF"), "koptu"},
		{errString("server rejected registration: too many open rooms for one sender"), "çok fazla"},
		{errString(`the other side sent an unsafe file name: "../evil.txt"`), "kabul edilemez"},
		{errString("server error: too many failed lookups, try again later"), "hatalı kod"},
		{errString("too many files: 9000, at most 5000 can be sent at once"), "çok dosya"},
		{errString("no files to send"), "seçilmedi"},
		{errString("the other side is already sending these files to someone else"), "başka bir transfer"},
		{errString("no answer to the security handshake: the other side stopped responding"), "yanıt vermeyi"},
		{errString("no answer to the security handshake: EOF"), "koptu"},
		{errString("could not read manifest: EOF"), "koptu"},
		{errString("the sender could not prepare its files: could not read the files being sent"), "okuyamadı"},
		{errString("the other side sent an invalid file list: malformed checksum"), "geçersiz"},
		{errString(`the other side sent a file name this computer cannot store: "CON.txt"`), "kullanılamıyor"},
		{errString("server error: the sender is reconnecting, try again in a moment"), "yeniden bağlanıyor"},
		{errString("server rejected registration: server is busy, try again in a minute"), "yoğun"},
		{errString("the room code expired"), "süresi doldu"},
		{errString(`"kiraz-liman" is not a room code`), "oda kodu değil"},
		{errString(`the word "kirez" is not used in room codes`), "tanınmayan"},
	} {
		headline, hints := explain(tc.err)
		if !strings.Contains(strings.ToLower(headline), tc.want) {
			t.Errorf("explain(%q) headline = %q, want it to mention %q", tc.err, headline, tc.want)
		}
		for _, jargon := range []string{"multiaddr", "peer", "stream", "reset", "relay", "EOF"} {
			if strings.Contains(headline, jargon) {
				t.Errorf("explain(%q) leaked the term %q: %q", tc.err, jargon, headline)
			}
		}
		if len(hints) == 0 {
			t.Errorf("explain(%q) offered nothing the user can do about it", tc.err)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// TestUnknownErrorsAreCleaned: the last-resort branch prints the raw
// error, and part of it may have been written by the other side.
func TestUnknownErrorsAreCleaned(t *testing.T) {
	headline, _ := explain(errString("something new: \x1b[2J\x1b[Hsurprise"))
	if strings.ContainsRune(headline, 0x1b) {
		t.Errorf("an escape sequence reached the screen: %q", headline)
	}
}

// event feeds one network event to the model, the way the event reader
// does.
func event(t *testing.T, m Model, ev p2p.Event) Model {
	t.Helper()
	next, _ := m.Update(eventMsg{ev})
	return next.(Model)
}

// TestCodeEntryChecksTheCode: a typo in the shape of the code, or a word
// the program never uses, is caught on the spot — without a trip to the
// server, which only allows a few misses.
func TestCodeEntryChecksTheCode(t *testing.T) {
	base := New(Config{Servers: []string{"x"}})
	base.mode = modeReceive
	base.screen = screenEnterCode

	for typed, want := range map[string]string{
		"kiraz-liman":    "iki kelime ve bir sayı",
		"kirez-liman-42": `"kirez"`,
	} {
		m := base
		m.codeInput.SetValue(typed)
		m = press(t, m, "enter")
		if m.screen != screenEnterCode {
			t.Errorf("%q: left the code screen for a code that cannot work", typed)
		}
		if !strings.Contains(m.View(), want) {
			t.Errorf("%q: the screen does not say what is wrong (%q):\n%s", typed, want, m.View())
		}
		if typedMore := press(t, m, "a"); typedMore.codeErr != "" {
			t.Errorf("%q: the complaint stayed after the user started fixing it", typed)
		}
	}

	m := base
	m.codeInput.SetValue("  KİRAZ liman 42 ")
	m = press(t, m, "enter")
	if m.screen != screenFinding {
		t.Fatalf("a valid code typed loosely did not go through: screen %v, error %q", m.screen, m.codeErr)
	}
	if got := m.codeInput.Value(); got != "kiraz-liman-42" {
		t.Errorf("the code was sent as %q, want it normalized", got)
	}
}

// TestBackgroundPreparationKeepsTheCode: the sender reads its files while
// the code is on screen. That progress is shown, but must never take the
// code away — the user may be reading it out at that very moment.
func TestBackgroundPreparationKeepsTheCode(t *testing.T) {
	for _, sc := range []screen{screenRoomCode, screenWaiting} {
		m := New(Config{Servers: []string{"x"}})
		m.mode = modeSend
		m.screen = sc
		m.room = "kiraz-liman-42"

		m = event(t, m, p2p.PreparingEvent{Name: "tatil/foto.jpg", Index: 2, Files: 5})
		if m.screen != sc {
			t.Fatalf("background reading moved the screen from %v to %v", sc, m.screen)
		}
		if !strings.Contains(m.View(), "hazırlanıyor (2/5)") {
			t.Errorf("progress of the background read is not shown:\n%s", m.View())
		}
		m = event(t, m, p2p.PreparedEvent{})
		if !strings.Contains(m.View(), "hazır") || !strings.Contains(m.View(), "kiraz-liman-42") {
			t.Errorf("the finished read is not reported next to the code:\n%s", m.View())
		}
	}
}

// TestLosingTheServerIsShown: the sender should know its code is
// temporarily unreachable, and see the warning go once it is back.
func TestLosingTheServerIsShown(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	m.mode = modeSend
	m.screen = screenWaiting
	m.room = "kiraz-liman-42"

	m = event(t, m, p2p.ServerLostEvent{})
	if !strings.Contains(m.View(), "yeniden bağlanılıyor") {
		t.Errorf("losing the meeting point is not shown:\n%s", m.View())
	}
	m = event(t, m, p2p.ServerBackEvent{})
	if strings.Contains(m.View(), "yeniden bağlanılıyor") {
		t.Error("the warning stayed after the room was back")
	}
}

// TestRoomLostEndsTheSession: when the code can no longer work, saying so
// beats a waiting screen that will never change.
func TestRoomLostEndsTheSession(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	m.mode = modeSend
	m.screen = screenWaiting
	m.room = "kiraz-liman-42"

	m = event(t, m, p2p.RoomLostEvent{Err: errString("the room code expired")})
	if m.screen != screenError || !strings.Contains(m.View(), "süresi doldu") {
		t.Errorf("screen %v:\n%s", m.screen, m.View())
	}
}

// TestWrongCodeAttemptIsMentioned: someone trying a wrong code is worth
// knowing about, but it changes nothing for the sender.
func TestWrongCodeAttemptIsMentioned(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	m.mode = modeSend
	m.screen = screenWaiting
	m.room = "kiraz-liman-42"

	m = event(t, m, p2p.RejectedEvent{})
	if m.screen != screenWaiting {
		t.Errorf("a rejected stranger moved the screen to %v", m.screen)
	}
	if !strings.Contains(m.View(), "yanlış bir kodla") {
		t.Errorf("the attempt is not mentioned:\n%s", m.View())
	}
}

// TestReceiverSeesSenderPreparing: a big folder takes the sender a while
// to read, and the receiver is told how far along it is.
func TestReceiverSeesSenderPreparing(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	m.mode = modeReceive
	m.screen = screenTransfer
	m.haveConn = true

	m = event(t, m, p2p.RemotePreparingEvent{Done: 3, Total: 10})
	if !strings.Contains(m.View(), "hazırlıyor (3/10)") {
		t.Errorf("the sender's progress is not shown:\n%s", m.View())
	}
}

// TestWelcomeShowsUpdateNotification tests that receiving an update notification
// displays the update banner on the welcome screen.
func TestWelcomeShowsUpdateNotification(t *testing.T) {
	m := New(Config{Servers: []string{"x"}, Version: "v0.2.0"})
	next, _ := m.Update(updateAvailableMsg{tag: "v0.3.0"})
	updated := next.(Model)

	view := updated.View()
	if !strings.Contains(view, "Yeni bir sürüm mevcut (v0.3.0)") {
		t.Errorf("expected update banner in view, got:\n%s", view)
	}
	if !strings.Contains(view, "puresend -update") {
		t.Errorf("expected update command instruction in view, got:\n%s", view)
	}
}

// TestLanguageToggle verifies switching languages with "l" and "L".
func TestLanguageToggle(t *testing.T) {
	m := New(Config{Servers: []string{"x"}})
	if m.lang != i18n.TR {
		t.Fatalf("expected initial default language to be TR, got %v", m.lang)
	}
	if !strings.Contains(m.View(), "Dosyalarını arkadaşına doğrudan gönderirsin.") {
		t.Errorf("expected Turkish welcome text, got:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "L English") {
		t.Errorf("expected footer to show 'L English' shortcut, got:\n%s", m.View())
	}

	// Press "l" to switch to English
	enModel := press(t, m, "l")
	if enModel.lang != i18n.EN {
		t.Fatalf("expected lang to be EN after 'l', got %v", enModel.lang)
	}
	enView := enModel.View()
	if !strings.Contains(enView, "Send files directly to your friend peer-to-peer.") {
		t.Errorf("expected English welcome text, got:\n%s", enView)
	}
	if !strings.Contains(enView, "I want to send files") {
		t.Errorf("expected English menu option, got:\n%s", enView)
	}
	if !strings.Contains(enView, "L Türkçe") {
		t.Errorf("expected footer to show 'L Türkçe' shortcut, got:\n%s", enView)
	}

	// Press uppercase "L" to switch back to Turkish
	trModel := press(t, enModel, "L")
	if trModel.lang != i18n.TR {
		t.Fatalf("expected lang to be TR after 'L', got %v", trModel.lang)
	}
	if !strings.Contains(trModel.View(), "Dosyalarını arkadaşına doğrudan gönderirsin.") {
		t.Errorf("expected Turkish welcome text after toggle back, got:\n%s", trModel.View())
	}
}

// TestConfigInitialLanguage verifies initializing Model with custom language config.
func TestConfigInitialLanguage(t *testing.T) {
	m := New(Config{Servers: []string{"x"}, Lang: "en"})
	if m.lang != i18n.EN {
		t.Fatalf("expected lang to be EN, got %v", m.lang)
	}
	if !strings.Contains(m.View(), "Send files directly to your friend peer-to-peer.") {
		t.Errorf("expected English view, got:\n%s", m.View())
	}

	mErr := m
	mErr.screen = screenError
	mErr.err = errString("room code expired")
	errView := mErr.View()
	if !strings.Contains(errView, "Room code expired.") {
		t.Errorf("expected English error headline, got:\n%s", errView)
	}
	if !strings.Contains(errView, "Start a new transfer to obtain a fresh code.") {
		t.Errorf("expected English error hint, got:\n%s", errView)
	}
}
