//go:build windows

// GORILLA OVERRIDE (2026-11-21): enforce a crisp TrueType console font on
// Windows conhost.exe (which cmd/install_windows.go deliberately launches so
// Gorilla OpenCode owns its window and taskbar icon). Without this, conhost
// frequently defaults to legacy Raster Fonts that look blurry or illegible.
package tui

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	stdOutputHandle  = ^uintptr(10) // STD_OUTPUT_HANDLE = (DWORD)-11
	ffModernTrueType = 0x36         // FF_MODERN (0x30) | TMPF_TRUETYPE (0x04) | fixed pitch
	fontWeightNormal = 400
	defaultWinFont   = "Cascadia Mono"
	defaultWinSize   = 18
	lfFacesize       = 32
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle         = kernel32.NewProc("GetStdHandle")
	procGetCurrentConsoleFnt = kernel32.NewProc("GetCurrentConsoleFontEx")
	procSetCurrentConsoleFnt = kernel32.NewProc("SetCurrentConsoleFontEx")
)

type consoleCoord struct {
	X int16
	Y int16
}

type consoleFontInfoEx struct {
	cbSize     uint32
	nFont      uint32
	dwFontSize consoleCoord
	fontFamily uint32
	fontWeight uint32
	faceName   [lfFacesize]uint16
}

// preferredWindowsFonts returns the ordered font-face fallback list, honouring
// GORILLA_FONT when set.
func preferredWindowsFonts() []string {
	defaults := []string{defaultWinFont, "Cascadia Code", "Consolas", "Lucida Console"}
	custom := strings.TrimSpace(os.Getenv("GORILLA_FONT"))
	if custom == "" {
		return defaults
	}
	out := []string{custom}
	for _, f := range defaults {
		if !strings.EqualFold(f, custom) {
			out = append(out, f)
		}
	}
	return out
}

// preferredWindowsFontSize returns the requested pixel height (default 18),
// clamped to a legible console range [10, 48].
func preferredWindowsFontSize() int16 {
	if raw := strings.TrimSpace(os.Getenv("GORILLA_FONT_SIZE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 10 && n <= 48 {
			return int16(n)
		}
	}
	return defaultWinSize
}

func encodeFaceName(name string) [lfFacesize]uint16 {
	var buf [lfFacesize]uint16
	u := utf16.Encode([]rune(name))
	if len(u) >= lfFacesize {
		u = u[:lfFacesize-1]
	}
	copy(buf[:], u)
	return buf
}

func decodeFaceName(buf [lfFacesize]uint16) string {
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(utf16.Decode(buf[:n]))
}

// EnsureConsoleFont configures a crisp TrueType monospace font (Cascadia Mono
// 18 by default) when running inside Windows conhost.exe. Skips cleanly when
// running inside Windows Terminal (WT_SESSION set), when redirected/non-TTY, or
// when GORILLA_NO_FONT_OVERRIDE is set. Returns the applied font name, or "".
func EnsureConsoleFont() string {
	if os.Getenv("GORILLA_NO_FONT_OVERRIDE") != "" || os.Getenv("WT_SESSION") != "" {
		return ""
	}
	h, _, _ := procGetStdHandle.Call(stdOutputHandle)
	if h == 0 || h == ^uintptr(0) {
		return ""
	}
	var current consoleFontInfoEx
	current.cbSize = uint32(unsafe.Sizeof(current))
	if ok, _, _ := procGetCurrentConsoleFnt.Call(h, 0, uintptr(unsafe.Pointer(&current))); ok == 0 {
		return "" // not a real console handle (pipe / redirected stdout)
	}

	size := preferredWindowsFontSize()
	for _, face := range preferredWindowsFonts() {
		req := consoleFontInfoEx{
			cbSize:     uint32(unsafe.Sizeof(consoleFontInfoEx{})),
			dwFontSize: consoleCoord{X: 0, Y: size},
			fontFamily: ffModernTrueType,
			fontWeight: fontWeightNormal,
			faceName:   encodeFaceName(face),
		}
		if ok, _, _ := procSetCurrentConsoleFnt.Call(h, 0, uintptr(unsafe.Pointer(&req))); ok == 0 {
			continue
		}
		var verify consoleFontInfoEx
		verify.cbSize = uint32(unsafe.Sizeof(verify))
		if ok, _, _ := procGetCurrentConsoleFnt.Call(h, 0, uintptr(unsafe.Pointer(&verify))); ok != 0 {
			got := decodeFaceName(verify.faceName)
			if strings.EqualFold(got, face) {
				return got
			}
		}
	}
	return ""
}
