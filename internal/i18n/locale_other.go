//go:build !windows

package i18n

import (
	"os"
	"os/exec"
	"strings"
)

// osLocale returns the system locale string on non-Windows platforms.
func osLocale() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			return val
		}
	}

	// On macOS, if terminal environment variables are unset, check AppleLocale
	if out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output(); err == nil {
		if loc := strings.TrimSpace(string(out)); loc != "" {
			return loc
		}
	}

	return ""
}
