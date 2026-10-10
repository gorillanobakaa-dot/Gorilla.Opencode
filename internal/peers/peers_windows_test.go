//go:build windows

package peers

// GORILLA (2026-10-10): the Windows pipe's access list, read back from the live
// object rather than trusted from the string that built it.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestPipeGrantsOnlyTheCurrentUser(t *testing.T) {
	ln, err := listen(pipePrefix + strconv.Itoa(os.Getpid()) + "-dacl")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close(); ln.free() })
	pl := ln.(*pipeListener)

	sd, err := windows.GetSecurityInfo(pl.next, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetSecurityInfo: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("DACL: %v", err)
	}
	if dacl == nil {
		t.Fatal("the pipe has a NULL DACL, which grants everyone everything")
	}
	if dacl.AceCount != 1 {
		t.Fatalf("the DACL has %d entries, want exactly 1 (the current user): %s", dacl.AceCount, sd.String())
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatalf("GetAce: %v", err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
		t.Errorf("the only entry is of type %d, want an allow entry", ace.Header.AceType)
	}
	got := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	me, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equals(me) {
		t.Errorf("the only entry names %s, want the current user %s", got.String(), me.String())
	}
	ctl, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Errorf("the DACL is not protected, so it could inherit entries: %s", sd.String())
	}
	if !strings.HasPrefix(pipeSDDL(me.String()), "D:P(A;;GA;;;S-1-") {
		t.Errorf("unexpected SDDL %q", pipeSDDL(me.String()))
	}
}

// FILE_FLAG_FIRST_PIPE_INSTANCE: a name somebody already holds is refused, not
// joined.
func TestASecondListenerOnTheSameNameFails(t *testing.T) {
	name := pipePrefix + strconv.Itoa(os.Getpid()) + "-first"
	ln, err := listen(name)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close(); ln.free() })
	if second, err := listen(name); err == nil {
		_ = second.Close()
		second.free()
		t.Fatal("a second listener took over a pipe name that was already in use")
	}
}

// The client refuses a server that is not the process the register names,
// before sending anything.
func TestClientRefusesAServerThatIsNotTheRegisteredProcess(t *testing.T) {
	dir := t.TempDir()
	a := startT(t, dir, filepath.Join(dir, "alpha"), "a")
	b := startT(t, dir, filepath.Join(dir, "beta"), "b")
	_, err := call(b.Endpoint(), os.Getpid()+4, Request{Type: TypeMessage, From: a.Identity(), Text: "hi"})
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("err %v, want a refusal naming the mismatch", err)
	}
	if b.Pending() != 0 {
		t.Error("the message was sent anyway")
	}
}

func TestPidAliveSeesThisProcess(t *testing.T) {
	if !pidAlive(os.Getpid()) {
		t.Error("this process reads as not running")
	}
}
