//go:build !windows

package shell

import (
	"context"
	"errors"
	"time"
)

// ConsoleSupported reports whether RunInConsole works on this platform.
func ConsoleSupported() bool { return false }

// ConsoleResult is what a console command left behind.
type ConsoleResult struct {
	Output      string
	ExitCode    int
	Interrupted bool
}

// RunInConsole is the Windows ConPTY mode. Elsewhere the bash tool's shell
// is the user's own, and console mode is not offered.
func RunInConsole(context.Context, string, string, string, string, time.Duration) (ConsoleResult, error) {
	return ConsoleResult{}, errors.New("console mode is Windows-only")
}
