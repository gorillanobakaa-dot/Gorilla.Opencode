package agent

import (
	"regexp"

	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// GORILLA OVERRIDE: tool-name normalisation, and the rules it must NEVER break.
//
// ─────────────────────────────────────────────────────────────────────────────
//  READ THIS BEFORE YOU TOUCH THIS FILE. THEN READ IT AGAIN.
//
//  DO NOT MAKE THIS FUZZY. NOT "close enough". NOT "starts with". NOT
//  "did you mean". NOT Levenshtein. NOT case-insensitive. NOT "strip anything
//  that isn't a letter". NOT a fallback to "the only tool that looks similar".
//
//  Every one of those turns a TYPO-FIX into a PRIVILEGE-ESCALATION PRIMITIVE.
//  The moment `bash_readonly`, `bash-safe` or `bashx` can resolve to `bash`,
//  an attacker who can influence the model's output — via a poisoned README, a
//  fetched web page, a crafted filename, a tool result — can pick which tool
//  runs. That is remote code execution wearing a helpful hat.
//
//  There is already a corpse in this codebase proving someone tried: the
//  commented-out `strings.HasPrefix` "monkey patch for Copilot Sonnet-4 tool
//  repetition obfuscation" in agent.go. Leave it dead.
// ─────────────────────────────────────────────────────────────────────────────
//
// WHY THIS EXISTS AT ALL
//
// Measured 2026-08-14: a research run of 10 helpers on local.meta/muse-glimmer-30b
// returned nothing. The database showed 30 of 44 tool calls failing with
// "Tool not found: ls<|message|>", "view<|message|>", "glob<|message|>",
// "grep<|message|>". The model was appending its own chat-template control
// token to the FUNCTION NAME. Every lane was handed a locked toolbox and asked
// to investigate.
//
// WHY THIS IS NOT A SECURITY RELAXATION
//
// It grants NO capability that did not already exist. A model that wants to run
// bash types `bash`, and that has always worked. Stripping a control token only
// rescues a model that is bad at spelling its own tool names — and an attacker
// steering a model would spell it CORRECTLY, because misspelling is the thing
// that makes their attack fail. This fixes a stutter. It does not touch a lock.
//
// THE FOUR RULES. All four are enforced by toolname_test.go. If you change the
// behaviour here, those tests must fail — if they still pass, they are no
// longer testing anything and you have a bigger problem than this file.
//
//	1. STRIP ONE SHAPE ONLY: a trailing <|...|> control token, and surrounding
//	   whitespace. Nothing else is removed, ever.
//	2. THE RESULT MUST BE A PLAIN IDENTIFIER: ^[a-zA-Z0-9_-]+$. If cleaning
//	   leaves anything else — a path, a quote, a shell metacharacter, an
//	   embedded token, an empty string — REFUSE. Do not "clean harder".
//	3. THEN EXACT MATCH, against THAT AGENT'S OWN TOOL LIST. Never nearest,
//	   never prefix, never a default.
//	4. IF IT STILL DOES NOT MATCH, REFUSE AND SAY BOTH NAMES. A tool call that
//	   cannot be resolved is an error, never a guess.
//
// WHAT KEEPS THIS SAFE EVEN IF SOMEONE IGNORES ALL OF THE ABOVE — defence in
// depth, and neither layer may be removed either:
//
//   - PERMISSIONS ARE ASKED BY THE TOOL, NOT BY THE CALLER. Every tool passes
//     its OWN constant (BashToolName, EditToolName…) into permission.Request.
//     A tool cannot be smuggled past a deny-list under an alias, because it
//     fills in its own name badge AFTER dispatch has already resolved it.
//     TestPermissionRequestsUseTheToolsOwnConstantName holds that.
//   - TOOL LISTS ARE PER-AGENT. A research helper's entire toolbox is
//     fetch/web_search/find/view/diagnostics — no bash, no edit, no
//     write (ResearchAgentTools). No spelling of any name reaches a tool that
//     is not in the calling agent's own list.

// controlTokenSuffix matches ONE trailing chat-template control token, e.g.
// "<|message|>", "<|end|>", "<|im_end|>", plus any whitespace around it.
//
// Anchored to the END on purpose. An EMBEDDED token ("ba<|x|>sh") is left
// untouched, fails rule 2, and is refused — exactly as it should be, because a
// name with a token in the middle is not a stutter, it is someone probing.
var controlTokenSuffix = regexp.MustCompile(`\s*<\|[^<>|]*\|>\s*$`)

// plainIdentifier is what a real tool name looks like. Rule 2.
var plainIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// maxStripPasses bounds the cleaning loop. A name carrying more than a couple
// of trailing tokens is not a stutter; refusing is correct and cheap.
const maxStripPasses = 3

// normaliseToolName removes trailing chat-template control tokens from a tool
// name emitted by a model.
//
// Returns the cleaned name and whether it is SAFE TO LOOK UP. ok=false means
// refuse the call — it does NOT mean "try something else".
//
// This function never returns a name that differs from its input by anything
// other than removed trailing control tokens and whitespace. It cannot invent,
// substitute, complete or approximate a name. That is the whole point.
// Caught by TestNoPrefixOrFuzzyNameEverResolves during development: an earlier
// version called strings.TrimSpace on the raw name FIRST, so "bash " and
// " bash" resolved to bash even with no control token anywhere. Harmless in
// isolation, but it is a second repair nobody asked for and nobody measured,
// and "one shape only" has to mean one shape or it is not a rule.
//
// So: if there is no trailing control token, this does NOTHING. Whitespace is
// only ever eaten as part of removing that token — see the \s* on both sides of
// the pattern. A name that is merely padded is refused, because a padded name
// has never been observed and we do not repair hypotheticals.
func normaliseToolName(raw string) (cleaned string, changed bool, ok bool) {
	if !controlTokenSuffix.MatchString(raw) {
		return raw, false, false
	}
	cleaned = raw
	for i := 0; i < maxStripPasses && controlTokenSuffix.MatchString(cleaned); i++ {
		cleaned = controlTokenSuffix.ReplaceAllString(cleaned, "")
	}
	changed = cleaned != raw

	// Rule 2: whatever is left must be a plain identifier, or we refuse.
	// An empty result, a leftover token, a path, a quote, leading space — all
	// refused. We do not "clean harder" to rescue them.
	if !plainIdentifier.MatchString(cleaned) {
		return raw, changed, false
	}
	return cleaned, changed, true
}

// findTool resolves a model-supplied tool name against the tools THIS agent
// actually has. Exact match first; then exactly one cleaning pass; then exact
// match again. There is deliberately no third attempt and no fallback.
//
// DO NOT ADD A FALLBACK. See the rules at the top of this file.
func findTool(available []toolNamer, raw string) (idx int, usedName string, cleaned bool) {
	for i, t := range available {
		if t.ToolName() == raw {
			return i, raw, false
		}
	}
	name, changed, ok := normaliseToolName(raw)
	if !ok || !changed {
		return -1, raw, false
	}
	for i, t := range available {
		if t.ToolName() == name {
			return i, name, true
		}
	}
	return -1, raw, false
}

// retiredToolHint returns extra guidance for the not-found ERROR when a model
// asks for a search/list tool by a name other harnesses use. glob, grep and ls
// were retired in favour of the find tool, and models are trained on harnesses
// that have them — so the refusal teaches instead of just refusing.
//
// This is NOT a resolution mechanism and must never become one: the call still
// fails, exactly as rule 4 requires. The comparison is EXACT string equality
// against a fixed list of retired/foreign names (after the one permitted
// control-token strip); nothing here routes a call to a tool. A model that
// reads the error retries with find on its own.
func retiredToolHint(raw string) string {
	name := raw
	if n, _, ok := normaliseToolName(raw); ok {
		name = n
	}
	switch name {
	case "glob", "grep", "ls", "list", "list_dir", "list_directory", "dir",
		"search", "search_files", "file_search", "find_files", "rg", "ripgrep":
		return ` Searching file contents, finding files by name, and listing directories are ALL done by the "find" tool here: find(query="text to search", glob="*.ext", path="directory"). Retry using find.`
	}
	return ""
}

// toolNamer is the minimum findTool needs, so it can be tested without
// constructing real tools.
type toolNamer interface{ ToolName() string }

// toolInfoNamer adapts a real tool to toolNamer.
type toolInfoNamer struct{ t tools.BaseTool }

func (n toolInfoNamer) ToolName() string { return n.t.Info().Name }
