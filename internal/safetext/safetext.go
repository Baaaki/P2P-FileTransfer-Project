// Package safetext keeps text that arrived from someone else from doing
// anything but read as text once it reaches a terminal.
//
// A file name, an error message from the other peer, a reason given by the
// meeting point — all of it is chosen by a party this program does not
// trust, and all of it ends up printed. A terminal treats control
// characters as commands: an escape sequence in a file name can repaint
// the approval screen, move the cursor over the file list, or rewrite the
// title bar. Bidirectional overrides do the same to the eye, turning
// "photo\u202Egpj.exe" into something that reads as "photoexe.jpg".
package safetext

import (
	"strings"
	"unicode"
)

// IsUnsafe reports whether r has no business in a name or a message: a
// control character (C0, DEL or C1) or a bidirectional formatting mark.
func IsUnsafe(r rune) bool {
	return unicode.IsControl(r) ||
		unicode.Is(unicode.Bidi_Control, r) ||
		r == '\u2028' || r == '\u2029' // line and paragraph separators
}

// Clean drops every unsafe rune from s and cuts it to at most max runes,
// so a hostile message can neither steer the terminal nor bury the screen.
func Clean(s string, max int) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if IsUnsafe(r) {
			continue
		}
		if n == max {
			b.WriteString("…")
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
