package completions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bigTree makes a folder with n small files spread over sub-folders and makes
// it the working folder for the test.
func bigTree(t *testing.T, n int) {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		sub := filepath.Join(dir, fmt.Sprintf("d%03d", i/100))
		if i%100 == 0 {
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(sub, fmt.Sprintf("file%05d.txt", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// THE FAULT: opened in a folder with a great many files, the program listed all
// of them before its window could appear. In the owner's home folder that was
// 3 GB of memory and more than two minutes with nothing on screen.
//
// The list shown when nothing has been typed must be a screenful, and must come
// back quickly however large the folder is.
func TestTheSuggestionListIsAScreenfulHoweverLargeTheFolder(t *testing.T) {
	bigTree(t, 3000)
	cg := &filesAndFoldersContextGroup{prefix: "file"}

	start := time.Now()
	got, err := cg.getFiles("")
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("no suggestions at all in a folder with 3000 files")
	}
	if len(got) > completionShown {
		t.Errorf("%d suggestions returned; the list is capped at %d. Returning the whole folder is the fault", len(got), completionShown)
	}
	if took > completionBudget+2*time.Second {
		t.Errorf("the empty-query list took %s; the budget is %s", took, completionBudget)
	}
}

// A typed query still finds a file that is far down the tree, and the answer is
// still capped.
func TestATypedQueryStillFindsAFileDeepInTheFolder(t *testing.T) {
	bigTree(t, 3000)
	cg := &filesAndFoldersContextGroup{prefix: "file"}
	got, err := cg.getFiles("file02987")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range got {
		found = found || strings.Contains(g, "file02987.txt")
	}
	if !found {
		t.Errorf("a file that exists was not suggested for its own name; got %d results", len(got))
	}
	if all, _ := cg.getFiles("file"); len(all) > completionShown {
		t.Errorf("a broad query returned %d suggestions; the cap is %d", len(all), completionShown)
	}
}

// The time limit is real: with a budget too small to finish, the listing comes
// back anyway, with whatever it had, and does not leave a lister running.
func TestTheListingStopsWhenItsTimeIsUp(t *testing.T) {
	bigTree(t, 3000)
	old := completionBudget
	completionBudget = 30 * time.Millisecond
	defer func() { completionBudget = old }()

	cg := &filesAndFoldersContextGroup{prefix: "file"}
	start := time.Now()
	if _, err := cg.getFiles("zzz-no-such-file"); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("a 30 ms budget took %s; the lister was not stopped", took)
	}
}

// Hidden files and folders stay out of the suggestions.
func TestHiddenFilesAreNotSuggested(t *testing.T) {
	bigTree(t, 100)
	if err := os.MkdirAll(".git", 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(".git", "config"), nil, 0o644)
	_ = os.WriteFile(".env", nil, 0o644)
	cg := &filesAndFoldersContextGroup{prefix: "file"}
	for _, q := range []string{"", "config", "env"} {
		got, _ := cg.getFiles(q)
		for _, g := range got {
			if strings.Contains(g, ".git") || strings.HasSuffix(g, ".env") {
				t.Errorf("query %q suggested a hidden path: %s", q, g)
			}
		}
	}
}
