//go:build !windows

package hostprobe

import (
	"os"
	"path/filepath"

	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
)

func probeOS(f *Facts) {
	if f.OS == "linux" {
		f.OSName = osRelease()
	} else if f.OS == "darwin" {
		f.OSName = "macOS"
	}
	if os.Geteuid() == 0 {
		f.Admin = "yes"
	}
	f.Shell = filepath.Base(shell.ShellPath())
}

func winPowerShellVersion() string { return "" }
func pwshVersion() string          { return "" }
