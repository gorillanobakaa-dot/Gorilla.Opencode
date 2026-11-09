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
const HelperBudget = 2300

// pathCandidates are the helper programs the probe looks for on PATH.
var pathCandidates = []string{"7za", "7z", "7zz", "ffmpeg", "ffprobe", "magick", "convert", "tesseract", "jq", "rg",
	"tail", "sha256sum", "pfind", "curl", "wget", "unzip", "exiftool"}

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
	if f.has("rg") {
		add("grep", "rg -n WORD DIR")
	}
	if f.has("fd") {
		add("find by name", "fd PATTERN DIR ; fd -e go")
	}
	add("zip", "files tool, action=archive")
	if x := f.first("7za", "7z", "7zz"); x != "" {
		add("7z", x+" a out.7z DIR ; "+x+" x in.7z -oDIR")
		add("unzip all", x+" x in.zip -oout")
	} else if win {
		add("unzip all", "Expand-Archive in.zip -DestinationPath out")
	} else if f.has("unzip") {
		add("unzip all", "unzip in.zip -d out")
	}
	switch {
	case win:
		add("tail / follow", "Get-Content app.log -Tail 30 -Wait")
	case f.has("tail"):
		add("tail / follow", "tail -n 30 -f app.log")
	case f.has("coreutils"):
		add("tail / follow", "coreutils tail -n 30 -f app.log")
	}
	if j := f.first("jq"); j != "" {
		add("JSON", "jq '.key' f.json ; jq '.n=7' f.json > t && mv t f.json")
	} else if win {
		add("JSON", "$j=Get-Content f.json -Raw|ConvertFrom-Json; $j.n=7; $j|ConvertTo-Json -Depth 9|Set-Content f.json")
	}
	switch {
	case win:
		add("download", "Invoke-WebRequest URL -OutFile f.txt")
	case f.has("curl"):
		add("download", "curl -L -o f.txt URL")
	case f.has("wget"):
		add("download", "wget -O f.txt URL")
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
	if f.has("pfind") {
		add("find files by name", "pfind PATTERN DIR")
	}
	if f.has("ast-grep") {
		add("search code by syntax", "ast-grep -p 'foo($A)' DIR")
	}
	if f.has("difft") {
		add("compare two files", "difft a.go b.go")
	}
	if f.has("sd") {
		add("replace text", "sd 'old' 'new' f.txt")
	}
	if f.has("ninja") {
		add("build", "ninja -C build")
	}
	if f.has("hyperfine") {
		add("time a command", "hyperfine 'cmd'")
	}
	if f.has("coreutils") {
		add("ls / sort / wc anywhere", "coreutils ls -l")
	}
	if px := f.first("cwebp", "avifenc", "oxipng"); px != "" {
		add("shrink pictures", "cwebp in.png -o out.webp ; avifenc in.png out.avif ; oxipng f.png")
	}
	if f.has("exiftool") {
		add("photo metadata", "exiftool -a f.jpg")
	}
	if len(rows) == 0 {
		return ""
	}
	if gone := f.missing(); len(gone) > 0 {
		rows = append(rows, "- not installed (never search the web for them, use the nearest row above): "+strings.Join(gone, ", "))
	}
	return "Helpers here (before any web search, read the how-to card everyday):\n" + strings.Join(rows, "\n")
}

// missing names the everyday programs a job could reach for that are neither
// embedded nor on PATH. The names are fixed, so the line is as stable as the
// probe.
func (f Facts) missing() []string {
	var gone []string
	for _, n := range []string{"jq", "rg", "exiftool", "tesseract"} {
		if !f.has(n) {
			gone = append(gone, n)
		}
	}
	if !f.has("magick") && !f.has("convert") {
		gone = append(gone, "ImageMagick")
	}
	if f.OS != "windows" && !f.has("curl") && !f.has("wget") {
		gone = append(gone, "curl/wget")
	}
	return gone
}
