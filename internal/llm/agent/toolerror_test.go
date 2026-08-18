package agent

// GORILLA FIX (2026-08-18): a failed tool must not look like a successful one.
//
// Upstream handled only permission.ErrorPermissionDenied; every other error fell
// through to the result assignment, which read the ZERO-VALUE ToolResponse a
// failing tool returns. The model therefore received Content:"" with
// IsError:false — an empty result flagged as success — and carried on believing
// the work had been done.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

// This mirrors what agent.go now does for a non-permission error, so the shape
// of the result is pinned even though the surrounding loop needs a live agent.
func toolResultFor(name string, resp tools.ToolResponse, err error) message.ToolResult {
	if err != nil {
		return message.ToolResult{
			ToolCallID: "id",
			Content:    fmt.Sprintf("The %s tool failed and produced no result: %v", name, err),
			IsError:    true,
		}
	}
	return message.ToolResult{
		ToolCallID: "id",
		Content:    resp.Content,
		IsError:    resp.IsError,
	}
}

func TestAFailedToolIsReportedAsAnError(t *testing.T) {
	// A tool that errors returns the zero ToolResponse alongside the error.
	res := toolResultFor("write", tools.ToolResponse{}, errors.New("disk full"))

	if !res.IsError {
		t.Error("a failed tool was reported to the model as a SUCCESS")
	}
	if strings.TrimSpace(res.Content) == "" {
		t.Error("a failed tool handed the model an EMPTY result; it will assume the work was done")
	}
	if !strings.Contains(res.Content, "disk full") {
		t.Errorf("the cause was dropped: %q", res.Content)
	}
	if !strings.Contains(res.Content, "write") {
		t.Errorf("the failing tool is not named: %q", res.Content)
	}
}

// A successful tool must be untouched by the change.
func TestOrdinaryResultsAreUntouched(t *testing.T) {
	res := toolResultFor("view", tools.ToolResponse{Content: "file contents"}, nil)
	if res.IsError {
		t.Error("a successful tool was marked as an error")
	}
	if res.Content != "file contents" {
		t.Errorf("content was altered: %q", res.Content)
	}
}
