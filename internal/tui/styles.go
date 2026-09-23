package tui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// palette holds the app's colours for one kind of terminal background, so
// the app is readable on both light and dark themes.
type palette struct {
	accent, ok, warn, err, muted, faint, text color.Color
}

func newPalette(dark bool) palette {
	pick := lipgloss.LightDark(dark)
	return palette{
		accent: pick(lipgloss.Color("#6C3DD1"), lipgloss.Color("#B69BFF")),
		ok:     pick(lipgloss.Color("#127A3E"), lipgloss.Color("#5BD98A")),
		warn:   pick(lipgloss.Color("#9A6100"), lipgloss.Color("#F0BC5E")),
		err:    pick(lipgloss.Color("#B3261E"), lipgloss.Color("#FF8A80")),
		muted:  pick(lipgloss.Color("#5F6368"), lipgloss.Color("#9AA0A6")),
		faint:  pick(lipgloss.Color("#9AA0A6"), lipgloss.Color("#5F6368")),
		text:   pick(lipgloss.Color("#1F1F1F"), lipgloss.Color("#E8EAED")),
	}
}

// styles is every style the screens draw with, built for one background.
// The model starts with the dark set, which is what most terminals use,
// and swaps in the light one if the terminal reports a light background.
type styles struct {
	// title heads every screen.
	title lipgloss.Style

	// body is the plain-language explanation under each title.
	body lipgloss.Style

	// help is the secondary explanation ("what is happening").
	help lipgloss.Style

	// footer lists the keys that work on this screen.
	footer lipgloss.Style

	// keyBadge highlights key shortcuts like [Enter] or [L].
	keyBadge lipgloss.Style

	ok, warn, err, updateNotice lipgloss.Style

	// choice / choiceSel render a menu entry.
	choice, choiceSel lipgloss.Style

	// code is the big room code box — the one thing on screen the user
	// has to read out to another person, so it gets the most weight.
	code lipgloss.Style

	// button / buttonSel render the "press Enter to continue" affordance.
	button, buttonSel lipgloss.Style

	// app pads the whole screen.
	app lipgloss.Style

	// file lists a selected or incoming file.
	file, size lipgloss.Style

	// spinner, barDone and barLeft draw progress.
	spinner, barDone, barLeft lipgloss.Style
}

func newStyles(dark bool) styles {
	c := newPalette(dark)
	return styles{
		title:        lipgloss.NewStyle().Bold(true).Foreground(c.accent),
		body:         lipgloss.NewStyle().Foreground(c.text),
		help:         lipgloss.NewStyle().Foreground(c.muted),
		footer:       lipgloss.NewStyle().Foreground(c.faint),
		keyBadge:     lipgloss.NewStyle().Foreground(c.text).Bold(true),
		ok:           lipgloss.NewStyle().Bold(true).Foreground(c.ok),
		warn:         lipgloss.NewStyle().Foreground(c.warn),
		err:          lipgloss.NewStyle().Bold(true).Foreground(c.err),
		updateNotice: lipgloss.NewStyle().Foreground(c.accent).Bold(true),
		choice:       lipgloss.NewStyle().Foreground(c.text).PaddingLeft(2),
		choiceSel:    lipgloss.NewStyle().Bold(true).Foreground(c.accent).PaddingLeft(0),
		code: lipgloss.NewStyle().
			Bold(true).
			Foreground(c.accent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.accent).
			Padding(1, 4).
			MarginLeft(2),
		button: lipgloss.NewStyle().
			Foreground(c.muted).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.faint).
			Padding(0, 3).
			MarginLeft(2),
		buttonSel: lipgloss.NewStyle().
			Bold(true).
			Foreground(c.accent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.accent).
			Padding(0, 3).
			MarginLeft(2),
		app:     lipgloss.NewStyle().Padding(1, 2),
		file:    lipgloss.NewStyle().Foreground(c.text).PaddingLeft(2),
		size:    lipgloss.NewStyle().Foreground(c.muted),
		spinner: lipgloss.NewStyle().Foreground(c.accent),
		barDone: lipgloss.NewStyle().Foreground(c.accent),
		barLeft: lipgloss.NewStyle().Foreground(c.faint),
	}
}

// inputStyles styles the code entry box. The prompt and the cursor keep
// the terminal's own text colour: the component's defaults draw them in a
// light grey that all but disappears on a light background.
func inputStyles(dark bool) textinput.Styles {
	s := textinput.DefaultStyles(dark)
	s.Focused.Prompt = lipgloss.NewStyle()
	s.Blurred.Prompt = lipgloss.NewStyle()
	s.Cursor.Color = newPalette(dark).text
	return s
}

// spinnerFrames is a simple braille spinner; drawn by hand so the app
// keeps working on terminals where fancier components misbehave.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// progressBar renders a fixed-width bar, e.g. [██████░░░░░░].
func (s styles) progressBar(done, total int64, width int) string {
	if total <= 0 {
		total = 1
	}
	filled := min(max(int(float64(done)/float64(total)*float64(width)), 0), width)
	bar := s.barDone.Render(repeat("█", filled)) + s.barLeft.Render(repeat("░", width-filled))
	return "[" + bar + "]"
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}

// formatFooter formats shortcut hints into a 3-column layout.
// Up to 3 shortcuts per line, with additional keys wrapping to the next line.
// Columns are aligned so shortcuts line up cleanly.
func (s styles) formatFooter(str string) string {
	str = strings.ReplaceAll(str, "\n", " · ")
	rawItems := strings.Split(str, "·")
	var items []string
	for _, it := range rawItems {
		it = strings.TrimSpace(it)
		if it != "" {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return ""
	}

	const cols = 3
	colWidths := make([]int, cols)
	for i, it := range items {
		col := i % cols
		w := lipgloss.Width(it)
		if w > colWidths[col] {
			colWidths[col] = w
		}
	}

	var b strings.Builder
	for i, it := range items {
		col := i % cols
		if col > 0 {
			b.WriteString(s.footer.Render("  ·  "))
		}
		b.WriteString(s.formatKeyBadges(it))

		if col < cols-1 && i < len(items)-1 && (i+1)%cols != 0 {
			w := lipgloss.Width(it)
			if colWidths[col] > w {
				b.WriteString(strings.Repeat(" ", colWidths[col]-w))
			}
		}

		if col == cols-1 && i < len(items)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (s styles) formatKeyBadges(str string) string {
	if !strings.Contains(str, "[") {
		return s.footer.Render(str)
	}
	var b strings.Builder
	for {
		start := strings.IndexByte(str, '[')
		if start == -1 {
			b.WriteString(s.footer.Render(str))
			break
		}
		end := strings.IndexByte(str[start:], ']')
		if end == -1 {
			b.WriteString(s.footer.Render(str))
			break
		}
		end += start
		if start > 0 {
			b.WriteString(s.footer.Render(str[:start]))
		}
		b.WriteString(s.keyBadge.Render(str[start : end+1]))
		str = str[end+1:]
	}
	return b.String()
}
