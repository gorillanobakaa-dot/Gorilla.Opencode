package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The macOS installer turns off Terminal's "reopen windows after a quit"
// (NSQuitAlwaysKeepsWindows), because restored windows came back beside the
// program on the Air (owner's choice B, 2026-10-23), and Undo must put back the
// value it found, exactly: absent, or the same type and value. Here the two
// functions are cut out of the scripts and run against a stand-in for
// macOS's defaults command that keeps the setting in two files.

func cutShellFunc(t *testing.T, text, name string) string {
	t.Helper()
	start := strings.Index(text, name+"() {")
	if start < 0 {
		t.Fatalf("%s not found", name)
	}
	end := strings.Index(text[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("end of %s not found", name)
	}
	return text[start : start+end+3]
}

func restoreLines(t *testing.T, text string) string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "RESTORE_KEY=") || strings.HasPrefix(l, "RESTORE_REC=") {
			out = append(out, l)
		}
	}
	if len(out) != 2 {
		t.Fatalf("want RESTORE_KEY and RESTORE_REC, got %q", out)
	}
	return strings.Join(out, "\n") + "\n"
}

const fakeDefaults = `
defaults() {
  [ "$2" = com.apple.Terminal ] && [ "$3" = NSQuitAlwaysKeepsWindows ] || { echo "unexpected: defaults $*" >&2; return 2; }
  case "$1" in
    read) [ -f "$STORE/v" ] || return 1; cat "$STORE/v"; echo ;;
    read-type) [ -f "$STORE/t" ] || return 1; echo "Type is $(cat "$STORE/t")" ;;
    write)
      case "$4" in
        -bool) printf boolean >"$STORE/t"; case "$5" in true) printf 1 ;; false) printf 0 ;; *) return 2 ;; esac >"$STORE/v" ;;
        -int) printf integer >"$STORE/t"; printf '%s' "$5" >"$STORE/v" ;;
        -float) printf float >"$STORE/t"; printf '%s' "$5" >"$STORE/v" ;;
        -string) printf string >"$STORE/t"; printf '%s' "$5" >"$STORE/v" ;;
        *) return 2 ;;
      esac ;;
    delete) [ -f "$STORE/v" ] || return 1; rm -f "$STORE/v" "$STORE/t" ;;
    *) return 2 ;;
  esac
}
state() { if [ -f "$STORE/v" ]; then printf '%s=%s' "$(cat "$STORE/t")" "$(cat "$STORE/v")"; else printf unset; fi; }
rec() { if [ -f "$RESTORE_REC" ]; then printf '[%s]' "$(cat "$RESTORE_REC")"; else printf none; fi; }
`

func TestMacTerminalRestoreOffAndBack(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(bash), `\windows\system32\`) {
		t.Skip("bash here is WSL's, not Git Bash")
	}
	install, err := os.ReadFile("../packaging/macos/Install Gorilla OpenCode.command")
	if err != nil {
		t.Fatal(err)
	}
	undo, err := os.ReadFile("../packaging/macos/Undo Gorilla OpenCode.command")
	if err != nil {
		t.Fatal(err)
	}
	if a, b := restoreLines(t, string(install)), restoreLines(t, string(undo)); a != b {
		t.Fatalf("the installer and Undo name different records:\n%s\n%s", a, b)
	}
	lib := restoreLines(t, string(install)) + cutShellFunc(t, string(install), "terminal_restore_off") +
		cutShellFunc(t, string(undo), "terminal_restore_back") + fakeDefaults
	// Install, install again (a repair), Undo: the state after each step, then the record.
	script := lib + `
before=$(state)
terminal_restore_off; a=$?; s1="$(state) $(rec)"
terminal_restore_off; b=$?; s2="$(state) $(rec)"
terminal_restore_back; c=$?; s3="$(state) $(rec)"
echo "$a $b $c | $before -> $s1 -> $s2 -> $s3"
`
	cases := []struct {
		name, setup, want string
	}{
		{"not set", ``,
			"0 0 0 | unset -> boolean=0 [unset] -> boolean=0 [unset] -> unset none"},
		{"on", `printf boolean >"$STORE/t"; printf 1 >"$STORE/v"`,
			"0 0 0 | boolean=1 -> boolean=0 [boolean 1] -> boolean=0 [boolean 1] -> boolean=1 none"},
		{"already off", `printf boolean >"$STORE/t"; printf 0 >"$STORE/v"`,
			"0 0 0 | boolean=0 -> boolean=0 [boolean 0] -> boolean=0 [boolean 0] -> boolean=0 none"},
		{"an integer", `printf integer >"$STORE/t"; printf 1 >"$STORE/v"`,
			"0 0 0 | integer=1 -> boolean=0 [integer 1] -> boolean=0 [integer 1] -> integer=1 none"},
		{"a string with a space", `printf string >"$STORE/t"; printf 'yes please' >"$STORE/v"`,
			"0 0 0 | string=yes please -> boolean=0 [string yes please] -> boolean=0 [string yes please] -> string=yes please none"},
		// A type it cannot write back: nothing is changed and nothing recorded.
		{"an array", `printf array >"$STORE/t"; printf x >"$STORE/v"`,
			"1 1 0 | array=x -> array=x none -> array=x none -> array=x none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			store := filepath.Join(home, "store")
			if err := os.Mkdir(store, 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bash, "-c", c.setup+"\n"+script)
			cmd.Env = append(os.Environ(), "HOME="+filepath.ToSlash(home), "STORE="+filepath.ToSlash(store))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

// Undo with no record (the installer never changed the setting) changes nothing.
func TestMacTerminalRestoreBackWithoutRecord(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(bash), `\windows\system32\`) {
		t.Skip("bash here is WSL's, not Git Bash")
	}
	undo, err := os.ReadFile("../packaging/macos/Undo Gorilla OpenCode.command")
	if err != nil {
		t.Fatal(err)
	}
	script := restoreLines(t, string(undo)) + cutShellFunc(t, string(undo), "terminal_restore_back") + fakeDefaults + `
printf boolean >"$STORE/t"; printf 0 >"$STORE/v"
terminal_restore_back; echo "$? $(state)"
`
	home := t.TempDir()
	store := filepath.Join(home, "store")
	if err := os.Mkdir(store, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, "-c", script)
	cmd.Env = append(os.Environ(), "HOME="+filepath.ToSlash(home), "STORE="+filepath.ToSlash(store))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "0 boolean=0" {
		t.Errorf("got %q, want the setting untouched", got)
	}
}
