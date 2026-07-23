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
	for i := 0; i < 40; i++ {
		name := filepath.Join(dir, string(rune('a'+i%26))+filepath.Base(t.Name())+string(rune('0'+i/26))+".txt")
		// unique names via index
		name = filepath.Join(dir, filepath.Base(t.Name())+"_"+itoa(i)+".txt")
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o644))
	}
	out := listTopLevelBrief(dir, 10)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// 10 entries + one "+N more" line
	assert.Equal(t, 11, len(lines))
	assert.Contains(t, out, "+30 more")
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
