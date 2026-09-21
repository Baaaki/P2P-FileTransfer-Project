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

	"puresend/internal/i18n"
	"puresend/internal/p2p"
	"puresend/internal/transfer"
	"puresend/internal/update"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
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

// mode is what the client was asked to do on the welcome screen.
type mode int

const (
	modeNone mode = iota
	modeSend
	modeReceive
)

type pickedFile struct {
	path  string
	name  string
	size  int64
	isDir bool
	files int
}

// Config starts the interface with baked-in defaults.
type Config struct {
	// Servers are the meeting point addresses, tried in order.
	Servers []string
	// ServerList is where to look for more addresses when none of
	// Servers answers. Empty disables the fallback.
	ServerList string
	// OutDir overrides where incoming files are saved. Empty picks the
	// friendliest default.
	OutDir string
	// Version is the current version of the application.
	Version string
	// STUNServers overrides the default STUN servers used to discover WAN IP.
	STUNServers []string
	// Lang sets the initial interface language ("tr" or "en"). If empty, defaults to Turkish for compatibility or FT_LANG.
	Lang string
}

// Model is the whole application state.
type Model struct {
	servers     []string
	serverList  string
	stunServers []string
	version     string
	lang        i18n.Lang
	screen      screen
	mode        mode
	width       int
	height      int

	ctx    context.Context
	cancel context.CancelFunc
	node   *p2p.Node

	// welcome / confirm menus
	menuIndex    int
	confirmIndex int

	// sending
	picker       filepicker.Model
	picked       []pickedFile
	room         string
	prepared     bool   // every file has been read and the offer is ready
	serverLost   bool   // the meeting point dropped; the room is being put back
	notice       string // something worth knowing that needs no action
	updateNotice string // notice about an available software update
	updateTag    string // release tag of available update

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
	fp.KeyMap.Open = key.NewBinding(key.WithKeys("enter", "right"), key.WithHelp("enter", "open"))

	dp := filepicker.New()
	dp.CurrentDirectory = home
	dp.DirAllowed = false // Enter walks in; "s" picks where we already are
	dp.FileAllowed = false
	dp.ShowPermissions = false
	dp.ShowSize = false
	dp.SetHeight(10)
	dp.KeyMap.Open = key.NewBinding(key.WithKeys("enter", "right"), key.WithHelp("enter", "open"))

	l := i18n.Normalize(cfg.Lang)
	if cfg.Lang == "" {
		if env := os.Getenv("FT_LANG"); env != "" {
			l = i18n.Normalize(env)
		} else {
			l = i18n.TR
		}
	}

	ti := textinput.New()
	ti.Placeholder = i18n.Get(l).EnterPlaceholder
	ti.CharLimit = 64
	ti.Prompt = "  ➜  "

	outDir := cfg.OutDir
	if outDir == "" {
		outDir = DefaultOutDir()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		servers:     cfg.Servers,
		serverList:  cfg.ServerList,
		stunServers: cfg.STUNServers,
		version:     cfg.Version,
		lang:        l,
		screen:      screenWelcome,
		ctx:         ctx,
		cancel:      cancel,
		picker:      fp,
		dirPicker:   dp,
		codeInput:   ti,
		outDir:      outDir,
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
func connectCmd(ctx context.Context, servers []string, serverList string, stunServers []string) tea.Cmd {
	return func() tea.Msg {
		node, err := p2p.New(ctx, servers, p2p.WithServerList(serverList), p2p.WithSTUNServers(stunServers))
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

type updateAvailableMsg struct{ tag string }

func checkUpdateCmd(version string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		info, err := update.CheckLatest(ctx, version)
		if err != nil || info == nil || !info.HasUpdate {
			return nil
		}
		return updateAvailableMsg{tag: info.TagName}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.picker.Init(), tick(), checkUpdateCmd(m.version))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case updateAvailableMsg:
		m.updateTag = msg.tag
		if m.lang == i18n.EN {
			m.updateNotice = fmt.Sprintf("A new version is available (%s)! To update: puresend -update", msg.tag)
		} else {
			m.updateNotice = fmt.Sprintf("Yeni bir sürüm mevcut (%s)! Güncellemek için: puresend -update", msg.tag)
		}
		return m, nil

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
		m.screen = screenWaiting
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
		if e.Direct {
			m.relayLimit = 0
		} else {
			m.relayLimit = e.RelayLimit
		}
		m.haveConn = true
		m.warn = ""
		if m.screen != screenConfirm && m.screen != screenDone {
			m.screen = screenTransfer
		}
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
		if m.lang == i18n.EN {
			m.notice = "Someone tried to connect with an invalid code. Nothing was shared with them."
		} else {
			m.notice = "Birisi yanlış bir kodla bağlanmayı denedi. Ona hiçbir şey gösterilmedi."
		}
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
				if m.lang == i18n.EN {
					m.warn = "An attempt was interrupted. Your friend can retry with the same code."
				} else {
					m.warn = "Bir deneme yarıda kaldı. Arkadaşın aynı kodla tekrar deneyebilir."
				}
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
		case "l", "L":
			m.lang = i18n.Toggle(m.lang)
			m.codeInput.Placeholder = i18n.Get(m.lang).EnterPlaceholder
			if m.updateTag != "" {
				if m.lang == i18n.EN {
					m.updateNotice = fmt.Sprintf("A new version is available (%s)! To update: puresend -update", m.updateTag)
				} else {
					m.updateNotice = fmt.Sprintf("Yeni bir sürüm mevcut (%s)! Güncellemek için: puresend -update", m.updateTag)
				}
			}
			return m, nil
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
			return m, connectCmd(m.ctx, m.servers, m.serverList, m.stunServers)
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
		case "esc":
			m.screen = screenWelcome
			m.picked = nil
			return m, nil
		case "l", "L":
			m.lang = i18n.Toggle(m.lang)
			return m, nil
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
		case "l", "L":
			m.lang = i18n.Toggle(m.lang)
			return m, nil
		case "backspace", "left", "u":
			parent := filepath.Dir(m.dirPicker.CurrentDirectory)
			if parent != "" && parent != m.dirPicker.CurrentDirectory {
				m.dirPicker.CurrentDirectory = parent
				return m, m.dirPicker.Init()
			}
		}
		var cmd tea.Cmd
		m.dirPicker, cmd = m.dirPicker.Update(msg)
		return m, cmd

	case screenRoomCode, screenWaiting:
		switch msg.String() {
		case "l", "L":
			m.lang = i18n.Toggle(m.lang)
		}
		return m, nil

	case screenEnterCode:
		switch msg.String() {
		case "esc":
			m.screen = screenWelcome
			m.codeInput.Reset()
			m.codeErr = ""
			return m, nil
		case "enter":
			if strings.TrimSpace(m.codeInput.Value()) == "" {
				return m, nil
			}
			// Check the code here, before the server sees it: a typo caught
			// now costs nothing, one caught by the server costs one of the
			// few tries it allows.
			code, err := p2p.CheckCode(m.codeInput.Value())
			if err != nil {
				m.codeErr = codeProblem(err, m.lang)
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
		case "left", "h", "right":
			m.confirmIndex = 1 - m.confirmIndex
		case "l", "L":
			m.lang = i18n.Toggle(m.lang)
			return m, nil
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
		switch msg.String() {
		case "enter":
			return m.reset(), nil
		case "l", "L":
			m.lang = i18n.Toggle(m.lang)
			return m, nil
		case "q":
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
		if m.lang == i18n.EN {
			return "Goodbye!\n"
		}
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
	case screenRoomCode, screenWaiting:
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
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render("📦  PureSend") + "\n\n")
	b.WriteString(bodyStyle.Render(t.WelcomeHeadline) + "\n")
	b.WriteString(helpStyle.Render(t.WelcomeHelp1) + "\n")
	b.WriteString(helpStyle.Render(t.WelcomeHelp2) + "\n\n")
	b.WriteString(bodyStyle.Render(t.WelcomeQuestion) + "\n\n")

	opts := []string{
		t.WelcomeSend,
		t.WelcomeRecv,
		t.WelcomeChangeDir,
	}
	for i, o := range opts {
		if i == m.menuIndex {
			b.WriteString(choiceSelStyle.Render("▸ "+o) + "\n")
		} else {
			b.WriteString(choiceStyle.Render(o) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render(t.WelcomeSavingTo) + "\n")
	b.WriteString(fileStyle.Render(m.outDir) + "\n")
	if m.updateNotice != "" {
		b.WriteString("\n" + updateNoticeStyle.Render("✨ "+m.updateNotice) + "\n")
	}
	b.WriteString("\n" + formatFooter(t.WelcomeFooter))
	return b.String()
}

func (m Model) viewConnecting() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.ConnectingTitle) + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render(t.ConnectingStatus) + "\n\n")
	b.WriteString(helpStyle.Render(t.ConnectingHelp1) + "\n")
	b.WriteString(helpStyle.Render(t.ConnectingHelp2) + "\n")
	b.WriteString(helpStyle.Render(t.ConnectingHelp3) + "\n\n")
	b.WriteString(formatFooter(t.ConnectingFooter))
	return b.String()
}

func (m Model) viewPickFiles() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.PickTitle) + "\n\n")
	b.WriteString(helpStyle.Render(t.PickHelp1) + "\n")
	b.WriteString(helpStyle.Render(t.PickHelp2) + "\n")
	b.WriteString(helpStyle.Render(t.PickHelp3) + "\n\n")
	b.WriteString(m.picker.View() + "\n")

	if len(m.picked) > 0 {
		var total int64
		b.WriteString(okStyle.Render(t.PickSelected(len(m.picked))) + "\n")
		for _, f := range m.picked {
			label := "• " + f.name
			detail := formatBytes(f.size)
			if f.isDir {
				label = "• 📁 " + f.name
				detail = t.PickFolderFiles(f.files, formatBytes(f.size))
			}
			b.WriteString(fileStyle.Render(label) + " " + sizeStyle.Render("("+detail+")") + "\n")
			total += f.size
		}
		b.WriteString(sizeStyle.Render(t.PickTotal(formatBytes(total))) + "\n\n")
		b.WriteString(buttonSelStyle.Render(t.PickStartBtn) + "\n\n")
		b.WriteString(formatFooter(t.PickFooterSelected))
	} else {
		b.WriteString("\n" + helpStyle.Render(t.PickEmpty) + "\n\n")
		b.WriteString(formatFooter(t.PickFooterEmpty))
	}
	return b.String()
}

func (m Model) viewOutDir() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.OutDirTitle) + "\n\n")
	b.WriteString(helpStyle.Render(t.OutDirHelp1) + "\n")
	b.WriteString(helpStyle.Render(t.OutDirHelp2) + "\n")
	b.WriteString(helpStyle.Render(t.OutDirHelp3) + "\n\n")
	b.WriteString(bodyStyle.Render(t.OutDirCurrent) + "\n")
	b.WriteString(fileStyle.Render(m.dirPicker.CurrentDirectory) + "\n\n")
	b.WriteString(m.dirPicker.View() + "\n")
	b.WriteString(buttonSelStyle.Render(t.OutDirSelectBtn(filepath.Base(m.dirPicker.CurrentDirectory))) + "\n\n")
	b.WriteString(formatFooter(t.OutDirFooter))
	return b.String()
}

func (m Model) viewRoomCode() string {
	return m.viewWaiting()
}

func (m Model) viewWaiting() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.WaitingTitle) + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render(t.WaitingStatus) + "\n\n")
	if m.warn != "" {
		b.WriteString(warnStyle.Render("! "+m.warn) + "\n\n")
	}
	b.WriteString(codeStyle.Render(m.room) + "\n\n")

	if len(m.picked) > 0 {
		var total int64
		b.WriteString(bodyStyle.Render(t.WaitingFiles(len(m.picked))) + "\n")
		limit := 5
		for i, f := range m.picked {
			if i >= limit {
				remaining := len(m.picked) - limit
				b.WriteString(helpStyle.Render(t.WaitingMoreFiles(remaining)) + "\n")
				break
			}
			label := "• " + f.name
			detail := formatBytes(f.size)
			if f.isDir {
				label = "• 📁 " + f.name
				detail = t.PickFolderFiles(f.files, formatBytes(f.size))
			}
			b.WriteString(fileStyle.Render(label) + " " + sizeStyle.Render("("+detail+")") + "\n")
		}
		for _, f := range m.picked {
			total += f.size
		}
		b.WriteString(sizeStyle.Render(t.PickTotal(formatBytes(total))) + "\n\n")
	}

	b.WriteString(m.hostingNotes())
	if t.WaitingHelp1 != "" {
		b.WriteString(helpStyle.Render(t.WaitingHelp1) + "\n")
	}
	if t.WaitingHelp2 != "" {
		b.WriteString(helpStyle.Render(t.WaitingHelp2) + "\n\n")
	}
	if t.WaitingHelp3 != "" {
		b.WriteString(helpStyle.Render(t.WaitingHelp3) + "\n\n")
	}
	b.WriteString(formatFooter(t.WaitingFooter))
	return b.String()
}

// hostingNotes is what the sender needs to know while the code is out:
// how far along reading the files is, whether the meeting point is
// reachable, and whether anyone tried a wrong code.
func (m Model) hostingNotes() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	switch {
	case m.prepared:
		b.WriteString(okStyle.Render(t.HostReady) + "\n\n")
	case m.prepFiles > 0:
		b.WriteString(m.spinner() + " " + helpStyle.Render(t.HostPreparing(m.prepIndex, m.prepFiles)) + "\n\n")
	}
	if m.serverLost {
		b.WriteString(warnStyle.Render(t.HostLost) + "\n")
		b.WriteString(helpStyle.Render(t.HostLostHelp1) + "\n")
		b.WriteString(helpStyle.Render(t.HostLostHelp2) + "\n\n")
	}
	if m.notice != "" {
		b.WriteString(helpStyle.Render("• "+m.notice) + "\n\n")
	}
	return b.String()
}

func (m Model) viewEnterCode() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.EnterTitle) + "\n\n")
	b.WriteString(helpStyle.Render(t.EnterHelp1) + "\n")
	b.WriteString(helpStyle.Render(t.EnterHelp2) + "\n\n")
	b.WriteString(m.codeInput.View() + "\n\n")
	if m.codeErr != "" {
		b.WriteString(warnStyle.Render("! "+m.codeErr) + "\n\n")
	}
	b.WriteString(helpStyle.Render(t.EnterSavingTo) + "\n")
	b.WriteString(fileStyle.Render(m.outDir) + "\n\n")
	b.WriteString(formatFooter(t.EnterFooter))
	return b.String()
}

func (m Model) viewFinding() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.FindingTitle) + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render(m.statusText()) + "\n\n")
	b.WriteString(helpStyle.Render(t.FindingHelp1) + "\n")
	b.WriteString(helpStyle.Render(t.FindingHelp2) + "\n\n")
	b.WriteString(formatFooter(t.FindingFooter))
	return b.String()
}

// statusText turns the internal status into a friendly sentence.
func (m Model) statusText() string {
	t := i18n.Get(m.lang)
	switch m.status {
	case "looking up the code":
		return t.StatusLookingUp
	case "connecting to the other computer":
		return t.StatusConnecting
	case "opening a direct route":
		return t.StatusDirect
	default:
		return t.StatusDefault
	}
}

func (m Model) viewConfirm() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.ConfirmTitle) + "\n\n")

	// A folder of a thousand files would bury the question, so show the
	// first few and count the rest.
	const maxListed = 12
	files := m.manifest.Files
	for i, f := range files {
		if i == maxListed {
			b.WriteString(helpStyle.Render(t.ConfirmMoreFiles(len(files)-maxListed)) + "\n")
			break
		}
		b.WriteString(fileStyle.Render("• "+f.Path) + " " +
			sizeStyle.Render("("+formatBytes(f.Size)+")") + "\n")
	}

	total := m.manifest.TotalSize()
	b.WriteString("\n" + sizeStyle.Render(t.ConfirmTotal(len(files), formatBytes(total))) + "\n\n")

	if warning := m.relayWarning(total); warning != "" {
		b.WriteString(warnStyle.Render(warning) + "\n\n")
	}

	b.WriteString(helpStyle.Render(t.ConfirmDest) + "\n")
	b.WriteString(fileStyle.Render(m.outDir) + "\n\n")
	b.WriteString(bodyStyle.Render(t.ConfirmQuestion) + "\n\n")

	yes, no := buttonStyle.Render(t.ConfirmYes), buttonStyle.Render(t.ConfirmNo)
	if m.confirmIndex == 0 {
		yes = buttonSelStyle.Render(t.ConfirmYes)
	} else {
		no = buttonSelStyle.Render(t.ConfirmNo)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, yes, no) + "\n\n")
	b.WriteString(formatFooter(t.ConfirmFooter))
	return b.String()
}

// relayWarning is the one thing a user cannot work out for themselves: on
// the fallback route there is a hard size limit, and hitting it means the
// transfer breaks off near the end. Better to say so before it starts.
func (m Model) relayWarning(total int64) string {
	if m.direct || !m.haveConn || m.relayLimit <= 0 || total <= m.relayLimit {
		return ""
	}
	t := i18n.Get(m.lang)
	return t.RelayWarning(formatBytes(m.relayLimit))
}

func (m Model) viewTransfer() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	if m.mode == modeSend {
		b.WriteString(titleStyle.Render(t.TransferSendTitle) + "\n\n")
	} else {
		b.WriteString(titleStyle.Render(t.TransferRecvTitle) + "\n\n")
	}

	if m.haveConn {
		if m.direct {
			b.WriteString(okStyle.Render(t.TransferDirectOk) + "\n")
			b.WriteString(helpStyle.Render(t.TransferDirectHelp) + "\n\n")
		} else {
			b.WriteString(warnStyle.Render(t.TransferRelayWarn) + "\n")
			b.WriteString(helpStyle.Render(t.TransferRelayHelp1) + "\n")
			b.WriteString(helpStyle.Render(t.TransferRelayHelp2) + "\n\n")
		}
	}

	switch {
	case m.prog.Total > 0 || m.prog.OverallTotal > 0:
		b.WriteString(m.viewProgress())
	case m.prep != "":
		b.WriteString(m.spinner() + " " + helpStyle.Render(t.TransferPrepFiles(m.prepIndex, m.prepFiles)) + "\n")
		b.WriteString(fileStyle.Render(truncate(m.prep, 46)) + "\n\n")
	case m.mode == modeReceive && m.remoteTotal > 0:
		// A large folder takes the sender a while to read; say so, with
		// numbers, rather than an open-ended "preparing".
		b.WriteString(m.spinner() + " " + helpStyle.Render(t.TransferRemotePrep(
			min(m.remoteDone, m.remoteTotal), m.remoteTotal)) + "\n\n")
	default:
		b.WriteString(m.spinner() + " " + helpStyle.Render(t.TransferPreparing) + "\n\n")
	}
	b.WriteString(formatFooter(t.TransferFooter))
	return b.String()
}

// viewProgress draws the bar, and under it the two numbers people actually
// watch: how fast it is going and how long is left.
func (m Model) viewProgress() string {
	t := i18n.Get(m.lang)
	p := m.prog
	var b strings.Builder

	if p.Files > 1 {
		b.WriteString(helpStyle.Render(t.TransferFileIndex(p.Index, p.Files)) + "\n")
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
		line := formatRateLang(rate, m.lang)
		if eta, ok := m.meter.eta(total - done); ok {
			line += "  ·  " + t.TransferEta(formatDurationLang(eta, m.lang))
		}
		b.WriteString("  " + sizeStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewDone() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	if m.mode == modeSend {
		b.WriteString(okStyle.Render(t.DoneSendTitle) + "\n\n")
		b.WriteString(bodyStyle.Render(t.DoneSendBody) + "\n\n")
		b.WriteString(helpStyle.Render(t.DoneSendHelp) + "\n\n")
	} else {
		b.WriteString(okStyle.Render(t.DoneRecvTitle) + "\n\n")
		b.WriteString(bodyStyle.Render(t.DoneRecvBody) + "\n\n")
		const maxListed = 12
		for i, p := range m.savedPaths {
			if i == maxListed {
				b.WriteString(helpStyle.Render(t.DoneMoreFiles(len(m.savedPaths)-maxListed)) + "\n")
				break
			}
			b.WriteString(fileStyle.Render("• "+p) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render(t.DoneVerified) + "\n\n")
	}
	b.WriteString(formatFooter(t.DoneFooter))
	return b.String()
}

func (m Model) viewError() string {
	t := i18n.Get(m.lang)
	headline, hints := i18n.Explain(m.err, m.lang)

	var b strings.Builder
	b.WriteString(errStyle.Render(t.ErrorTitle) + "\n\n")
	b.WriteString(bodyStyle.Render(headline) + "\n\n")
	if len(hints) > 0 {
		b.WriteString(helpStyle.Render(t.ErrorWhatCan) + "\n")
		for _, h := range hints {
			b.WriteString(helpStyle.Render("  • "+h) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(formatFooter(t.ErrorFooter))
	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// explain turns a technical error into a plain-language headline and a
// short list of things the user can actually do about it.
func explain(err error) (string, []string) {
	return i18n.Explain(err, i18n.TR)
}

// codeProblem says what is wrong with a typed code, in words the user can
// act on without leaving the screen.
func codeProblem(err error, lang i18n.Lang) string {
	var ce *p2p.CodeError
	if errors.As(err, &ce) && ce.Word != "" {
		if lang == i18n.EN {
			return fmt.Sprintf("\"%s\" is not a recognized word in room codes — check the spelling.", ce.Word)
		}
		return fmt.Sprintf("\"%s\" kodlarda geçen bir kelime değil — yazımını kontrol et.", ce.Word)
	}
	if lang == i18n.EN {
		return "Room code consists of two words and a number, e.g. cherry-harbor-42."
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
	return formatRateLang(bytesPerSecond, i18n.TR)
}

func formatRateLang(bytesPerSecond float64, lang i18n.Lang) string {
	sec := "sn"
	if lang == i18n.EN {
		sec = "s"
	}
	switch {
	case bytesPerSecond >= 1<<20:
		return fmt.Sprintf("%.1f MB/%s", bytesPerSecond/(1<<20), sec)
	case bytesPerSecond >= 1<<10:
		return fmt.Sprintf("%.0f KB/%s", bytesPerSecond/(1<<10), sec)
	default:
		return fmt.Sprintf("%.0f B/%s", bytesPerSecond, sec)
	}
}

// formatDuration rounds hard on purpose: "yaklaşık 3 dakika" is what
// someone wants to know, not "3 dakika 07 saniye".
func formatDuration(d time.Duration) string {
	return formatDurationLang(d, i18n.TR)
}

func formatDurationLang(d time.Duration, lang i18n.Lang) string {
	if lang == i18n.EN {
		switch {
		case d >= time.Hour:
			h := int(d.Hours())
			return fmt.Sprintf("~%d hr %d min", h, int(d.Minutes())-h*60)
		case d >= time.Minute:
			return fmt.Sprintf("~%d min", int(d.Minutes())+1)
		case d >= 10*time.Second:
			return fmt.Sprintf("~%d sec", int(d.Seconds()))
		default:
			return "a few seconds"
		}
	}
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
