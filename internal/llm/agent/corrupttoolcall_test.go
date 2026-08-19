package agent

// GORILLA OVERRIDE (2026-08-19): recovered from the session database after a
// real run the owner photographed.
//
// A bash call arrived as {"������command":""} —
// six U+FFFD REPLACEMENT CHARACTERs prepended to the parameter name. U+FFFD is
// what a decoder emits for bytes that were not valid UTF-8, so this is a
// TRANSPORT fault: a stream chunk split mid-character, or bytes decoded before
// the sequence was complete. The model did not send that.
//
// What happened without this check is the part worth remembering. The key was
// no longer "command", so the bash tool saw no command, ran nothing, and
// returned an empty result. The model — given no explanation — apologised for
// a mistake it had not made ("That command was malformed") and tried again.
//
// A model misled into blaming itself keeps doing the same thing, because it is
// fixing the wrong thing.

import (
	"strings"
	"testing"
)

func TestReplacementCharactersAreReportedAsTransportDamage(t *testing.T) {
	// The exact payload from the session database.
	raw := "{\"������command\":\"\"}"
	reason := corruptedToolInput(raw)
	if reason == "" {
		t.Fatal("the real corrupted payload was accepted as intact")
	}
	if !strings.Contains(reason, "not valid UTF-8") {
		t.Errorf("the reason does not name the cause: %q", reason)
	}
}

func TestUnparseableArgumentsAreReported(t *testing.T) {
	if corruptedToolInput(`{"command": "ls`) == "" {
		t.Error("truncated JSON was accepted as intact")
	}
	if corruptedToolInput(`{"parameters": "query": "x"}`) == "" {
		t.Error("the broken-brace shape seen in leaked Llama tool calls was accepted")
	}
}

// It must not become a general judge of whether arguments are sensible — that
// is the tool's job. This layer only asks whether they arrived intact.
func TestIntactArgumentsPassThroughIncludingAwkwardOnes(t *testing.T) {
	for _, ok := range []string{
		`{"command":"ls -la"}`,
		`{"command":"grep -r 'für' ."}`,          // non-ASCII, legitimately
		`{"command":"echo 日本語"}`,                 // multi-byte, legitimately
		`{"command":"printf '\\xef\\xbf\\xbd'"}`, // asking to PRINT the byte is fine
		`{}`,
		``,
		`   `,
	} {
		if reason := corruptedToolInput(ok); reason != "" {
			t.Errorf("rejected intact arguments %q: %s", ok, reason)
		}
	}
}
