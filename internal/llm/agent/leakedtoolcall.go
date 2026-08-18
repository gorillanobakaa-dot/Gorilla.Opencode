// GORILLA OVERRIDE (2026-08-18): recognise a tool call that arrived as TEXT.
//
// WHAT HAPPENS, AND WHY IT IS NOT THE MODEL MISBEHAVING
//
// Llama 3.x tool calling is a TEXT protocol. Meta's own prompt format documents
// the model emitting `<|python_tag|>{"type":"function","name":…,"parameters":…}`
// and the SERVING LAYER is expected to lift that out of the stream and return it
// as a structured tool call. Hosted stacks that detokenise with
// skip_special_tokens=True destroy the `<|python_tag|>` frame first — vLLM's own
// Hermes parser force-sets that flag to false precisely to avoid this — so the
// bare JSON falls through to us as ordinary assistant content and is printed to
// the user as the answer.
//
// MEASURED, and the measurement decided the design. Replaying the session
// database on 2026-08-18: 4 of 13 `local.meta/llama-3.3-70b-instruct` assistant
// turns (31%) were a leaked call as the whole content. Every single one was
// INVALID JSON — `"parameters": "query": "…"`, with the inner brace missing:
//
//	{"type": "function", "name": "web_search", "parameters": "query": "…"}
//
// That fact killed the obvious implementations. A detector requiring
// json.Unmarshal to succeed matches NONE of the real occurrences, so the shape
// test below is deliberately structural rather than a parse.
//
// WHY THIS ONLY LABELS, AND NEVER DISPATCHES
//
// Synthesising a tool call from model TEXT and executing it is exactly the
// fuzzy-output-to-dispatch path internal/llm/agent/toolname.go exists to forbid:
// "an attacker who can influence the model's output — via a poisoned README, a
// fetched web page, a crafted filename, a tool result — can pick which tool
// runs. That is remote code execution wearing a helpful hat." A malformed
// payload would additionally have to be REPAIRED before it could be dispatched,
// which is strictly worse.
//
// The asymmetry is the whole design: a label can afford a lenient detector,
// because a wrong label costs one sentence above text that is preserved
// verbatim. A dispatcher cannot, because every leniency is a bypass. Since we
// never execute, we never need the arguments — only the name, which survives
// the malformation.
package agent

import (
	"regexp"
	"strings"
)

// leakedCallRe matches the SHAPE of a tool call, not valid JSON: an object with a
// "name" and either "parameters" or "arguments". Anchored to the whole trimmed
// content, so a model discussing JSON mid-paragraph is never caught.
var leakedCallRe = regexp.MustCompile(
	`^\{\s*"(?:type|name)"\s*:.*"name"\s*:\s*"([A-Za-z0-9_.\-]+)".*"(?:parameters|arguments)"\s*:.*\}$`,
)

// nameOnlyRe covers the ordering where "name" precedes "type".
var nameOnlyRe = regexp.MustCompile(
	`^\{\s*"name"\s*:\s*"([A-Za-z0-9_.\-]+)".*"(?:parameters|arguments)"\s*:.*\}$`,
)

// LeakedToolCallName reports the tool name when content is ENTIRELY a
// tool-call-shaped payload, or "" otherwise.
//
// known is the set of tools actually registered this turn. Requiring the name to
// bind to a real tool is the gate that keeps false positives near zero: a model
// explaining JSON does not usually produce an object whose "name" happens to be
// a registered tool AND which constitutes the entire message.
func LeakedToolCallName(content string, known []string) string {
	trimmed := strings.TrimSpace(content)
	// Cheap rejections first: it must be a single JSON-ish object.
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return ""
	}
	// A leaked call is one line of payload. Multi-paragraph prose that merely
	// starts and ends with braces is not this.
	if strings.Count(trimmed, "\n") > 4 {
		return ""
	}

	var name string
	if m := leakedCallRe.FindStringSubmatch(trimmed); m != nil {
		name = m[1]
	} else if m := nameOnlyRe.FindStringSubmatch(trimmed); m != nil {
		name = m[1]
	}
	if name == "" {
		return ""
	}

	for _, k := range known {
		if k == name {
			return name
		}
	}
	return ""
}

// LeakedToolCallNotice is what the user is told. It explains the cause, because
// "the model returned JSON" is useless on its own and the fix — change model, or
// use one whose server parses its tool calls — is not guessable.
func LeakedToolCallNotice(name string) string {
	return "This model tried to call the " + name + " tool but wrote the call as plain " +
		"text instead of making a real tool call, so nothing was run. That is a fault in " +
		"the model or the service serving it, not in your request — Llama-family models " +
		"emit tool calls as text and the provider is supposed to convert them. Ask again, " +
		"or switch model with /models."
}
