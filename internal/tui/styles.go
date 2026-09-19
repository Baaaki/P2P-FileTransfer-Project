package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Adaptive colours so the app is readable on both light and dark
// terminal themes.
var (
	colAccent  = lipgloss.AdaptiveColor{Light: "#6C3DD1", Dark: "#B69BFF"}
	colOK      = lipgloss.AdaptiveColor{Light: "#127A3E", Dark: "#5BD98A"}
	colWarn    = lipgloss.AdaptiveColor{Light: "#9A6100", Dark: "#F0BC5E"}
	colErr     = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF8A80"}
	colMuted   = lipgloss.AdaptiveColor{Light: "#5F6368", Dark: "#9AA0A6"}
	colFaint   = lipgloss.AdaptiveColor{Light: "#9AA0A6", Dark: "#5F6368"}
	colDefault = lipgloss.AdaptiveColor{Light: "#1F1F1F", Dark: "#E8EAED"}
)

var (
	// titleStyle heads every screen.
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)

	// bodyStyle is the plain-language explanation under each title.
	bodyStyle = lipgloss.NewStyle().Foreground(colDefault)

	// helpStyle is the secondary explanation ("what is happening").
	helpStyle = lipgloss.NewStyle().Foreground(colMuted)

	// footerStyle lists the keys that work on this screen.
	footerStyle = lipgloss.NewStyle().Foreground(colFaint)

	// keyBadgeStyle highlights key shortcuts like [Enter] or [L].
	keyBadgeStyle = lipgloss.NewStyle().Foreground(colDefault).Bold(true)

	okStyle           = lipgloss.NewStyle().Bold(true).Foreground(colOK)
	warnStyle         = lipgloss.NewStyle().Foreground(colWarn)
	errStyle          = lipgloss.NewStyle().Bold(true).Foreground(colErr)
	updateNoticeStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	// choiceStyle / choiceSelStyle render a menu entry.
	choiceStyle    = lipgloss.NewStyle().Foreground(colDefault).PaddingLeft(2)
	choiceSelStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent).PaddingLeft(0)

	// codeStyle is the big room code box — the one thing on screen the
	// user has to read out to another person, so it gets the most weight.
	codeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colAccent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(1, 4).
			MarginLeft(2)

	// buttonStyle / buttonSelStyle render the "press Enter to continue"
	// affordance.
	buttonStyle = lipgloss.NewStyle().
			Foreground(colMuted).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colFaint).
			Padding(0, 3).
			MarginLeft(2)

	buttonSelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colAccent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 3).
			MarginLeft(2)

	// appStyle pads the whole screen.
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	// fileStyle lists a selected or incoming file.
	fileStyle = lipgloss.NewStyle().Foreground(colDefault).PaddingLeft(2)
	sizeStyle = lipgloss.NewStyle().Foreground(colMuted)
)

// spinnerFrames is a simple braille spinner; drawn by hand so the app
// keeps working on terminals where fancier components misbehave.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// progressBar renders a fixed-width bar, e.g. [██████░░░░░░].
func progressBar(done, total int64, width int) string {
	if total <= 0 {
		total = 1
	}
	filled := min(max(int(float64(done)/float64(total)*float64(width)), 0), width)
	bar := lipgloss.NewStyle().Foreground(colAccent).Render(repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(colFaint).Render(repeat("░", width-filled))
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

// formatFooter highlights key brackets [Key] within the footer text so that
// shortcuts clearly stand out from descriptions.
func formatFooter(s string) string {
	if !strings.Contains(s, "[") {
		return footerStyle.Render(s)
	}
	var b strings.Builder
	for {
		start := strings.IndexByte(s, '[')
		if start == -1 {
			b.WriteString(footerStyle.Render(s))
			break
		}
		end := strings.IndexByte(s[start:], ']')
		if end == -1 {
			b.WriteString(footerStyle.Render(s))
			break
		}
		end += start
		if start > 0 {
			b.WriteString(footerStyle.Render(s[:start]))
		}
		b.WriteString(keyBadgeStyle.Render(s[start : end+1]))
		s = s[end+1:]
	}
	return b.String()
}

