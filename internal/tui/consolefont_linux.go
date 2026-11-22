//go:build linux

// GORILLA OVERRIDE (2026-11-21): best-effort high-legibility monospace font
// override on Linux terminals that accept OSC 50 (xterm / rxvt). Emulators
// whose font is controlled exclusively by their profile config (GNOME Terminal,
// Kitty, Alacritty) are left untouched so no stray escape sequences leak.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var defaultLinuxFonts = []string{
	"Cascadia Mono",
	"JetBrains Mono",
	"Fira Code",
	"Ubuntu Mono",
	"DejaVu Sans Mono",
	"Liberation Mono",
}

func preferredLinuxFontSize() int {
	if raw := strings.TrimSpace(os.Getenv("GORILLA_FONT_SIZE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 8 && n <= 36 {
			return n
		}
	}
	return 14
}

func supportsOSC50Font() bool {
	if os.Getenv("XTERM_VERSION") != "" || os.Getenv("RXVT_SOCKET") != "" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	return strings.HasPrefix(term, "rxvt")
}

func pickInstalledLinuxFont() string {
	if custom := strings.TrimSpace(os.Getenv("GORILLA_FONT")); custom != "" {
		return custom
	}
	fcMatch, err := exec.LookPath("fc-match")
	if err != nil {
		return "monospace"
	}
	for _, candidate := range defaultLinuxFonts {
		out, err := exec.Command(fcMatch, "-f", "%{family}", candidate).Output()
		if err != nil {
			continue
		}
		got := strings.TrimSpace(string(out))
		if strings.EqualFold(got, candidate) {
			return candidate
		}
	}
	return "monospace"
}

// EnsureConsoleFont emits an OSC 50 TrueType font selection on Linux terminals
// that support it (xterm/rxvt), avoiding bitmap raster defaults. Returns the
// font name sent, or "" if skipped.
func EnsureConsoleFont() string {
	if os.Getenv("GORILLA_NO_FONT_OVERRIDE") != "" || !supportsOSC50Font() {
		return ""
	}
	fi, err := os.Stdout.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return ""
	}
	font := pickInstalledLinuxFont()
	size := preferredLinuxFontSize()
	_, _ = fmt.Fprintf(os.Stdout, "\x1b]50;xft:%s:size=%d\x07", font, size)
	return font
}
