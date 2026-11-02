package arsenal

import (
	"strings"
	"testing"
)

// W6: the line shown is the one that works on this system: winget on Windows,
// brew on macOS, apt/dnf/pacman on Linux.
func TestShowCommandsPerSystem(t *testing.T) {
	tess := entryByID(t, "tesseract")
	cases := []struct {
		os   string
		pm   PackageManager
		want string
	}{
		{"windows", Scoop, "winget install --id UB-Mannheim.TesseractOCR -e"},
		{"windows", Unknown, "winget install --id UB-Mannheim.TesseractOCR -e"},
		{"darwin", Brew, "brew install tesseract"},
		{"darwin", Unknown, "brew install tesseract"},
		{"linux", APT, "sudo apt-get install -y tesseract-ocr tesseract-ocr-eng"},
		{"linux", DNF, "sudo dnf install -y tesseract tesseract-langpack-eng"},
		{"linux", Pacman, "sudo pacman -S --needed tesseract tesseract-data-eng"},
	}
	for _, c := range cases {
		useMachine(t, c.os, nil)
		got := strings.Join(ShowCommands(tess, c.pm), "\n")
		if got != c.want {
			t.Errorf("%s/%q: got %q, want %q", c.os, c.pm, got, c.want)
		}
	}
}

func TestImageMagickHasACommandEverywhere(t *testing.T) {
	im := entryByID(t, "imagemagick")
	for _, os := range []string{"windows", "darwin", "linux"} {
		useMachine(t, os, nil)
		if len(ShowCommands(im, Unknown)) == 0 {
			t.Errorf("%s: imagemagick has no command", os)
		}
	}
}

func TestPowerToysIsWindowsOnlyWithWinget(t *testing.T) {
	e := entryByID(t, "powertoys")
	useMachine(t, "windows", nil)
	if !Applicable(e) {
		t.Fatal("PowerToys must be listed on Windows")
	}
	if got := strings.Join(ShowCommands(e, Scoop), "\n"); got != "winget install --id Microsoft.PowerToys -e" {
		t.Fatalf("windows command = %q", got)
	}
	for _, os := range []string{"linux", "darwin"} {
		useMachine(t, os, nil)
		if Applicable(e) {
			t.Errorf("PowerToys must not be listed on %s", os)
		}
	}
}

func TestTesseractSaysWhatOCRFallsBackTo(t *testing.T) {
	c := entryByID(t, "tesseract").Caveats
	if !strings.Contains(c, "falls back to a model that can see images") {
		t.Fatalf("tesseract caveat does not say what happens without it: %q", c)
	}
}

func TestDetectsWingetDnfBrew(t *testing.T) {
	t.Setenv("SCOOP", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	useMachine(t, "windows", map[string]string{"winget": "C:/w/winget.exe"})
	if got := DetectPackageManager(); got != Winget {
		t.Errorf("windows: got %q", got)
	}
	useMachine(t, "darwin", map[string]string{"brew": "/opt/homebrew/bin/brew"})
	if got := DetectPackageManager(); got != Brew {
		t.Errorf("darwin: got %q", got)
	}
	useMachine(t, "linux", map[string]string{"dnf": "/usr/bin/dnf"})
	if got := DetectPackageManager(); got != DNF {
		t.Errorf("linux: got %q", got)
	}
}
