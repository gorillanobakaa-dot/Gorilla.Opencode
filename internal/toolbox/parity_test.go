package toolbox

import (
	"sort"
	"strings"
	"testing"
)

// GORILLA RULE (owner, 2026-10-31): "we need to make sure that the windows side
// does not get MORE capabilities than the Linux one ... We do not want to be
// accused of favoritism." Every helper pinned for one system is pinned for the
// other, by the same name. Runs in every build, tagged or not: it reads only
// SHA256SUMS, which is always compiled in.

var paritySystems = []string{"windows-amd64", "linux-amd64"}

// parityBacklog is the gap that existed when the rule was made: helpers pinned
// for Windows only (job 04, 2026-10-27). It may only SHRINK: a helper ported to
// Linux must leave this list (the test says so), and nothing may be added here
// to let a new one-system helper in. Port it to the other system instead.
var parityBacklog = map[string]bool{
	"ast-grep": true, "coreutils": true, "difft": true, "hyperfine": true,
	"ninja": true, "pfind": true, "sd": true,
}

func pinnedHelpers(pins map[string]string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for key := range pins {
		platform, file, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(file, ".exe")
		if out[platform] == nil {
			out[platform] = map[string]bool{}
		}
		out[platform][name] = true
	}
	return out
}

// parityProblems returns every helper one system has and the other lacks,
// outside the frozen backlog, and every backlog entry that is no longer a gap.
func parityProblems(pins map[string]string, backlog map[string]bool) []string {
	have := pinnedHelpers(pins)
	var problems []string
	gap := map[string]bool{}
	for _, a := range paritySystems {
		for _, b := range paritySystems {
			if a == b {
				continue
			}
			for name := range have[a] {
				if have[b][name] {
					continue
				}
				gap[name] = true
				if !backlog[name] {
					problems = append(problems, name+": pinned for "+a+" but not for "+b+
						" (port it; the backlog may not grow)")
				}
			}
		}
	}
	for name := range backlog {
		if !gap[name] {
			problems = append(problems, name+": on both systems now; remove it from parityBacklog")
		}
	}
	sort.Strings(problems)
	return problems
}

func TestToolboxSameHelpersOnWindowsAndLinux(t *testing.T) {
	pins, err := ParsePins(pinsText)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range parityProblems(pins, parityBacklog) {
		t.Error(p)
	}
}

// The check itself must be able to fail, both ways.
func TestParityCheckCatchesAOneSystemHelper(t *testing.T) {
	h := strings.Repeat("a", 64)
	pins := map[string]string{"windows-amd64/rg.exe": h, "linux-amd64/fd": h, "windows-amd64/sd.exe": h,
		"linux-amd64/sd": h}
	got := strings.Join(parityProblems(pins, map[string]bool{"jq": true}), "\n")
	for _, want := range []string{"rg: pinned for windows-amd64 but not for linux-amd64",
		"fd: pinned for linux-amd64 but not for windows-amd64", "jq: on both systems now"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "sd:") {
		t.Errorf("sd is on both systems and must pass:\n%s", got)
	}
}
