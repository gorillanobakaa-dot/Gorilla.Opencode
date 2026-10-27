package toolbox

import (
	"fmt"
	"os"
	"runtime"

	"github.com/opencode-ai/opencode/internal/pathutil"
)

// CacheDirIsPrivate refuses a cache directory that other local users can write
// to. A file that lands there is about to be EXECUTED, so a directory anyone
// can drop a file into is a local privilege-escalation path. (Moved here from
// internal/llm/tools/find.go, which now calls it.)
//
// GORILLA OVERRIDE (2026-09-01): the Unix permission-bit test cannot be run on
// Windows and must not simply be skipped there.
//
// Go's os.Stat reports a synthesised 0777 for every directory on Windows — it
// does not read the ACL — so the original `perm & 0o022 != 0` test was true for
// EVERY directory and the find tool refused to run at all. The local session
// database has two recorded failures reading "cache dir
// C:\Users\...\AppData\Local\gorilla-opencode is writable by other users;
// refusing to execute from it", which is that false positive.
//
// Rev 1 responded by skipping the check on Windows entirely, which removed the
// protection instead of translating it. What the check is really asking is "can
// somebody else put a file here", and on Windows the answer is carried by
// location rather than by mode bits: %LOCALAPPDATA% lives under the user's own
// profile, which is ACL'd to that user. So the Windows form of the same question
// is "is this inside my profile" — a real check, not a waived one. It needs no
// existing folder, which is why ensurePrivateDir runs it before making one.
func CacheDirIsPrivate(dir string) error {
	if runtime.GOOS != "windows" {
		st, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if st.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("cache dir %s is writable by other users; refusing to execute from it", dir)
		}
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine the user profile directory to validate %s: %w", dir, err)
	}
	// pathutil.Within, not a string comparison: the same folder has a short
	// (8.3) and a long spelling, and a junction to elsewhere is not inside.
	if !pathutil.Within(home, dir) {
		return fmt.Errorf(
			"cache dir %s is outside your user profile (%s), so other users may be able to write to it; "+
				"refusing to execute a helper from there", dir, home)
	}
	return nil
}
