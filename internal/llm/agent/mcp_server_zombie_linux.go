package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// isZombie reads the state letter in /proc/<pid>/stat (after the command
// name, which may itself hold spaces and parentheses).
func isZombie(pid int) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] == 'Z'
	}
	return false
}
