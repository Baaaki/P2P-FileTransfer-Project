package rendezvous

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewSecretMakesValidCodes(t *testing.T) {
	for _, nameplate := range []string{"10", "42", "427", "9999"} {
		for range 50 {
			code := JoinCode(NewSecret(), nameplate)
			if !ValidCode(code) {
				t.Fatalf("NewSecret made an invalid code: %q", code)
			}
			if w := UnknownWord(code); w != "" {
				t.Fatalf("NewSecret used a word not in the list: %q in %q", w, code)
			}
			if got := Nameplate(code); got != nameplate {
				t.Fatalf("Nameplate(%q) = %q, want %q", code, got, nameplate)
			}
		}
	}
}

// TestSecretStaysOffTheNameplate: the nameplate is what the server is
// told, so nothing of the secret may end up in it.
func TestSecretStaysOffTheNameplate(t *testing.T) {
	code := JoinCode("kiraz-liman", "427")
	if got := Nameplate(code); got != "427" {
		t.Fatalf("Nameplate(%q) = %q, want %q", code, got, "427")
	}
	if strings.Contains(Nameplate(code), "kiraz") || strings.Contains(Nameplate(code), "liman") {
		t.Fatalf("the nameplate %q carries the secret", Nameplate(code))
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
		"kiraz-liman-42":     true,
		"kiraz-liman-427":    true,
		"kiraz-liman-9999":   true,
		"kiraz-liman-10000":  true,
		"kiraz-liman-123456": true,
		"kiraz-liman-4":      false,
		"kiraz-liman-042":    false,
		"kiraz-liman":        false,
		"kiraz-liman-42-":    false,
		"Kiraz-liman-42":     false,
		"kiraz-liman-4a":     false,
	} {
		if got := ValidCode(code); got != want {
			t.Errorf("ValidCode(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestValidNameplate(t *testing.T) {
	for np, want := range map[string]bool{
		"10":     true,
		"427":    true,
		"9999":   true,
		"10000":  true,
		"123456": true,
		"":       false,
		"7":      false,
		"042":    false,
		"4a":     false,
		"-42":    false,
		"42\n":   false,
		"kiraz":  false,
	} {
		if got := ValidNameplate(np); got != want {
			t.Errorf("ValidNameplate(%q) = %v, want %v", np, got, want)
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
