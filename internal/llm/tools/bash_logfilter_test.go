package tools

import (
	"fmt"
	"strings"
	"testing"
)

// GORILLA OVERRIDE: filterBuildLog shipped untested. These tests exist
// because it was dropping the exact lines it was written to preserve —
// `ld:`, `cc1plus:` and any path under arch/ — while still announcing
// "showing the N signal lines". A filter that silently removes the error
// is worse than no filter, because the model reasons about the remainder
// as if it were complete.

// noisyBuild returns a log long enough (>=200 lines) and marker-dense
// enough (>=5) to engage the filter, with the given lines appended.
func noisyBuild(signal ...string) string {
	var b strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&b, "  CC      kernel/sched/core_%d.o\n", i)
	}
	for _, l := range signal {
		b.WriteString(l + "\n")
	}
	return b.String()
}

// TestFilterBuildLogKeepsRealErrors is the regression guard. Every line
// here is a real first-line-of-failure from gcc, clang, lld or make.
func TestFilterBuildLogKeepsRealErrors(t *testing.T) {
	errors := []string{
		"ld: cannot find -lssl",
		"ld.lld: error: undefined symbol: mozilla::Bar()",
		"cc1plus: error: unrecognized command line option '-fno-foo'",
		"cc1: fatal error: no input files",
		"arch/x86/kernel/head.o: undefined reference to `start_kernel'",
		"gen/nsGkAtomList.h:12:3: error: expected ';'",
		"make[2]: *** [Makefile:1842: vmlinux] Error 2",
	}

	for _, want := range errors {
		t.Run(want, func(t *testing.T) {
			got := filterBuildLog(noisyBuild(want))
			if !strings.Contains(got, want) {
				t.Errorf("filter dropped the error line.\n  want present: %q\n  got:\n%s", want, got)
			}
		})
	}
}

// TestFilterBuildLogStillDropsNoise is the other half: the guard above
// must not be satisfiable by disabling filtering. If this fails, the fix
// went too far and the filter no longer earns its tokens.
func TestFilterBuildLogStillDropsNoise(t *testing.T) {
	in := noisyBuild("ld: cannot find -lssl")
	got := filterBuildLog(in)

	if strings.Contains(got, "core_100.o") {
		t.Error("progress noise survived filtering; the filter is not filtering")
	}
	if inLines, gotLines := strings.Count(in, "\n"), strings.Count(got, "\n"); gotLines >= inLines/2 {
		t.Errorf("filter kept %d of %d lines; expected a large reduction", gotLines, inLines)
	}
	if !strings.Contains(got, "build log filtered") {
		t.Error("filtered output must say it was filtered")
	}
}

// TestFilterBuildLogLeavesShortOutputAlone guards the engage conditions:
// ordinary command output must pass through byte-for-byte.
func TestFilterBuildLogLeavesShortOutputAlone(t *testing.T) {
	in := "total 8\ndrwxr-xr-x 2 gorilla gorilla 4096 Aug  6 14:10 .\n"
	if got := filterBuildLog(in); got != in {
		t.Errorf("short non-build output was modified.\n want: %q\n  got: %q", in, got)
	}
}
