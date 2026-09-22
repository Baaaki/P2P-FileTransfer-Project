package rendezvous

import (
	"crypto/rand"
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
// The two halves do different jobs. The number is the room's nameplate:
// the meeting point hands it out and finds the room by it, so it is no
// secret. The words are the secret. They are picked on the sender's
// machine, never sent to the server, and only ever used as the password of
// the handshake the two ends run (see transfer/auth.go). A meeting point —
// or whoever controls the server list — that wants to pass off a peer of
// its own as the sender, or to pose as the receiver, has to guess them,
// once per attempt, and a sender closes its room after a few wrong ones.
//
// Were the server told the whole code, as it used to be, it could run the
// handshake with both ends itself and sit in the middle of every transfer.
// Hashing the code before sending it would not help: 5.9 million codes are
// tried in milliseconds.
//
// 256 words, twice, make 65,536 secrets — 16 bits. That is small for a
// password anyone could test offline, and plenty for one that can only be
// tried against a live sender, three times.
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

// codeFormat is the shape every code has: the two secret words, then the
// nameplate.
var codeFormat = regexp.MustCompile(`^[a-z]+-[a-z]+-[1-9][0-9]+$`)

// nameplateFormat is the part of a code the server sees, and all a room
// name on the server can be: at least two digits, never a leading zero, so
// "042" and "42" cannot name two different rooms.
var nameplateFormat = regexp.MustCompile(`^[1-9][0-9]+$`)

// maxCodeLen is comfortably more than two of the longest words and a number.
const maxCodeLen = 64

// maxNameplateLen bounds what a room number can be on the server.
const maxNameplateLen = 16

// NewSecret picks the two words of a new code. They never leave this
// machine except as a password the other end has to prove it knows.
func NewSecret() string {
	return pickWord() + "-" + pickWord()
}

// JoinCode puts the secret words and the nameplate the server handed out
// together into the code a person reads out.
func JoinCode(secret, nameplate string) string {
	return secret + "-" + nameplate
}

// Nameplate is the part of a code the server is told: the number at its
// end. Call it on a code ValidCode accepts.
func Nameplate(code string) string {
	return code[strings.LastIndexByte(code, '-')+1:]
}

func pickWord() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	return words[n.Int64()]
}

// ValidCode reports whether s has the shape of a room code.
func ValidCode(s string) bool {
	return len(s) <= maxCodeLen && codeFormat.MatchString(s)
}

// ValidNameplate reports whether s has the shape of a nameplate.
func ValidNameplate(s string) bool {
	return len(s) <= maxNameplateLen && nameplateFormat.MatchString(s)
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
