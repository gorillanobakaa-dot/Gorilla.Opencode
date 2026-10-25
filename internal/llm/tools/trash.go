package tools

// GORILLA OVERRIDE (2026-10-25): a plain delete is a PERMANENT delete, and a
// request for the Recycle Bin is not consent to one.
//
// Measured on the 2026-10-25 pressure bench (bench-2026-10-25, Gemma 4 12B,
// auto mode): asked "MOVE ALL THESE FILES ... to trash", the model first tried
// the Fieldkit recycle tool. When that failed, a later step ran
//
//	Remove-Item *.png -Force -Recurse
//
// through this tool. That skips the Recycle Bin: the files are gone. The
// permission system saw an ordinary command and auto-approved it; nothing in
// the prompt said the files would not be recoverable. Qwen3.6 and Gemma 26B on
// the same request used the real Recycle Bin (the Shell COM object and
// [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile(..., 'SendToRecycleBin')).
//
// The rules, each with a test in trash_test.go:
//
//  1. A plain delete (rm, Remove-Item and its aliases, del, erase, rd, rmdir,
//     unlink, shred, find -delete, the .NET and Python delete calls) is classed
//     as permanent. The prompt says so in plain words, in the dialog and over
//     ACP: "deletes for good, does not go to the Recycle Bin".
//  2. When the PERSON's own words in this conversation asked for the trash or
//     the Recycle Bin, and nobody is answering prompts (auto mode, /yolo, a -p
//     run), the command is refused and the tool result names the safe way.
//  3. Not touched: a delete the person asked for in so many words
//     ("permanently", "for good", "skip the recycle bin"), any delete in a
//     conversation that never mentioned the trash, a delete of temp or build
//     output, and the huge-delete refusal (dangerous_delete.go), which still
//     comes first.
//
// Only the person's text sets or clears the record. A helper's prompt is
// written by the model, so a helper inherits its parent's record and its own
// prompt changes nothing: otherwise the model could talk itself out of rule 2
// by writing "permanently" into a task.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
)

// PermanentDeleteWhy is the sentence the prompt carries for a plain delete.
const PermanentDeleteWhy = "deletes for good, does not go to the Recycle Bin"

// permanentDeleteVerbs are programs whose whole job is a permanent delete.
// deleteVerbs (dangerous_delete.go) plus the ones that never recurse.
var permanentDeleteVerbs = map[string]bool{
	"rm": true, "remove-item": true, "ri": true, "rd": true, "rmdir": true, "del": true, "erase": true,
	"unlink": true, "shred": true, "srm": true,
}

// deleteWrappers run the command that follows them: `sudo rm`, `xargs rm`,
// `cmd /c del`. They are skipped to find the real program.
var deleteWrappers = map[string]bool{
	"sudo": true, "doas": true, "xargs": true, "command": true, "nohup": true, "time": true,
	"then": true, "do": true, "else": true, "&": true, "cmd": true, "/c": true, "/k": true,
	"foreach-object": true, "%": true, "where-object": true, "?": true,
}

// permanentDeleteCalls are delete calls written as code rather than as a
// program: .NET on PowerShell, Python's os/shutil in a -c string.
var permanentDeleteCalls = regexp.MustCompile(`(?i)(\]::delete\s*\(|\)\.delete\s*\(|\$\w+\.delete\s*\(|\bos\.(remove|unlink|rmdir|removedirs)\s*\(|\bshutil\.rmtree\s*\(|\.unlink\s*\(|\.rmtree\s*\()`)

// PermanentDeleteIn returns the delete program or call found anywhere in the
// command line, or "". Over-matching is cheap (a sentence in a prompt that
// asks anyway); a miss is the 2026-10-25 case. So quotes, brackets and braces
// are treated as command boundaries too: `powershell -Command "Remove-Item x"`
// and `ForEach-Object { Remove-Item $_ }` are both found.
func PermanentDeleteIn(cmd string) string {
	if m := permanentDeleteCalls.FindString(cmd); m != "" {
		return strings.TrimSpace(m)
	}
	// find's own delete actions, wherever they sit on its line. Judged before
	// the quotes are cut, which would part `find . -name '*.png' -delete`.
	for _, seg := range splitShellCommands(cmd) {
		words := stripDeleteWrappers(shellWords(seg))
		if len(words) == 0 || programWord(words[0]) != "find" {
			continue
		}
		for i, w := range words {
			low := strings.ToLower(w)
			if low == "-delete" {
				return "find -delete"
			}
			if (low == "-exec" || low == "-execdir") && i+1 < len(words) && permanentDeleteVerbs[programWord(words[i+1])] {
				return programWord(words[i+1])
			}
		}
	}
	flat := strings.NewReplacer(`"`, ";", `'`, ";", "{", ";", "}", ";", "(", ";", ")", ";").Replace(cmd)
	for _, seg := range splitShellCommands(flat) {
		words := stripDeleteWrappers(strings.Fields(seg))
		if len(words) > 0 && permanentDeleteVerbs[programWord(words[0])] {
			return words[0]
		}
	}
	return ""
}

// stripDeleteWrappers drops the words that run the command after them, and
// leading FOO=bar assignments.
func stripDeleteWrappers(words []string) []string {
	for len(words) > 0 && (deleteWrappers[strings.ToLower(words[0])] ||
		(strings.Contains(words[0], "=") && !strings.HasPrefix(words[0], "-"))) {
		words = words[1:]
	}
	return words
}

// programWord is a command word without its directory and .exe, lowercased.
func programWord(w string) string {
	w = strings.ReplaceAll(strings.Trim(w, `"'`), `\`, "/")
	if i := strings.LastIndex(w, "/"); i >= 0 {
		w = w[i+1:]
	}
	return strings.TrimSuffix(strings.ToLower(w), ".exe")
}

// scratchTargetNames are what a temp or build clean removes. Losing one costs
// a rebuild, never a person's file.
var scratchTargetNames = map[string]bool{
	"build": true, "dist": true, "out": true, "target": true, "obj": true, "bin": true,
	"node_modules": true, "__pycache__": true, ".pytest_cache": true, ".mypy_cache": true,
	".ruff_cache": true, ".cache": true, ".gradle": true, ".next": true, ".turbo": true,
	"coverage": true, ".coverage": true, "tmp": true, "temp": true, ".tox": true,
}

// scratchTargetSuffixes are file kinds produced by a build or a tool run.
var scratchTargetSuffixes = []string{".tmp", ".o", ".obj", ".pyc", ".log", "~"}

// tempRoots are the spellings of the temp folder.
var tempRoots = []string{"$env:temp", "${env:temp}", "%temp%", "$env:tmp", "%tmp%", "/tmp", "$tmpdir", "${tmpdir}", "/var/tmp"}

// OnlyScratchTargets reports whether every delete on the line is aimed at temp
// or build output, so rule 2 does not stand in the way of a clean. A line with
// no target it can read is NOT scratch: unknown means not excused.
func OnlyScratchTargets(cmd string) bool {
	found := false
	for _, seg := range splitShellCommands(cmd) {
		words := shellWords(seg)
		for len(words) > 0 && deleteWrappers[strings.ToLower(words[0])] {
			words = words[1:]
		}
		if len(words) == 0 || !permanentDeleteVerbs[programWord(words[0])] {
			continue
		}
		for _, w := range words[1:] {
			low := strings.ToLower(w)
			if strings.HasPrefix(low, "-") || (len(low) == 2 && low[0] == '/') {
				continue // a flag or a cmd.exe switch
			}
			if !isScratchTarget(low) {
				return false
			}
			found = true
		}
	}
	return found
}

// machineTempDir is this machine's temp folder as isScratchTarget compares
// paths: lowercase, forward slashes, no trailing slash. A variable so a test
// can play another OS.
var machineTempDir = func() string {
	return strings.TrimRight(strings.ToLower(filepath.ToSlash(os.TempDir())), "/")
}

func isScratchTarget(p string) bool {
	p = strings.ReplaceAll(p, `\`, "/")
	// GORILLA FIX (2026-10-25, PR #23 CI): the temp folder written out in
	// full counts too, so the rule is the same on every OS. Only /tmp was
	// listed, so `rm /tmp/x/a.png` was a clean on Linux while the same
	// folder on Windows (C:\Users\me\AppData\Local\Temp\x\a.png) was not.
	roots := append([]string{machineTempDir()}, tempRoots...)
	for _, r := range roots {
		if r != "" && (p == r || strings.HasPrefix(p, r+"/")) {
			return true
		}
	}
	if strings.Contains(p, "*") && !strings.HasSuffix(p, "/*") {
		// A glob picks files by name: only a build-output kind is excused.
		for _, s := range scratchTargetSuffixes {
			if strings.HasSuffix(p, "*"+s) {
				return true
			}
		}
		return false
	}
	p = strings.TrimSuffix(strings.TrimSuffix(p, "/*"), "/")
	for _, s := range scratchTargetSuffixes {
		if strings.HasSuffix(p, s) && !strings.HasSuffix(p, "/"+s) {
			return true
		}
	}
	// A build folder by name only inside the project: a rooted or home path
	// named "out" or "temp" may be a person's own folder.
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || strings.HasPrefix(p, "$") ||
		strings.HasPrefix(p, "%") || (len(p) > 1 && p[1] == ':') {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, "./"), "/")
	return scratchTargetNames[parts[len(parts)-1]]
}

// trashWords mark a request for the trash or the Recycle Bin, in the
// languages the owner's users write in. Lowercase; matched as substrings.
var trashWords = []string{
	"trash", "recycle bin", "recycling bin", "recycle", "wastebasket", "deleted items",
	"the bin", "to bin", "in bin",
	"corbeille",             // French
	"papelera",              // Spanish
	"papierkorb",            // German
	"lixeira", "reciclagem", // Portuguese
	"cestino",                      // Italian
	"coș de gunoi", "cos de gunoi", // Romanian
	"coșul de gunoi", "cosul de gunoi", "gunoi",
}

// permanentWords mark the person asking for a delete that does not go to the
// bin. They win over trashWords in the same message: "delete them for good,
// not to the trash" is a permanent delete the person asked for.
var permanentWords = []string{
	"permanently", "permanent delete", "for good", "irrecoverabl", "unrecoverabl",
	"skip the recycle", "skip the trash", "bypass the recycle", "bypass the trash",
	"not to the trash", "not to the recycle", "not in the trash", "not in the recycle",
	"don't recycle", "do not recycle", "dont recycle", "without the recycle", "without recycl",
	"empty the trash", "empty the recycle", "empty trash", "empty recycle",
	"delete permanent", "définitivement", "endgültig", "eliminar definitivamente", "apagar definitivamente",
	"borrar definitivamente", "eliminare definitivamente",
	"șterge definitiv", "sterge definitiv", "ștergere definitivă", "stergere definitiva",
}

// deleteWords mark the person asking for a delete. In a message that names
// no trash they are the latest explicit order and clear the record (owner,
// 2026-10-25: "we do want to be able to delete files"): "to trash", then
// later "ok, just delete them", deletes.
var deleteWords = regexp.MustCompile(`(?i)\b(delete|remove|erase|wipe|get rid of|rm|del|supprime[rz]?|efface[rz]?|borra[r]?|elimina[r]?|lösche[n]?|entferne[n]?|apaga[r]?|cancella(re)?)\b|[șş]terg|\bsterg`)

// deleteCommand is a delete COMMAND the person typed ("use rm -rf *.png",
// "Remove-Item *.png"). Owner, 2026-10-25: after "to trash" they insisted on
// `rm -rf *.png`, and that is the latest explicit order even when the same
// message also says "trash" ("rm -rf *.png instead of the trash").
var deleteCommand = regexp.MustCompile(`(?i)(^|[\s` + "`" + `"'(;|&])(rm\s+(-[a-z]+\s+)*\S|remove-item\s+\S|rd\s+/s|rmdir\s+/s|del\s+/[qsf]|del\s+\S+\.\w+|erase\s+\S)`)

// negations, among the three words before a permanent phrase, turn it around:
// "to the trash, do not delete them permanently" still means the trash, while
// "no wait, delete them permanently" does not.
var negations = map[string]bool{
	"don't": true, "dont": true, "do": false, "not": true, "never": true, "nu": true, "ne": true,
	"pas": true, "nicht": true, "no": true, "without": true, "nicio": true,
}

// mentionsPermanent reports a permanent phrase that is not negated in the few
// words before it. A wrong yes here is the unsafe direction (it clears the
// record), so any negation word close before the phrase wins.
func mentionsPermanent(low string) bool {
	for _, w := range permanentWords {
		for from := 0; ; {
			i := strings.Index(low[from:], w)
			if i < 0 {
				break
			}
			i += from
			if !negatedAt(low, i, 3, false) {
				return true
			}
			from = i + len(w)
		}
	}
	return false
}

// deleteOrdered reports a delete word that is not negated just before it.
// "no" is not a negation here: "no, just delete them" is an order. A wrong
// yes clears the record, so the window is the two words before.
func deleteOrdered(low string) bool {
	for _, m := range deleteWords.FindAllStringIndex(low, -1) {
		if !negatedAt(low, m[0], 2, true) {
			return true
		}
	}
	return false
}

// typedDelete reports a delete command the person typed, not negated just
// before it ("don't rm them" is no order).
func typedDelete(low string) bool {
	for _, m := range deleteCommand.FindAllStringIndex(low, -1) {
		if !negatedAt(low, m[0]+1, 2, true) {
			return true
		}
	}
	return false
}

// negatedAt reports a negation among the `window` words before byte i.
func negatedAt(low string, i, window int, ignoreNo bool) bool {
	before := strings.Fields(strings.NewReplacer(",", " ", ".", " ", ";", " ", "’", "'").Replace(low[:i]))
	for _, n := range before[max(0, len(before)-window):] {
		if negations[n] && !(ignoreNo && n == "no") {
			return true
		}
	}
	return false
}

// trashIntent is the per-session record. parent links a helper to the session
// that spawned it; a helper's own entry is never read.
type trashIntent struct {
	mu     sync.Mutex
	asked  map[string]bool
	parent map[string]string
}

var trashRecord = &trashIntent{asked: map[string]bool{}, parent: map[string]string{}}

// NoteUserRequest records what the PERSON's latest message says about the
// trash; the latest explicit order wins. A permanent phrase clears the record;
// a trash word sets it; a plain "delete" with no trash word clears it. A
// message naming none of them leaves the record as it was: "now the jpgs too"
// still means the trash. Call it with the person's
// text only, never with a prompt the model wrote.
func NoteUserRequest(sessionID, text string) {
	if sessionID == "" {
		return
	}
	low := strings.ToLower(text)
	trash, permanent := false, mentionsPermanent(low)
	for _, w := range trashWords {
		if strings.Contains(low, w) {
			trash = true
			break
		}
	}
	trashRecord.mu.Lock()
	defer trashRecord.mu.Unlock()
	switch {
	case permanent, typedDelete(low):
		delete(trashRecord.asked, sessionID)
	case trash:
		trashRecord.asked[sessionID] = true
	case deleteOrdered(low):
		delete(trashRecord.asked, sessionID)
	}
}

// LinkHelperSession makes a helper session answer TrashAsked with its
// parent's record. TrashAsked never reads a linked session's own entry, so
// whatever NoteUserRequest writes for a helper changes nothing.
func LinkHelperSession(child, parent string) {
	if child == "" || parent == "" || child == parent {
		return
	}
	trashRecord.mu.Lock()
	trashRecord.parent[child] = parent
	trashRecord.mu.Unlock()
}

// TrashAsked reports whether the person asked, in this conversation, for the
// trash or the Recycle Bin and has not since asked for a permanent delete.
func TrashAsked(sessionID string) bool {
	trashRecord.mu.Lock()
	defer trashRecord.mu.Unlock()
	for i := 0; i < 32 && sessionID != ""; i++ {
		p, ok := trashRecord.parent[sessionID]
		if !ok {
			break
		}
		sessionID = p
	}
	return trashRecord.asked[sessionID]
}

// lookPathFieldkit finds the Fieldkit command (the gorilla-size-recycle card
// runs as `fieldkit size recycle`). Looked up, never started.
var lookPathFieldkit = func() (string, error) { return exec.LookPath("fieldkit") }

// RecycleAdvice is the safe way to do what the person asked, for this
// machine: what the refusal names and what the tool guidance teaches.
func RecycleAdvice() string {
	fieldkit := ""
	if _, err := lookPathFieldkit(); err == nil {
		fieldkit = "`fieldkit size recycle <path> [<path> ...] --json` (the Fieldkit gorilla-size-recycle tool: verified in the bin, undo with `fieldkit size undo`); give it real paths, expand a wildcard first. If it fails, use "
	}
	switch {
	case runtime.GOOS == "windows" || shell.IsWindowsPowerShell():
		return fieldkit + "PowerShell: `Add-Type -AssemblyName Microsoft.VisualBasic; Get-ChildItem *.png | ForEach-Object { [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($_.FullName, 'OnlyErrorDialogs', 'SendToRecycleBin') }` " +
			"(a folder: `[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory($path, 'OnlyErrorDialogs', 'SendToRecycleBin')`). Pass full paths."
	case runtime.GOOS == "darwin":
		return fieldkit + "`trash <path>` (macOS 15 and later), or `osascript -e 'tell application \"Finder\" to delete POSIX file \"/full/path\"'`."
	default:
		return fieldkit + "`gio trash <path>` or `trash-put <path>` (whichever `command -v` finds). If neither is installed, say so and ask; do not delete."
	}
}

// trashRefusal is the tool result for a permanent delete refused under rule 2.
func trashRefusal(found string) string {
	return "NOT RUN: `" + found + "` " + PermanentDeleteWhy + ", and the user asked for the trash / Recycle Bin. " +
		"Nobody is answering prompts, so a permanent delete is refused. Nothing was deleted. " +
		"Move the files to the Recycle Bin instead: " + RecycleAdvice() +
		" If the user wants them gone for good, they must say so (\"delete permanently\")."
}
