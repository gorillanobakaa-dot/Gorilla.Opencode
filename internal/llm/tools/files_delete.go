package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/pathutil"
	"github.com/opencode-ai/opencode/internal/permission"
)

// delete is Remove-Item with the Recycle Bin as the default. path alone is
// that item (a folder with content needs recurse=true, as Remove-Item asks
// for -Recurse); path + pattern is the matching files in that folder (and
// below it with recurse). Read-only files are deleted too (-Force).
// permanent=true deletes for good: asked as irreversible, and refused when
// the person asked for the trash (trash.go rule 2).

func deleteTargets(p filesParams) (targets []string, files, dirs int, bytes int64, err error) {
	fi, err := os.Lstat(p.Path)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	count := func(root string) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && !isLinkEntry(d) {
				dirs++
				return nil
			}
			files++
			if info, e := d.Info(); e == nil && info.Mode().IsRegular() {
				bytes += info.Size()
			}
			return nil
		})
	}
	if p.Pattern == "" {
		if fi.IsDir() && !p.Recurse {
			if entries, _ := os.ReadDir(p.Path); len(entries) > 0 {
				return nil, 0, 0, 0, fmt.Errorf("%s is a folder with %d item(s) in it; recurse=true deletes it with everything inside", p.Path, len(entries))
			}
		}
		count(p.Path)
		return []string{p.Path}, files, dirs, bytes, nil
	}
	if !fi.IsDir() {
		return nil, 0, 0, 0, fmt.Errorf("pattern needs path to be a folder")
	}
	_ = filepath.WalkDir(p.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == p.Path {
			return nil
		}
		if d.IsDir() && !isLinkEntry(d) {
			if !p.Recurse {
				return filepath.SkipDir
			}
			return nil
		}
		if nameMatches(p.Pattern, d.Name()) {
			targets = append(targets, path)
			files++
			if info, e := d.Info(); e == nil {
				bytes += info.Size()
			}
		}
		return nil
	})
	return targets, files, dirs, bytes, nil
}

// refuseDelete stops a delete of a drive root, the profile folder, the
// working directory or a folder holding it.
func refuseDelete(p string) string {
	if why := refuseRoot("target", p); why != "" {
		return why
	}
	if wd := config.WorkingDirectory(); wd != "" && pathutil.Within(p, wd) {
		return fmt.Sprintf("refused: %s is the working directory or holds it; nothing was deleted", p)
	}
	return ""
}

func (t filesTool) delete(ctx context.Context, p filesParams) (ToolResponse, error) {
	if why := refuseDelete(p.Path); why != "" {
		return NewTextErrorResponse(why), nil
	}
	sessionID, _ := GetContextValues(ctx)
	if p.Permanent && TrashAsked(sessionID) {
		return NewTextErrorResponse("NOT RUN: permanent=true " + PermanentDeleteWhy +
			", and the user asked for the trash / Recycle Bin. Nothing was deleted. Call delete without permanent " +
			"(it goes to the Recycle Bin). If the user wants it gone for good, they must say so (\"delete permanently\")."), nil
	}
	targets, files, dirs, bytes, err := deleteTargets(p)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	where := "to the Recycle Bin"
	irreversible := ""
	if p.Permanent {
		where = "FOR GOOD (" + PermanentDeleteWhy + ")"
		irreversible = fmt.Sprintf("%s: %d file(s), %d folder(s)", PermanentDeleteWhy, files, dirs)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DRY RUN (nothing changed yet): delete %s: %d file(s), %d folder(s), %s\n", where, files, dirs, filesHumanBytes(bytes))
	writeList(&b, "  - ", targets, p.Full)
	preview := b.String()
	if p.DryRun || len(targets) == 0 {
		return NewTextResponse(preview), nil
	}
	if !t.ask(ctx, "delete", p.Path, "delete:"+p.Path+":"+p.Pattern, preview, irreversible, targets) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	if p.Permanent {
		var failed []string
		for _, target := range targets {
			if err := forceRemoveAll(target); err != nil {
				failed = append(failed, err.Error())
			}
		}
		if len(failed) > 0 {
			var fb strings.Builder
			fmt.Fprintf(&fb, "%d item(s) could not be deleted:\n", len(failed))
			writeList(&fb, "  ! ", failed, p.Full)
			return NewTextErrorResponse(fb.String()), nil
		}
	} else if err := recycleFunc(targets); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	return NewTextResponse(strings.Replace(preview, "DRY RUN (nothing changed yet)", "DONE", 1)), nil
}
