package tools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
)

// bashRequestPath is the path a shell command's permission request is about:
// the first path the command names that lies outside every workspace root, or
// the working directory when it names none.
//
// It reads the command; it does not run or expand it. Home spellings
// (~, $HOME, $env:USERPROFILE, %USERPROFILE%) are resolved because they are how
// a command reaches a person's own folders. The command word of each segment is
// skipped: C:\Program Files\Go\bin\go.exe is a program being run, not a place
// being touched.
func bashRequestPath(command string) string {
	home, _ := os.UserHomeDir()
	for _, seg := range splitShellCommands(command) {
		words := shellWords(seg)
		// GORILLA FIX (2026-10-23): the command word is the first word that is
		// not an environment assignment. Only i == 0 was skipped, so in
		// `PYTHONPATH=... /usr/local/bin/python3 -m fieldkit.core.doctor` the
		// program itself counted as a place outside the roots and the run asked
		// (MacBook Air, air-2026-10-23-perms-dialog-4.png).
		cmdWord := 0
		for cmdWord < len(words) && strings.Contains(words[cmdWord], "=") && !strings.HasPrefix(words[cmdWord], "-") {
			cmdWord++
		}
		for i, w := range words {
			if i <= cmdWord || strings.HasPrefix(w, "-") || strings.Contains(w, "://") {
				continue
			}
			// A redirection target or an option value may be glued on.
			if j := strings.LastIndexAny(w, "=>"); j >= 0 && j+1 < len(w) {
				w = w[j+1:]
			}
			if home != "" {
				low := strings.ToLower(w)
				for _, h := range []string{"~", "$home", "${home}", "$env:userprofile", "${env:userprofile}", "%userprofile%"} {
					if low == h || strings.HasPrefix(low, h+"/") || strings.HasPrefix(low, h+`\`) {
						w = home + w[len(h):]
						break
					}
				}
			}
			if w == "/dev/null" || strings.EqualFold(w, "nul") || !filepath.IsAbs(w) {
				continue
			}
			p := filepath.Clean(w)
			if _, inside := config.RootFor(p); !inside {
				return p
			}
		}
	}
	return config.WorkingDirectory()
}
