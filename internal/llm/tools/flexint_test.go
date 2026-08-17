package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexIntAcceptsTheShapesModelsSend(t *testing.T) {
	for input, want := range map[string]int{
		`100`:     100,
		`"100"`:   100,
		`" 100 "`: 100,
		`100.0`:   100,
		`null`:    0,
		`""`:      0,
	} {
		var f FlexInt
		require.NoError(t, json.Unmarshal([]byte(input), &f), "input %s", input)
		assert.Equal(t, want, f.Int(), "input %s", input)
	}
	for _, garbage := range []string{`"abc"`, `100.5`, `{}`, `true`} {
		var f FlexInt
		assert.Error(t, json.Unmarshal([]byte(garbage), &f),
			"%s is not a lossless number and must stay an error", garbage)
	}
}

// TestViewAcceptsStringNumbers reproduces the recorded live failure verbatim:
// on 2026-08-16 llama-3.3-70b called view with string-typed offset/limit and
// burned three billed turns on `cannot unmarshal string into Go struct field
// ViewParams.offset of type int`. The exact input from the session database
// must now parse and run.
func TestViewAcceptsStringNumbers(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "big.txt")
	require.NoError(t, os.WriteFile(fp, []byte(strings.Repeat("line\n", 300)), 0o644))

	input, _ := json.Marshal(map[string]any{
		"file_path": fp,
		"offset":    "100", // the llama shape, verbatim
		"limit":     "50",
	})
	resp, err := NewViewTool(nil).Run(context.Background(), ToolCall{Input: string(input)})
	require.NoError(t, err)
	assert.False(t, resp.IsError, "string-typed numbers must coerce, got: %s", resp.Content)
	assert.Contains(t, resp.Content, "line", "the read must actually happen")
}
