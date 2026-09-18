// Package tui is the terminal interface a non-technical person uses.
//
// The guiding rule: never show a technical term. The user is walked
// through one question at a time, in plain Turkish, and is told what is
// happening and what to do next on every screen. Words like "multiaddr",
// "peer", "NAT" or "relay" never reach the screen.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"puresend/internal/p2p"
	"puresend/internal/safetext"
	"puresend/internal/transfer"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenWelcome screen = iota
	screenConnecting
	screenPickFiles
	screenRoomCode
	screenWaiting
	screenEnterCode
	screenFinding
	screenConfirm
	screenTransfer
	screenDone
	screenError
	screenOutDir
)

type mode int

const (
	modeNone mode = iota
	modeSend
	modeReceive
)

// pickedFile is one thing the sender chose: a file, or a whole folder.
type pickedFile struct {
	path  string
	name  string
	size  int64
	isDir bool
	files int // number of files inside, for a folder
}

// Config is what the program knows before the user does anything.
type Config struct {
	// Servers are the meeting point addresses, tried in order.
	Servers []string
	// ServerList is where to look for more addresses when none of
	// Servers answers. Empty disables the fallback.
	ServerList string
	// OutDir overrides where incoming files are saved. Empty picks the
	// friendliest default.
	OutDir string
}

// Model is the whole application state.
type Model struct {
	servers    []string
	serverList string
	screen     screen
	mode       mode
	width      int
	height     int

	ctx    context.Context
	cancel context.CancelFunc
	node   *p2p.Node

	// welcome / confirm menus
	menuIndex    int
	confirmIndex int

	// sending
	picker     filepicker.Model
	picked     []pickedFile
	room       string
	prepared   bool   // every file has been read and the offer is ready
	serverLost bool   // the meeting point dropped; the room is being put back
	notice     string // something worth knowing that needs no action

	// receiving
	codeErr     string // what is wrong with the code as typed
	remoteDone  int    // the sender is still reading its files: done of total
	remoteTotal int

	codeInput textinput.Model
	outDir    string

	// choosing the download folder
	dirPicker  filepicker.Model
	backScreen screen

	// transfer state
	direct     bool
	haveConn   bool
	relayLimit int64
	prep       string
	prepIndex  int
	prepFiles  int
	prog       transfer.Progress
	meter      rateMeter
	manifest   transfer.Manifest
	reply      chan<- bool
	savedPaths []string

	status  string
	warn    string
	err     error
	frame   int
	quitted bool
}

// New builds the initial model.
func New(cfg Config) Model {
	home, _ := os.UserHomeDir()

	fp := filepicker.New()
	fp.CurrentDirectory = home
	fp.DirAllowed = false // Enter walks into a folder; "f" sends the whole thing
	fp.FileAllowed = true
	fp.ShowPermissions = false
	fp.SetHeight(10)

	dp := filepicker.New()
	dp.CurrentDirectory = home
	dp.DirAllowed = false // Enter walks in; "s" picks where we already are
	dp.FileAllowed = false
	dp.ShowPermissions = false
	dp.ShowSize = false
	dp.SetHeight(10)

	ti := textinput.New()
	ti.Placeholder = "kiraz-liman-42"
	ti.CharLimit = 64
	ti.Prompt = "  ➜  "

	outDir := cfg.OutDir
	if outDir == "" {
		outDir = DefaultOutDir()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		servers:    cfg.Servers,
		serverList: cfg.ServerList,
		screen:     screenWelcome,
		ctx:        ctx,
		cancel:     cancel,
		picker:     fp,
		dirPicker:  dp,
		codeInput:  ti,
		outDir:     outDir,
	}
}

// Run starts the interface and blocks until the user quits.
func Run(cfg Config) error {
	m := New(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	m.cancel()
	if fm, ok := final.(Model); ok && fm.node != nil {
		_ = fm.node.Close()
	}
	return err
}

// ---------------------------------------------------------------------------
// Messages and commands
// ---------------------------------------------------------------------------

type nodeReadyMsg struct{ node *p2p.Node }
type roomReadyMsg struct{ room string }
type eventMsg struct{ ev p2p.Event }
type errMsg struct{ err error }
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// connectCmd starts the network layer and reaches the meeting point.
func connectCmd(ctx context.Context, servers []string, serverList string) tea.Cmd {
	return func() tea.Msg {
		node, err := p2p.New(ctx, servers, p2p.WithServerList(serverList))
		if err != nil {
			return errMsg{err}
		}
		return nodeReadyMsg{node}
	}
}

// hostCmd claims a room code and starts waiting for the receiver.
func hostCmd(ctx context.Context, node *p2p.Node, files []pickedFile) tea.Cmd {
	return func() tea.Msg {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.path
		}
		room, err := node.Host(ctx, paths)
		if err != nil {
			return errMsg{err}
		}
		return roomReadyMsg{room}
	}
}

// fetchCmd downloads from a room; its progress arrives as events.
func fetchCmd(ctx context.Context, node *p2p.Node, room, outDir string) tea.Cmd {
	return func() tea.Msg {
		go node.Fetch(ctx, room, outDir)
		return nil
	}
}

// waitEvent pulls the next event off the node's channel.
//
// Exactly one of these may be in flight at a time: it is armed once when
// the node comes up and re-armed by handleEvent after every event. Two
// readers on one channel would each take a different event and hand them
// to the update loop in whatever order they happened to win the race —
// enough to make the progress bar jump backwards, or to let the final
// "done" overtake a straggling progress event and leave the user staring
// at a transfer screen that never finishes.
func waitEvent(node *p2p.Node) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev := <-node.Events():
			return eventMsg{ev}
		case <-node.Done():
			return nil
		}
	}
}

// ---------------------------------------------------------------------------
// Bubble Tea plumbing
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.picker.Init(), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.picker.SetHeight(max(msg.Height-16, 5))
		m.dirPicker.SetHeight(max(msg.Height-16, 5))
		return m, nil

	case tickMsg:
		m.frame++
		return m, tick()

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitted = true
			m.cancel()
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case nodeReadyMsg:
		m.node = msg.node
		next, cmd := m.beginMode()
		// The one place the event reader is armed; handleEvent keeps it
		// going from here on.
		return next, tea.Batch(cmd, waitEvent(msg.node))

	case roomReadyMsg:
		m.room = msg.room
		m.screen = screenRoomCode
		return m, nil

	case eventMsg:
		return m.handleEvent(msg.ev)

	case errMsg:
		m.err = msg.err
		m.screen = screenError
		return m, nil
	}

	// Anything else goes to the active component.
	switch m.screen {
	case screenPickFiles:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	case screenOutDir:
		var cmd tea.Cmd
		m.dirPicker, cmd = m.dirPicker.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleEvent folds a p2p event into the model.
func (m Model) handleEvent(ev p2p.Event) (tea.Model, tea.Cmd) {
	switch e := ev.(type) {

	case p2p.StatusEvent:
		m.status = e.Text
		return m, waitEvent(m.node)

	case p2p.ConnectedEvent:
		m.direct = e.Direct
		m.relayLimit = e.RelayLimit
		m.haveConn = true
		m.warn = ""
		m.screen = screenTransfer
		return m, waitEvent(m.node)

	case p2p.PreparingEvent:
		m.prep, m.prepIndex, m.prepFiles = e.Name, e.Index, e.Files
		// A sender reads its files in the background while the code is on
		// screen; that must not take the code away.
		if m.mode != modeSend || m.haveConn {
			m.screen = screenTransfer
		}
		return m, waitEvent(m.node)

	case p2p.PreparedEvent:
		m.prepared = true
		m.prep = ""
		return m, waitEvent(m.node)

	case p2p.RemotePreparingEvent:
		m.remoteDone, m.remoteTotal = max(e.Done, 0), max(e.Total, 0)
		return m, waitEvent(m.node)

	case p2p.RejectedEvent:
		m.notice = "Birisi yanlış bir kodla bağlanmayı denedi. Ona hiçbir şey gösterilmedi."
		return m, waitEvent(m.node)

	case p2p.ServerLostEvent:
		m.serverLost = true
		return m, waitEvent(m.node)

	case p2p.ServerBackEvent:
		m.serverLost = false
		return m, waitEvent(m.node)

	case p2p.RoomLostEvent:
		m.err = e.Err
		m.screen = screenError
		return m, waitEvent(m.node)

	case p2p.ManifestEvent:
		m.manifest = e.Manifest
		m.reply = e.Reply
		m.confirmIndex = 0
		m.screen = screenConfirm
		return m, waitEvent(m.node)

	case p2p.ProgressEvent:
		if m.prog.OverallTotal == 0 {
			m.meter.reset(e.OverallDone)
		}
		m.prog = e.Progress
		m.prep = ""
		m.meter.observe(e.OverallDone)
		if m.screen != screenTransfer {
			m.screen = screenTransfer
		}
		return m, waitEvent(m.node)

	case p2p.DoneEvent:
		if e.Err != nil {
			// While hosting, a failed attempt is not fatal: the room is
			// still registered, so go back to waiting and let the
			// receiver try the same code again.
			if m.mode == modeSend && m.room != "" {
				m.warn = "Bir deneme yarıda kaldı. Arkadaşın aynı kodla tekrar deneyebilir."
				m.haveConn = false
				m.prog = transfer.Progress{}
				m.meter = rateMeter{}
				m.prep = ""
				m.screen = screenWaiting
				return m, waitEvent(m.node)
			}
			m.err = e.Err
			m.screen = screenError
			return m, nil
		}
		m.savedPaths = e.Paths
		m.screen = screenDone
		return m, nil
	}
	return m, waitEvent(m.node)
}

// handleKey routes a keypress to the active screen.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {

	case screenWelcome:
		switch msg.String() {
		case "up", "k":
			m.menuIndex = max(m.menuIndex-1, 0)
		case "down", "j":
			m.menuIndex = min(m.menuIndex+1, 2)
		case "enter":
			switch m.menuIndex {
			case 0:
				m.mode = modeSend
			case 1:
				m.mode = modeReceive
			default:
				// Changing the download folder needs no network at all.
				m.backScreen = screenWelcome
				m.screen = screenOutDir
				m.dirPicker.CurrentDirectory = m.outDir
				return m, m.dirPicker.Init()
			}
			m.screen = screenConnecting
			return m, connectCmd(m.ctx, m.servers, m.serverList)
		case "q":
			m.quitted = true
			m.cancel()
			return m, tea.Quit
		}
		return m, nil

	case screenPickFiles:
		// These are checked before the browser sees the key, so folder
		// navigation is unaffected.
		switch msg.String() {
		case "s":
			if len(m.picked) > 0 {
				m.screen = screenConnecting
				return m, hostCmd(m.ctx, m.node, m.picked)
			}
			return m, nil
		case "f":
			m.addPath(m.picker.CurrentDirectory)
			return m, nil
		case "x":
			if len(m.picked) > 0 {
				m.picked = m.picked[:len(m.picked)-1]
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		if ok, path := m.picker.DidSelectFile(msg); ok {
			m.addPath(path)
		}
		return m, cmd

	case screenOutDir:
		switch msg.String() {
		case "s":
			m.outDir = m.dirPicker.CurrentDirectory
			m.screen = m.backScreen
			return m, nil
		case "esc", "q":
			m.screen = m.backScreen
			return m, nil
		}
		var cmd tea.Cmd
		m.dirPicker, cmd = m.dirPicker.Update(msg)
		return m, cmd

	case screenRoomCode:
		if msg.String() == "enter" {
			m.screen = screenWaiting
		}
		return m, nil

	case screenEnterCode:
		switch msg.String() {
		case "enter":
			if strings.TrimSpace(m.codeInput.Value()) == "" {
				return m, nil
			}
			// Check the code here, before the server sees it: a typo caught
			// now costs nothing, one caught by the server costs one of the
			// few tries it allows.
			code, err := p2p.CheckCode(m.codeInput.Value())
			if err != nil {
				m.codeErr = codeProblem(err)
				return m, nil
			}
			m.codeInput.SetValue(code)
			m.codeErr = ""
			m.screen = screenFinding
			m.status = "arkadaşın aranıyor"
			// The event reader is already running; only start the fetch.
			return m, fetchCmd(m.ctx, m.node, code, m.outDir)
		case "ctrl+o":
			m.backScreen = screenEnterCode
			m.screen = screenOutDir
			m.dirPicker.CurrentDirectory = m.outDir
			return m, m.dirPicker.Init()
		}
		var cmd tea.Cmd
		m.codeInput, cmd = m.codeInput.Update(msg)
		m.codeErr = ""
		return m, cmd

	case screenConfirm:
		switch msg.String() {
		case "left", "h", "right", "l":
			m.confirmIndex = 1 - m.confirmIndex
		case "y", "Y":
			m.confirmIndex = 0
			return m.answerConfirm(true)
		case "n", "N":
			return m.answerConfirm(false)
		case "enter":
			return m.answerConfirm(m.confirmIndex == 0)
		}
		return m, nil

	case screenDone, screenError:
		if msg.String() == "enter" {
			return m.reset(), nil
		}
		if msg.String() == "q" {
			m.quitted = true
			m.cancel()
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

// beginMode opens the first screen of the chosen mode, once the node is
// connected.
func (m Model) beginMode() (Model, tea.Cmd) {
	switch m.mode {
	case modeSend:
		m.screen = screenPickFiles
		return m, m.picker.Init()
	case modeReceive:
		m.screen = screenEnterCode
		return m, m.codeInput.Focus()
	}
	return m, nil
}

// answerConfirm sends the user's decision back to the transfer.
func (m Model) answerConfirm(ok bool) (tea.Model, tea.Cmd) {
	if m.reply != nil {
		m.reply <- ok
		m.reply = nil
	}
	if !ok {
		// Declining ends the session. Tear it down rather than just
		// walking back to the menu: the refused transfer is about to
		// report itself as failed, and the user who pressed "no" should
		// not then be shown an error about it.
		return m.reset(), nil
	}
	m.screen = screenTransfer
	// No waitEvent here: handling the manifest event already re-armed it.
	return m, nil
}

// addPath appends a chosen file or folder, skipping duplicates.
func (m *Model) addPath(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	for _, f := range m.picked {
		if f.path == path {
			return
		}
	}
	pick := pickedFile{path: path, name: info.Name(), size: info.Size()}
	if info.IsDir() {
		size, count := folderSize(path)
		if count == 0 {
			return // an empty folder would carry nothing across
		}
		pick.isDir, pick.size, pick.files = true, size, count
	} else if !info.Mode().IsRegular() {
		return
	}
	m.picked = append(m.picked, pick)
}

// folderSize adds up what sending a folder would actually move. Symbolic
// links are skipped here for the same reason the transfer skips them.
func folderSize(root string) (int64, int) {
	var total int64
	var count int
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return nil //nolint:nilerr // an unreadable corner should not stop the count
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
			count++
		}
		return nil
	})
	return total, count
}

// reset ends the session and returns to the main menu. The network layer
// goes down with it and the next transfer builds a fresh one, which
// guarantees no leftover event can bleed into the next transfer.
// Reconnecting costs a second, behind the spinner the user already sees.
func (m Model) reset() Model {
	if m.node != nil {
		_ = m.node.Close()
		m.node = nil
	}
	m.screen = screenWelcome
	m.mode = modeNone
	m.menuIndex = 0
	m.picked = nil
	m.room = ""
	m.prepared = false
	m.serverLost = false
	m.notice = ""
	m.codeErr = ""
	m.remoteDone, m.remoteTotal = 0, 0
	m.direct = false
	m.haveConn = false
	m.relayLimit = 0
	m.prog = transfer.Progress{}
	m.prep, m.prepIndex, m.prepFiles = "", 0, 0
	m.meter = rateMeter{}
	m.manifest = transfer.Manifest{}
	m.savedPaths = nil
	m.err = nil
	m.warn = ""
	m.status = ""
	m.codeInput.SetValue("")
	return m
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func (m Model) View() string {
	if m.quitted {
		return "Görüşürüz!\n"
	}
	var body string
	switch m.screen {
	case screenWelcome:
		body = m.viewWelcome()
	case screenConnecting:
		body = m.viewConnecting()
	case screenPickFiles:
		body = m.viewPickFiles()
	case screenRoomCode:
		body = m.viewRoomCode()
	case screenWaiting:
		body = m.viewWaiting()
	case screenEnterCode:
		body = m.viewEnterCode()
	case screenFinding:
		body = m.viewFinding()
	case screenConfirm:
		body = m.viewConfirm()
	case screenTransfer:
		body = m.viewTransfer()
	case screenDone:
		body = m.viewDone()
	case screenError:
		body = m.viewError()
	case screenOutDir:
		body = m.viewOutDir()
	}
	return appStyle.Render(body)
}

func (m Model) spinner() string {
	return lipgloss.NewStyle().Foreground(colAccent).
		Render(spinnerFrames[m.frame%len(spinnerFrames)])
}

func (m Model) viewWelcome() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📦  PureSend") + "\n\n")
	b.WriteString(bodyStyle.Render("Dosyalarını arkadaşına doğrudan gönderirsin.") + "\n")
	b.WriteString(helpStyle.Render("Dosyaların hiçbir siteye yüklenmez — senin bilgisayarından") + "\n")
	b.WriteString(helpStyle.Render("çıkar, arkadaşının bilgisayarına iner.") + "\n\n")
	b.WriteString(bodyStyle.Render("Ne yapmak istiyorsun?") + "\n\n")

	opts := []string{
		"📤  Dosya göndereceğim",
		"📥  Bana dosya gönderilecek",
		"📁  İndirme klasörünü değiştir",
	}
	for i, o := range opts {
		if i == m.menuIndex {
			b.WriteString(choiceSelStyle.Render("▸ "+o) + "\n")
		} else {
			b.WriteString(choiceStyle.Render(o) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("İnenler şuraya kaydediliyor:") + "\n")
	b.WriteString(fileStyle.Render(m.outDir) + "\n")
	b.WriteString("\n" + footerStyle.Render("↑ ↓ ile seç  ·  Enter ile onayla  ·  q ile çık"))
	return b.String()
}

func (m Model) viewConnecting() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Bağlanılıyor") + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render("Buluşma noktasına bağlanılıyor...") + "\n\n")
	b.WriteString(helpStyle.Render("Bu, iki bilgisayarın birbirini bulmasını sağlayan küçük bir") + "\n")
	b.WriteString(helpStyle.Render("adres defteri. Dosyaların oraya gitmiyor, sadece") + "\n")
	b.WriteString(helpStyle.Render("\"buradayım\" diyorsun.") + "\n\n")
	b.WriteString(footerStyle.Render("Ctrl+C ile çık"))
	return b.String()
}

func (m Model) viewPickFiles() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📤  Ne göndereceksin?") + "\n\n")
	b.WriteString(helpStyle.Render("Klasörlerin içine girmek için Enter'a bas. Göndermek") + "\n")
	b.WriteString(helpStyle.Render("istediğin dosyanın üzerinde Enter'a basınca listeye eklenir.") + "\n")
	b.WriteString(helpStyle.Render("İçinde olduğun klasörün tamamını göndermek için f'ye bas.") + "\n\n")
	b.WriteString(m.picker.View() + "\n")

	if len(m.picked) > 0 {
		var total int64
		b.WriteString(okStyle.Render(fmt.Sprintf("Seçtiklerin (%d):", len(m.picked))) + "\n")
		for _, f := range m.picked {
			label := "• " + f.name
			detail := formatBytes(f.size)
			if f.isDir {
				label = "• 📁 " + f.name
				detail = fmt.Sprintf("%d dosya, %s", f.files, formatBytes(f.size))
			}
			b.WriteString(fileStyle.Render(label) + " " + sizeStyle.Render("("+detail+")") + "\n")
			total += f.size
		}
		b.WriteString(sizeStyle.Render("  Toplam: "+formatBytes(total)) + "\n\n")
		b.WriteString(buttonSelStyle.Render("s  ·  Göndermeye başla") + "\n\n")
		b.WriteString(footerStyle.Render("↑ ↓ gez  ·  Enter aç/seç  ·  f klasörü ekle  ·  x son seçimi sil  ·  Ctrl+C çık"))
	} else {
		b.WriteString("\n" + helpStyle.Render("Henüz bir şey seçmedin.") + "\n\n")
		b.WriteString(footerStyle.Render("↑ ↓ gez  ·  Enter aç/seç  ·  f klasörü ekle  ·  Ctrl+C çık"))
	}
	return b.String()
}

func (m Model) viewOutDir() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📁  İnen dosyalar nereye kaydedilsin?") + "\n\n")
	b.WriteString(helpStyle.Render("Klasörlerin içine girmek için Enter'a bas. İçinde olduğun") + "\n")
	b.WriteString(helpStyle.Render("klasörü seçmek için s'ye bas.") + "\n\n")
	b.WriteString(bodyStyle.Render("Şu an burası: ") + "\n")
	b.WriteString(fileStyle.Render(m.dirPicker.CurrentDirectory) + "\n\n")
	b.WriteString(m.dirPicker.View() + "\n")
	b.WriteString(buttonSelStyle.Render("s  ·  Burayı seç") + "\n\n")
	b.WriteString(footerStyle.Render("↑ ↓ gez  ·  Enter aç  ·  s seç  ·  Esc vazgeç"))
	return b.String()
}

func (m Model) viewRoomCode() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🔑  Oda kodun hazır!") + "\n\n")
	b.WriteString(codeStyle.Render(m.room) + "\n\n")
	b.WriteString(bodyStyle.Render("Şimdi arkadaşına bu kodu ilet.") + "\n")
	b.WriteString(helpStyle.Render("WhatsApp'tan yaz, SMS at ya da telefonda söyle — fark etmez.") + "\n\n")
	b.WriteString(helpStyle.Render("Arkadaşın programı açacak, \"Bana dosya gönderilecek\"i") + "\n")
	b.WriteString(helpStyle.Render("seçecek ve bu kodu yazacak.") + "\n\n")
	b.WriteString(helpStyle.Render("Kod tek kullanımlık: dosyalar gittiği anda geçersiz olur.") + "\n\n")
	b.WriteString(m.hostingNotes())
	b.WriteString(buttonSelStyle.Render("✓  Arkadaşıma ilettim") + "\n\n")
	b.WriteString(footerStyle.Render("Enter ile devam et  ·  Ctrl+C ile çık"))
	return b.String()
}

func (m Model) viewWaiting() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Bekleniyor") + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render("Arkadaşının kodu girmesi bekleniyor...") + "\n\n")
	if m.warn != "" {
		b.WriteString(warnStyle.Render("! "+m.warn) + "\n\n")
	}
	b.WriteString(bodyStyle.Render("Kod: ") + codeStyle.Render(m.room) + "\n\n")
	b.WriteString(m.hostingNotes())
	b.WriteString(helpStyle.Render("Bu pencereyi kapatma. Arkadaşın kodu girdiği anda") + "\n")
	b.WriteString(helpStyle.Render("gönderme kendiliğinden başlayacak.") + "\n\n")
	b.WriteString(helpStyle.Render("Kod en fazla 1 saat geçerli.") + "\n\n")
	b.WriteString(footerStyle.Render("Ctrl+C ile vazgeç"))
	return b.String()
}

// hostingNotes is what the sender needs to know while the code is out:
// how far along reading the files is, whether the meeting point is
// reachable, and whether anyone tried a wrong code.
func (m Model) hostingNotes() string {
	var b strings.Builder
	switch {
	case m.prepared:
		b.WriteString(okStyle.Render("✓ Dosyalar gönderilmeye hazır") + "\n\n")
	case m.prepFiles > 0:
		b.WriteString(m.spinner() + " " + helpStyle.Render(fmt.Sprintf(
			"Dosyalar hazırlanıyor (%d/%d)...", m.prepIndex, m.prepFiles)) + "\n\n")
	}
	if m.serverLost {
		b.WriteString(warnStyle.Render("! Buluşma noktasıyla bağlantı koptu, yeniden bağlanılıyor...") + "\n")
		b.WriteString(helpStyle.Render("  Kodun geçerliliğini koruyor. Arkadaşın bu arada denerse") + "\n")
		b.WriteString(helpStyle.Render("  birkaç saniye sonra tekrar denesin.") + "\n\n")
	}
	if m.notice != "" {
		b.WriteString(helpStyle.Render("• "+m.notice) + "\n\n")
	}
	return b.String()
}

func (m Model) viewEnterCode() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📥  Arkadaşının verdiği kodu yaz") + "\n\n")
	b.WriteString(helpStyle.Render("Arkadaşın sana 3 parçalı bir kod verdi.") + "\n")
	b.WriteString(helpStyle.Render("Şuna benziyor: kiraz-liman-42") + "\n\n")
	b.WriteString(m.codeInput.View() + "\n\n")
	if m.codeErr != "" {
		b.WriteString(warnStyle.Render("! "+m.codeErr) + "\n\n")
	}
	b.WriteString(helpStyle.Render("İnenler şuraya kaydedilecek:") + "\n")
	b.WriteString(fileStyle.Render(m.outDir) + "\n\n")
	b.WriteString(footerStyle.Render("Enter ile devam et  ·  Ctrl+O ile klasörü değiştir  ·  Ctrl+C ile çık"))
	return b.String()
}

func (m Model) viewFinding() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Aranıyor") + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render(m.statusText()) + "\n\n")
	b.WriteString(helpStyle.Render("İki bilgisayar arasında doğrudan bir yol açılmaya") + "\n")
	b.WriteString(helpStyle.Render("çalışılıyor. Bu birkaç saniye sürebilir.") + "\n\n")
	b.WriteString(footerStyle.Render("Ctrl+C ile vazgeç"))
	return b.String()
}

// statusText turns the internal status into a friendly sentence.
func (m Model) statusText() string {
	switch m.status {
	case "looking up the code":
		return "Kod kontrol ediliyor..."
	case "connecting to the other computer":
		return "Arkadaşının bilgisayarına bağlanılıyor..."
	case "opening a direct route":
		return "Doğrudan yol açılıyor..."
	default:
		return "Arkadaşın aranıyor..."
	}
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📥  Sana dosya gönderilmek isteniyor") + "\n\n")

	// A folder of a thousand files would bury the question, so show the
	// first few and count the rest.
	const maxListed = 12
	files := m.manifest.Files
	for i, f := range files {
		if i == maxListed {
			b.WriteString(helpStyle.Render(fmt.Sprintf("  ... ve %d dosya daha", len(files)-maxListed)) + "\n")
			break
		}
		b.WriteString(fileStyle.Render("• "+f.Path) + " " +
			sizeStyle.Render("("+formatBytes(f.Size)+")") + "\n")
	}

	total := m.manifest.TotalSize()
	b.WriteString("\n" + sizeStyle.Render(fmt.Sprintf("  Toplam: %d dosya, %s",
		len(files), formatBytes(total))) + "\n\n")

	if warning := m.relayWarning(total); warning != "" {
		b.WriteString(warnStyle.Render(warning) + "\n\n")
	}

	b.WriteString(helpStyle.Render("Kaydedilecek yer:") + "\n")
	b.WriteString(fileStyle.Render(m.outDir) + "\n\n")
	b.WriteString(bodyStyle.Render("Bu dosyaları almak istiyor musun?") + "\n\n")

	yes, no := buttonStyle.Render("Evet, indir"), buttonStyle.Render("Hayır, iptal")
	if m.confirmIndex == 0 {
		yes = buttonSelStyle.Render("Evet, indir")
	} else {
		no = buttonSelStyle.Render("Hayır, iptal")
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, yes, no) + "\n\n")
	b.WriteString(footerStyle.Render("← → ile seç  ·  Enter ile onayla  ·  y / n kısayolları"))
	return b.String()
}

// relayWarning is the one thing a user cannot work out for themselves: on
// the fallback route there is a hard size limit, and hitting it means the
// transfer breaks off near the end. Better to say so before it starts.
func (m Model) relayWarning(total int64) string {
	if m.direct || !m.haveConn || m.relayLimit <= 0 || total <= m.relayLimit {
		return ""
	}
	return "! Bu kadarı yedek yoldan geçemez.\n" +
		"  Doğrudan yol açılamadı ve yedek yolun " + formatBytes(m.relayLimit) + " sınırı var;\n" +
		"  transfer büyük ihtimalle yarıda kesilecek. Yarım kalırsa kaldığı\n" +
		"  yerden devam eder — ama önce ikinizin de başka bir ağ denemesi\n" +
		"  (mesela wifi yerine mobil veri) daha hızlı sonuç verir."
}

func (m Model) viewTransfer() string {
	var b strings.Builder
	if m.mode == modeSend {
		b.WriteString(titleStyle.Render("📤  Gönderiliyor") + "\n\n")
	} else {
		b.WriteString(titleStyle.Render("📥  İndiriliyor") + "\n\n")
	}

	if m.haveConn {
		if m.direct {
			b.WriteString(okStyle.Render("✓ Doğrudan bağlantı kuruldu") + "\n")
			b.WriteString(helpStyle.Render("  Dosyalar iki bilgisayar arasında akıyor, kimse aradan geçmiyor.") + "\n\n")
		} else {
			b.WriteString(warnStyle.Render("! Yedek yol kullanılıyor") + "\n")
			b.WriteString(helpStyle.Render("  Doğrudan yol açılamadı (bazı internet bağlantıları buna izin") + "\n")
			b.WriteString(helpStyle.Render("  vermiyor). Transfer yine de şifreli, sadece biraz daha yavaş.") + "\n\n")
		}
	}

	switch {
	case m.prog.Total > 0 || m.prog.OverallTotal > 0:
		b.WriteString(m.viewProgress())
	case m.prep != "":
		b.WriteString(m.spinner() + " " + helpStyle.Render(fmt.Sprintf(
			"Dosyalar kontrol ediliyor (%d/%d)...", m.prepIndex, m.prepFiles)) + "\n")
		b.WriteString(fileStyle.Render(truncate(m.prep, 46)) + "\n\n")
	case m.mode == modeReceive && m.remoteTotal > 0:
		// A large folder takes the sender a while to read; say so, with
		// numbers, rather than an open-ended "preparing".
		b.WriteString(m.spinner() + " " + helpStyle.Render(fmt.Sprintf(
			"Arkadaşının bilgisayarı dosyaları hazırlıyor (%d/%d)...",
			min(m.remoteDone, m.remoteTotal), m.remoteTotal)) + "\n\n")
	default:
		b.WriteString(m.spinner() + " " + helpStyle.Render("Hazırlanıyor...") + "\n\n")
	}
	b.WriteString(footerStyle.Render("Ctrl+C ile vazgeç"))
	return b.String()
}

// viewProgress draws the bar, and under it the two numbers people actually
// watch: how fast it is going and how long is left.
func (m Model) viewProgress() string {
	p := m.prog
	var b strings.Builder

	if p.Files > 1 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("Dosya %d / %d", p.Index, p.Files)) + "\n")
	}
	b.WriteString(fileStyle.Render(truncate(p.Name, 46)) + "\n")

	done, total := p.OverallDone, p.OverallTotal
	if total == 0 {
		done, total = p.Done, p.Total
	}
	pct := int64(0)
	if total > 0 {
		pct = done * 100 / total
	}
	b.WriteString("  " + progressBar(done, total, 30) +
		fmt.Sprintf("  %3d%%  ", pct) +
		sizeStyle.Render(formatBytes(done)+" / "+formatBytes(total)) + "\n")

	if rate := m.meter.rate(); rate > 0 {
		line := formatRate(rate)
		if eta, ok := m.meter.eta(total - done); ok {
			line += "  ·  kalan süre " + formatDuration(eta)
		}
		b.WriteString("  " + sizeStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder
	if m.mode == modeSend {
		b.WriteString(okStyle.Render("✓  Gönderildi!") + "\n\n")
		b.WriteString(bodyStyle.Render(
			"Dosyaların arkadaşına ulaştı ve eksiksiz indiği doğrulandı.") + "\n\n")
		b.WriteString(helpStyle.Render("Kod artık geçersiz — aynı kodla kimse bir daha indiremez.") + "\n\n")
	} else {
		b.WriteString(okStyle.Render("✓  İndi!") + "\n\n")
		b.WriteString(bodyStyle.Render("Dosyalar şuraya kaydedildi:") + "\n\n")
		const maxListed = 12
		for i, p := range m.savedPaths {
			if i == maxListed {
				b.WriteString(helpStyle.Render(fmt.Sprintf("  ... ve %d dosya daha", len(m.savedPaths)-maxListed)) + "\n")
				break
			}
			b.WriteString(fileStyle.Render("• "+p) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("Her dosyanın eksiksiz indiği doğrulandı.") + "\n\n")
	}
	b.WriteString(footerStyle.Render("Enter ile ana menüye dön  ·  q ile çık"))
	return b.String()
}

func (m Model) viewError() string {
	headline, hints := explain(m.err)

	var b strings.Builder
	b.WriteString(errStyle.Render("✗  Bir sorun çıktı") + "\n\n")
	b.WriteString(bodyStyle.Render(headline) + "\n\n")
	if len(hints) > 0 {
		b.WriteString(helpStyle.Render("Ne yapabilirsin:") + "\n")
		for _, h := range hints {
			b.WriteString(helpStyle.Render("  • "+h) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(footerStyle.Render("Enter ile ana menüye dön  ·  q ile çık"))
	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// explain turns a technical error into a plain-language headline and a
// short list of things the user can actually do about it.
//
// Every branch here exists because something reached a real screen without
// one. The default branch prints the raw error, which is the last resort:
// if you find yourself looking at one, that is a missing case, not a
// working fallback.
func explain(err error) (string, []string) {
	if err == nil {
		return "Bilinmeyen bir hata oldu.", nil
	}
	s := strings.ToLower(err.Error())
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(s, p) {
				return true
			}
		}
		return false
	}
	retryTogether := []string{
		"İkiniz de programı açık tutup aynı kodla tekrar deneyin — indirme kaldığı yerden devam eder.",
		"Doğrudan yol açılamadıysa yedek yolun bir boyut sınırı var; büyük dosyalarda bu sınıra takılmış olabilirsiniz.",
		"Mümkünse ikiniz de başka bir ağa geçin (wifi yerine mobil veri gibi).",
	}

	switch {
	case has("room code does not match"):
		return "Kod eşleşmedi.", []string{
			"Kodu harfi harfine doğru yazdığından emin ol.",
			"Arkadaşın sana kodu yeniden okusun — bir harf bile fark eder.",
			"Kod tek kullanımlık: daha önce kullanıldıysa yenisini istemen gerekir.",
		}
	case has("already sending"):
		return "Bu kodla şu an başka bir transfer sürüyor.", []string{
			"Arkadaşına ekranında ne yazdığını sor — dosyalar başka birine gidiyor olabilir.",
			"Kodu senden başka kimseye vermediyse, yeni bir kodla baştan başlasın.",
		}
	case has("not a room code", "malformed room code"):
		return "Bu bir oda kodu değil.", []string{
			"Kod iki kelime ve bir sayıdan oluşur, örneğin kiraz-liman-42.",
		}
	case has("not used in room codes"):
		return "Kodda tanınmayan bir kelime var.", []string{
			"Kodu arkadaşından harf harf yeniden iste.",
		}
	case has("room code expired"):
		return "Kodun süresi doldu.", []string{
			"Yeni bir gönderim başlat; yeni bir kod alırsın.",
		}
	case has("reconnecting"):
		return "Arkadaşının programı buluşma noktasına yeniden bağlanıyor.", []string{
			"Birkaç saniye bekleyip aynı kodla tekrar dene.",
		}
	case has("sender could not prepare"):
		return "Arkadaşının bilgisayarı dosyaları okuyamadı.", []string{
			"Arkadaşın dosyaların yerinde durduğunu kontrol edip baştan göndersin.",
		}
	case has("invalid file list", "message exceeds"):
		return "Karşı taraf geçersiz bir dosya listesi gönderdi.", []string{
			"Güvenlik için transfer durduruldu, diske hiçbir şey yazılmadı.",
		}
	case has("cannot store"):
		return "Gelen dosyalardan birinin adı bu bilgisayarda kullanılamıyor.", []string{
			"Arkadaşın dosyanın adından ? * < > | \" gibi işaretleri çıkarıp tekrar göndersin.",
			"CON, NUL, AUX gibi adlar da Windows'ta kullanılamaz.",
		}
	case has("unsafe file name", "reserved file name"):
		return "Karşı taraf kabul edilemez bir dosya adı gönderdi.", []string{
			"Güvenlik için transfer durduruldu, diske hiçbir şey yazılmadı.",
			"Arkadaşın dosyanın adını değiştirip tekrar denesin.",
		}
	case has("stopped responding"):
		return "Karşı taraf yanıt vermeyi bıraktı.", retryTogether
	case has("connection lost", "stream reset", "connection closed", "acknowledgement",
		"no answer from receiver", "could not read manifest", "security handshake",
		"no confirmation from the other side"):
		return "Bağlantı transfer sırasında koptu.", retryTogether
	case has("too many failed lookups"):
		return "Çok fazla hatalı kod denendi.", []string{
			"Kodu arkadaşından yeniden iste ve dikkatle yaz.",
			"Bir dakika bekleyip ana menüden tekrar dene.",
		}
	case has("too many open rooms"):
		return "Aynı anda çok fazla gönderim başlattın.", []string{
			"Açık kalan pencerelerden birini kapatıp tekrar dene.",
		}
	case has("server is full", "server is busy"):
		return "Buluşma noktası şu an çok yoğun.", []string{
			"Birkaç dakika sonra tekrar dene.",
		}
	case has("already in use"):
		return "Bu kod şu an başkası tarafından kullanılıyor.", []string{
			"Tekrar dene — yeni bir kod üretilecek.",
		}
	// Keep this one late: "room" appears in several more specific messages
	// above, and a user who hits one of those needs to hear about that, not
	// about a code that was never opened.
	case has("not found", "room"):
		return "Bu kodla açılmış bir oda bulunamadı.", []string{
			"Kodu doğru yazdığından emin ol (üç parça, aralarında tire).",
			"Arkadaşının programı hâlâ açık ve bekliyor olmalı.",
			"Kod tek kullanımlık ve en fazla 1 saat geçerli; kullanıldıysa yeni kod istesin.",
		}
	case has("meeting point"):
		return "Buluşma noktasına ulaşılamadı.", []string{
			"İnternet bağlantını kontrol et.",
			"Birkaç dakika sonra tekrar dene.",
		}
	case has("could not connect to the other computer"):
		return "Arkadaşının bilgisayarına bağlanılamadı.", []string{
			"Arkadaşının programı açık ve bekliyor durumda mı, sor.",
			"İkiniz de tekrar deneyin — çoğu zaman ikinci denemede olur.",
		}
	case has("declined"):
		return "Karşı taraf transferi kabul etmedi.", nil
	case has("checksum"):
		return "Dosya eksik veya bozuk indi.", []string{
			"Transferi tekrar başlatın; bozuk dosya diske kaydedilmedi.",
		}
	case has("too many files"):
		return "Tek seferde gönderilemeyecek kadar çok dosya var.", []string{
			"Klasörü birkaç parçaya bölüp ayrı ayrı gönderin.",
		}
	case has("no files", "not an ordinary file"):
		return "Gönderilecek bir şey seçilmedi.", []string{
			"Listeden en az bir dosya ya da içi dolu bir klasör seç.",
		}
	case has("could not read", "could not open file"):
		return "Seçtiğin dosyalardan biri okunamadı.", []string{
			"Dosya yerinde duruyor mu ve açma iznin var mı, kontrol et.",
		}
	case has("no space left", "could not write to disk"):
		return "Diske yazılamadı.", []string{
			"Kaydedilecek yerde yeterli boş alan var mı, bak.",
			"Ana menüden başka bir indirme klasörü seçebilirsin.",
		}
	default:
		// Whatever this is, part of it may have come from the other side
		// or the server, so it is cleaned before it reaches the terminal.
		return safetext.Clean(err.Error(), 300), []string{"Tekrar denemek için Enter'a bas."}
	}
}

// codeProblem says what is wrong with a typed code, in words the user can
// act on without leaving the screen.
func codeProblem(err error) string {
	var ce *p2p.CodeError
	if errors.As(err, &ce) && ce.Word != "" {
		return fmt.Sprintf("\"%s\" kodlarda geçen bir kelime değil — yazımını kontrol et.", ce.Word)
	}
	return "Kod iki kelime ve bir sayıdan oluşur, örneğin kiraz-liman-42."
}

// DefaultOutDir picks the friendliest place to save incoming files.
func DefaultOutDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "received"
	}
	for _, name := range []string{"Downloads", "İndirilenler", "Indirilenler"} {
		p := filepath.Join(home, name)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return filepath.Join(p, "PureSend")
		}
	}
	return filepath.Join(home, "PureSend")
}

func truncate(s string, max int) string {
	// Slice by runes, not bytes: cutting a multi-byte character in half
	// (e.g. "fotoğraf.jpg") would print garbage.
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatRate(bytesPerSecond float64) string {
	switch {
	case bytesPerSecond >= 1<<20:
		return fmt.Sprintf("%.1f MB/sn", bytesPerSecond/(1<<20))
	case bytesPerSecond >= 1<<10:
		return fmt.Sprintf("%.0f KB/sn", bytesPerSecond/(1<<10))
	default:
		return fmt.Sprintf("%.0f B/sn", bytesPerSecond)
	}
}

// formatDuration rounds hard on purpose: "yaklaşık 3 dakika" is what
// someone wants to know, not "3 dakika 07 saniye".
func formatDuration(d time.Duration) string {
	switch {
	case d >= time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("~%d saat %d dakika", h, int(d.Minutes())-h*60)
	case d >= time.Minute:
		return fmt.Sprintf("~%d dakika", int(d.Minutes())+1)
	case d >= 10*time.Second:
		return fmt.Sprintf("~%d saniye", int(d.Seconds()))
	default:
		return "birkaç saniye"
	}
}
