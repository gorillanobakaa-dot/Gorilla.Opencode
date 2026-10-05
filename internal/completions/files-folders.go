package completions

import (
	"bufio"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/opencode-ai/opencode/internal/fileutil"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

type filesAndFoldersContextGroup struct {
	prefix string
}

func (cg *filesAndFoldersContextGroup) GetId() string {
	return cg.prefix
}

func (cg *filesAndFoldersContextGroup) GetEntry() dialog.CompletionItemI {
	return dialog.NewCompletionItem(dialog.CompletionItem{
		Title: "Files & Folders",
		Value: "files",
	})
}

// Limits on one listing. See getFiles.
const (
	// completionShown is how many suggestions a list can usefully hold. Nobody
	// reads past the first screenful; they type another letter instead.
	completionShown = 200
	// completionScan is how many file names are looked at for one query before
	// the search stops. A project of any ordinary size is far below it.
	completionScan = 60000
)

// completionBudget is how long one listing may take. A variable so tests can
// shorten it.
var completionBudget = 1500 * time.Millisecond

// getFiles lists files for the @-mention suggestions.
//
// GORILLA FIX (2026-10-05): BOUNDED. This used to list every file under the
// working folder, however many there were, before returning anything, and it is
// called once while the window is being built. So the program could not start
// until the whole tree had been walked.
//
// Measured on the owner's machine with the released v0.1.139:
//
//	opened in an empty folder   ready in 8.5 s,  79 MB
//	opened in C:\Users\gorilla1  NOT ready after 150 s, 3,157 MB, 125 s of CPU
//
// with `rg --files -L --null C:\Users\gorilla1` feeding `fzf --filter ""`: an
// empty filter passes everything, so every path in the home folder was read into
// memory twice over. The owner saw a window frozen on its first three lines.
// Every test had started the program in a small folder, where this costs
// nothing, which is why it was never seen.
//
// A suggestion list needs a screenful, not the tree. So the listing stops at
// whichever comes first: enough matches to show, completionScan names looked
// at, or completionBudget elapsed. The lister is then stopped, not left running.
// fzf is no longer used here: it cannot be told to stop early, and the fuzzy
// match it did is done in Go on the names already read.
func (cg *filesAndFoldersContextGroup) getFiles(query string) ([]string, error) {
	deadline := time.Now().Add(completionBudget)
	names := scanFileNames(deadline, func(seen int, kept []string) bool {
		if query == "" {
			return len(kept) >= completionShown
		}
		return seen >= completionScan
	})
	if query == "" {
		return names, nil
	}
	matches := fuzzy.Find(query, names)
	if len(matches) > completionShown {
		matches = matches[:completionShown]
	}
	return matches, nil
}

// scanFileNames yields visible file names under the workspace until enough()
// says stop or the deadline passes. It uses ripgrep when it is installed, which
// honours .gitignore, and a plain walk when it is not. In both cases the lister
// is stopped as soon as the answer is in hand.
func scanFileNames(deadline time.Time, enough func(seen int, kept []string) bool) []string {
	kept := make([]string, 0, completionShown)
	seen := 0
	take := func(path string) (stop bool) {
		seen++
		path = filepath.Join(".", path)
		if !fileutil.SkipHidden(path) {
			kept = append(kept, path)
		}
		return enough(seen, kept) || time.Now().After(deadline)
	}

	if cmd := fileutil.GetRgCmd(""); cmd != nil {
		out, err := cmd.StdoutPipe()
		if err == nil && cmd.Start() == nil {
			// Whatever happens below, the lister does not outlive this call.
			stopped := make(chan struct{})
			timer := time.AfterFunc(time.Until(deadline), func() { _ = cmd.Process.Kill() })
			go func() { _ = cmd.Wait(); close(stopped) }()

			r := bufio.NewReaderSize(out, 64<<10)
			for {
				name, err := r.ReadString(0)
				if n := strings.TrimSuffix(name, "\x00"); n != "" {
					if take(n) {
						break
					}
				}
				if err != nil {
					break
				}
			}
			timer.Stop()
			_ = cmd.Process.Kill()
			<-stopped
			return kept
		}
		logging.Debug("ripgrep could not be started for file completions; walking instead")
	}

	_ = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable folder is skipped, not fatal
		}
		if d.IsDir() {
			if path != "." && fileutil.SkipHidden(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if take(path) {
			return filepath.SkipAll
		}
		return nil
	})
	return kept
}

func (cg *filesAndFoldersContextGroup) GetChildEntries(query string) ([]dialog.CompletionItemI, error) {
	matches, err := cg.getFiles(query)
	if err != nil {
		return nil, err
	}

	items := make([]dialog.CompletionItemI, 0, len(matches))
	for _, file := range matches {
		item := dialog.NewCompletionItem(dialog.CompletionItem{
			Title: file,
			Value: file,
		})
		items = append(items, item)
	}

	return items, nil
}

func NewFileAndFolderContextGroup() dialog.CompletionProvider {
	return &filesAndFoldersContextGroup{
		prefix: "file",
	}
}
