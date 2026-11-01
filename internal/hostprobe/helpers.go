package hostprobe

import (
	"os/exec"
	"strings"
)

// GORILLA (2026-10-31, P5c): the acceptance run showed models asking
// fieldkit_discover, hunting the disk or reaching for the web before looking at
// the machine. HelperBlock is the one short block of the system prompt that
// names the everyday helpers with one example each. It is built from what the
// probe found (toolbox + PATH), never from a fixed list: a helper that is not
// here is not named, and the shape is the same on Windows and Linux.

// HelperBudget is the most bytes HelperBlock may take.
const HelperBudget = 1500

// pathCandidates are the helper programs the probe looks for on PATH.
var pathCandidates = []string{"7za", "7z", "7zz", "ffmpeg", "ffprobe", "magick", "convert", "tesseract", "jq", "rg",
	"tail", "sha256sum", "pfind"}

func lookPathTools() []string {
	var out []string
	for _, n := range pathCandidates {
		if _, err := exec.LookPath(n); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// has reports whether the program is embedded in the toolbox or on PATH.
func (f Facts) has(name string) bool {
	for _, l := range [][]string{f.Toolbox, f.PathTools} {
		for _, n := range l {
			if n == name {
				return true
			}
		}
	}
	return false
}

func (f Facts) first(names ...string) string {
	for _, n := range names {
		if f.has(n) {
			return n
		}
	}
	return ""
}

// HelperBlock returns the block, or "" when nothing applies.
func (f Facts) HelperBlock() string {
	win := f.OS == "windows"
	var rows []string
	add := func(what, example string) { rows = append(rows, "- "+what+": "+example) }

	// Always present: Gorilla's own tools.
	add("list / info / delete (to trash) / move", "files tool, action=list|info|delete|move")
	add("read a file", "view tool")
	add("search text or names", "find tool (query=..., regex ok)")
	add("zip", "files tool, action=archive")
	if x := f.first("7za", "7z", "7zz"); x != "" {
		add("7z", x+" a out.7z DIR ; "+x+" x in.7z -oDIR")
	}
	switch {
	case win:
		add("tail / follow", "Get-Content app.log -Tail 30 -Wait")
	case f.has("tail"):
		add("tail / follow", "tail -n 30 -f app.log")
	}
	if j := f.first("jq"); j != "" {
		add("JSON", "jq '.key' f.json")
	} else if win {
		add("JSON", "Get-Content f.json -Raw | ConvertFrom-Json")
	}
	switch {
	case win:
		add("hash", "Get-FileHash f.zip -Algorithm SHA256")
	case f.has("sha256sum"):
		add("hash", "sha256sum f.zip")
	}
	if m := f.first("magick"); m != "" {
		add("image convert", m+" in.png out.webp")
	} else if c := f.first("convert"); c != "" {
		add("image convert", c+" in.png out.webp")
	} else if f.has("ffmpeg") {
		add("image convert", "ffmpeg -i in.png out.webp")
	}
	if f.has("tesseract") {
		add("OCR", "tesseract scan.png out")
	}
	if f.has("ffprobe") {
		add("probe a video", "ffprobe -hide_banner clip.mp4")
	}
	if f.has("ffmpeg") {
		add("extract audio", "ffmpeg -i clip.mp4 -vn out.mp3")
	}
	if len(rows) == 0 {
		return ""
	}
	return "Helpers here (before any web search, read the how-to card everyday):\n" + strings.Join(rows, "\n")
}
