package prompt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTopLevelBrief_DepthOneOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("x"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "pkg"), 0o755))
	// Nested file must NOT appear in depth-1 listing.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "deep.go"), []byte("x"), 0o644))
	// Hidden entries skipped.
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".secret"), []byte("x"), 0o644))

	out := listTopLevelBrief(dir, 25)
	assert.Contains(t, out, "a.go")
	assert.Contains(t, out, "b.md")
	assert.Contains(t, out, "pkg/")
	assert.NotContains(t, out, "deep.go")
	assert.NotContains(t, out, ".secret")
}

func TestListTopLevelBrief_Cap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// GORILLA FIX (2026-08-09): these used to be name_0.txt … name_39.txt, which
	// is precisely a version family, so collapseVersionFamilies now folds all 40
	// into one line and the cap never engages. The test's intent - does the limit
	// hold? - is still valid; only the fixture was accidentally exercising the
	// new behaviour. Distinct stems keep it testing the cap.
	for i := 0; i < 40; i++ {
		name := filepath.Join(dir, "stem"+string(rune('a'+i%26))+string(rune('a'+i/26))+".txt")
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o644))
	}
	out := listTopLevelBrief(dir, 10)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// 10 entries + one "+N more" line
	assert.Equal(t, 11, len(lines))
	assert.Contains(t, out, "+30 more")
}

// Directories must lead, whatever the alphabet says. Before this, sort.Strings
// over everything meant ASCII order, CAPITALS first - and in the real repo that
// let GITHUB-RELEASE-NOTES-* consume all 25 slots while cmd/ and internal/ never
// appeared at all.
func TestListTopLevelBrief_DirectoriesFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AAA-README.md"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ZZZ-notes.md"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "zzz_src"), 0o755))

	out := listTopLevelBrief(dir, 25)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(t, "zzz_src/", lines[0],
		"a directory must lead even when its name sorts last: %v", lines)
}

// A run of files differing only by a version number collapses to one line.
//
// This is not only about tokens. Thirteen consecutive
// GITHUB-RELEASE-NOTES-0.1.65 … 0.1.77 lines read as a monotonic counter, and on
// 2026-08-09 a model sent the single word "oi" continued the sequence: it ran
// `git tag -a v0.1.78`, and the tag was really created. One collapsed line
// carries the same fact and invites nothing.
func TestListTopLevelBrief_CollapsesVersionFamilies(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 65; i <= 77; i++ {
		n := filepath.Join(dir, "GITHUB-RELEASE-NOTES-0.1."+itoa(i)+".md")
		require.NoError(t, os.WriteFile(n, []byte("x"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644))

	out := listTopLevelBrief(dir, 25)
	assert.Contains(t, out, "(13 files)", "the family should be counted, not listed: %s", out)
	assert.Contains(t, out, "README.md", "unrelated files must survive")
	// The point of the exercise: no individual version number reaches the model.
	assert.NotContains(t, out, "0.1.77")
	assert.NotContains(t, out, "0.1.65")
}

// Two files are not a family. Collapsing them would hide more than it saves.
func TestListTopLevelBrief_LeavesSmallGroupsAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note-1.md"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note-2.md"), []byte("x"), 0o644))

	out := listTopLevelBrief(dir, 25)
	assert.Contains(t, out, "note-1.md")
	assert.Contains(t, out, "note-2.md")
	assert.NotContains(t, out, "files)")
}

func TestGitStatusBrief_CleanRepo(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi"), 0o644))
	run("git", "add", "f.txt")
	run("git", "commit", "-m", "init")

	out := gitStatusBrief(dir, 10)
	assert.Contains(t, out, "clean working tree")
	assert.True(t, strings.Contains(out, "branch:") || strings.Contains(out, "master") || strings.Contains(out, "main") || out == "clean working tree")
}

func TestGitStatusBrief_CapsLines(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi"), 0o644))
	run("git", "add", "f.txt")
	run("git", "commit", "-m", "init")
	for i := 0; i < 15; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "u_"+itoa(i)+".txt"), []byte("x"), 0o644))
	}
	out := gitStatusBrief(dir, 5)
	assert.Contains(t, out, "+10 more changed paths")
	// status lines (excluding branch line) should be capped
	nonBranch := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "branch:") || strings.HasPrefix(line, "…") {
			continue
		}
		if line != "" {
			nonBranch++
		}
	}
	assert.LessOrEqual(t, nonBranch, 5)
}

func TestProjectSummary_NoRecursiveDump(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hi"), 0o644))

	out := projectSummary(dir, false)
	assert.Contains(t, out, "README.md")
	assert.Contains(t, out, "src/")
	assert.NotContains(t, out, "main.go")
	assert.NotContains(t, out, "package main")
	// Rough size guard: shallow summary must stay tiny.
	assert.Less(t, len(out), 2000)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
