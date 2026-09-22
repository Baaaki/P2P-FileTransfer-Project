package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"puresend/internal/i18n"
	"puresend/internal/transfer"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.quitted {
		return i18n.Get(m.lang).Goodbye + "\n"
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
	if m.updateTag != "" {
		b.WriteString("\n" + updateNoticeStyle.Render("✨ "+t.UpdateAvailable(m.updateTag)) + "\n")
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

func (m Model) viewWaiting() string {
	t := i18n.Get(m.lang)
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.WaitingTitle) + "\n\n")
	b.WriteString(m.spinner() + " " + bodyStyle.Render(t.WaitingStatus) + "\n\n")
	if m.retrying {
		b.WriteString(warnStyle.Render("! "+t.HostRetry) + "\n\n")
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
	if m.wrongLeft > 0 {
		b.WriteString(helpStyle.Render("• "+t.HostRejected(m.wrongLeft)) + "\n\n")
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

	// A folder of a thousand files would bury the question. A short list
	// is shown whole; a long one is shown as what it would put directly
	// into the download folder — a few folders, usually — since that is
	// where a file could matter, with hidden ones first so that none can
	// sit unseen at the end of a list cut short.
	const maxListed = 12
	files := m.manifest.Files
	entries := topLevel(m.manifest)
	if len(files) <= maxListed {
		for _, f := range files {
			b.WriteString(fileStyle.Render("• "+f.Path) + " " +
				sizeStyle.Render("("+formatBytes(f.Size)+")") + "\n")
		}
	} else {
		for i, e := range entries {
			if i == maxListed {
				b.WriteString(helpStyle.Render(t.ConfirmMoreEntries(len(entries)-maxListed)) + "\n")
				break
			}
			if e.dir {
				b.WriteString(fileStyle.Render("• "+e.name+"/") + " " +
					sizeStyle.Render("("+t.ConfirmFolder(e.files, formatBytes(e.size))+")") + "\n")
			} else {
				b.WriteString(fileStyle.Render("• "+e.name) + " " +
					sizeStyle.Render("("+formatBytes(e.size)+")") + "\n")
			}
		}
	}
	if hidden := hiddenNames(entries); hidden != "" {
		b.WriteString("\n" + warnStyle.Render(t.ConfirmHidden(hidden)) + "\n")
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

// topEntry is one thing a transfer would create directly in the download
// folder: a file, or a folder with everything under it.
type topEntry struct {
	name  string
	dir   bool
	files int
	size  int64
}

// topLevel groups a manifest by what it would create directly in the
// download folder: hidden entries first, manifest order otherwise.
func topLevel(m transfer.Manifest) []topEntry {
	index := make(map[string]int)
	var out []topEntry
	for _, f := range m.Files {
		// The first real part, the way the receiver will read the path:
		// "./.config/x" lands in ".config".
		var name string
		nested := false
		for part := range strings.SplitSeq(f.Path, "/") {
			if part == "" || part == "." {
				continue
			}
			if name != "" {
				nested = true
				break
			}
			name = part
		}
		i, ok := index[name]
		if !ok {
			i = len(out)
			index[name] = i
			out = append(out, topEntry{name: name})
		}
		out[i].dir = out[i].dir || nested
		out[i].files++
		out[i].size += f.Size
	}
	sort.SliceStable(out, func(a, b int) bool {
		return isHidden(out[a].name) && !isHidden(out[b].name)
	})
	return out
}

func isHidden(name string) bool { return strings.HasPrefix(name, ".") }

// hiddenNames lists the hidden entries for the warning, a few at most.
func hiddenNames(entries []topEntry) string {
	const most = 5
	var names []string
	for _, e := range entries {
		if !isHidden(e.name) {
			break // they come first
		}
		if len(names) == most {
			names = append(names, "…")
			break
		}
		if e.dir {
			names = append(names, e.name+"/")
		} else {
			names = append(names, e.name)
		}
	}
	return strings.Join(names, ", ")
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
