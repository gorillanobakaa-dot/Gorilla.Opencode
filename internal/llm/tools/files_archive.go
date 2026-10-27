package tools

// GORILLA (2026-10-27, v0.1.156): files action=archive. Owner, on GPT-6-Luna,
// "archive all the files in the working folder": the model hand-wrote .NET
// ZipFile in PowerShell, put the zip INSIDE the folder it was archiving and
// asked nothing. The rules, each with a test in files_archive_test.go:
//
//  1. The archive goes OUTSIDE the folder it packs: by default beside it
//     (<parent>\<name>-<date>-<time>.zip); a dest inside it is refused, and
//     so is a dest that already exists (nothing is overwritten).
//  2. The preview and the reply say the defaults that were used AND the
//     options there are, so the person can ask for another format, level,
//     password or place. Writing asks first, like every other change.
//  3. zip without a password is written here (archive/zip, Deflate). zip with
//     a password (AES-256), 7z and tar.xz go to bsdtar: Windows' own
//     System32\tar.exe (libarchive with Windows CNG crypto; measured on
//     Windows 11 26200: bsdtar 3.8.9, liblzma 5.8.3), never a tar from PATH
//     there. Nothing is downloaded and no new module is linked in.
//  4. Links are stored as links, never followed (the Go walk does not enter
//     them; bsdtar does not follow symlinks without -L).
//
// The password reaches bsdtar on its command line (--passphrase): another
// process of the same user can read it while the archive is being written.
// bsdtar has no other non-interactive way to take it.

import (
	"archive/zip"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/pathutil"
	"github.com/opencode-ai/opencode/internal/permission"
)

// archiveDefaultLevel is what "normal" means in 7-Zip and xz.
const archiveDefaultLevel = 6

// archiveOptions is said in every archive preview and reply (rule 2).
const archiveOptions = "options: format=zip|7z|tar.xz, level=0 (store) .. 9 (smallest), " +
	"password=... (zip only, AES-256), dest=a file or folder OUTSIDE the folder being archived"

// archiveExt maps a format to its file extension.
var archiveExt = map[string]string{"zip": ".zip", "7z": ".7z", "tar.xz": ".tar.xz"}

// archiveFormat normalizes what a model or person may call a format.
func archiveFormat(s string) (string, error) {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), ".") {
	case "", "zip":
		return "zip", nil
	case "7z", "7zip", "7-zip":
		return "7z", nil
	case "tar.xz", "txz", "xz", "tarxz":
		return "tar.xz", nil
	}
	return "", fmt.Errorf("unknown archive format %q; %s", s, archiveOptions)
}

// archiveSpec is one archive action, worked out before anything is written.
type archiveSpec struct {
	Src, Out, Format string
	Level            int
	Password         string
	Defaults         []string // what was not asked for and was chosen here
	Files, Dirs      int
	Links            int
	Bytes            int64
}

// archiveDest is the archive's path: dest when given (a folder gets the
// default name inside it, a name without the extension gets it), else beside
// src. Refused inside src, or when it already exists.
func archiveDest(src, dest, format string, now time.Time) (out string, defaulted bool, err error) {
	name := filepath.Base(src) + "-" + now.Format("20060102-150405") + archiveExt[format]
	dest = strings.TrimSpace(dest)
	switch {
	case dest == "":
		out, defaulted = filepath.Join(filepath.Dir(src), name), true
	default:
		out = filesAbs(dest)
		if d, e := os.Stat(out); (e == nil && d.IsDir()) || strings.HasSuffix(dest, "/") || strings.HasSuffix(dest, `\`) {
			out = filepath.Join(out, name)
		} else if !strings.HasSuffix(strings.ToLower(out), archiveExt[format]) {
			out += archiveExt[format]
		}
	}
	if pathutil.Within(src, out) {
		return "", false, fmt.Errorf("refused: %s is inside %s, the folder being archived; the archive would pack "+
			"itself. Leave dest empty (it goes beside the folder) or name a place outside it", out, src)
	}
	if _, e := os.Lstat(out); e == nil {
		return "", false, fmt.Errorf("refused: %s already exists; nothing was overwritten. Name another dest", out)
	}
	return out, defaulted, nil
}

func planArchive(p filesParams, now time.Time) (archiveSpec, error) {
	s := archiveSpec{Src: p.Path, Password: p.Password}
	if strings.TrimSpace(p.Format) == "" {
		s.Defaults = append(s.Defaults, "format zip")
	}
	var err error
	if s.Format, err = archiveFormat(p.Format); err != nil {
		return s, err
	}
	s.Level = archiveDefaultLevel
	if p.Level != nil {
		s.Level = *p.Level
	} else {
		s.Defaults = append(s.Defaults, fmt.Sprintf("level %d (normal)", archiveDefaultLevel))
	}
	if s.Level < 0 || s.Level > 9 {
		return s, fmt.Errorf("level %d is out of range; %s", s.Level, archiveOptions)
	}
	if s.Password != "" && s.Format != "zip" {
		return s, fmt.Errorf("refused: a password works with format=zip only (AES-256); "+
			"Windows' tar.exe cannot encrypt %s. Ask with format=zip, or without a password", s.Format)
	}
	if s.Password == "" {
		s.Defaults = append(s.Defaults, "no password")
	}
	out, defaulted, err := archiveDest(s.Src, p.Dest, s.Format, now)
	if err != nil {
		return s, err
	}
	s.Out = out
	if defaulted {
		s.Defaults = append(s.Defaults, "saved beside the folder, not inside it")
	}
	err = filepath.WalkDir(s.Src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		switch {
		case isLinkEntry(d):
			s.Links++
			if d.IsDir() {
				return filepath.SkipDir
			}
		case d.IsDir():
			if path != s.Src {
				s.Dirs++
			}
		default:
			s.Files++
			if info, e := d.Info(); e == nil {
				s.Bytes += info.Size()
			}
		}
		return nil
	})
	return s, err
}

// describe is the preview and the reply (rule 2).
func (s archiveSpec) describe(head string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: archive %s (%d file(s), %d folder(s), %s) -> %s\n", head, s.Src, s.Files, s.Dirs,
		filesHumanBytes(s.Bytes), s.Out)
	pw := "no password"
	if s.Password != "" {
		pw = "password set (AES-256)"
	}
	fmt.Fprintf(&b, "format %s, level %d, %s\n", s.Format, s.Level, pw)
	if s.Links > 0 {
		fmt.Fprintf(&b, "%d link(s) stored as links, not followed\n", s.Links)
	}
	if len(s.Defaults) > 0 {
		fmt.Fprintf(&b, "defaults used (not asked for): %s\n", strings.Join(s.Defaults, ", "))
	}
	b.WriteString(archiveOptions + "\n")
	return b.String()
}

func (t filesTool) archive(ctx context.Context, p filesParams) (ToolResponse, error) {
	if _, err := os.Lstat(p.Path); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if why := refuseRoot("source", p.Path); why != "" {
		return NewTextErrorResponse(strings.Replace(why, "nothing was changed", "nothing was written", 1)), nil
	}
	s, err := planArchive(p, time.Now())
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	preview := s.describe("DRY RUN (nothing written yet)")
	if p.DryRun {
		return NewTextResponse(preview), nil
	}
	params := map[string]any{"source": s.Src, "archive": s.Out, "format": s.Format, "level": s.Level,
		"password": s.Password != ""}
	if !t.ask(ctx, "archive", s.Out, "archive:"+s.Src+"->"+s.Out, preview, "", params) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	if err := writeArchive(ctx, s); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("archive failed, nothing kept: %v\n\n%s", err, preview)), nil
	}
	fi, err := os.Stat(s.Out)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("archive reported success but %s is missing: %v", s.Out, err)), nil
	}
	return NewTextResponse(s.describe("DONE") + fmt.Sprintf("written: %s (%s)\n", s.Out, filesHumanBytes(fi.Size()))), nil
}

// writeArchive writes to <out>.partial and renames it when complete, so a
// failed run leaves no half archive that looks whole.
func writeArchive(ctx context.Context, s archiveSpec) error {
	tmp := s.Out + ".partial"
	os.Remove(tmp)
	var err error
	if s.Format == "zip" && s.Password == "" {
		err = writeZip(ctx, s, tmp)
	} else {
		err = runBsdtar(ctx, s, tmp)
	}
	if err == nil {
		err = os.Rename(tmp, s.Out)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// writeZip is zip without a password, written here: entries are named from
// the folder's own name down (folder/sub/file), as Explorer's "Send to
// compressed folder" does.
func writeZip(ctx context.Context, s archiveSpec, out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	level := s.Level
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) { return flate.NewWriter(w, level) })
	base := filepath.Dir(s.Src)
	walkErr := filepath.WalkDir(s.Src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if isLinkEntry(d) {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			h, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			h.Name, h.Method = name, zip.Store
			h.SetMode(fs.ModeSymlink | 0o777) // a junction too: stored as a link to its target
			w, err := zw.CreateHeader(h)
			if err == nil {
				_, err = io.WriteString(w, target)
			}
			if err == nil && d.IsDir() {
				return filepath.SkipDir
			}
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = name
		if d.IsDir() {
			h.Name += "/"
			_, err = zw.CreateHeader(h)
			return err
		}
		h.Method = zip.Deflate
		if level == 0 {
			h.Method = zip.Store
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, in)
		in.Close()
		return err
	})
	cerr := zw.Close()
	ferr := f.Close()
	return errors.Join(walkErr, cerr, ferr)
}

// bsdtarArgs is the command line for zip with a password, 7z and tar.xz.
// gnu is a GNU tar (tar.xz only, outside Windows, when no bsdtar exists): it
// has no --options, so the level goes through XZ_OPT.
func bsdtarArgs(s archiveSpec, out string, gnu bool) (args []string, env []string) {
	parent, name := filepath.Dir(s.Src), filepath.Base(s.Src)
	switch s.Format {
	case "tar.xz":
		args = []string{"-c", "-J", "-f", out}
		if gnu {
			env = []string{fmt.Sprintf("XZ_OPT=-%d", s.Level)}
		} else {
			args = append(args, "--options", fmt.Sprintf("xz:compression-level=%d", s.Level))
		}
	case "7z":
		args = []string{"-c", "--format", "7zip", "-f", out, "--options", fmt.Sprintf("7zip:compression-level=%d", s.Level)}
	default:
		opts := fmt.Sprintf("zip:compression-level=%d", s.Level)
		if s.Password != "" {
			opts += ",zip:encryption=aes256"
		}
		args = []string{"-c", "--format", "zip", "-f", out, "--options", opts}
		if s.Password != "" {
			args = append(args, "--passphrase", s.Password)
		}
	}
	return append(args, "-C", parent, "--", name), env
}

func runBsdtar(ctx context.Context, s archiveSpec, out string) error {
	tar, gnu, err := archiverPath(s.Format)
	if err != nil {
		return err
	}
	args, env := bsdtarArgs(s, out, gnu)
	cmd := exec.CommandContext(ctx, tar, args...)
	cmd.Env = append(os.Environ(), env...)
	hideConsole(cmd)
	if b, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(b))
		if s.Password != "" {
			msg = strings.ReplaceAll(msg, s.Password, "***")
		}
		return fmt.Errorf("%s: %v %s", filepath.Base(tar), err, msg)
	}
	return nil
}
