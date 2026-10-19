package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// noRequiredTool has parameters but none of them is required: its Required is nil.
type noRequiredTool struct{}

func (noRequiredTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        "peers",
		Description: "List the other sessions.",
		Parameters:  map[string]any{"all": map[string]any{"type": "boolean"}},
	}
}

func (noRequiredTool) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	return tools.ToolResponse{}, nil
}

// 2026-10-19: LM Studio answered 400 "Expected array, received null" at
// tools[12].function.parameters.required, so every local-model turn failed
// (0.1.147 as well). A tool with no required parameters must send [], not null.
func TestAToolWithNoRequiredParametersSendsAnEmptyList(t *testing.T) {
	o := &openaiClient{}
	raw, err := json.Marshal(o.convertTools([]tools.BaseTool{noRequiredTool{}}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, `"required":null`) || !strings.Contains(s, `"required":[]`) {
		t.Fatalf("required must be an empty JSON array, got: %s", s)
	}
	if got := requiredList(nil); got == nil || len(got) != 0 {
		t.Fatalf("requiredList(nil) = %#v, want an empty non-nil list", got)
	}
	if got := requiredList([]string{"path"}); len(got) != 1 || got[0] != "path" {
		t.Fatalf("requiredList kept %#v", got)
	}
}
