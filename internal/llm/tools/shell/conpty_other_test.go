//go:build !windows

package shell

import "testing"

const noConPTY = "ConPTY is a Windows API; elsewhere the shell is the user's own terminal"

func TestReadHostInConsole_conpty(t *testing.T)             { t.Skip(noConPTY) }
func TestConsoleIsARealConsole_conpty(t *testing.T)         { t.Skip(noConPTY) }
func TestConsoleTimeoutStopsTheCommand_conpty(t *testing.T) { t.Skip(noConPTY) }
