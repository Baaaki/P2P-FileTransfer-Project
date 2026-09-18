package safetext

import "testing"

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"plain text":                    "plain text",
		"fotoğraf.jpg":                  "fotoğraf.jpg",
		"\x1b[2J\x1b[Hsilindi":          "[2J[Hsilindi", // the escape is what makes it a command
		"line\nbreak\rreturn":           "linebreakreturn",
		"photo\u202Egpj.exe":            "photogpj.exe",
		"a\u2066b\u2069c\u200Fd\u061Ce": "abcde",
		"\u0085next line":               "next line", // C1 control
		"sep\u2028arator":               "separator",
		"":                              "",
	} {
		if got := Clean(in, 100); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanCutsLongText(t *testing.T) {
	if got := Clean("abcdefgh", 3); got != "abc…" {
		t.Errorf("Clean cut to %q, want %q", got, "abc…")
	}
	if got := Clean("çğış", 4); got != "çğış" {
		t.Errorf("Clean cut a string that fits: %q", got)
	}
}
