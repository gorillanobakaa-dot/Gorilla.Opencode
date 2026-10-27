//go:build windows

package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

// gap #11 through the tool: console=true runs the command in a ConPTY and
// input answers its prompt; without it the schema would not offer the mode.
func TestBashToolConsoleMode_conpty(t *testing.T) {
	b := NewBashTool(&allowAllPerms{pubsub.NewBroker[permission.PermissionRequest]()})
	if _, ok := b.Info().Parameters["console"]; !ok {
		t.Fatal("the bash tool does not offer console on Windows")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, SessionIDContextKey, "conpty")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m")
	r, err := b.Run(ctx, ToolCall{Input: `{"command":"$a = Read-Host 'Continue'; Write-Output ('GOT-' + $a)","console":true,"input":"yes","timeout":30000}`})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(r.Content, "GOT-yes") {
		t.Fatalf("the answer did not reach Read-Host: %q", r.Content)
	}
}
