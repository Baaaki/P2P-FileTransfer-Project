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

func FuzzClean(f *testing.F) {
	seeds := []struct {
		s   string
		max int
	}{
		{"plain text", 100},
		{"fotoğraf.jpg", 10},
		{"\x1b[2J\x1b[Hsilindi", 50},
		{"line\nbreak\rreturn", 5},
		{"photo\u202Egpj.exe", 20},
		{"a\u2066b\u2069c\u200Fd\u061Ce", 3},
		{"\u0085next line", 100},
		{"sep\u2028arator", 0},
		{"", 10},
		{"aşırı-uzun-dosya-adı-türkçe-karakterler-şçöğü", 15},
	}
	for _, tc := range seeds {
		f.Add(tc.s, tc.max)
	}

	f.Fuzz(func(t *testing.T, in string, max int) {
		if max < 0 || max > 5000 {
			return
		}
		got := Clean(in, max)

		// Invariant 1: Result must contain no unsafe runes.
		for _, r := range got {
			if IsUnsafe(r) {
				t.Fatalf("Clean(%q, %d) returned unsafe rune %U in %q", in, max, r, got)
			}
		}

		// Invariant 2: Result rune length must not exceed max + 1 (ellipsis).
		runes := []rune(got)
		if len(runes) > max+1 {
			t.Fatalf("Clean(%q, %d) returned %d runes, exceeds max+1", in, max, len(runes))
		}
	})
}

func BenchmarkClean_CleanText(b *testing.B) {
	text := "tatil-fotograflari_2026_istanbul_arsivi.tar.gz"
	b.ReportAllocs()
	for b.Loop() {
		_ = Clean(text, 100)
	}
}

func BenchmarkClean_UnsafeText(b *testing.B) {
	text := "\x1b[2J\x1b[Hphoto\u202Egpj.exe\u2066\u2069\r\n"
	b.ReportAllocs()
	for b.Loop() {
		_ = Clean(text, 100)
	}
}
