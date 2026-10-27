package tools

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GORILLA FIX (2026-10-27, job 07): a plain listing is pfind --files with
// --max-depth 2, which lists FILES only. A folder whose files all sit deeper
// than that did not appear at all, and nothing said the listing was cut:
// "What is in the folder Folder?" was answered "a.txt, b.log and sub1" because
// Folder\sub2 held only sub2\deep\d.txt. The listing now ends with the folders
// right inside the searched one and says that deeper files are not shown.

// findDepthNoteMax caps how many folder names the note carries.
const findDepthNoteMax = 40

// depthCutNote is the line appended to a depth-limited plain listing of dir,
// or "" when dir holds no visible folder. Hidden and system folders (and
// dot-folders) are left out, as pfind leaves them out of the listing itself.
func depthCutNote(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if a, err := readAttrs(filepath.Join(dir, e.Name())); err == nil && a&(attrHidden|attrSystem) != 0 {
			continue
		}
		names = append(names, e.Name()+string(filepath.Separator))
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	more := ""
	if len(names) > findDepthNoteMax {
		more = ", ..."
		names = names[:findDepthNoteMax]
	}
	return "[files to depth 2 only; folders here: " + strings.Join(names, ", ") + more +
		" - files deeper than that are not listed: give path=FOLDER to look inside]"
}

// depthNoteFor is depthCutNote when the listing was cut, "" otherwise.
func depthNoteFor(cut bool, dir string) string {
	if !cut {
		return ""
	}
	return depthCutNote(dir)
}
