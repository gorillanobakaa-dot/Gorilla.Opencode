package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

// GORILLA FIX (2026-10-21): the metadata stored beside a result is masked like
// the result. MEASURED before: view's metadata kept a sign-in token in
// cleartext in the database and in exports while the content showed [REDACTED].
func TestToolResultMetadataIsMaskedLikeItsContent(t *testing.T) {
	tok := "ghp_" + strings.Repeat("A", 36)
	calls := []message.ToolCall{{ID: "1", Name: tools.BashToolName}}
	results := []message.ToolResult{{ToolCallID: "1", Content: "out " + tok, Metadata: `{"output":"` + tok + `"}`}}
	finishToolResults("metadata-mask-test", calls, results, []bool{true})
	if strings.Contains(results[0].Content, tok) {
		t.Errorf("content kept the token: %s", results[0].Content)
	}
	if strings.Contains(results[0].Metadata, tok) {
		t.Errorf("metadata kept the token: %s", results[0].Metadata)
	}
}
