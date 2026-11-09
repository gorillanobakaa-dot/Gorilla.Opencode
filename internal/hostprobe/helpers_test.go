package hostprobe

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/opencode-ai/opencode/internal/toolbox"
)

// fakeBox is a toolbox in memory holding the named helpers for platform.
func fakeBox(platform string, names ...string) *toolbox.Box {
	fsys := fstest.MapFS{}
	for _, n := range names {
		fsys[platform+"/"+n+".exe"] = &fstest.MapFile{Data: []byte("x")}
	}
	return toolbox.New(fsys, platform, nil, func() (string, error) { return "", nil })
}

func shape(block string) string {
	var labels []string
	for _, l := range strings.Split(block, "\n")[1:] {
		labels = append(labels, strings.SplitN(strings.TrimPrefix(l, "- "), ":", 2)[0])
	}
	return strings.Join(labels, "|")
}

func TestHelperBlockNamesOnlyWhatIsPresent(t *testing.T) {
	box := fakeBox("windows-amd64", "7za", "ffprobe")
	f := Facts{OS: "windows", Toolbox: box.Names()}
	b := f.HelperBlock()
	for _, want := range []string{"7za a ", "ffprobe -hide_banner", "files tool"} {
		if !strings.Contains(b, want) {
			t.Errorf("block lacks %q:\n%s", want, b)
		}
	}
	for _, not := range []string{"ffmpeg", "tesseract scan", "magick in", "jq '", "7z a"} {
		if strings.Contains(b, not) {
			t.Errorf("block names absent helper %q:\n%s", not, b)
		}
	}
}

func TestHelperBlockStaysUnderBudgetWithEverything(t *testing.T) {
	all := []string{"7za", "7z", "ffmpeg", "ffprobe", "magick", "convert", "tesseract", "jq", "tail", "sha256sum", "rg"}
	for _, os := range []string{"windows", "linux"} {
		b := Facts{OS: os, Toolbox: all, PathTools: all}.HelperBlock()
		if len(b) > HelperBudget {
			t.Errorf("%s: %d bytes, budget %d", os, len(b), HelperBudget)
		}
	}
}

func TestHelperBlockHasTheSameShapeOnWindowsAndLinux(t *testing.T) {
	all := []string{"7za", "ffmpeg", "ffprobe", "magick", "tesseract", "jq", "tail", "sha256sum", "curl"}
	w := shape(Facts{OS: "windows", PathTools: all}.HelperBlock())
	l := shape(Facts{OS: "linux", PathTools: all}.HelperBlock())
	if w != l {
		t.Errorf("shape differs:\nwindows %s\nlinux   %s", w, l)
	}
	for _, label := range []string{"list / info", "read a file", "7z", "tail / follow", "JSON", "hash", "image convert", "OCR", "probe a video"} {
		if !strings.Contains(w, label) {
			t.Errorf("family %q missing from %s", label, w)
		}
	}
}

func TestHelperBlockOnBareLinuxNamesOnlyOwnTools(t *testing.T) {
	b := Facts{OS: "linux"}.HelperBlock()
	if strings.Contains(b, "tail") || strings.Contains(b, "7z") || strings.Contains(b, "Get-") {
		t.Errorf("bare Linux block names absent helpers:\n%s", b)
	}
}

// Every carried helper that has an example row in the block: pfind, ast-grep, difft, sd, ninja, hyperfine,
// coreutils, cwebp, avifenc, oxipng. A helper that is not present is not named.
func TestHelperBlockNamesEachCarriedHelperWhenPresent(t *testing.T) {
	for _, n := range []string{"pfind", "ast-grep", "difft", "sd", "ninja", "hyperfine", "coreutils", "cwebp", "avifenc", "oxipng"} {
		with := Facts{OS: "linux", PathTools: []string{n}}.HelperBlock()
		if !strings.Contains(with, n) {
			t.Errorf("%s is present but the Helpers block does not name it:\n%s", n, with)
		}
		if without := (Facts{OS: "linux"}).HelperBlock(); strings.Contains(without, n+" ") {
			t.Errorf("%s is absent but the block names it:\n%s", n, without)
		}
	}
}

// P5 stage 2 (2026-11-09): the 13 everyday jobs of `fieldkit accept`. Each job names the row label that must
// teach it on a machine that has everything, on both systems.
var acceptJobRows = map[string]string{
	"see-file": "read a file", "delete-file": "delete (to trash)", "move-file": "list / info / delete (to trash) / move",
	"tail-file": "tail / follow", "find-file": "search text or names", "grep-text": "search text or names",
	"unzip": "unzip all", "json-read": "JSON", "json-edit": "JSON", "download": "download",
	"image-convert": "image convert", "ocr": "OCR", "probe-video": "probe a video",
}

func TestEveryAcceptJobHasARowOnBothSystems(t *testing.T) {
	all := []string{"7za", "ffmpeg", "ffprobe", "magick", "tesseract", "jq", "tail", "sha256sum", "curl", "rg"}
	for _, os := range []string{"windows", "linux"} {
		b := Facts{OS: os, Toolbox: all}.HelperBlock()
		for job, label := range acceptJobRows {
			if !strings.Contains(b, label) && !strings.Contains(b, strings.ReplaceAll(label, "delete (to trash) / move", "list / info / delete (to trash) / move")) {
				t.Errorf("%s: job %s has no row %q:\n%s", os, job, label, b)
			}
		}
	}
}

// A helper added to helpers.json without an example row in the block fails here; a name in the block that is not
// in the catalog is not a bundled helper.
func TestEveryCatalogHelperIsNamedAndNothingElseIsClaimedBundled(t *testing.T) {
	cat, err := toolbox.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, h := range cat {
		names = append(names, h.Name)
	}
	for _, os := range []string{"windows", "linux"} {
		b := Facts{OS: os, Toolbox: names}.HelperBlock()
		for _, n := range names {
			if !strings.Contains(b, n) {
				t.Errorf("%s: catalog helper %s is bundled but the block does not name it", os, n)
			}
		}
	}
	// ImageMagick, Tesseract, jq, rg, curl, exiftool are not bundled: with an empty toolbox and PATH they are
	// said to be absent, never offered as a row.
	b := Facts{OS: "linux"}.HelperBlock()
	for _, n := range []string{"magick ", "tesseract scan", "jq '", "rg -n", "curl -L", "exiftool -a"} {
		if strings.Contains(b, n) {
			t.Errorf("absent program offered: %q\n%s", n, b)
		}
	}
	for _, n := range []string{"jq", "rg", "tesseract", "ImageMagick", "curl/wget", "exiftool"} {
		if !strings.Contains(b, n) || !strings.Contains(b, "not installed") {
			t.Errorf("bare Linux block does not say %s is absent:\n%s", n, b)
		}
	}
}

func TestPresentProgramsLeaveTheAbsentLine(t *testing.T) {
	b := Facts{OS: "linux", PathTools: []string{"jq", "magick", "curl"}}.HelperBlock()
	line := b[strings.Index(b, "not installed"):]
	for _, n := range []string{"jq", "ImageMagick", "curl"} {
		if strings.Contains(line, n) {
			t.Errorf("%s is present but listed as not installed: %s", n, line)
		}
	}
}
