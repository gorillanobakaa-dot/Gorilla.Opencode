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
	expectedContext := fmt.Sprintf("# From:%s/file.txt\nfile.txt: test content\n# From:%s/directory/file_a.txt\ndirectory/file_a.txt: test content\n# From:%s/directory/file_b.txt\ndirectory/file_b.txt: test content\n# From:%s/directory/file_c.txt\ndirectory/file_c.txt: test content", tmpDir, tmpDir, tmpDir, tmpDir)
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
