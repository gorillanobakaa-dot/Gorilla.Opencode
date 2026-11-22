package permission

// GORILLA OVERRIDE (2026-11-22, Fieldkit plan row F0.7): the job mandate.
// Each test fails on the old code, where an unlisted request raised a dialog
// and waited for a person.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

func mandateService(t *testing.T, body string) (*permissionService, <-chan struct{}) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if body != "" {
		f := filepath.Join(t.TempDir(), "mandate.json")
		if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GORILLA_MANDATE_FILE", f)
	}
	t.Cleanup(PermissionWaitForTest(2 * time.Second))
	s := NewPermissionService().(*permissionService)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	prompts := make(chan struct{}, 8)
	ch := s.Subscribe(ctx)
	go func() {
		for ev := range ch {
			if ev.Type == "created" {
				prompts <- struct{}{}
			}
		}
	}()
	return s, prompts
}

func noPrompt(t *testing.T, prompts <-chan struct{}) {
	t.Helper()
	select {
	case <-prompts:
		t.Fatal("a permission dialog was raised under a mandate")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestMandateApprovesListedToolWithoutAsking(t *testing.T) {
	s, prompts := mandateService(t, `{"allow":["bash","edit"]}`)
	start := time.Now()
	if !s.Request(CreatePermissionRequest{SessionID: "j", ToolName: "bash", Action: "execute", Path: config.WorkingDirectory()}) {
		t.Fatal("a tool on the mandate was refused")
	}
	if time.Since(start) > time.Second {
		t.Fatal("the mandated call waited")
	}
	noPrompt(t, prompts)
}

func TestMandateRefusesUnlistedToolInsteadOfPrompting(t *testing.T) {
	s, prompts := mandateService(t, `{"allow":["bash"]}`)
	start := time.Now()
	if s.Request(CreatePermissionRequest{SessionID: "j", ToolName: "web_fetch", Action: "fetch", Path: config.WorkingDirectory()}) {
		t.Fatal("a tool outside the mandate was approved")
	}
	if time.Since(start) > time.Second {
		t.Fatal("the refusal waited for a person")
	}
	noPrompt(t, prompts)
}

func TestMandateNeverApprovesIrreversible(t *testing.T) {
	s, prompts := mandateService(t, `{"allow":["*"]}`)
	if s.Request(CreatePermissionRequest{SessionID: "j", ToolName: "bash", Action: "execute",
		Path: config.WorkingDirectory(), Irreversible: "deletes a disk"}) {
		t.Fatal("the mandate approved an irreversible request")
	}
	noPrompt(t, prompts)
}

func TestMandateFileMissingFailsClosedWithoutPrompting(t *testing.T) {
	s, prompts := mandateService(t, "")
	t.Setenv("GORILLA_MANDATE_FILE", filepath.Join(t.TempDir(), "absent.json"))
	if s.Request(CreatePermissionRequest{SessionID: "j", ToolName: "bash", Action: "execute", Path: config.WorkingDirectory()}) {
		t.Fatal("an unreadable mandate approved a call")
	}
	noPrompt(t, prompts)
}
