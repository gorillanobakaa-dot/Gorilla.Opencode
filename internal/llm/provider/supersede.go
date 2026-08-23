package provider

import (
	"fmt"

	"github.com/opencode-ai/opencode/internal/message"
)

// GORILLA OVERRIDE (2026-08-23): collapse the content of a tool result once a
// LATER call with byte-identical arguments has run in the same session.
//
// WHY. Measured 2026-08-23: tool RESULTS, not tool schemas, are the context
// cost curve. Every result joins the conversation and is re-uploaded on every
// later turn for the rest of the session. A `find .` run at 12:52 was still
// being paid for at 13:07, and the same session ran an identical `find .` twice
// and paid for both. Over seven prompts the context went 10K -> 58K, almost all
// of it accumulated results.
//
// WHAT IS SAFE TO COLLAPSE. Only read-only tools whose output is fully
// determined by their arguments (`view`, `find`). For those, an identical later
// call returns equivalent information, so the earlier copy is dead weight and
// the newest copy is authoritative. bash/edit/write/patch are excluded on
// purpose: their output is side-effectful or non-reproducible, and the model
// may need the exact earlier text.
//
// WHAT IS PRESERVED. The tool_call/tool_result pairing is untouched — an
// orphaned tool_result is a hard API error (see anthropic.go). Only the
// redundant Content string is replaced with a one-line stub. Nothing is deleted
// from the session store; this shapes only what goes on the wire, so the full
// transcript stays on disk and greppable (the never-delete rule holds).
var supersedableReadTools = map[string]bool{
	"view": true,
	"find": true,
}

// supersedeMinContent is the smallest result worth stubbing. The stub itself is
// ~160 chars; collapsing anything near that size saves nothing and adds churn.
const supersedeMinContent = 400

const supersededStubFmt = "[superseded: an identical %s call ran later in this " +
	"session and its result appears below; this earlier copy is omitted to save " +
	"context. The full output remains in the session store on disk.]"

// supersedeStaleReads returns a view of messages in which every read-only tool
// result that has a later identical-argument twin has had its Content replaced
// by a stub. The input slice and the stored message Parts are never mutated:
// only messages that actually change are rebuilt with a fresh Parts slice.
func supersedeStaleReads(messages []message.Message) []message.Message {
	// Map each read-only tool_call id to a key of (name + raw arguments).
	// Identical arguments to the same read-only tool => identical key.
	callKey := make(map[string]string)
	for mi := range messages {
		for _, tc := range messages[mi].ToolCalls() {
			if supersedableReadTools[tc.Name] {
				callKey[tc.ID] = tc.Name + "\x00" + tc.Input
			}
		}
	}
	if len(callKey) == 0 {
		return messages
	}

	// For each key, remember the position of the NEWEST result carrying it.
	type pos struct{ msg, part int }
	newest := make(map[string]pos)
	for mi := range messages {
		for pi, part := range messages[mi].Parts {
			if tr, ok := part.(message.ToolResult); ok {
				if key, tracked := callKey[tr.ToolCallID]; tracked {
					newest[key] = pos{mi, pi}
				}
			}
		}
	}

	out := make([]message.Message, len(messages))
	copy(out, messages)
	for mi := range out {
		var cloned []message.ContentPart
		for pi, part := range out[mi].Parts {
			tr, ok := part.(message.ToolResult)
			if !ok {
				continue
			}
			key, tracked := callKey[tr.ToolCallID]
			if !tracked || tr.IsError || len(tr.Content) < supersedeMinContent {
				continue
			}
			if n := newest[key]; n.msg == mi && n.part == pi {
				continue // the authoritative newest copy — keep it in full
			}
			if cloned == nil {
				cloned = make([]message.ContentPart, len(out[mi].Parts))
				copy(cloned, out[mi].Parts)
			}
			tr.Content = fmt.Sprintf(supersededStubFmt, tr.Name)
			cloned[pi] = tr
		}
		if cloned != nil {
			out[mi].Parts = cloned
		}
	}
	return out
}
