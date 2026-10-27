package tools

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// The Go engine does what robocopyArgs asks robocopy for, where there is no
// robocopy: the same plan (new or changed files by size and time, extras for
// mirror/purge, the copied files for move), the same junction rule (links are
// never followed: WalkDir does not descend into them and they are skipped),
// and the copy spread over op.Threads workers.

// nameMatches applies a pattern to a base name: case-insensitive on Windows,
// "" and "*" match everything.
func nameMatches(pattern, name string) bool {
	if pattern == "" || pattern == "*" || pattern == "*.*" {
		return true
	}
	if runtime.GOOS == "windows" {
		pattern, name = strings.ToLower(pattern), strings.ToLower(name)
	}
	ok, _ := filepath.Match(pattern, name)
	return ok
}

// isLinkEntry reports a symbolic link, a junction or another reparse point
// the walk must not follow.
func isLinkEntry(d fs.DirEntry) bool {
	return d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0
}

// sameFile is robocopy's default test: same size and same modification time
// (2 s tolerance, as FAT stores times).
func sameFile(a, b os.FileInfo) bool {
	d := a.ModTime().Sub(b.ModTime())
	return a.Size() == b.Size() && d < 2*time.Second && d > -2*time.Second
}

// goPlan works out a tree action without changing anything.
func goPlan(op syncOp) (syncPlan, error) {
	plan := syncPlan{Engine: "go", ExitCode: -1}
	srcRel := map[string]bool{}
	purgeOnly := op.Action == "purge"
	err := filepath.WalkDir(op.Src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			plan.Errors = append(plan.Errors, err.Error())
			return nil
		}
		rel, _ := filepath.Rel(op.Src, path)
		if isLinkEntry(d) {
			return nil
		}
		if rel == "." {
			return nil
		}
		srcRel[rel] = true
		target := filepath.Join(op.Dst, rel)
		if d.IsDir() {
			if _, err := os.Lstat(target); err != nil && !purgeOnly {
				plan.NewDirs = append(plan.NewDirs, path+string(filepath.Separator))
			}
			return nil
		}
		if !nameMatches(op.Pattern, d.Name()) || purgeOnly {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			plan.Errors = append(plan.Errors, err.Error())
			return nil
		}
		if old, err := os.Lstat(target); err == nil {
			if sameFile(info, old) {
				return nil
			}
			plan.Overwrites++
		}
		plan.Copies = append(plan.Copies, path)
		plan.Bytes += info.Size()
		return nil
	})
	if err != nil {
		return plan, err
	}
	if op.Action == "mirror" || op.Action == "purge" {
		if _, err := os.Stat(op.Dst); err == nil {
			_ = filepath.WalkDir(op.Dst, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(op.Dst, path)
				if rel == "." || srcRel[rel] {
					return nil
				}
				if d.IsDir() && !isLinkEntry(d) {
					plan.DeleteDirs = append(plan.DeleteDirs, path+string(filepath.Separator))
					// As robocopy's *EXTRA File lines: every file inside the
					// folder is named, so the preview shows what goes for good.
					_ = filepath.WalkDir(path, func(p string, e fs.DirEntry, err error) error {
						if err == nil && !e.IsDir() {
							plan.Deletes = append(plan.Deletes, p)
						}
						return nil
					})
					return filepath.SkipDir
				}
				if nameMatches(op.Pattern, d.Name()) {
					plan.Deletes = append(plan.Deletes, path)
				}
				return nil
			})
		}
	}
	if op.Action == "move" || op.Action == "move_files" {
		plan.Deletes = append(plan.Deletes, plan.Copies...)
	}
	return plan, nil
}

// goRun carries a plan out: folders, then the copies on op.Threads workers,
// then the deletes. It re-plans first, so what runs is what is there now.
func goRun(op syncOp) (syncPlan, error) {
	plan, err := goPlan(op)
	if err != nil {
		return plan, err
	}
	if err := os.MkdirAll(op.Dst, 0o755); err != nil {
		return plan, err
	}
	for _, d := range plan.NewDirs {
		rel, _ := filepath.Rel(op.Src, strings.TrimRight(d, `\/`))
		if err := os.MkdirAll(filepath.Join(op.Dst, rel), 0o755); err != nil {
			plan.Errors = append(plan.Errors, err.Error())
		}
	}
	var mu sync.Mutex
	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < filesThreads(op.Threads); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for src := range jobs {
				rel, _ := filepath.Rel(op.Src, src)
				fi, err := os.Stat(src)
				if err == nil {
					err = copyOneFile(src, filepath.Join(op.Dst, rel), fi)
				}
				if err != nil {
					mu.Lock()
					plan.Errors = append(plan.Errors, err.Error())
					mu.Unlock()
				}
			}
		}()
	}
	for _, c := range plan.Copies {
		jobs <- c
	}
	close(jobs)
	wg.Wait()
	if len(plan.Errors) > 0 && (op.Action == "move" || op.Action == "move_files") {
		// A source file whose copy failed must not be deleted.
		return plan, errors.New("some copies failed; nothing was deleted from the source")
	}
	for _, d := range plan.DeleteDirs {
		if err := forceRemoveAll(d); err != nil {
			plan.Errors = append(plan.Errors, err.Error())
		}
	}
	for _, f := range plan.Deletes {
		if err := forceRemove(f); err != nil && !os.IsNotExist(err) {
			plan.Errors = append(plan.Errors, err.Error())
		}
	}
	if op.Action == "move" {
		removeEmptyDirs(op.Src)
	}
	return plan, nil
}

// removeEmptyDirs removes the folders a move emptied, deepest first.
func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && !isLinkEntry(d) {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		_ = os.Remove(d) // fails, as it should, on a folder that is not empty
	}
}

// copyOneFile copies a file, keeps its modification time, and replaces a
// read-only destination (as robocopy does).
func copyOneFile(src, dst string, fi os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if old, err := os.Lstat(dst); err == nil && old.Mode().Perm()&0o200 == 0 {
		_ = os.Chmod(dst, old.Mode().Perm()|0o200)
	}
	tmp := dst + ".gorilla-partial"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm()|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("copy %s: %w", src, err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	if fi.Mode().Perm()&0o200 == 0 {
		_ = os.Chmod(dst, fi.Mode().Perm())
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

// moveOneFile renames, or copies and deletes across volumes.
func moveOneFile(src, dst string, fi os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if old, err := os.Lstat(dst); err == nil && old.Mode().Perm()&0o200 == 0 {
		_ = os.Chmod(dst, old.Mode().Perm()|0o200)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyOneFile(src, dst, fi); err != nil {
		return err
	}
	return forceRemove(src)
}

// forceRemove deletes one file, clearing read-only first (Remove-Item -Force).
func forceRemove(p string) error {
	if fi, err := os.Lstat(p); err == nil && fi.Mode().Perm()&0o200 == 0 {
		_ = os.Chmod(p, fi.Mode().Perm()|0o200)
	}
	return os.Remove(p)
}

// forceRemoveAll deletes a tree, clearing read-only on the way, never
// following a link out of it.
func forceRemoveAll(root string) error {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !isLinkEntry(d) {
			if fi, err := d.Info(); err == nil && fi.Mode().Perm()&0o200 == 0 {
				_ = os.Chmod(path, fi.Mode().Perm()|0o200)
			}
		}
		return nil
	})
	return os.RemoveAll(root)
}
