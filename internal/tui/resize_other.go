//go:build !windows

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// WatchTerminalResize does nothing off Windows.
//
// GORILLA OVERRIDE: every other platform bubbletea supports delivers SIGWINCH,
// and bubbletea installs its own handler for it. Polling alongside that handler
// would send a second, redundant tea.WindowSizeMsg for every resize and make
// every component lay itself out twice.
//
// The signature matches the Windows build so the caller does not need a build
// tag of its own. See resize_windows.go for what this exists to correct.
func WatchTerminalResize(p *tea.Program, stop <-chan struct{}) {}
