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
	for _, not := range []string{"ffmpeg", "tesseract", "magick", "jq ", "7z a"} {
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
	all := []string{"7za", "ffmpeg", "ffprobe", "magick", "tesseract", "jq", "tail", "sha256sum"}
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
