package tools

// GORILLA OVERRIDE (2026-10-25): one test per rule of trash.go. The case: a
// "to trash" request, Gemma 4 12B, auto mode, `Remove-Item *.png -Force
// -Recurse` auto-approved and the files gone for good.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlainDeletesAreClassedAsPermanent(t *testing.T) {
	for _, cmd := range []string{
		`Remove-Item *.png -Force -Recurse`, // the 2026-10-25 case
		`Remove-Item -Path C:\pics\a.png`,
		`ri a.png`, `rm a.png`, `rm -rf photos`, `del /q a.png`, `erase a.png`,
		`rd /s /q photos`, `rmdir photos`, `unlink a.png`, `shred -u a.png`,
		`Get-ChildItem *.png | Remove-Item -Force`,
		`Get-ChildItem *.png | ForEach-Object { Remove-Item $_.FullName }`,
		`gci *.png | % { del $_ }`,
		`cmd /c "del /q a.png"`,
		`powershell -Command "Remove-Item a.png"`,
		`sudo rm a.png`, `ls | xargs rm`, `/bin/rm a.png`, `C:\tools\rm.exe a.png`,
		`find . -name '*.png' -delete`, `find . -name '*.png' -exec rm {} \;`,
		`[System.IO.File]::Delete('C:\pics\a.png')`, `[IO.Directory]::Delete('C:\pics', $true)`,
		`(Get-Item a.png).Delete()`, `$f.Delete()`,
		`python -c "import os; os.remove('a.png')"`, `python -c "import shutil; shutil.rmtree('pics')"`,
		`echo start; Remove-Item a.png`,
	} {
		assert.NotEmpty(t, PermanentDeleteIn(cmd), "not classed as a permanent delete: %s", cmd)
	}
}

func TestRecycleWaysAreNotPermanentDeletes(t *testing.T) {
	for _, cmd := range []string{
		`Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile('C:\pics\a.png', 'OnlyErrorDialogs', 'SendToRecycleBin')`,
		`[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory('C:\pics', 'OnlyErrorDialogs', 'SendToRecycleBin')`,
		`(New-Object -ComObject Shell.Application).Namespace(0).ParseName('C:\pics\a.png').InvokeVerb('delete')`,
		`gio trash a.png`, `trash-put a.png`, `trash a.png`, `fieldkit size recycle C:\pics\a.png --json`,
		`git rm a.png`, `Get-ChildItem *.png`, `Write-Host Deleted`, `go build ./...`, `find . -name '*.png'`,
	} {
		assert.Empty(t, PermanentDeleteIn(cmd), "a recycle or harmless command was classed as a permanent delete: %s", cmd)
	}
}

func TestScratchTargetsAreTempOrBuildOutputOnly(t *testing.T) {
	for _, cmd := range []string{
		`rm -rf build dist node_modules`, `Remove-Item -Recurse -Force $env:TEMP\gorilla-x`,
		`rm -f *.pyc`, `rm -rf /tmp/x`, `Remove-Item -Recurse .\bin, .\obj`, `rm -rf ./target/`,
		`del /q %TEMP%\x.txt`, `rm -rf __pycache__ .pytest_cache`, `Remove-Item out.log`,
	} {
		assert.True(t, OnlyScratchTargets(cmd), "a temp/build clean was not recognised: %s", cmd)
	}
	for _, cmd := range []string{
		`Remove-Item *.png -Force -Recurse`, `rm -rf build photos`, `rm -rf C:\Users\me\out`,
		`rm -rf ~/temp`, `rm -rf /home/me/build`, `Remove-Item -Force`, `rm -rf $HOME/dist`,
		`rm thesis.docx.bak`, `rm *`, `Remove-Item .\build\*.png`,
	} {
		assert.False(t, OnlyScratchTargets(cmd), "a person's files were excused as scratch: %s", cmd)
	}
}

func TestTrashRequestIsRecordedFromThePersonsWords(t *testing.T) {
	s := "trash-words-" + t.Name()
	assert.False(t, TrashAsked(s))
	NoteUserRequest(s, "MOVE ALL THESE FILES in Pictures to trash")
	assert.True(t, TrashAsked(s))
	NoteUserRequest(s, "now the jpgs too")
	assert.True(t, TrashAsked(s), "a follow-up that names neither must keep the record")
	NoteUserRequest(s, "to the bin, and do not delete them permanently")
	assert.True(t, TrashAsked(s), "a negated permanent phrase cleared the record")
	NoteUserRequest(s, "actually, delete them permanently")
	assert.False(t, TrashAsked(s), "the person asking for a permanent delete must clear it")
	NoteUserRequest(s, "put the rest in the Recycle Bin")
	assert.True(t, TrashAsked(s))
	NoteUserRequest(s, "delete them for good, not to the trash")
	assert.False(t, TrashAsked(s), "a permanent phrase wins over a trash word in the same message")
	NoteUserRequest(s, "move the screenshots to trash")
	require.True(t, TrashAsked(s))
	NoteUserRequest(s, "and do not delete anything else")
	assert.True(t, TrashAsked(s), "a negated delete is no delete order")
	NoteUserRequest(s, "no, just delete them")
	assert.False(t, TrashAsked(s), "the latest explicit order (a plain delete) must win over an earlier trash request")
	NoteUserRequest(s, "delete them to the recycle bin")
	assert.True(t, TrashAsked(s), "a delete word next to a trash word still means the trash")
	NoteUserRequest(s, "șterge-le")
	assert.False(t, TrashAsked(s), "Romanian 'șterge' is a delete order")
	NoteUserRequest(s, "to the recycle bin please")
	NoteUserRequest(s, "the model is wrong, try again")
	assert.True(t, TrashAsked(s), "'model' is not 'del': a word inside a word is no order")

	for _, msg := range []string{
		"mets-les à la corbeille", "a la papelera", "in den Papierkorb", "para a lixeira",
		"nel cestino", "mută-le în coșul de gunoi", "muta-le in cosul de gunoi", "empty the wastebasket later",
	} {
		id := "trash-lang-" + msg
		NoteUserRequest(id, msg)
		assert.True(t, TrashAsked(id), "not recognised as a trash request: %q", msg)
	}
	NoteUserRequest("trash-none", "delete the old logs")
	assert.False(t, TrashAsked("trash-none"), "a plain delete request is not a trash request")
}

// The owner's three cases (command post, 2026-10-25 18:38: "we do want to be
// able to delete files ... that's the whole point of this exercise").
func TestOwnerCaseTrashThenTypedRmRunsTheDelete(t *testing.T) {
	for i, later := range []string{
		"use rm -rf *.png",
		"rm -rf *.png instead of the trash",
		"no, Remove-Item *.png -Force, forget the recycle bin",
	} {
		s := "owner-typed-" + string(rune('a'+i))
		NoteUserRequest(s, "MOVE ALL THESE FILES to trash")
		require.True(t, TrashAsked(s))
		NoteUserRequest(s, later)
		assert.False(t, TrashAsked(s), "the person's typed delete command must win: %q", later)
		perms := newTrashPerms(true)
		r := trashRun(t, s, perms, `rm -rf *.png`)
		assert.NotContains(t, r.Content, "NOT RUN", later)
		require.Len(t, perms.asked, 1, "it goes to the permission service like any command (auto approves it)")
		assert.Contains(t, perms.asked[0].Description, PermanentDeleteWhy, "the prompt may say so")
		assert.NotContains(t, perms.asked[0].Description, "You asked for the Recycle Bin")
		assert.Empty(t, perms.asked[0].Irreversible, "no second confirmation: an ordinary delete is not an auto-approve carve-out")
		assert.Empty(t, perms.asked[0].HardStop)
	}
	s := "owner-typed-negated"
	NoteUserRequest(s, "to the trash please, don't rm -rf them")
	assert.True(t, TrashAsked(s), "a negated typed command is no order")
}

func TestOwnerCaseDeleteAllThePngsRunsInAutoMode(t *testing.T) {
	s := "owner-delete-all"
	NoteUserRequest(s, "delete all the pngs")
	perms := newTrashPerms(true)
	r := trashRun(t, s, perms, `Remove-Item *.png -Force`)
	assert.NotContains(t, r.Content, "NOT RUN")
	require.Len(t, perms.asked, 1)
	assert.Empty(t, perms.asked[0].Irreversible)
	assert.Empty(t, perms.asked[0].HardStop)
}

func TestOwnerCaseTrashThenModelChosenRemoveItemIsRefused(t *testing.T) {
	s := "owner-trash-only"
	NoteUserRequest(s, "MOVE ALL THESE FILES to trash")
	perms := newTrashPerms(true)
	r := trashRun(t, s, perms, `Remove-Item *.png -Force -Recurse`)
	assert.Contains(t, r.Content, "NOT RUN")
	assert.Contains(t, r.Content, "Recycle Bin instead")
	if runtime.GOOS == "windows" {
		assert.Contains(t, r.Content, "SendToRecycleBin")
	}
	assert.Empty(t, perms.asked)
}

func TestAHelperInheritsTheRecordAndCannotClearIt(t *testing.T) {
	parent, child := "trash-parent-"+t.Name(), "trash-child-"+t.Name()
	NoteUserRequest(parent, "move these to the recycle bin")
	LinkHelperSession(child, parent)
	assert.True(t, TrashAsked(child))
	NoteUserRequest(child, "delete the files permanently") // written by the model
	assert.True(t, TrashAsked(child), "a model-written helper prompt cleared the person's record")
	assert.True(t, TrashAsked(parent))
}

// trashPerms records what the bash tool asks and answers no, so nothing runs
// unless a test wants it to.
type trashPerms struct {
	allowAllPerms
	auto  bool
	asked []permission.CreatePermissionRequest
}

func (p *trashPerms) IsAutoApproved(string) bool { return p.auto }
func (p *trashPerms) Request(r permission.CreatePermissionRequest) bool {
	p.asked = append(p.asked, r)
	return false
}

func trashRun(t *testing.T, session string, perms *trashPerms, cmd string) ToolResponse {
	t.Helper()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, session)
	ctx = context.WithValue(ctx, MessageIDContextKey, "m")
	b := NewBashTool(perms)
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(cmd)
	r, err := b.Run(ctx, ToolCall{Input: `{"command":"` + esc + `"}`})
	if err != nil {
		r.Content = err.Error()
	}
	return r
}

func newTrashPerms(auto bool) *trashPerms {
	return &trashPerms{allowAllPerms: allowAllPerms{pubsub.NewBroker[permission.PermissionRequest]()}, auto: auto}
}

func TestUnwatchedPermanentDeleteIsRefusedAfterATrashRequest(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "a.png")
	require.NoError(t, os.WriteFile(png, []byte("x"), 0o644))
	cmd := "rm -f " + filepath.ToSlash(png)
	if runtime.GOOS == "windows" {
		cmd = "Remove-Item '" + png + "' -Force"
	}
	s := "trash-run-" + t.Name()
	NoteUserRequest(s, "MOVE ALL THESE FILES to trash")
	perms := newTrashPerms(true)
	r := trashRun(t, s, perms, cmd)

	assert.True(t, r.IsError)
	assert.Contains(t, r.Content, "NOT RUN")
	assert.Contains(t, r.Content, PermanentDeleteWhy)
	if runtime.GOOS == "windows" {
		assert.Contains(t, r.Content, "SendToRecycleBin", "the refusal must name the safe way")
	} else if runtime.GOOS == "linux" {
		assert.Contains(t, r.Content, "gio trash")
	}
	assert.Empty(t, perms.asked, "refused before any prompt: nobody is there to answer it")
	_, err := os.Stat(png)
	assert.NoError(t, err, "the file must survive")
}

func TestWatchedPermanentDeleteAsksInPlainWords(t *testing.T) {
	s := "trash-ask-" + t.Name()
	NoteUserRequest(s, "send these to the recycle bin")
	perms := newTrashPerms(false)
	trashRun(t, s, perms, `Remove-Item *.png -Force`)
	require.Len(t, perms.asked, 1, "ask mode must ask the person")
	d := perms.asked[0].Description
	assert.Contains(t, d, "deletes for good, does not go to the Recycle Bin")
	assert.Contains(t, d, "You asked for the Recycle Bin")
	assert.Contains(t, CommandPurpose(`Remove-Item *.png -Force`), "does not go to the Recycle Bin")
}

func TestPermanentDeletesTheRulesDoNotTouch(t *testing.T) {
	// No trash request: an auto-mode delete reaches the permission service
	// exactly as before (which auto-approves it).
	perms := newTrashPerms(true)
	r := trashRun(t, "trash-untouched-none", perms, `Remove-Item old.log -Force`)
	assert.NotContains(t, r.Content, "NOT RUN")
	assert.Len(t, perms.asked, 1)
	assert.NotContains(t, perms.asked[0].Description, "You asked for the Recycle Bin")

	// The person asked for the delete in so many words.
	s := "trash-untouched-explicit"
	NoteUserRequest(s, "move them to trash")
	NoteUserRequest(s, "no wait, delete them permanently")
	perms = newTrashPerms(true)
	r = trashRun(t, s, perms, `Remove-Item *.png -Force`)
	assert.NotContains(t, r.Content, "NOT RUN")
	assert.Len(t, perms.asked, 1)

	// A build clean in a trash conversation.
	s = "trash-untouched-build"
	NoteUserRequest(s, "move the screenshots to the recycle bin, then rebuild")
	perms = newTrashPerms(true)
	r = trashRun(t, s, perms, `rm -rf build node_modules`)
	assert.NotContains(t, r.Content, "NOT RUN")
	assert.Len(t, perms.asked, 1)
}

func TestTheHugeDeleteRefusalStillComesFirst(t *testing.T) {
	v := bashGateVerdict(`Remove-Item -Recurse -Force C:\Users\gorilla1`)
	assert.NotEmpty(t, v.Irreversible)
	assert.True(t, strings.HasPrefix(v.Description, "DANGEROUS"), v.Description)
	assert.Empty(t, v.PermanentDelete, "the huge-delete verdict must not be replaced")
	assert.True(t, strings.HasPrefix(CommandPurpose(`rm -rf ~/`), "Cannot be undone"))
}

func TestToolGuidanceTeachesTheRecycleWay(t *testing.T) {
	g := trashGuidance()
	assert.Contains(t, g, "delete FOR GOOD")
	assert.Contains(t, g, "NEVER fall back to a plain delete")
	assert.Contains(t, bashDescription(), g)
	a := RecycleAdvice()
	switch runtime.GOOS {
	case "windows":
		assert.Contains(t, g, "[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($full, 'OnlyErrorDialogs', 'SendToRecycleBin')")
		assert.Contains(t, g, "DeleteDirectory")
		assert.Contains(t, a, "[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($_.FullName, 'OnlyErrorDialogs', 'SendToRecycleBin')")
		assert.Contains(t, a, "DeleteDirectory($path, 'OnlyErrorDialogs', 'SendToRecycleBin')")
	case "darwin":
		assert.Contains(t, g, "Finder")
		assert.Contains(t, a, "Finder")
	default:
		assert.Contains(t, g, "gio trash")
		assert.Contains(t, g, "trash-put")
		assert.Contains(t, a, "gio trash")
	}
	if _, err := lookPathFieldkit(); err == nil {
		assert.Contains(t, g, "fieldkit size recycle")
		assert.Contains(t, a, "fieldkit size recycle")
	}
}

func TestTheFieldkitRecycleToolIsNamedOnlyWhenPresent(t *testing.T) {
	saved := lookPathFieldkit
	defer func() { lookPathFieldkit = saved }()
	lookPathFieldkit = func() (string, error) { return "", os.ErrNotExist }
	assert.NotContains(t, trashGuidance(), "fieldkit")
	assert.NotContains(t, RecycleAdvice(), "fieldkit")
	lookPathFieldkit = func() (string, error) { return "fieldkit", nil }
	assert.Contains(t, trashGuidance(), "fieldkit size recycle")
	assert.Contains(t, RecycleAdvice(), "fieldkit size recycle")
}
