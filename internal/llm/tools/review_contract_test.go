package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Decode real script output with the REAL struct the tool uses. A schema that
// only matches on paper is the failure this catches.
func TestReviewScriptOutputDecodesWithTheRealStruct(t *testing.T) {
	script := filepath.Join("codereview", "toolkit", "code_review.py")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("script not present: %v", err)
	}
	py, pre, err := getPythonBinary()
	if err != nil {
		t.Skipf("no python: %v", err)
	}
	args := append(append([]string{}, pre...), script, "../../config", "--audience", "agent", "--max-files", "12")
	out, err := exec.Command(py, args...).Output()
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	var rep agentReport
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("the Go struct could not decode the script's JSON: %v", err)
	}
	if rep.Target == "" {
		t.Error("target did not decode")
	}
	if rep.FilesScanned == 0 {
		t.Error("files_scanned did not decode")
	}
	if len(rep.Languages) == 0 {
		t.Error("languages did not decode")
	}
	if !rep.Trust.PositionCheck {
		t.Error("trust.position_checked did not decode")
	}
	if rep.Trust.Caveat == "" {
		t.Error("trust.caveat did not decode — the block that stops a false clean report")
	}
	t.Logf("decoded: %d files, %v, ran=%v missing=%v",
		rep.FilesScanned, rep.Languages, rep.Trust.ToolsRan, rep.Trust.ToolsMissing)

	// And the summariser must render it without error.
	summary, err := summariseReview(out, "full")
	if err != nil {
		t.Fatalf("summariseReview: %v", err)
	}
	if len(summary) < 100 {
		t.Errorf("summary implausibly short:\n%s", summary)
	}
	t.Logf("summary first line: %.120s", summary)
}
