// GORILLA (2026-09-02): deferred tool loading, and the search tool that finds them.
//
// THE PROBLEM. Every tool's JSON schema is sent on EVERY turn. Gorilla's
// fourteen tools cost 9,733 tokens before the user has typed anything, and the
// bill repeats for the life of the conversation. Adding tools makes the agent
// more capable and every single turn more expensive, which is a bad trade to be
// forced into: the tools worth having are usually the specialised ones nobody
// needs twice in a row.
//
// THE MECHANISM. Most tools are DEFERRED: the model is told they exist, in one
// line each, but their schemas are withheld. One tool is never deferred --
// tool_search -- and when the model finds it needs a capability it has not got,
// it searches, the schema is loaded, and it stays loaded for the rest of the
// session.
//
// WHY THIS IS CLIENT-SIDE, which is the part that differs from Anthropic's API.
//
// The Claude API implements this SERVER-side: you send every schema on every
// request and set defer_loading, and the API decides what enters the model's
// context. That is elegant and it is unavailable to most of this program's
// users, because Gorilla talks to llama.cpp, LM Studio, Ollama and any
// OpenAI-compatible endpoint, none of which have the feature. Deferring there
// means genuinely NOT SENDING the schema -- the saving has to happen on this
// side of the wire.
//
// So the filter sits above every provider, on the []BaseTool slice the agent
// hands over, and works identically for Anthropic, OpenAI, Gemini and a local
// GGUF. One implementation, five providers, no provider-specific code.
//
// WHAT THIS COSTS, stated plainly because it is not free. Discovery is a round
// trip: the model searches, gets an answer, then calls the tool. On a local
// model at 40 tokens/second that is real seconds, and a small model may not
// reliably notice it lacks a capability at all. That is why the everyday tools
// -- bash, edit, view, write, find -- are NEVER deferred. A model that never
// searches can still do the whole job; it just cannot review code or port a
// patch until it asks.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const ToolSearchToolName = "tool_search"

// minSearchScore is the relevance floor below which a match is treated as no
// match. See the measurement in searchCatalogue.
const minSearchScore = 1.2

// deferredByDefault names the tools withheld until asked for.
//
// The split is by FREQUENCY, not by importance. Everything needed to read,
// write and run code stays loaded, because that is most turns and because a
// model that has to search before it can edit a file is worse than one that
// costs more. What is deferred is specialised: valuable when wanted, dead
// weight the rest of the time.
var deferredByDefault = map[string]bool{
	ReviewToolName:    true, // 759 tokens, used on demand
	PatchPortToolName: true, // 734, kernel and Firefox work only
	BioDataToolName:   true, // 506, a minority of users ever
	SparseToolName:    true, // 340, kernel only
	WebSearchToolName: true, // 846
	FetchToolName:     true, // 789
	// The sub-agent tool. Its constant lives in the agent package, which
	// imports this one, so the name is written out rather than imported --
	// the alternative is an import cycle. Guarded by a test that fails if the
	// two ever disagree.
	"agent": true,
}

// NeverDeferred are the tools that must always be present, whatever else
// happens. tool_search is here for the obvious reason -- deferring the thing
// that undefers everything else leaves no way back in. The Claude API rejects
// that configuration outright with "At least one tool must have
// defer_loading=false", and the same reasoning applies to a local model that
// would simply have nothing to call.
var NeverDeferred = map[string]bool{
	ToolSearchToolName: true,
}

// ── discovery state, per session ────────────────────────────────────────────
//
// Session-scoped rather than global: two conversations should not silently
// inherit each other's discoveries, and a fresh session should start cheap
// again. Keyed by session id, cleared when a session is cleared.

var (
	discoveredMu sync.RWMutex
	discovered   = map[string]map[string]bool{}
)

// MarkDiscovered records that a session may now use a tool.
func MarkDiscovered(sessionID string, names ...string) {
	if sessionID == "" {
		return
	}
	discoveredMu.Lock()
	defer discoveredMu.Unlock()
	set := discovered[sessionID]
	if set == nil {
		set = map[string]bool{}
		discovered[sessionID] = set
	}
	for _, n := range names {
		set[n] = true
	}
}

// IsDiscovered reports whether a session has already loaded a tool.
func IsDiscovered(sessionID, name string) bool {
	discoveredMu.RLock()
	defer discoveredMu.RUnlock()
	return discovered[sessionID][name]
}

// DiscoveredCount is what a status line or a test can ask for.
func DiscoveredCount(sessionID string) int {
	discoveredMu.RLock()
	defer discoveredMu.RUnlock()
	return len(discovered[sessionID])
}

// ForgetSession drops a session's discoveries. Called when a conversation is
// cleared, so /clear really does start from the cheap state again.
func ForgetSession(sessionID string) {
	discoveredMu.Lock()
	delete(discovered, sessionID)
	discoveredMu.Unlock()
}

// IsDeferrable reports whether a tool is withheld by default.
func IsDeferrable(name string) bool {
	if NeverDeferred[name] {
		return false
	}
	return deferredByDefault[name]
}

// VisibleTools filters a toolset down to what this session should actually be
// sent: everything non-deferred, plus whatever it has discovered.
//
// This is the whole saving. It runs above the providers, so the schemas of
// undiscovered tools are never serialised for any of them.
// The ORDER is not incidental. Stable tools come first, in their original
// order, and anything discovered is APPENDED after them.
//
// Tool definitions are part of the cached prompt prefix on every provider that
// caches one — Anthropic explicitly, OpenAI and Gemini implicitly. Returning
// the discovered tool in its original position would insert it in the MIDDLE,
// shifting every definition after it and invalidating the cache from that point
// on every single discovery.
//
// That would have been a bad trade and an invisible one: 4,298 tokens saved per
// turn, paid for by re-reading the entire prefix at full price whenever the
// model searched. Worst for exactly the people this helps most — someone on a
// free tier, where a cache miss is not a rounding error.
//
// Appending keeps every byte before the last stable tool identical from turn to
// turn, so the cached prefix survives a discovery. This is the same shape as
// Anthropic's server-side design, whose docs say the prefix is untouched
// precisely so that caching is preserved.
func VisibleTools(all []BaseTool, sessionID string, enabled bool) []BaseTool {
	if !enabled {
		return all
	}
	stable := make([]BaseTool, 0, len(all))
	found := make([]BaseTool, 0, 4)
	for _, t := range all {
		name := t.Info().Name
		switch {
		case !IsDeferrable(name):
			stable = append(stable, t)
		case IsDiscovered(sessionID, name):
			found = append(found, t)
		}
	}
	return append(stable, found...)
}

// StableToolCount reports how many of a visible set are the always-present
// ones. The prompt-cache breakpoint belongs on the last of these: everything
// after it can change mid-conversation, and a breakpoint there would be
// re-cached on every discovery.
func StableToolCount(visible []BaseTool) int {
	n := 0
	for _, t := range visible {
		if IsDeferrable(t.Info().Name) {
			break
		}
		n++
	}
	return n
}

// ── the search tool ─────────────────────────────────────────────────────────

type ToolSearchParams struct {
	// Query is what to look for. Two forms:
	//   "select:review,patch_port"  exact names
	//   "port a patch to a kernel"  keywords, scored against name+description
	Query string `json:"query"`
	// MaxResults caps how many tools are loaded at once. Default 5.
	MaxResults int `json:"max_results"`
}

type toolSearchTool struct {
	// catalogue is every tool that could be discovered, injected rather than
	// looked up globally so the tool is testable without a running agent.
	catalogue func() []BaseTool
}

func NewToolSearchTool(catalogue func() []BaseTool) BaseTool {
	return &toolSearchTool{catalogue: catalogue}
}

func (t *toolSearchTool) Info() ToolInfo {
	return ToolInfo{
		Name: ToolSearchToolName,
		Description: `Load a tool you do not currently have.

Most of this agent's tools are not loaded up front, to keep every turn cheap. You can see their names and one-line summaries below, but NOT their parameters — you cannot call one until you load it here.

Call this the moment you realise you need a capability you have not got. Do not apologise for not having a tool, and do not tell the user a tool is unavailable: search for it first. It will be loaded and stay loaded for the rest of this conversation.

QUERY FORMS
  select:review               load exactly this tool
  select:review,patch_port    load several by name
  port a patch onto a kernel  keywords; the closest matches are loaded

After this returns, call the loaded tool normally. If nothing matches, the capability genuinely does not exist here — say so then, and not before.`,
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Tool names as \"select:a,b\", or keywords describing what you need to do.",
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "How many tools to load (default 5).",
			},
		},
		Required: []string{"query"},
	}
}

func (t *toolSearchTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ToolSearchParams
	if call.Input != "" {
		if err := UnmarshalToolInput(call.Input, &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("could not read the parameters: %s", err)), nil
		}
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return NewTextErrorResponse("query is empty. Use \"select:name\" or describe what you need."), nil
	}
	limit := params.MaxResults
	if limit <= 0 {
		limit = 5
	}
	if limit > 25 {
		limit = 25
	}

	sessionID, _ := GetContextValues(ctx)

	var candidates []BaseTool
	for _, c := range t.catalogue() {
		if IsDeferrable(c.Info().Name) {
			candidates = append(candidates, c)
		}
	}

	matches := searchCatalogue(candidates, query, limit)
	if len(matches) == 0 {
		return NewTextResponse(fmt.Sprintf(
			"Nothing matched %q.\n\nTools that can be loaded here:\n%s\n\n"+
				"If none of these do what you need, that capability does not exist in this "+
				"agent — say so plainly rather than describing what you would do if it did.",
			query, catalogueListing(candidates))), nil
	}

	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Info().Name)
	}
	MarkDiscovered(sessionID, names...)

	var b strings.Builder
	fmt.Fprintf(&b, "Loaded %d tool(s): %s\n\n", len(names), strings.Join(names, ", "))
	b.WriteString("Their full definitions are now available and will stay available for the " +
		"rest of this conversation. Call them normally.\n\n")
	for _, m := range matches {
		info := m.Info()
		fmt.Fprintf(&b, "── %s ──\n%s\n", info.Name, firstParagraph(info.Description))
		if len(info.Required) > 0 {
			fmt.Fprintf(&b, "required: %s\n", strings.Join(info.Required, ", "))
		}
		b.WriteString("\n")
	}
	return NewTextResponse(b.String()), nil
}

func catalogueListing(candidates []BaseTool) string {
	var b strings.Builder
	for _, c := range candidates {
		info := c.Info()
		fmt.Fprintf(&b, "  %-14s %s\n", info.Name, oneLineOf(firstParagraph(info.Description), 90))
	}
	return b.String()
}

// firstParagraph is the summary line, so a listing does not carry a whole
// description and undo the saving it exists to make.
func firstParagraph(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n\n"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	if i := strings.Index(s, "\n"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// searchCatalogue scores tools against a query.
//
// Two forms. "select:a,b" is exact and wins outright -- a model that knows the
// name should not have its request re-interpreted. Otherwise it is a small BM25
// -flavoured overlap score across name, description and parameter names, which
// is what the Claude API's bm25 variant searches too.
func searchCatalogue(candidates []BaseTool, query string, limit int) []BaseTool {
	if rest, ok := cutPrefixFold(query, "select:"); ok {
		var out []BaseTool
		for _, want := range strings.Split(rest, ",") {
			want = strings.TrimSpace(want)
			for _, c := range candidates {
				if strings.EqualFold(c.Info().Name, want) {
					out = append(out, c)
				}
			}
		}
		return out
	}

	terms := tokenise(query)
	if len(terms) == 0 {
		return nil
	}
	type scored struct {
		tool  BaseTool
		score float64
	}
	var ranked []scored
	for _, c := range candidates {
		info := c.Info()
		hay := tokenise(info.Name + " " + info.Description + " " + paramWords(info))
		if len(hay) == 0 {
			continue
		}
		counts := map[string]int{}
		for _, w := range hay {
			counts[w]++
		}
		var score float64
		for _, term := range terms {
			if n := counts[term]; n > 0 {
				// Saturating, so one word repeated forty times in a long
				// description cannot outrank a tool that matches three terms.
				score += float64(n) / (float64(n) + 1.5)
			}
			// A term appearing in the NAME is worth more than in the prose.
			if strings.Contains(strings.ToLower(info.Name), term) {
				score += 2
			}
		}
		// MEASURED 2026-09-02, not guessed. Scored every tool against seven
		// queries: deliberate nonsense topped out at 0.97, while the correct
		// tool for a real request scored between 1.39 and 7.82. 1.2 sits in
		// that gap.
		//
		// Without a floor, "nothing like this exists" loaded three tools --
		// common words in long descriptions add up. That is worse than finding
		// nothing: it spends the tokens deferral was meant to save AND hands
		// the model tools that do not do what it wants. It also trims the tail
		// on good queries, so a search for patch porting stops dragging in the
		// two tools that merely mention patches.
		if score >= minSearchScore {
			ranked = append(ranked, scored{c, score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].tool.Info().Name < ranked[j].tool.Info().Name
	})
	out := make([]BaseTool, 0, limit)
	for _, r := range ranked {
		if len(out) >= limit {
			break
		}
		out = append(out, r.tool)
	}
	return out
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return "", false
}

func paramWords(info ToolInfo) string {
	if len(info.Parameters) == 0 {
		return ""
	}
	var b strings.Builder
	keys := make([]string, 0, len(info.Parameters))
	for k := range info.Parameters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(" ")
		// Parameter descriptions are searched too, as the API's variants do.
		if m, ok := info.Parameters[k].(map[string]any); ok {
			if d, ok := m["description"].(string); ok {
				b.WriteString(d)
				b.WriteString(" ")
			}
		}
	}
	return b.String()
}

var searchStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true, "for": true,
	"in": true, "on": true, "and": true, "or": true, "is": true, "it": true,
	"i": true, "need": true, "want": true, "how": true, "do": true, "can": true,
	"with": true, "that": true, "this": true, "my": true, "me": true,
}

func tokenise(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		w := strings.ToLower(cur.String())
		cur.Reset()
		if len(w) < 2 || searchStopWords[w] {
			return
		}
		out = append(out, w)
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// DeferredCatalogueBlock is the one-line-per-tool summary that goes into the
// system prompt, so the model knows what CAN be loaded without carrying the
// schemas. Returns "" when nothing is deferred.
//
// This is the honest cost of the mechanism: a few hundred tokens of index in
// exchange for several thousand of withheld schema. Without it the model has no
// idea what to search for, and either never searches or searches blindly.
func DeferredCatalogueBlock(all []BaseTool) string {
	var deferred []BaseTool
	for _, t := range all {
		if IsDeferrable(t.Info().Name) {
			deferred = append(deferred, t)
		}
	}
	if len(deferred) == 0 {
		return ""
	}
	sort.Slice(deferred, func(i, j int) bool {
		return deferred[i].Info().Name < deferred[j].Info().Name
	})
	var b strings.Builder
	b.WriteString("TOOLS YOU CAN LOAD BUT DO NOT HAVE YET\n\n")
	b.WriteString("These exist. Their parameters are not loaded, so you cannot call them " +
		"directly — use the " + ToolSearchToolName + " tool first and they become available " +
		"for the rest of this conversation.\n\n")
	for _, t := range deferred {
		info := t.Info()
		fmt.Fprintf(&b, "  %-14s %s\n", info.Name, oneLineOf(firstParagraph(info.Description), 100))
	}
	b.WriteString("\nNever tell the user a capability is missing without searching for it first.\n")
	return b.String()
}

// toolReferenceBlocks renders discoveries in the Claude API's client-side
// custom-search format, for the day the Anthropic provider wants to hand the
// expansion back to the server. Unused by the local path, which does its own
// filtering, but kept next to the code it mirrors so the two cannot drift.
func toolReferenceBlocks(names []string) string {
	type ref struct {
		Type string `json:"type"`
		Name string `json:"tool_name"`
	}
	refs := make([]ref, 0, len(names))
	for _, n := range names {
		refs = append(refs, ref{"tool_reference", n})
	}
	b, _ := json.Marshal(refs)
	return string(b)
}
