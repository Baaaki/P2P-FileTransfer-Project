package rendezvous

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewRoomCodeFormat(t *testing.T) {
	for range 200 {
		code := NewRoomCode()
		if !ValidCode(code) {
			t.Fatalf("NewRoomCode made an invalid code: %q", code)
		}
		if w := UnknownWord(code); w != "" {
			t.Fatalf("NewRoomCode used a word not in the list: %q in %q", w, code)
		}
	}
}

// TestWordList holds the list to the rules that make a code easy to pass
// on by voice. Each one exists because breaking it costs someone a retry.
func TestWordList(t *testing.T) {
	// Fewer words would shrink the code space and make guessing easier.
	if len(words) != 256 {
		t.Errorf("word list has %d entries, want 256", len(words))
	}
	plain := regexp.MustCompile(`^[a-z]{3,9}$`)
	seen := make(map[string]bool, len(words))
	for _, w := range words {
		if seen[w] {
			t.Errorf("duplicate word: %q", w)
		}
		seen[w] = true
		// Only letters every keyboard has: no ç ğ ı ö ş ü to get wrong.
		if !plain.MatchString(w) {
			t.Errorf("word %q is not 3-9 plain lowercase letters", w)
		}
	}
	for i, a := range words {
		for _, b := range words[i+1:] {
			// One letter apart is one misheard letter apart: "liman" and
			// "limon" over a bad phone line.
			if oneEditApart(a, b) {
				t.Errorf("%q and %q are one letter apart", a, b)
			}
			// A word that starts another invites a half-heard code.
			if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
				t.Errorf("%q and %q start the same way", a, b)
			}
		}
	}
}

func oneEditApart(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	switch len(b) - len(a) {
	case 0:
		diff := 0
		for i := range a {
			if a[i] != b[i] {
				diff++
			}
		}
		return diff <= 1
	case 1:
		for i := range b {
			if b[:i]+b[i+1:] == a {
				return true
			}
		}
	}
	return false
}

// TestNormalizeCode covers what people actually type. A capital letter or
// a space used to be "room not found".
func TestNormalizeCode(t *testing.T) {
	for in, want := range map[string]string{
		"kiraz-liman-42":       "kiraz-liman-42",
		"Kiraz-Liman-42":       "kiraz-liman-42",
		"KİRAZ-LİMAN-42":       "kiraz-liman-42",
		"KIRAZ LIMAN 42":       "kiraz-liman-42",
		"kıraz lıman 42":       "kiraz-liman-42",
		"  kiraz  liman  42  ": "kiraz-liman-42",
		"kiraz_liman.42":       "kiraz-liman-42",
		"kiraz--liman--42":     "kiraz-liman-42",
		"kiraz liman42":        "kiraz-liman-42",
		"kiraz-liman-42.":      "kiraz-liman-42",
		"çiçek-göl-11":         "cicek-gol-11",
		"":                     "",
		"---":                  "",
	} {
		if got := NormalizeCode(in); got != want {
			t.Errorf("NormalizeCode(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NormalizeCode(strings.Repeat("a", 10_000)); len(got) > 2*maxCodeLen+1 {
		t.Errorf("NormalizeCode kept %d bytes of an absurd input", len(got))
	}
}

func TestValidCode(t *testing.T) {
	for code, want := range map[string]bool{
		"kiraz-liman-42":  true,
		"kiraz-liman-4":   false,
		"kiraz-liman":     false,
		"kiraz-liman-42-": false,
		"Kiraz-liman-42":  false,
		"kiraz-liman-4a":  false,
	} {
		if got := ValidCode(code); got != want {
			t.Errorf("ValidCode(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestUnknownWord(t *testing.T) {
	if w := UnknownWord("kiraz-liman-42"); w != "" {
		t.Errorf("UnknownWord flagged %q in a good code", w)
	}
	if w := UnknownWord("kirez-liman-42"); w != "kirez" {
		t.Errorf("UnknownWord = %q, want %q", w, "kirez")
	}
	if w := UnknownWord("kiraz-limen-42"); w != "limen" {
		t.Errorf("UnknownWord = %q, want %q", w, "limen")
	}
}
