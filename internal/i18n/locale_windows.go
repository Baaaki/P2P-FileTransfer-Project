//go:build windows

package i18n

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// osLocale returns the system locale string on Windows.
func osLocale() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := strings.TrimSpace(os.Getenv(env)); val != "" {
			return val
		}
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getUserDefaultLocaleName := kernel32.NewProc("GetUserDefaultLocaleName")
	if getUserDefaultLocaleName.Find() == nil {
		buf := make([]uint16, 85)
		ret, _, _ := getUserDefaultLocaleName.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		if ret > 0 {
			return syscall.UTF16ToString(buf)
		}
	}

	// Fallback to UI language ID (0x041F = Turkish)
	getUserDefaultUILanguage := kernel32.NewProc("GetUserDefaultUILanguage")
	if getUserDefaultUILanguage.Find() == nil {
		langID, _, _ := getUserDefaultUILanguage.Call()
		if (langID & 0x3ff) == 0x1f { // LANG_TURKISH
			return "tr_TR"
		}
	}

	return ""
}
