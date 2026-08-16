package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// A TOOL CALL WITH NO ARGUMENTS MUST STILL CARRY AN ARGUMENTS OBJECT.
//
// Measured 2026-08-14. A session died permanently with
//
//	messages.5.content.0.tool_use.input: Field required   (HTTP 400)
//
// The same message index, on every subsequent turn, on every Anthropic-family
// model — switching from Muse Glimmer to Opus 4.6 changed nothing, because the
// damage was already written into the session history and was being replayed.
//
// Two independent defects combined, and BOTH have to stay fixed:
//
//  1. CAPTURE. collectParts did json.Marshal(p.FunctionCall.Args) on a nil map,
//     which produces the four bytes `null`, and stored that as ToolCall.Input.
//     A model that calls a tool with no arguments therefore poisoned its own
//     history on the spot. (Seen in the wild as `bash.command: {}`.)
//
//  2. REPLAY. caFunctionCall.Args was `json:"args,omitempty"`. omitempty drops
//     a map when len == 0 — so a nil map AND a legitimately empty one both
//     vanish from the wire entirely. Antigravity translates the envelope into
//     the native Anthropic shape, that shape declares input as required, and
//     the request is rejected before any model sees it.
//
// Defect 2 is why the fix cannot be "normalise on the way in" alone: sessions
// poisoned before this change still hold Input == "null", and must be repaired
// as they are replayed. TestPoisonedHistoryIsRepairedOnReplay holds that.
//
// These tests were written against the unfixed code and observed to FAIL first:
// null vs {} on capture, and a missing "args" key on the wire. If they ever
// pass without exercising those paths, they have stopped testing anything.

// Defect 1, at the point of capture: nil args must become {}, never null.
func TestNoArgToolCallIsCapturedAsEmptyObjectNotNull(t *testing.T) {
	_, calls := collectParts([]caPart{
		{FunctionCall: &caFunctionCall{ID: "toolu_1", Name: "bash", Args: nil}},
	})
	if len(calls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(calls))
	}
	if calls[0].Input == "null" {
		t.Fatalf("nil args captured as the literal `null` — this is the exact value that poisons a session and 400s every later turn")
	}
	if calls[0].Input != "{}" {
		t.Fatalf("Input = %q, want %q", calls[0].Input, "{}")
	}
	// It must be a JSON object, not merely non-null.
	var m map[string]any
	if err := json.Unmarshal([]byte(calls[0].Input), &m); err != nil || m == nil {
		t.Fatalf("Input %q does not unmarshal to a non-nil object (err=%v)", calls[0].Input, err)
	}
}

// Real arguments must round-trip untouched. The repair must not be a rewrite.
func TestToolCallArgsRoundTripUnchanged(t *testing.T) {
	_, calls := collectParts([]caPart{
		{FunctionCall: &caFunctionCall{ID: "toolu_1", Name: "ls", Args: map[string]any{"path": "/etc"}}},
	})
	var m map[string]any
	if err := json.Unmarshal([]byte(calls[0].Input), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["path"] != "/etc" {
		t.Fatalf("args mangled: %v", m)
	}
}

// wireFor renders the outbound envelope's messages exactly as they are sent.
func wireFor(t *testing.T, call message.ToolCall) string {
	t.Helper()
	contents := caConvertMessages([]message.Message{{
		Role:  message.Assistant,
		Parts: []message.ContentPart{call},
	}}, true)
	b, err := json.Marshal(contents)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return string(b)
}

// Defect 2, on the wire: the args key must be present even when empty.
func TestEmptyArgsStillAppearOnTheWire(t *testing.T) {
	wire := wireFor(t, message.ToolCall{ID: "toolu_1", Name: "bash", Input: "{}", Finished: true})
	if !strings.Contains(wire, `"args"`) {
		t.Fatalf("no \"args\" key on the wire — omitempty dropped it; the backend translates this to tool_use with no input and 400s.\nwire: %s", wire)
	}
	if strings.Contains(wire, `"args":null`) {
		t.Fatalf("args serialised as null, which fails the required check just as hard as omitting it.\nwire: %s", wire)
	}
	if !strings.Contains(wire, `"args":{}`) {
		t.Fatalf("want \"args\":{} on the wire, got: %s", wire)
	}
}

// Sessions poisoned before the capture fix still hold Input == "null". They
// must be repaired as they replay, or every existing broken session stays dead.
func TestPoisonedHistoryIsRepairedOnReplay(t *testing.T) {
	wire := wireFor(t, message.ToolCall{ID: "toolu_1", Name: "bash", Input: "null", Finished: true})
	if !strings.Contains(wire, `"args":{}`) {
		t.Fatalf("history holding the poisoned `null` was not repaired on replay; the session stays 400-dead forever.\nwire: %s", wire)
	}
}

// Unparseable input must not silently become a dropped field either. Whatever
// went wrong upstream, the wire shape stays valid.
func TestUnparseableInputStillProducesAnArgsObject(t *testing.T) {
	wire := wireFor(t, message.ToolCall{ID: "toolu_1", Name: "bash", Input: "not json at all", Finished: true})
	if !strings.Contains(wire, `"args":{}`) {
		t.Fatalf("unparseable input produced an invalid wire shape: %s", wire)
	}
}

// The direct Anthropic provider reaches the same defect by a different route:
// the SDK tags Input `omitzero,required`, so a nil map is dropped from the
// request body entirely. Proven by probe: "null" -> {"input":null} -> 400.
func TestAnthropicToolUseAlwaysCarriesAnInputObject(t *testing.T) {
	client := &anthropicClient{}
	for _, input := range []string{"null", "{}", "not json at all", ""} {
		msgs := client.convertMessages([]message.Message{{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "toolu_1", Name: "bash", Input: input, Finished: true},
			},
		}})
		if len(msgs) != 1 {
			t.Fatalf("input %q: assistant message was dropped entirely (%d messages) — its tool_result would be orphaned and 400 on its own", input, len(msgs))
		}
		b, err := json.Marshal(msgs[0])
		if err != nil {
			t.Fatalf("input %q: marshal: %v", input, err)
		}
		wire := string(b)
		if !strings.Contains(wire, `"input":{}`) {
			t.Fatalf("input %q: want \"input\":{} on the wire, got: %s", input, wire)
		}
	}
}
