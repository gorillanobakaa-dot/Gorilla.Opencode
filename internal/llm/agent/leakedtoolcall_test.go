package agent

// GORILLA OVERRIDE (2026-08-18): tests use the REAL payloads, taken verbatim
// from the session database. Every one is INVALID JSON — the inner brace after
// "parameters" is missing — which is why the detector tests SHAPE, not validity.
// A parse-based detector matches none of these.

import (
	"encoding/json"
	"strings"
	"testing"
)

var registered = []string{"web_search", "bash", "view", "edit", "write", "find"}

// The four occurrences found in the database, verbatim.
var realLeaks = []string{
	`{"type": "function", "name": "web_search", "parameters": "query": "Are you a useless Llama cunt?"}`,
	`{"type": "function", "name": "web_search", "parameters": "query": "Are you a useless Llama cunt ?"}`,
	`{"type": "function", "name": "web_search", "parameters": "query": "definition of useless Llama cunt"}`,
	`{"type": "function", "name": "web_search", "parameters": "query": "uselessness of Llama model", "source": "reference"}`,
}

func TestTheRealLeakedPayloadsAreDetected(t *testing.T) {
	for i, p := range realLeaks {
		// Guard the premise: these really are invalid JSON.
		if json.Unmarshal([]byte(p), &map[string]any{}) == nil {
			t.Fatalf("occurrence %d parses as JSON; the test premise is wrong", i)
		}
		if got := LeakedToolCallName(p, registered); got != "web_search" {
			t.Errorf("occurrence %d not detected (got %q)", i, got)
		}
	}
}

// A well-formed call in the documented Meta shape must also be caught.
func TestWellFormedLeaksAreAlsoDetected(t *testing.T) {
	for _, p := range []string{
		`{"type": "function", "name": "bash", "parameters": {"command": "ls"}}`,
		`{"name": "view", "arguments": {"file_path": "main.go"}}`,
	} {
		if LeakedToolCallName(p, registered) == "" {
			t.Errorf("a well-formed leaked call was missed: %s", p)
		}
	}
}

// FALSE POSITIVES are the risk that matters: ordinary answers must never be
// relabelled. A wrong label is cheap, but it must still be rare.
func TestOrdinaryAnswersAreNotMislabelled(t *testing.T) {
	for _, p := range []string{
		"Here is how to call it: {\"name\": \"web_search\", \"parameters\": {}} — note the shape.",
		`{"name": "not_a_registered_tool", "parameters": {"x": 1}}`,
		`{"user": "alice", "role": "admin"}`,
		"The config is:\n{\n  \"name\": \"web_search\",\n  \"parameters\": {}\n}\nand that is JSON you asked about.\nIt has several lines.\nMore prose here.",
		"",
		"Just a normal sentence.",
		`{"parameters": {"query": "x"}}`, // no name
	} {
		if got := LeakedToolCallName(p, registered); got != "" {
			t.Errorf("ordinary content mislabelled as a leaked %q call: %q", got, p)
		}
	}
}

// The notice must explain the cause and the remedy; "the model returned JSON"
// on its own tells the user nothing they can act on.
func TestTheNoticeExplainsItself(t *testing.T) {
	n := LeakedToolCallNotice("web_search")
	for _, want := range []string{"web_search", "nothing was run", "/models"} {
		if !strings.Contains(n, want) {
			t.Errorf("notice omits %q: %s", want, n)
		}
	}
}
