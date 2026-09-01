// GORILLA OVERRIDE: this file did not exist upstream. It adds
// `gorilla-opencode install` / `gorilla-opencode uninstall` so that people who
// do not live in a terminal can get a working desktop application from a
// single downloaded binary. Everything it creates is listed by path as it goes
// and removed again by `uninstall` — no hidden state.
//
// GORILLA OVERRIDE (2026-09-01): split per platform.
//
// This used to BE the Linux implementation: hicolor icon directories, a
// .desktop entry, gtk-update-icon-cache, os.Geteuid. A Windows user running
// `install` got a .desktop file written into their home directory, no shortcut
// on the desktop, nothing in the Start menu, and a binary left wherever they
// had downloaded it. The command reported success.
//
// The platform-specific halves now live in install_unix.go and
// install_windows.go; what remains here is the command surface they share.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencode-ai/opencode/internal/assets"
	"github.com/spf13/cobra"
)

const appBinName = "gorilla-opencode"

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install this binary, its icons, and a launcher you can click",
	Long: `Copies this binary somewhere permanent, unpacks its embedded icons, and
creates a launcher you can click — so the program is a normal application
rather than a file you have to find again.

Windows: installs into %LOCALAPPDATA%\Programs, adds a Desktop shortcut and a
Start menu entry, and puts itself on your PATH. No administrator rights needed.

Linux/macOS: installs under ~/.local (no sudo needed), or under /usr/local for
all users when run as root, and writes a desktop entry into the application
grid.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return platformInstall()
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove everything `install` created",
	RunE: func(cmd *cobra.Command, args []string) error {
		return platformUninstall()
	},
}

// installAsset writes one embedded file to disk, creating its directory.
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

// copySelf writes the running binary to dst. Returns false when the running
// binary IS dst, so the caller can say so rather than pointlessly rewriting it.
func copySelf(dst string) (copied bool, err error) {
	self, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("locating running binary: %w", err)
	}
	if resolved, rerr := filepath.EvalSymlinks(self); rerr == nil {
		self = resolved
	}
	if sameFile(self, dst) {
		return false, nil
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return false, fmt.Errorf("reading running binary: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return false, fmt.Errorf("writing %s: %w", dst, err)
	}
	return true, nil
}

// sameFile reports whether two paths name the same file on disk.
func sameFile(a, b string) bool {
	if a == b {
		return true
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func init() {
	rootCmd.AddCommand(installCmd, uninstallCmd)
}
