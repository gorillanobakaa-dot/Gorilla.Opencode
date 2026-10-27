package tools

import (
	"regexp"
	"strings"
)

// Programs whose non-zero exit code is information, not failure. Reporting
// `robocopy` exit 1 ("files were copied") as a failed command made the
// model retry a copy that had worked, and the receipt and the claim guard
// call a successful mirror a failure. The list is small on purpose: each
// entry cites where its codes are defined, and anything not here keeps the
// plain rule (non-zero = failure).

// informationalExit decodes one program's exit code. ok is false when the
// code is a real failure (or unknown); meaning is said either way when known.
type informationalExit func(code int) (meaning string, ok bool)

var informationalExits = map[string]informationalExit{
	// Robocopy: "Return codes that are used by the Robocopy utility",
	// learn.microsoft.com/troubleshoot/windows-server/backup-and-storage/return-codes-used-robocopy-utility
	// Bit flags: 1 files were copied, 2 extra files or folders in the
	// destination, 4 mismatched files or folders; 8 some could not be copied
	// (retries exhausted), 16 serious error (nothing copied). 0-7 = success.
	"robocopy": robocopyExit,
	// fc: Microsoft's fc page (learn.microsoft.com/windows-server/administration/windows-commands/fc)
	// lists no codes. Measured on Windows 11 26200 through PowerShell
	// ($LASTEXITCODE): 0 the files are the same, 1 they differ, 2 a file
	// could not be opened, -1 a bad switch.
	"fc": func(code int) (string, bool) {
		switch code {
		case 1:
			return "fc: the files differ (a successful comparison)", true
		case 2:
			return "fc: a file could not be opened", false
		}
		return "", false
	},
	// GNU diff and cmp: "An exit status of 0 means no differences were
	// found, 1 means some differences were found, and 2 means trouble."
	// gnu.org/software/diffutils/manual/html_node/Invoking-diff.html and
	// .../Invoking-cmp.html
	"diff": diffExit("diff"),
	"cmp":  diffExit("cmp"),
}

func diffExit(name string) informationalExit {
	return func(code int) (string, bool) {
		if code == 1 {
			return name + ": the files differ (a successful comparison)", true
		}
		return "", false
	}
}

func robocopyExit(code int) (string, bool) {
	if code < 0 {
		return "", false
	}
	var parts []string
	if code&1 != 0 {
		parts = append(parts, "files were copied")
	}
	if code&2 != 0 {
		parts = append(parts, "the destination has extra files or folders not in the source")
	}
	if code&4 != 0 {
		parts = append(parts, "mismatched files or folders were found")
	}
	if code&8 != 0 {
		parts = append(parts, "some files or folders could NOT be copied (retries exhausted)")
	}
	if code&16 != 0 {
		parts = append(parts, "serious error: nothing was copied (bad usage, or no access to the source or destination)")
	}
	if code >= 32 {
		return "", false
	}
	if code < 8 {
		return "robocopy: success - " + strings.Join(parts, "; "), true
	}
	return "robocopy: FAILED - " + strings.Join(parts, "; "), false
}

var cmdletRe = regexp.MustCompile(`^[A-Za-z]+-[A-Za-z]+$`)

// ExitCodeMeaning says what a non-zero exit code of command means when the
// program that set it is one whose codes are information. success is true
// when the code is not a failure for that program. The program is the last
// segment of the line that is not a PowerShell cmdlet (Verb-Noun): cmdlets
// do not set the exit code, so in `robocopy a b /E | Out-Null` it is
// robocopy's. Anything else returns ("", false): non-zero stays failure.
func ExitCodeMeaning(command string, code int) (meaning string, success bool) {
	if code == 0 || strings.TrimSpace(command) == "" {
		return "", false
	}
	segs := splitShellCommands(command)
	for i := len(segs) - 1; i >= 0; i-- {
		base := baseCommandOf(segs[i])
		if cmdletRe.MatchString(base) {
			continue
		}
		base = strings.ToLower(base[strings.LastIndexAny(base, `/\`)+1:])
		base = strings.TrimSuffix(base, ".exe")
		if decode, ok := informationalExits[base]; ok {
			return decode(code)
		}
		return "", false
	}
	return "", false
}
