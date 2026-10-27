//go:build !windows

package shell

// stopSignal has no Unix counterpart: bash builtins are stopped by ending
// the shell, as before.
type stopSignal struct{}

func newStopSignal() *stopSignal        { return nil }
func (s *stopSignal) armScript() string { return "" }
func (s *stopSignal) fire() bool        { return false }
func (s *stopSignal) close()            {}
