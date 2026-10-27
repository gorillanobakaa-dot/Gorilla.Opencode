//go:build !windows && !linux && !darwin

package agent

// isZombie: no portable way to ask on the other BSDs; a zombie counts as
// running there.
func isZombie(pid int) bool { return false }
