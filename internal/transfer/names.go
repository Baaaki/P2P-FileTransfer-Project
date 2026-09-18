package transfer

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"filetransferilla/internal/safetext"
)

// maxNameBytes is the longest single file or folder name any common
// filesystem will store.
const maxNameBytes = 255

// checkWindowsNames decides whether names Windows cannot store are refused.
// It is a variable only so the tests can exercise the Windows rules on the
// machines they actually run on.
var checkWindowsNames = runtime.GOOS == "windows"

// checkManifest refuses a manifest before anything in it is used, and
// returns where on disk each file would land. Every field arrives from the
// other side and is treated accordingly: a path that climbs out of the
// target folder, a digest that is not a digest, a size that is negative or
// that overflows the total — none of them has an honest explanation, so
// each is a reason to stop, not something to repair.
//
// The digest matters more than it looks. It names the partial download a
// resume continues from, so an unchecked one — "../../../.bashrc" — would
// be a path, and the promise that nothing lands outside the chosen folder
// would not hold for it.
func checkManifest(outDir string, m Manifest) ([]string, error) {
	invalid := func(why string) ([]string, error) {
		return nil, fmt.Errorf("the other side sent an invalid file list: %s", why)
	}
	if len(m.Files) == 0 {
		return invalid("it is empty")
	}
	if len(m.Files) > maxFiles {
		return nil, fmt.Errorf("too many files: %d, at most %d can be received at once", len(m.Files), maxFiles)
	}

	targets := make([]string, len(m.Files))
	var total int64
	for i, f := range m.Files {
		if f.Size < 0 || f.Size > math.MaxInt64-total {
			return invalid("impossible file size")
		}
		total += f.Size
		if !validDigest(f.SHA256) {
			return invalid("malformed checksum")
		}
		t, err := safeJoin(outDir, f.Path)
		if err != nil {
			return nil, err
		}
		targets[i] = t
	}
	return targets, nil
}

// validDigest accepts exactly what the sender produces: a SHA-256 in
// lowercase hex.
func validDigest(s string) bool {
	if len(s) != 2*32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// safeJoin turns a manifest path into a real one inside outDir, refusing
// anything that could escape it. The sender is not trusted to be honest
// here: "../../.bashrc", "/etc/passwd" and "C:\Windows" all have to lose.
//
// Anything suspicious is rejected rather than quietly repaired. Cleaning
// "../evil.txt" into "evil.txt" would save the file somewhere the sender
// did not ask for and the receiver was never told about; an honest sender
// never produces such a path, so one is reason to stop, not to guess.
func safeJoin(outDir, rel string) (string, error) {
	unsafe := func() (string, error) {
		return "", fmt.Errorf("the other side sent an unsafe file name: %q", rel)
	}
	// Windows separators and drive letters are rejected rather than
	// normalised: on Linux `C:\x` is a legal single file name, and
	// accepting it would create bizarre files instead of saying what is
	// wrong.
	if rel == "" || strings.ContainsAny(rel, `\:`) || strings.HasPrefix(rel, "/") {
		return unsafe()
	}
	// Control characters and bidirectional marks never belong in a name,
	// and this one is about to be printed on the approval screen. An escape
	// sequence there could redraw the very list the user is approving.
	if strings.IndexFunc(rel, safetext.IsUnsafe) >= 0 {
		return unsafe()
	}

	var parts []string
	for part := range strings.SplitSeq(rel, "/") {
		switch part {
		case "", ".":
			continue // harmless noise: "a//b", "a/./b"
		case "..":
			return unsafe()
		}
		if len(part) > maxNameBytes {
			return unsafe()
		}
		if checkWindowsNames && !windowsCanStore(part) {
			return "", fmt.Errorf("the other side sent a file name this computer cannot store: %q", rel)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return unsafe()
	}
	if parts[0] == partialDir {
		return "", fmt.Errorf("the other side sent a reserved file name: %q", rel)
	}
	return filepath.Join(append([]string{outDir}, parts...)...), nil
}

// windowsReserved are the device names Windows will not let a file take,
// with or without an extension: "nul.txt" is the null device too.
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"CONIN$": true, "CONOUT$": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"COM¹": true, "COM²": true, "COM³": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	"LPT¹": true, "LPT²": true, "LPT³": true,
}

// windowsCanStore reports whether Windows would store a file or folder
// under this name as it is. A name from a Mac or Linux sender can be
// perfectly ordinary there and still fail here, and it is better to say
// so before the transfer than to fail on the last rename with an error
// nobody can read.
func windowsCanStore(name string) bool {
	if strings.ContainsAny(name, `<>"|?*`) {
		return false
	}
	// Windows drops a trailing dot or space, so "a." and "a" would be the
	// same file — and "a." cannot be created at all.
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	base, _, _ := strings.Cut(name, ".")
	return !windowsReserved[strings.ToUpper(strings.TrimRight(base, " "))]
}

// availablePath picks a free name like "photo (1).jpg" if a file with the
// same name already exists; we never overwrite existing files.
func availablePath(target string) string {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}
	dir, name := filepath.Split(target)
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
