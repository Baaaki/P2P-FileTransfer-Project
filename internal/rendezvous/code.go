package rendezvous

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode"
)

// A room code is two words and a number: "kiraz-liman-42". It is what one
// person reads out to another over the phone, so everything about it is
// chosen for that: common Turkish words spelt only with the letters every
// keyboard has, none of them one letter away from another, none of them
// the start of another.
//
// 256 words, twice, and 90 numbers make 256 × 256 × 90 ≈ 5.9 million
// codes — about 22.5 bits. Not much on its own, which is why the transfer
// protocol turns the code into a key both sides must prove they hold, and
// why the server limits how fast anyone can guess: see Registry.lookup.
//
// Changing this list changes which codes a client accepts. A word that is
// dropped stops working in any code an older copy of the program hands out.
var words = []string{
	"ada", "ahtapot", "akik", "anahtar", "ananas", "araba", "armut", "arpa",
	"aslan", "atmaca", "ayna", "ayran", "badem", "bahar", "baklava", "balina",
	"balon", "bambu", "bamya", "bardak", "bayrak", "beyaz", "bezelye", "biber",
	"bilet", "bilmece", "bilye", "bisiklet", "bordo", "boza", "bronz", "bulgur",
	"bulut", "buzul", "cetvel", "ceviz", "ceylan", "damla", "dans", "davul",
	"defne", "defter", "demir", "demlik", "deniz", "dere", "dolap", "dolma",
	"domates", "dut", "efsane", "ekmek", "eldiven", "elma", "enginar", "erik",
	"fasulye", "fener", "fil", "fincan", "flamingo", "fok", "gece", "gemi",
	"geyik", "gezegen", "gitar", "hamam", "hamsi", "hardal", "harita", "havlu",
	"havuz", "hediye", "helva", "heykel", "hindi", "horoz", "hurma", "ihlamur",
	"incir", "inek", "iskele", "ispanak", "kadife", "kahve", "kalem", "kalkan",
	"kamera", "kamyon", "kanarya", "kandil", "kanguru", "kaplan", "karaca", "karanfil",
	"karga", "karides", "karpuz", "kartal", "kasaba", "kavak", "kavun", "kaya",
	"kaykay", "kaymak", "kedi", "kekik", "kelebek", "keman", "kemer", "kereviz",
	"kestane", "keten", "kilit", "kiraz", "kirpi", "kitap", "kivi", "koala",
	"kova", "koyun", "kristal", "kukla", "kule", "kumru", "kumsal", "kupa",
	"kurabiye", "kurt", "kutu", "lacivert", "ladin", "lahana", "lale", "lamba",
	"lavanta", "leylek", "liman", "lodos", "lokum", "madalya", "makas", "mango",
	"mantar", "marul", "masal", "mavi", "maymun", "mehtap", "mektup", "melodi",
	"meltem", "mendil", "menemen", "mercan", "mermer", "metro", "meydan", "midye",
	"mor", "muz", "nane", "nar", "nehir", "nergis", "nohut", "nota",
	"okyanus", "orkide", "orman", "oyuncak", "palamut", "palmiye", "pamuk", "pancar",
	"panda", "papatya", "park", "pasta", "patates", "paten", "pazar", "pekmez",
	"pelikan", "pembe", "pencere", "penguen", "perde", "peynir", "pide", "pilav",
	"piyano", "portakal", "poyraz", "pusula", "radyo", "resim", "ritim", "roka",
	"roket", "saat", "sabah", "sabun", "safir", "safran", "salep", "sandalye",
	"saray", "sedef", "sepet", "silgi", "simit", "sincap", "sis", "sokak",
	"somon", "sucuk", "susam", "tabak", "tablo", "tahin", "tavuk", "telefon",
	"tepe", "termos", "tilki", "timsah", "toprak", "tramvay", "tren", "trompet",
	"turna", "turp", "turuncu", "ufuk", "uskumru", "vadi", "valiz", "vapur",
	"volkan", "yakut", "yaprak", "yayla", "yelken", "yonca", "yorgan", "yosun",
	"yulaf", "yunus", "zambak", "zarf", "zebra", "zencefil", "zeytin", "zirve",
}

// wordSet is words, for looking a word up.
var wordSet = func() map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}()

// codeFormat is the shape every code has. The server holds any code to it,
// which caps what a room name can be at a few dozen bytes of letters.
var codeFormat = regexp.MustCompile(`^[a-z]+-[a-z]+-[0-9]{2}$`)

// maxCodeLen is comfortably more than two of the longest words and a number.
const maxCodeLen = 32

// NewRoomCode returns an easy-to-read room code like "kiraz-liman-42".
func NewRoomCode() string {
	return fmt.Sprintf("%s-%s-%d", pickWord(), pickWord(), pickNumber())
}

func pickWord() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	return words[n.Int64()]
}

func pickNumber() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(90))
	return n.Int64() + 10 // range 10-99
}

// ValidCode reports whether s has the shape of a room code.
func ValidCode(s string) bool {
	return len(s) <= maxCodeLen && codeFormat.MatchString(s)
}

// UnknownWord returns the first word of a well-formed code that is not in
// the list, or "" if both are. A code with a word we never hand out cannot
// be anyone's room, and saying which word is wrong beats "not found".
func UnknownWord(code string) string {
	parts := strings.Split(code, "-")
	if len(parts) != 3 {
		return ""
	}
	for _, w := range parts[:2] {
		if !wordSet[w] {
			return w
		}
	}
	return ""
}

// turkishFold maps the letters a Turkish keyboard adds to the plain ones
// codes are written in. Someone typing "kıraz" or "KİRAZ" means "kiraz".
var turkishFold = map[rune]rune{
	'ç': 'c', 'Ç': 'c', 'ğ': 'g', 'Ğ': 'g', 'ı': 'i', 'İ': 'i',
	'ö': 'o', 'Ö': 'o', 'ş': 's', 'Ş': 's', 'ü': 'u', 'Ü': 'u',
	'â': 'a', 'Â': 'a', 'î': 'i', 'Î': 'i', 'û': 'u', 'Û': 'u',
}

// NormalizeCode turns what a person typed into the form codes are written
// in: lower case, plain letters, parts joined by single hyphens. Spaces,
// dots or underscores between the parts all count as a separator, and a
// number typed straight after a word ("liman42") is split off.
//
// It never guesses beyond that. The result may still not be a valid code;
// ValidCode says whether it is.
func NormalizeCode(s string) string {
	var b strings.Builder
	var last rune // last rune written, 0 at the start
	for _, r := range s {
		if f, ok := turkishFold[r]; ok {
			r = f
		}
		r = unicode.ToLower(r)
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if r >= '0' && r <= '9' && last >= 'a' && last <= 'z' {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			last = r
		case last != 0 && last != '-':
			b.WriteByte('-')
			last = '-'
		}
		if b.Len() > 2*maxCodeLen {
			break
		}
	}
	return strings.TrimRight(b.String(), "-")
}
