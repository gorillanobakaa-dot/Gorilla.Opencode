//go:build windows

package hostprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindows11IsNamedFromTheBuildNumber(t *testing.T) {
	if got := windowsNameFrom("Windows 10 Home", "25H2", "26200", 6899); got != "Windows 11 Home 25H2 (build 26200.6899)" {
		t.Errorf("got %q", got)
	}
	if got := windowsNameFrom("Windows 10 Pro", "22H2", "19045", 0); got != "Windows 10 Pro 22H2 (build 19045)" {
		t.Errorf("got %q", got)
	}
}

func TestMSVCNeedsTheCppToolset(t *testing.T) {
	root := t.TempDir()
	inst := filepath.Join(root, "instances")
	withVC := filepath.Join(root, "vs-bt")
	noVC := filepath.Join(root, "vs-novc")
	for _, d := range []string{filepath.Join(inst, "a"), filepath.Join(inst, "b"), filepath.Join(withVC, "VC", "Tools", "MSVC"), noVC} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state := func(dir, path, ver, title string) {
		js := `{"installationPath":` + quote(path) + `,"installationVersion":"` + ver +
			`","catalogInfo":{"productDisplayVersion":"17.14.40","productLineVersion":"2022"},` +
			`"localizedResources":[{"language":"en-us","title":"` + title + `"}]}`
		if err := os.WriteFile(filepath.Join(inst, dir, "state.json"), []byte(js), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state("a", withVC, "17.14.1", "Visual Studio Build Tools 2022")
	state("b", noVC, "17.99.0", "Visual Studio Community 2022") // newer, but no C++ toolset
	if got, cmd := msvc(inst); got != "Visual Studio Build Tools 2022 17.14.40" || cmd != "" {
		t.Errorf("got %q, dev shell %q (none was made yet)", got, cmd)
	}
	devCmd := filepath.Join(withVC, "Common7", "Tools", "VsDevCmd.bat")
	if err := os.MkdirAll(filepath.Dir(devCmd), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(devCmd, []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, cmd := msvc(inst); cmd != devCmd {
		t.Errorf("dev shell %q, want %q", cmd, devCmd)
	}
	if got, cmd := msvc(filepath.Join(root, "missing")); got != "" || cmd != "" {
		t.Errorf("no instances folder: got %q %q", got, cmd)
	}
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

func TestPowerToysCLIsComeFromTheInstalledFiles(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "nope")
	pt := filepath.Join(root, "PowerToys")
	if err := os.MkdirAll(filepath.Join(pt, "WinUI3Apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"PowerToys.exe", "PowerToys.Awake.exe", "PowerToys.FancyZones.exe", filepath.Join("WinUI3Apps", "FileLocksmithCLI.exe")} {
		if err := os.WriteFile(filepath.Join(pt, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir, clis := powerToys([]string{missing, pt})
	if dir != pt || strings.Join(clis, ",") != "FileLocksmithCLI.exe,PowerToys.Awake.exe" {
		t.Errorf("got %q %v", dir, clis)
	}
	if dir, _ := powerToys([]string{missing}); dir != "" {
		t.Errorf("not installed, got %q", dir)
	}
}
