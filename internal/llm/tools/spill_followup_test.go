package tools

// GORILLA OVERRIDE (2026-10-04): the truncation notice tells the model to search
// the saved file with find or read it with view. If either tool cannot open that
// file, the notice is an instruction that leads nowhere — worse than no notice.
// So the instruction is carried out here, with the real tools.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func spilledNeedleFile(t *testing.T) (path, needle string) {
	t.Helper()
	useTempSpillDir(t)
	needle = "NEEDLE: widget 7731 failed its checksum"
	out := strings.Repeat("record processed ok\n", 3000) + needle + "\n" + strings.Repeat("record processed ok\n", 3000)
	m := spillPathInNotice.FindStringSubmatch(truncateOutput(out))
	if m == nil {
		t.Fatal("no file named in the notice")
	}
	return m[1], needle
}

func TestTheSavedOutputCanBeSearchedWithFindAsTheNoticeSays(t *testing.T) {
	path, needle := spilledNeedleFile(t)
	in, _ := json.Marshal(map[string]any{"query": "NEEDLE", "path": path})
	resp, err := NewFindTool().Run(context.Background(), ToolCall{ID: "t", Name: FindToolName, Input: string(in)})
	if err != nil {
		t.Fatalf("find could not be run on the saved output: %v", err)
	}
	if !strings.Contains(resp.Content, needle) {
		t.Errorf("find on the saved output did not return the line the truncation removed:\n%s", resp.Content)
	}
}

func TestTheNoticeGivesTheExactNextCall(t *testing.T) {
	useTempSpillDir(t)
	got := truncateOutput(strings.Repeat("line\n", 20000))
	tail := got[len(got)-900:]
	// Found with a real model (gemma-4-e2b, 2026-10-04), twice. Told only that
	// the complete output "is saved at <path> — read the part you need with view
	// or search it with find", it did neither, and on the second run invented a
	// line that was not the answer. A small model follows a call it is handed;
	// it does not compose one from a description.
	for _, want := range []string{"NEXT:", `"query"`, `"path"`, "find"} {
		if !strings.Contains(tail, want) {
			t.Errorf("the notice does not hand over the next call (%s missing):\n%s", want, tail)
		}
	}
	m := spillPathInNotice.FindStringSubmatch(got)
	if m == nil {
		t.Fatal("no file named in the notice")
	}
	// The path inside the suggested call must be valid JSON as written: a
	// Windows path pasted unescaped is the exact failure toolinput.go repairs.
	i := strings.LastIndex(got, "{")
	j := strings.LastIndex(got, "}")
	var call map[string]string
	if i < 0 || j < i || json.Unmarshal([]byte(got[i:j+1]), &call) != nil {
		t.Fatalf("the suggested call is not valid JSON: %q", got[max(0, i):])
	}
	if call["path"] != m[1] {
		t.Errorf("the suggested call names %q, the saved file is %q", call["path"], m[1])
	}
}
