// GORILLA OVERRIDE: this file did not exist upstream. It adds
// `opencode-dino install` / `opencode-dino uninstall` so that people who
// do not live in a terminal can get a working desktop application from a
// single downloaded binary: the binary copies itself onto the PATH,
// unpacks its embedded icons into the hicolor theme, writes a desktop
// entry, and refreshes the caches. Everything it creates is listed by
// path below and removed again by `uninstall` — no hidden state.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/opencode-ai/opencode/internal/assets"
	"github.com/spf13/cobra"
)

const (
	appBinName   = "opencode-dino"
	desktopEntry = `[Desktop Entry]
Type=Application
Name=OpenCode Dino
Comment=Terminal AI coding agent (revived original OpenCode) — bring your own API keys
Exec=` + appBinName + `
Icon=` + appBinName + `
Terminal=true
Categories=Development;IDE;
Keywords=ai;coding;agent;terminal;llm;
`
)

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

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install this binary, its icons, and a desktop entry",
	Long: `Copies this binary onto your PATH, unpacks the embedded icons into the
hicolor icon theme, writes a desktop entry so the app appears in your
application grid, and refreshes the desktop caches.

Run as a normal user it installs under ~/.local (no sudo needed).
Run as root it installs under /usr/local for all users.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		for _, s := range assets.IconSizes {
			src := fmt.Sprintf("icons/%s-%d.png", appBinName, s)
			dstDir := filepath.Join(iconRoot, fmt.Sprintf("%dx%d", s, s), "apps")
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

		refreshCaches(iconRoot, appsDir)
		fmt.Println("done. Set your API key and run:", appBinName)
		fmt.Println("  NVIDIA NIM: LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1 LOCAL_ENDPOINT_API_KEY=nvapi-...")
		fmt.Println("  Google:     GEMINI_API_KEY=...")
		fmt.Println("  Ollama:     LOCAL_ENDPOINT=http://localhost:11434/v1")
		return nil
	},
}

func installAsset(embedPath, dst string) error {
	data, err := assets.Icons.ReadFile(embedPath)
	if err != nil {
		return fmt.Errorf("embedded asset %s: %w", embedPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove everything `install` created",
	RunE: func(cmd *cobra.Command, args []string) error {
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
	},
}

func init() {
	rootCmd.AddCommand(installCmd, uninstallCmd)
}
