package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContextFromPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	_, err := config.Load(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := config.Get()
	cfg.WorkingDir = tmpDir
	cfg.ContextPaths = []string{
		"file.txt",
		"directory/",
	}
	testFiles := []string{
		"file.txt",
		"directory/file_a.txt",
		"directory/file_b.txt",
		"directory/file_c.txt",
	}

	createTestFiles(t, tmpDir, testFiles)

	// The context cache is process-wide and config.Load() is a no-op once
	// something else has loaded it, so a fresh temp dir is not enough — the
	// cache has to be told the roots changed. This is the same call /add-dir
	// and /cd make. Without it this test passes at -count=1 and fails at
	// -count=2+, being served the first run's content.
	InvalidateContextCache()

	context := getContextFromPaths()

	// GORILLA OVERRIDE (2026-09-01): build the expected paths the way the code
	// builds them. The expectation was assembled by string concatenation with
	// forward slashes while getContextFromPaths uses filepath.Join — so on
	// Windows it compared `...\001/file.txt` against `...\001\file.txt` and
	// could never pass, while saying nothing about whether context loading
	// actually worked.
	//
	// The label after "# From:" is a real path and should look like one on the
	// platform it names. The relative header inside each file keeps its forward
	// slashes, because that is what the fixture writes into the file.
	j := func(rel string) string { return filepath.Join(tmpDir, filepath.FromSlash(rel)) }
	expectedContext := fmt.Sprintf(
		"# From:%s\nfile.txt: test content\n"+
			"# From:%s\ndirectory/file_a.txt: test content\n"+
			"# From:%s\ndirectory/file_b.txt: test content\n"+
			"# From:%s\ndirectory/file_c.txt: test content",
		j("file.txt"), j("directory/file_a.txt"), j("directory/file_b.txt"), j("directory/file_c.txt"))
	assert.Equal(t, expectedContext, context)
}

func createTestFiles(t *testing.T, tmpDir string, testFiles []string) {
	t.Helper()
	for _, path := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if path[len(path)-1] == '/' {
			err := os.MkdirAll(fullPath, 0755)
			require.NoError(t, err)
		} else {
			dir := filepath.Dir(fullPath)
			err := os.MkdirAll(dir, 0755)
			require.NoError(t, err)
			err = os.WriteFile(fullPath, []byte(path+": test content"), 0644)
			require.NoError(t, err)
		}
	}
}
