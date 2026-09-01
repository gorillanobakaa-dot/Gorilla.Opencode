package main

import (
	"github.com/opencode-ai/opencode/cmd"
	"github.com/opencode-ai/opencode/internal/bootstrap"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/osutil"
	"github.com/spf13/cobra"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	cobra.MousetrapHelpText = ""
	// GORILLA OVERRIDE (2026-09-01): name the window before anything paints.
	//
	// Launched through conhost — which the Windows shortcut does so the app owns
	// its own window, and therefore its own taskbar icon — the title bar
	// otherwise reads "C:\Windows\System32\conhost.exe". That is the host's
	// path, not the program's name, and it is the first thing anyone sees.
	osutil.SetWindowTitle("Gorilla OpenCode")
	bootstrap.EnsureWindowsDependencies()
	cmd.Execute()
}
