package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// Releases can be signed with minisign (https://jedisct1.github.io/minisign/):
// the release workflow signs checksums.txt, and a build that carries the
// public key refuses a checksum file the matching secret key did not sign.
//
// The checksums alone prove only that an archive is the one the release
// page lists, which is worth having — a truncated or swapped download is
// caught — but whoever can change the release page can change the list
// with it. The signature is what ties the list to a key kept elsewhere.
//
// Minisign rather than cosign because its signatures are small files that
// can be checked with nothing but ed25519, here and by hand
// (`minisign -Vm checksums.txt -P <key>`), with no transparency log or
// certificate chain to reach.

// minisignKey is a minisign public key.
type minisignKey struct {
	id  [8]byte
	key ed25519.PublicKey
}

// parseMinisignKey reads a public key, either the bare base64 line or the
// whole .pub file with its comment line.
func parseMinisignKey(s string) (minisignKey, error) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[len(lines)-1]))
	if err != nil || len(raw) != 2+8+ed25519.PublicKeySize || string(raw[:2]) != "Ed" {
		return minisignKey{}, errors.New("minisign açık anahtarı değil")
	}
	var k minisignKey
	copy(k.id[:], raw[2:10])
	k.key = ed25519.PublicKey(raw[10:])
	return k, nil
}

// verifyMinisign checks a .minisig file over message. Both kinds of
// signature minisign makes are accepted: over the message itself ("Ed",
// the legacy format) and over its BLAKE2b-512 digest ("ED", the default
// since minisign 0.8). The trusted comment is signed too, so it is checked
// along with the rest.
func verifyMinisign(k minisignKey, message, sigFile []byte) error {
	lines := strings.Split(strings.ReplaceAll(string(sigFile), "\r\n", "\n"), "\n")
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "untrusted comment:") ||
		!strings.HasPrefix(lines[2], "trusted comment: ") {
		return errors.New("imza dosyası bozuk")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil || len(sig) != 2+8+ed25519.SignatureSize {
		return errors.New("imza bozuk")
	}
	global, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if err != nil || len(global) != ed25519.SignatureSize {
		return errors.New("imza bozuk")
	}

	algorithm, keyID, signature := string(sig[:2]), sig[2:10], sig[10:]
	if !bytes.Equal(keyID, k.id[:]) {
		return fmt.Errorf("başka bir anahtarla imzalanmış (%X)", keyID)
	}
	signed := message
	switch algorithm {
	case "Ed":
	case "ED":
		digest := blake2b.Sum512(message)
		signed = digest[:]
	default:
		return fmt.Errorf("bilinmeyen imza algoritması %q", algorithm)
	}
	if !ed25519.Verify(k.key, signed, signature) {
		return errors.New("imza eşleşmiyor")
	}
	trusted := strings.TrimPrefix(lines[2], "trusted comment: ")
	if !ed25519.Verify(k.key, append(append([]byte(nil), signature...), trusted...), global) {
		return errors.New("imzalı yorum değiştirilmiş")
	}
	return nil
}
