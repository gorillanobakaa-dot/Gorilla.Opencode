package tools

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// The owner's prompt "check your launch date" raised five dialogs: a search
// and a fetch per site. With the default (allow) none of these ask.
func TestWebPolicyPublicReadsNeverAsk(t *testing.T) {
	t.Cleanup(func() { _ = config.SetWebPermission("allow"); _ = config.SetPermissionMode("ask") })
	_ = config.SetWebPermission("allow")
	for _, m := range []string{"ask", "auto", "accept-edits"} {
		_ = config.SetPermissionMode(m)
		for _, u := range []string{
			"https://deepmind.google/models/", "https://blog.google/technology/x?utm=1",
			"https://docs.cloud.google.com/a/b", "http://example.com/",
		} {
			if webNeedsAsk(u) {
				t.Errorf("mode %s: %s asked", m, u)
			}
		}
		if webSearchNeedsAsk("gemini 4 hydrogen launch date") {
			t.Errorf("mode %s: search asked", m)
		}
	}
}

func TestWebPolicyStillAsks(t *testing.T) {
	t.Cleanup(func() { _ = config.SetWebPermission("allow"); _ = config.SetPermissionMode("ask") })
	_ = config.SetWebPermission("allow")
	_ = config.SetPermissionMode("auto")
	for name, u := range map[string]string{
		"file scheme": "file:///etc/passwd",
		"ftp":         "ftp://example.com/x",
		"localhost":   "http://localhost:8080/",
		"loopback":    "http://127.0.0.1/",
		"lan":         "http://192.168.1.1/admin",
		"metadata":    "http://169.254.169.254/latest",
		"long query":  "https://example.com/?d=" + strings.Repeat("A", 400),
		"userinfo":    "https://user:pw@example.com/",
		"no host":     "https:///x",
	} {
		if !webNeedsAsk(u) {
			t.Errorf("%s: should ask: %s", name, u)
		}
	}
	if !webSearchNeedsAsk(strings.Repeat("secret ", 80)) {
		t.Error("long search query should ask")
	}
}

func TestWebPolicyAskSettingAndPlanMode(t *testing.T) {
	t.Cleanup(func() { _ = config.SetWebPermission("allow"); _ = config.SetPermissionMode("ask") })
	_ = config.SetPermissionMode("auto")
	_ = config.SetWebPermission("ask")
	if !webNeedsAsk("https://example.com/") || !webSearchNeedsAsk("x") {
		t.Error("permissions.web=ask must ask")
	}
	_ = config.SetWebPermission("allow")
	_ = config.SetPermissionMode("plan")
	if !webNeedsAsk("https://example.com/") || !webSearchNeedsAsk("x") {
		t.Error("plan mode must ask for web reads")
	}
	if err := config.SetWebPermission("maybe"); err == nil {
		t.Error("bad value accepted")
	}
}
