package dialog

// GORILLA (2026-11-01, W6): /arsenal shows what is built in FIRST. The helpers
// carried inside this program (internal/toolbox) are ready now with nothing to
// install; the owner looked at /arsenal, /help, /settings and /context and saw no
// mention of them. This page is generated from the pins and helpers.json, so a
// helper added to the toolbox appears here with no one typing it.

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/opencode-ai/opencode/internal/arsenal"
	"github.com/opencode-ai/opencode/internal/toolbox"
)

// featuredIDs are the installable entries the built-in page names with their
// install line: the ones the owner asked to see (OCR, pictures, PowerToys).
var featuredIDs = []string{"tesseract", "imagemagick", "powertoys"}

// ocrFallback is what OCR does when Tesseract is absent (internal/llm/tools/ocr.go).
const ocrFallback = "Without Tesseract the agent cannot read the text in a picture itself: it falls back to a " +
	"model that can see images, if one is connected, and otherwise says it cannot read the picture."

// OpenOnBuiltIn makes the page open on the built-in helpers, the first thing
// shown. Esc closes it; enter goes on to the full list.
func (m ArsenalCmp) OpenOnBuiltIn() ArsenalCmp {
	m.view, m.scrollTop = viewBuiltIn, 0
	return m
}

// builtInCount is how many helpers this program carries for this system.
func (m ArsenalCmp) builtInCount() int { return len(m.built) }

// installable lists the manifest entries not present here that have a command
// for this system.
func installableNow(man arsenal.Manifest, status map[string]arsenal.Status, pm arsenal.PackageManager) []arsenal.Entry {
	var out []arsenal.Entry
	for _, s := range man.Series {
		for _, e := range s.Entries {
			if !status[e.ID].Present && len(arsenal.ShowCommands(e, pm)) > 0 {
				out = append(out, e)
			}
		}
	}
	return out
}

// ArsenalCounts is "N built-in helpers, M installable" for /context: N carried in
// this program, M entries of the arsenal this machine lacks and could fetch.
func ArsenalCounts() (builtIn, installable int) {
	full, _ := arsenal.Load()
	man := arsenal.ForPlatform(full)
	pm, st := arsenal.DetectAll(man)
	return len(toolbox.BuiltIn()), len(installableNow(man, st, pm))
}

// ArsenalCountsLine is the sentence /context shows.
func ArsenalCountsLine(builtIn, installable int) string {
	return fmt.Sprintf("%d built-in helpers, %d installable (type /arsenal to see them)", builtIn, installable)
}

func (m ArsenalCmp) builtInLines() []arsLine {
	out := []arsLine{
		{"h1", "BUILT IN (ready now) - carried inside this program, nothing to install"},
	}
	if len(m.built) == 0 {
		out = append(out,
			arsLine{"warn", "This build carries no built-in helpers (a plain developer build). Release builds do."},
			arsLine{"", ""})
	} else {
		out = append(out, arsLine{"mute",
			fmt.Sprintf("%d helpers. Each is checked against its pinned hash before every run.", len(m.built))})
		width := 0
		for _, h := range m.built {
			width = max(width, len(h.Name))
		}
		for _, h := range m.built {
			out = append(out, arsLine{"have", fmt.Sprintf("  %-*s %-7s %s", width, h.Name, h.VersionOn(m.platform), h.Purpose)})
		}
		if runtime.GOOS == "darwin" || strings.HasPrefix(m.platform, "darwin-") {
			var need []string
			for _, h := range m.built {
				if h.MacNeeds() != "10.13" {
					need = append(need, h.Name+" needs macOS "+h.MacNeeds())
				}
			}
			note := "On macOS every helper runs on 10.13 (High Sierra) or newer."
			if len(need) > 0 {
				note = "On macOS the others run on 10.13 or newer, but " + strings.Join(need, ", ") + "."
			}
			out = append(out, arsLine{"warn", note})
		}
		out = append(out, arsLine{"", ""})
	}

	out = append(out, arsLine{"h1", "CAN BE ADDED - not built in; the exact command for this system"})
	shown := 0
	for _, id := range featuredIDs {
		e, ok := m.entryByID(id)
		if !ok {
			continue
		}
		shown++
		state := "not installed"
		kind := "mute"
		if m.status[id].Present {
			state, kind = "installed", "have"
		}
		out = append(out, arsLine{kind, fmt.Sprintf("  %s - %s", e.ID, state)})
		out = append(out, arsLine{"", "    " + e.Title})
		if m.status[id].Present {
			continue
		}
		for _, c := range arsenal.ShowCommands(e, m.pm) {
			out = append(out, arsLine{"sel", "    " + c})
		}
		if id == "tesseract" {
			out = append(out, arsLine{"mute", "    " + ocrFallback})
		}
	}
	if shown == 0 {
		out = append(out, arsLine{"mute", "  Nothing extra is offered on this system."})
	}
	out = append(out, arsLine{"", ""},
		arsLine{"mute", "Nothing here is installed by this program. It shows you the exact command; you run it."},
		arsLine{"mute", "enter the full list of capabilities | up/down scroll | esc close"})
	return out
}

func (m ArsenalCmp) entryByID(id string) (arsenal.Entry, bool) {
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			if e.ID == id {
				return e, true
			}
		}
	}
	return arsenal.Entry{}, false
}
