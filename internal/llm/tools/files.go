package tools

// GORILLA (2026-10-27, job 05 of goal windows-capabilities): one tool for the
// file operations a person asks for in plain words and a small model could not
// do: copy, mirror, purge and move trees (robocopy on Windows, up to 128
// threads; a Go fallback elsewhere), attributes, bulk rename, details, a
// listing that shows hidden items, and delete to the Recycle Bin.
//
// Owner, 2026-10-26: the model "was not aware it can use robocopy with up to
// 128 threads ... no idea how to view and check attributes". The rules, each
// with a test in files_test.go:
//
//  1. Every action that changes files works out what it will do FIRST (for
//     robocopy a real /L dry run), shows it, asks, and only then runs.
//     dry_run=true stops after the preview.
//  2. mirror, purge, move and move_files delete files for good. The preview
//     lists what will be DELETED (counts + the first paths, full=true for
//     all) and the prompt is marked irreversible, so auto-approve does not
//     wave it through (owner decision 2026-10-26: the real switch, after the
//     preview).
//  3. Refused before asking: a destination that is a drive root or the user
//     profile root for mirror/purge (a source for move), and either folder
//     inside the other.
//  4. Junctions and symbolic links are never followed (/XJ; the Go walk does
//     not descend into them).
//  5. Read-only actions (info, list, attributes without set/clear) never ask.
//  6. delete goes to the Recycle Bin; permanent=true asks as irreversible and
//     is refused when the person asked for the trash (trash.go, rule 2).
//  7. archive writes outside the folder it packs (files_archive.go).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/howto"
	"github.com/opencode-ai/opencode/internal/pathutil"
	"github.com/opencode-ai/opencode/internal/permission"
)

const FilesToolName = "files"

// maxFilesThreads is robocopy's /MT limit.
const maxFilesThreads = 128

// filesPreviewPaths is how many paths per list a preview shows without full.
const filesPreviewPaths = 20

type filesParams struct {
	Action      string `json:"action"`
	Path        string `json:"path"`
	Dest        string `json:"dest"`
	Pattern     string `json:"pattern"`
	Threads     int    `json:"threads"`
	Restartable bool   `json:"restartable"`
	Recurse     bool   `json:"recurse"`
	Set         string `json:"set"`
	Clear       string `json:"clear"`
	Find        string `json:"find"`
	Replace     string `json:"replace"`
	Template    string `json:"template"`
	Start       int    `json:"start"`
	Case        string `json:"case"`
	Kind        string `json:"kind"`
	Hidden      bool   `json:"hidden"`
	Hash        bool   `json:"hash"`
	Details     bool   `json:"details"`
	Format      string `json:"format"`
	Level       *int   `json:"level"`
	Password    string `json:"password"`
	Permanent   bool   `json:"permanent"`
	DryRun      bool   `json:"dry_run"`
	Full        bool   `json:"full"`
}

type filesTool struct {
	permissions permission.Service
}

// NewFilesTool builds the files tool. Changes ask first; reads never do.
func NewFilesTool(permissions permission.Service) BaseTool {
	return filesTool{permissions: permissions}
}

const filesDescription = "Copy, mirror, move, rename, delete, archive files; attributes; list incl. hidden. " +
	"info/details cover owner, created, permissions (ACL), streams and Mark of the Web; " +
	"list details=true gives them for every entry in one call. " +
	"archive: zip, 7z or tar.xz, written OUTSIDE the folder; tell the person the defaults used and the other options. " +
	"Windows: robocopy, up to 128 threads. Changes preview first (copied AND deleted) and ask; " +
	"mirror/purge/move delete for good; delete uses the Recycle Bin unless permanent."

// Summary is the files tool's line in the system prompt's tool table.
func (filesTool) Summary() string {
	if runtime.GOOS == "windows" {
		return "Copy/mirror/move via robocopy (1-128 threads), rename, attrs; howto"
	}
	return "Copy/mirror/move, bulk rename, attributes, details, list, delete to the trash"
}

func (filesTool) Info() ToolInfo {
	s := func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	b := func(d string) map[string]any { return map[string]any{"type": "boolean", "description": d} }
	return ToolInfo{
		Name:        FilesToolName,
		Description: filesDescription,
		Parameters: map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"copy", "mirror", "purge", "move", "move_files", "attributes", "rename", "info", "list", "delete", "archive", "howto"},
				"description": "mirror=/MIR, purge=delete from dest what path lacks, move=/MOVE (folders go too), move_files=/MOV (folders stay), archive=pack path into dest, howto=read card path",
			},
			"path":        s("Source (left as it is by purge), or the item"),
			"dest":        s("Destination; rename: new name of one file"),
			"pattern":     s("Name filter, e.g. *.tmp"),
			"threads":     map[string]any{"type": "integer", "description": "1-128"},
			"restartable": b("robocopy /Z"),
			"recurse":     b("Include subfolders"),
			"set":         s("e.g. ReadOnly,Hidden,System,Archive,NotIndexed or Normal"),
			"clear":       s("attributes to clear"),
			"find":        s("rename: regex in names"),
			"replace":     s("rename: replacement ($1 ok)"),
			"template":    s("rename: {n:D2}_{name}, {stem}, {ext}"),
			"start":       map[string]any{"type": "integer", "description": "rename: first {n}"},
			"case":        s("rename: lower, upper or title"),
			"kind":        s("list: file or dir"),
			"hidden":      b("list: enter hidden/system folders"),
			"hash":        b("info: SHA-256"),
			"details":     b("list: owner, created, ACL, streams, Mark of the Web per entry"),
			"format":      s("archive: zip (default), 7z or tar.xz"),
			"level":       map[string]any{"type": "integer", "description": "archive: 0 store .. 9 smallest; default 6"},
			"password":    s("archive: encrypt (zip, AES-256)"),
			"permanent":   b("delete: skip the Recycle Bin"),
			"dry_run":     b("Preview only"),
			"full":        b(fullDescription),
		},
		Required: []string{"action", "path"},
	}
}

func (t filesTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p filesParams
	if err := UnmarshalToolInput(call.Input, &p); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	if strings.EqualFold(strings.TrimSpace(p.Action), "howto") {
		return filesHowto(p.Path), nil
	}
	if strings.TrimSpace(p.Path) == "" {
		return NewTextErrorResponse("path is required"), nil
	}
	p.Path = filesAbs(p.Path)
	if why := RefuseSensitiveRead(p.Path); why != "" {
		return NewTextErrorResponse(why), nil
	}
	action := strings.ToLower(strings.TrimSpace(p.Action))
	switch action {
	case "copy", "mirror", "purge", "move", "move_files":
		return t.sync(ctx, action, p)
	case "attributes":
		return t.attributes(ctx, p)
	case "rename":
		return t.rename(ctx, p)
	case "info":
		return filesInfo(p)
	case "list":
		return filesList(p)
	case "delete":
		return t.delete(ctx, p)
	case "archive":
		return t.archive(ctx, p)
	}
	return NewTextErrorResponse(fmt.Sprintf(
		"unknown action %q: use copy, mirror, purge, move, move_files, attributes, rename, info, list, delete, archive or howto", p.Action)), nil
}

// filesHowto returns one how-to card (internal/howto), or the list of the
// cards that apply on this machine when the name is empty or unknown.
func filesHowto(name string) ToolResponse {
	if card, ok := howto.Get(name); ok {
		return NewTextResponse(card)
	}
	avail := howto.Available()
	if len(avail) == 0 {
		return NewTextErrorResponse("no how-to cards apply on this machine")
	}
	msg := "cards here: " + strings.Join(avail, ", ")
	if strings.TrimSpace(name) != "" {
		return NewTextErrorResponse(fmt.Sprintf("no card %q here; %s", name, msg))
	}
	return NewTextResponse(msg)
}

// filesAbs makes a path absolute against the working directory.
func filesAbs(p string) string {
	p = strings.TrimSpace(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(config.WorkingDirectory(), p)
	}
	return filepath.Clean(p)
}

// filesThreads is the /MT value: 1..128, default the logical CPU count.
func filesThreads(asked int) int {
	n := asked
	if n <= 0 {
		n = runtime.NumCPU()
	}
	if n > maxFilesThreads {
		n = maxFilesThreads
	}
	if n < 1 {
		n = 1
	}
	return n
}

// isVolumeRoot reports a drive root (C:\, \\server\share\) or /.
func isVolumeRoot(p string) bool {
	c := pathutil.Canonical(p)
	return filepath.Dir(c) == c
}

// isProfileRoot reports the user profile folder itself.
func isProfileRoot(p string) bool {
	home, err := os.UserHomeDir()
	return err == nil && home != "" && pathutil.Same(p, home)
}

// refuseRoot names why a folder that a delete-capable action empties is
// refused, or "".
func refuseRoot(role, p string) string {
	switch {
	case isVolumeRoot(p):
		return fmt.Sprintf("refused: the %s %s is a drive root; nothing was changed. Name a folder.", role, p)
	case isProfileRoot(p):
		return fmt.Sprintf("refused: the %s %s is the user profile folder itself; nothing was changed. Name a folder inside it.", role, p)
	}
	return ""
}

// refuseSync applies rule 3 to a tree action, before anything is asked.
func refuseSync(action, src, dst string) string {
	if pathutil.Within(src, dst) || pathutil.Within(dst, src) {
		return fmt.Sprintf("refused: %s and %s are the same folder or one is inside the other; nothing was changed.", src, dst)
	}
	switch action {
	case "mirror", "purge":
		return refuseRoot("destination", dst)
	case "move", "move_files":
		return refuseRoot("source", src)
	}
	return ""
}

// syncOp is one tree action.
type syncOp struct {
	Ctx         context.Context // cancels a running robocopy
	Action      string
	Src, Dst    string
	Pattern     string
	Threads     int
	Restartable bool
}

// syncPlan is what a tree action did or would do.
type syncPlan struct {
	Copies     []string // source files copied (or to be)
	Overwrites int      // of those, how many replace a file in dest
	NewDirs    []string
	Deletes    []string // files deleted for good
	DeleteDirs []string // folders deleted for good
	Bytes      int64
	Errors     []string
	ExitCode   int    // robocopy's, -1 for the Go engine
	Engine     string // "robocopy" or "go"
	Log        string // where the full log is, if kept
}

func (s syncPlan) deletes() int { return len(s.Deletes) + len(s.DeleteDirs) }

func (t filesTool) sync(ctx context.Context, action string, p filesParams) (ToolResponse, error) {
	if strings.TrimSpace(p.Dest) == "" {
		return NewTextErrorResponse(action + " needs dest"), nil
	}
	src, dst := p.Path, filesAbs(p.Dest)
	fi, err := os.Stat(src)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("source: %v", err)), nil
	}
	if !fi.IsDir() {
		return t.syncFile(ctx, action, src, dst, fi, p)
	}
	if why := refuseSync(action, src, dst); why != "" {
		return NewTextErrorResponse(why), nil
	}
	op := syncOp{Ctx: ctx, Action: action, Src: src, Dst: dst, Pattern: p.Pattern, Threads: filesThreads(p.Threads), Restartable: p.Restartable}
	plan, err := previewSync(op)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("dry run failed, nothing was changed: %v", err)), nil
	}
	preview := describePlan(op, plan, true, p.Full)
	// Measured: robocopy /L to a drive that does not exist exits 1 ("files
	// would be copied") and logs ERROR 3 on the destination. The exit code
	// alone would wave a broken plan through, so any error in the dry run
	// stops here.
	if len(plan.Errors) > 0 {
		return NewTextErrorResponse("the dry run reported errors; nothing was changed and nothing was asked.\n" + preview), nil
	}
	if p.DryRun {
		return NewTextResponse(preview), nil
	}
	if len(plan.Copies) == 0 && plan.deletes() == 0 && len(plan.NewDirs) == 0 {
		return NewTextResponse(preview + "\nNothing to do."), nil
	}
	if !t.ask(ctx, action, dst, action+":"+src+"->"+dst, preview, syncIrreversible(op, plan), plan) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	done, err := runSync(op)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("%s failed: %v\n\n%s", action, err, describePlan(op, done, false, p.Full))), nil
	}
	out := describePlan(op, done, false, p.Full)
	if _, ok := robocopyExit(done.ExitCode); done.Engine == "robocopy" && !ok {
		return NewTextErrorResponse(out), nil
	}
	if len(done.Errors) > 0 {
		return NewTextErrorResponse(out), nil
	}
	return NewTextResponse(out), nil
}

// syncIrreversible is the sentence the prompt carries when a wrong yes cannot
// be taken back: anything deleted or overwritten.
func syncIrreversible(op syncOp, plan syncPlan) string {
	var parts []string
	if n := plan.deletes(); n > 0 {
		where := "in " + op.Dst
		if op.Action == "move" || op.Action == "move_files" {
			where = "from the source " + op.Src
		}
		parts = append(parts, fmt.Sprintf("deletes %d item(s) %s for good, not to the Recycle Bin", n, where))
	}
	if plan.Overwrites > 0 {
		parts = append(parts, fmt.Sprintf("overwrites %d file(s) in %s", plan.Overwrites, op.Dst))
	}
	return strings.Join(parts, "; ")
}

// ask raises the permission prompt with the preview in it.
func (t filesTool) ask(ctx context.Context, action, scope, key, preview, irreversible string, params any) bool {
	sessionID, _ := GetContextValues(ctx)
	return t.permissions.Request(permission.CreatePermissionRequest{
		SessionID:    sessionID,
		Path:         permissionScope(scope),
		ToolName:     FilesToolName,
		GrantKey:     key,
		Action:       action,
		Description:  preview,
		Params:       params,
		Irreversible: irreversible,
	})
}

// describePlan renders a preview (would) or a result (did).
func describePlan(op syncOp, s syncPlan, preview bool, full bool) string {
	var b strings.Builder
	verb := map[bool]string{true: "DRY RUN (nothing changed yet)", false: "DONE"}[preview]
	if _, ok := robocopyExit(s.ExitCode); !preview && (len(s.Errors) > 0 || (s.Engine == "robocopy" && !ok && s.ExitCode != 0)) {
		verb = "FINISHED WITH ERRORS"
	}
	fmt.Fprintf(&b, "%s: %s %s -> %s", verb, op.Action, op.Src, op.Dst)
	if op.Pattern != "" {
		fmt.Fprintf(&b, " [%s]", op.Pattern)
	}
	fmt.Fprintf(&b, " (engine %s", s.Engine)
	if s.Engine == "robocopy" && !preview {
		fmt.Fprintf(&b, ", %d threads", op.Threads)
	}
	b.WriteString(")\n")
	will := map[bool]string{true: "will be", false: "were"}[preview]
	fmt.Fprintf(&b, "%d file(s) %s copied (%s", len(s.Copies), will, filesHumanBytes(s.Bytes))
	if s.Overwrites > 0 {
		fmt.Fprintf(&b, ", %d replace an existing file", s.Overwrites)
	}
	b.WriteString(")\n")
	writeList(&b, "  ", s.Copies, full)
	if len(s.NewDirs) > 0 {
		fmt.Fprintf(&b, "%d new folder(s)\n", len(s.NewDirs))
	}
	if n := s.deletes(); n > 0 || op.Action != "copy" {
		where := op.Dst
		if op.Action == "move" || op.Action == "move_files" {
			where = op.Src + " (the source, after copying)"
		}
		fmt.Fprintf(&b, "DELETED for good from %s: %d file(s), %d folder(s)", where, len(s.Deletes), len(s.DeleteDirs))
		if preview && n > 0 {
			b.WriteString(" - NOT to the Recycle Bin")
		}
		b.WriteString("\n")
		writeList(&b, "  - ", append(append([]string(nil), s.DeleteDirs...), s.Deletes...), full)
	}
	if !preview && s.Engine == "robocopy" {
		meaning, _ := robocopyExit(s.ExitCode)
		if s.ExitCode == 0 {
			meaning = "robocopy: success - nothing needed copying"
		}
		fmt.Fprintf(&b, "robocopy exit %d: %s\n", s.ExitCode, meaning)
	}
	if len(s.Errors) > 0 {
		fmt.Fprintf(&b, "ERRORS (%d):\n", len(s.Errors))
		writeList(&b, "  ! ", s.Errors, full)
	}
	if s.Log != "" {
		fmt.Fprintf(&b, "full log: %s\n", s.Log)
	}
	return b.String()
}

// writeList writes the first filesPreviewPaths entries, or all with full,
// and SAYS when it stopped.
func writeList(b *strings.Builder, prefix string, items []string, full bool) {
	writeListN(b, prefix, items, full, filesPreviewPaths)
}

// filesListLines caps a listing (list, attributes get) without full.
const filesListLines = 200

func writeListN(b *strings.Builder, prefix string, items []string, full bool, limit int) {
	n := len(items)
	if !full && n > limit {
		n = limit
	}
	for _, it := range items[:n] {
		b.WriteString(prefix + it + "\n")
	}
	if n < len(items) {
		fmt.Fprintf(b, "%s... and %d more (full=true lists all %d)\n", prefix, len(items)-n, len(items))
	}
}

func filesHumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB (%d bytes)", float64(n)/float64(div), "KMGTPE"[exp], n)
}

// syncFile handles copy/move of ONE file, like Copy-Item / Move-Item: dest
// is a folder (existing, or ending in a separator) or the new file path.
func (t filesTool) syncFile(ctx context.Context, action, src, dst string, fi os.FileInfo, p filesParams) (ToolResponse, error) {
	if action != "copy" && action != "move" && action != "move_files" {
		return NewTextErrorResponse(action + " needs a folder as path; for one file use copy or move"), nil
	}
	if d, err := os.Stat(dst); (err == nil && d.IsDir()) || strings.HasSuffix(p.Dest, "/") || strings.HasSuffix(p.Dest, `\`) {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	if pathutil.Same(src, dst) {
		return NewTextErrorResponse("refused: source and destination are the same file"), nil
	}
	verb := "Copy"
	if action != "copy" {
		verb = "Move"
	}
	preview := fmt.Sprintf("DRY RUN (nothing changed yet): %s %s (%s) -> %s", verb, src, filesHumanBytes(fi.Size()), dst)
	irreversible := ""
	if old, err := os.Stat(dst); err == nil {
		if old.IsDir() {
			return NewTextErrorResponse(fmt.Sprintf("refused: %s is a folder", dst)), nil
		}
		preview += fmt.Sprintf("\nREPLACES the existing %s (%s) for good", dst, filesHumanBytes(old.Size()))
		irreversible = "overwrites " + dst
	}
	if p.DryRun {
		return NewTextResponse(preview), nil
	}
	if !t.ask(ctx, action, dst, action+":"+src+"->"+dst, preview, irreversible, map[string]string{"source": src, "dest": dst}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	var err error
	if action == "copy" {
		err = copyOneFile(src, dst, fi)
	} else {
		err = moveOneFile(src, dst, fi)
	}
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("%s failed: %v", strings.ToLower(verb), err)), nil
	}
	return NewTextResponse(fmt.Sprintf("DONE: %s %s -> %s (%s)", strings.ToLower(verb), src, dst, filesHumanBytes(fi.Size()))), nil
}

// sortedKeys is used by the Go engine and the tests.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
