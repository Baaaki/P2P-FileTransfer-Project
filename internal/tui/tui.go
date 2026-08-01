// Package tui is the terminal interface a non-technical person uses.
//
// The guiding rule: never show a technical term. The user is walked
// through one question at a time, in plain Turkish, and is told what is
// happening and what to do next on every screen. Words like "multiaddr",
// "peer", "NAT" or "relay" never reach the screen.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"puresend/internal/p2p"
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
)

type mode int

const (
	modeNone mode = iota
	modeSend
	modeReceive
)

// pickedFile is one file the sender chose.
type pickedFile struct {
	path string
	name string
	size int64
}

// Model is the whole application state.
type Model struct {
	serverAddr string
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
	picker filepicker.Model
	picked []pickedFile
	room   string

	// receiving
	codeInput textinput.Model
	outDir    string

	// transfer state
	direct     bool
	haveConn   bool
	progName   string
	progDone   int64
	progTotal  int64
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
func New(serverAddr string) Model {
	fp := filepicker.New()
	fp.CurrentDirectory, _ = os.UserHomeDir()
	fp.DirAllowed = false
	fp.FileAllowed = true
	fp.ShowPermissions = false
	fp.SetHeight(10)

	ti := textinput.New()
	ti.Placeholder = "kiraz-liman-42"
	ti.CharLimit = 64
	ti.Prompt = "  ➜  "

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		serverAddr: serverAddr,
		screen:     screenWelcome,
		ctx:        ctx,
		cancel:     cancel,
		picker:     fp,
		codeInput:  ti,
		outDir:     defaultOutDir(),
	}
}

// Run starts the interface and blocks until the user quits.
func Run(serverAddr string) error {
	m := New(serverAddr)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	m.cancel()
	if fm, ok := final.(Model); ok && fm.node != nil {
		fm.node.Close()
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
func connectCmd(ctx context.Context, addr string) tea.Cmd {
	return func() tea.Msg {
		node, err := p2p.New(ctx, addr)
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
func waitEvent(node *p2p.Node) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-node.Events()
		if !ok {
			return nil
		}
		return eventMsg{ev}
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
		switch m.mode {
		case modeSend:
			m.screen = screenPickFiles
			return m, m.picker.Init()
		case modeReceive:
			m.screen = screenEnterCode
			return m, m.codeInput.Focus()
		}
		return m, nil

	case roomReadyMsg:
		m.room = msg.room
		m.screen = screenRoomCode
		return m, waitEvent(m.node)

	case eventMsg:
		return m.handleEvent(msg.ev)

	case errMsg:
		m.err = msg.err
		m.screen = screenError
		return m, nil
	}

	// Anything else goes to the active component.
	if m.screen == screenPickFiles {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
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
		m.haveConn = true
		m.warn = ""
		m.screen = screenTransfer
		return m, waitEvent(m.node)

	case p2p.ManifestEvent:
		m.manifest = e.Manifest
		m.reply = e.Reply
		m.confirmIndex = 0
		m.screen = screenConfirm
		return m, waitEvent(m.node)

	case p2p.ProgressEvent:
		m.progName, m.progDone, m.progTotal = e.Name, e.Done, e.Total
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
				m.progTotal = 0
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
			m.menuIndex = 0
		case "down", "j":
			m.menuIndex = 1
		case "enter":
			if m.menuIndex == 0 {
				m.mode = modeSend
			} else {
				m.mode = modeReceive
			}
			m.screen = screenConnecting
			return m, connectCmd(m.ctx, m.serverAddr)
		case "q":
			m.quitted = true
			m.cancel()
			return m, tea.Quit
		}
		return m, nil

	case screenPickFiles:
		// "s" starts the transfer; checked before the browser sees the
		// key so navigation is unaffected.
		if msg.String() == "s" && len(m.picked) > 0 {
			m.screen = screenConnecting
			return m, hostCmd(m.ctx, m.node, m.picked)
		}
		if msg.String() == "backspace" && len(m.picked) > 0 {
			m.picked = m.picked[:len(m.picked)-1]
			return m, nil
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		if ok, path := m.picker.DidSelectFile(msg); ok {
			m.addFile(path)
		}
		return m, cmd

	case screenRoomCode:
		if msg.String() == "enter" {
			m.screen = screenWaiting
		}
		return m, nil

	case screenEnterCode:
		if msg.String() == "enter" {
			code := strings.TrimSpace(m.codeInput.Value())
			if code == "" {
				return m, nil
			}
			m.screen = screenFinding
			m.status = "arkadaşın aranıyor"
			return m, tea.Batch(fetchCmd(m.ctx, m.node, code, m.outDir), waitEvent(m.node))
		}
		var cmd tea.Cmd
		m.codeInput, cmd = m.codeInput.Update(msg)
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

// answerConfirm sends the user's decision back to the transfer.
func (m Model) answerConfirm(ok bool) (tea.Model, tea.Cmd) {
	if m.reply != nil {
		m.reply <- ok
		m.reply = nil
	}
	if ok {
		m.screen = screenTransfer
	} else {
		m.screen = screenWelcome
		m.mode = modeNone
	}
	return m, waitEvent(m.node)
}

// addFile appends a chosen file, skipping duplicates.
func (m *Model) addFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	for _, f := range m.picked {
		if f.path == path {
			return
		}
	}
	m.picked = append(m.picked, pickedFile{path: path, name: info.Name(), size: info.Size()})
}

// reset returns to the main menu, keeping the open connection.
func (m Model) reset() Model {
	if m.mode == modeSend && m.node != nil {
		m.node.StopHosting()
	}
	m.screen = screenWelcome
	m.mode = modeNone
	m.picked = nil
	m.room = ""
	m.direct = false
	m.haveConn = false
	m.progName, m.progDone, m.progTotal = "", 0, 0
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

	opts := []string{"📤  Dosya göndereceğim", "📥  Bana dosya gönderilecek"}
	for i, o := range opts {
		if i == m.menuIndex {
			b.WriteString(choiceSelStyle.Render("▸ "+o) + "\n")
		} else {
			b.WriteString(choiceStyle.Render(o) + "\n")
		}
	}
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
	b.WriteString(titleStyle.Render("📤  Hangi dosyaları göndereceksin?") + "\n\n")
	b.WriteString(helpStyle.Render("Klasörlerin içine girmek için Enter'a bas. Göndermek") + "\n")
	b.WriteString(helpStyle.Render("istediğin dosyanın üzerinde Enter'a basınca listeye eklenir.") + "\n\n")
	b.WriteString(m.picker.View() + "\n")

	if len(m.picked) > 0 {
		var total int64
		b.WriteString(okStyle.Render(fmt.Sprintf("Seçtiklerin (%d):", len(m.picked))) + "\n")
		for _, f := range m.picked {
			b.WriteString(fileStyle.Render("• "+f.name) + " " +
				sizeStyle.Render("("+formatBytes(f.size)+")") + "\n")
			total += f.size
		}
		b.WriteString(sizeStyle.Render("  Toplam: "+formatBytes(total)) + "\n\n")
		b.WriteString(buttonSelStyle.Render("s  ·  Göndermeye başla") + "\n\n")
		b.WriteString(footerStyle.Render("↑ ↓ gez  ·  Enter seç  ·  Backspace son seçimi sil  ·  Ctrl+C çık"))
	} else {
		b.WriteString("\n" + helpStyle.Render("Henüz dosya seçmedin.") + "\n\n")
		b.WriteString(footerStyle.Render("↑ ↓ gez  ·  Enter seç  ·  Ctrl+C çık"))
	}
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
	b.WriteString(helpStyle.Render("Bu pencereyi kapatma. Arkadaşın kodu girdiği anda") + "\n")
	b.WriteString(helpStyle.Render("gönderme kendiliğinden başlayacak.") + "\n\n")
	b.WriteString(helpStyle.Render("Kod 1 saat geçerli.") + "\n\n")
	b.WriteString(footerStyle.Render("Ctrl+C ile vazgeç"))
	return b.String()
}

func (m Model) viewEnterCode() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📥  Arkadaşının verdiği kodu yaz") + "\n\n")
	b.WriteString(helpStyle.Render("Arkadaşın sana 3 parçalı bir kod verdi.") + "\n")
	b.WriteString(helpStyle.Render("Şuna benziyor: kiraz-liman-42") + "\n\n")
	b.WriteString(m.codeInput.View() + "\n\n")
	b.WriteString(footerStyle.Render("Enter ile devam et  ·  Ctrl+C ile çık"))
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

	var total int64
	for _, f := range m.manifest.Files {
		b.WriteString(fileStyle.Render("• "+f.Name) + " " +
			sizeStyle.Render("("+formatBytes(f.Size)+")") + "\n")
		total += f.Size
	}
	b.WriteString("\n" + sizeStyle.Render(fmt.Sprintf("  Toplam: %d dosya, %s",
		len(m.manifest.Files), formatBytes(total))) + "\n\n")
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

	if m.progTotal > 0 {
		pct := m.progDone * 100 / m.progTotal
		b.WriteString(fileStyle.Render(truncate(m.progName, 40)) + "\n")
		b.WriteString("  " + progressBar(m.progDone, m.progTotal, 30) +
			fmt.Sprintf("  %3d%%  ", pct) +
			sizeStyle.Render(formatBytes(m.progDone)+" / "+formatBytes(m.progTotal)) + "\n\n")
	} else {
		b.WriteString(m.spinner() + " " + helpStyle.Render("Hazırlanıyor...") + "\n\n")
	}
	b.WriteString(footerStyle.Render("Ctrl+C ile vazgeç"))
	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder
	if m.mode == modeSend {
		b.WriteString(okStyle.Render("✓  Gönderildi!") + "\n\n")
		b.WriteString(bodyStyle.Render(fmt.Sprintf(
			"%d dosya arkadaşına ulaştı ve eksiksiz indiği doğrulandı.", len(m.picked))) + "\n\n")
	} else {
		b.WriteString(okStyle.Render("✓  İndi!") + "\n\n")
		b.WriteString(bodyStyle.Render("Dosyalar şuraya kaydedildi:") + "\n\n")
		for _, p := range m.savedPaths {
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
func explain(err error) (string, []string) {
	if err == nil {
		return "Bilinmeyen bir hata oldu.", nil
	}
	s := strings.ToLower(err.Error())

	switch {
	case strings.Contains(s, "not found") || strings.Contains(s, "room"):
		return "Bu kodla açılmış bir oda bulunamadı.", []string{
			"Kodu doğru yazdığından emin ol (üç parça, aralarında tire).",
			"Arkadaşının programı hâlâ açık ve bekliyor olmalı.",
			"Kodlar 1 saat sonra geçersiz olur; süresi geçtiyse yeni kod istesin.",
		}
	case strings.Contains(s, "meeting point"):
		return "Buluşma noktasına ulaşılamadı.", []string{
			"İnternet bağlantını kontrol et.",
			"Birkaç dakika sonra tekrar dene.",
		}
	case strings.Contains(s, "could not connect to the other computer"):
		return "Arkadaşının bilgisayarına bağlanılamadı.", []string{
			"Arkadaşının programı açık ve bekliyor durumda mı, sor.",
			"İkiniz de tekrar deneyin — çoğu zaman ikinci denemede olur.",
		}
	case strings.Contains(s, "declined"):
		return "Karşı taraf transferi kabul etmedi.", nil
	case strings.Contains(s, "checksum"):
		return "Dosya eksik veya bozuk indi.", []string{
			"Transferi tekrar başlatın; bozuk dosya diske kaydedilmedi.",
		}
	case strings.Contains(s, "no files"):
		return "Hiç dosya seçmedin.", []string{"Listeden en az bir dosya seç."}
	default:
		return err.Error(), []string{"Tekrar denemek için Enter'a bas."}
	}
}

// defaultOutDir picks the friendliest place to save incoming files.
func defaultOutDir() string {
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
