//go:build !windows

// GORILLA OVERRIDE (2026-09-01): the Unix half of `install` / `uninstall`,
// unchanged in behaviour and moved here so Windows can have a real one.
//
// Everything below was previously the WHOLE of install.go: hicolor icon
// directories, a .desktop entry, gtk-update-icon-cache. None of it means
// anything on Windows, and `os.Geteuid` does not even exist there — so a
// Windows user running `gorilla-opencode install` got a Linux desktop entry
// written into their home directory and no shortcut anywhere.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/opencode-ai/opencode/internal/assets"
)

const desktopEntry = `[Desktop Entry]
Type=Application
Name=Gorilla OpenCode
Comment=Terminal AI coding agent (revived original OpenCode) — bring your own API keys
Exec=` + appBinName + ` launch
Icon=` + appBinName + `
Terminal=true
Categories=Development;IDE;
Keywords=ai;coding;agent;terminal;llm;

# GORILLA OVERRIDE: a right-click action for the copyable interface. The icon runs
# ` + "`launch`" + ` with no arguments, so a mode reachable only by typing --plain is a
# mode most users never get. The standing preference lives in /settings; this is
# for using it once. Keep in step with packaging/gorilla-opencode.desktop — the
# .deb installs that file, this string serves ` + "`" + appBinName + ` install` + "`" + `.
Actions=plain;

[Desktop Action plain]
Name=Plain mode (selectable and copyable)
Exec=` + appBinName + ` launch --plain
Icon=` + appBinName + `
`

// installPaths resolves the three installation roots. Running as root
// installs system-wide; otherwise everything stays inside $HOME.
func installPaths() (bin, icons, apps string) {
	if os.Geteuid() == 0 {
		return "/usr/local/bin",
			"/usr/local/share/icons/hicolor",
			"/usr/local/share/applications"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".local", "share", "icons", "hicolor"),
		filepath.Join(home, ".local", "share", "applications")
}

func installedFiles() (binPath, desktopPath string, iconPaths []string) {
	bin, icons, apps := installPaths()
	binPath = filepath.Join(bin, appBinName)
	desktopPath = filepath.Join(apps, appBinName+".desktop")
	for _, s := range assets.IconSizes {
		iconPaths = append(iconPaths,
			filepath.Join(icons, fmt.Sprintf("%dx%d", s, s), "apps", appBinName+".png"))
	}
	iconPaths = append(iconPaths,
		filepath.Join(icons, "scalable", "apps", appBinName+".svg"))
	return binPath, desktopPath, iconPaths
}

// refreshCaches is best-effort: a missing tool must never fail the install.
func refreshCaches(icons, apps string) {
	if p, err := exec.LookPath("gtk-update-icon-cache"); err == nil {
		_ = exec.Command(p, "-f", "-t", icons).Run()
	}
	if p, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(p, apps).Run()
	}
}

// platformInstall performs the Unix installation.
func platformInstall() error {
	binDir, iconRoot, appsDir := installPaths()
	binPath, desktopPath, _ := installedFiles()

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating running binary: %w", err)
	}
	self, _ = filepath.EvalSymlinks(self)

	// 1. Binary onto PATH (skip if running the installed copy).
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	if self != binPath {
		data, err := os.ReadFile(self)
		if err != nil {
			return fmt.Errorf("reading running binary: %w", err)
		}
		if err := os.WriteFile(binPath, data, 0o755); err != nil {
			return fmt.Errorf("writing %s: %w", binPath, err)
		}
		fmt.Println("installed binary:", binPath)
	} else {
		fmt.Println("binary already running from:", binPath)
	}

	// 2. Icons into hicolor.
	for _, sz := range assets.IconSizes {
		src := fmt.Sprintf("icons/%s-%d.png", appBinName, sz)
		dstDir := filepath.Join(iconRoot, fmt.Sprintf("%dx%d", sz, sz), "apps")
		if err := installAsset(src, filepath.Join(dstDir, appBinName+".png")); err != nil {
			return err
		}
	}
	if err := installAsset("icons/"+appBinName+".svg",
		filepath.Join(iconRoot, "scalable", "apps", appBinName+".svg")); err != nil {
		return err
	}
	fmt.Println("installed icons:", iconRoot, "(128/256/512/1024 + scalable)")

	// 3. Desktop entry.
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(desktopPath, []byte(desktopEntry), 0o644); err != nil {
		return err
	}
	fmt.Println("installed desktop entry:", desktopPath)

	// 4. Env-file template for desktop launches (never overwrite).
	if ensureEnvTemplate() {
		fmt.Println("created key file for desktop launches:", envFilePath())
	}

	refreshCaches(iconRoot, appsDir)
	fmt.Println("done. Set your API key and run:", appBinName)
	return nil
}

// platformUninstall removes everything platformInstall created.
func platformUninstall() error {
	_, iconRoot, appsDir := installPaths()
	binPath, desktopPath, iconPaths := installedFiles()
	removed := 0
	for _, p := range append(iconPaths, desktopPath, binPath) {
		if err := os.Remove(p); err == nil {
			fmt.Println("removed:", p)
			removed++
		}
	}
	refreshCaches(iconRoot, appsDir)
	if removed == 0 {
		fmt.Println("nothing to remove (was it installed in this scope?)")
	}
	return nil
}
