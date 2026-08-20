// GORILLA OVERRIDE: this file did not exist upstream. It splits the coder prompt
// into its "# " sections so each can be switched off individually in /context.
//
// Honest scoping, because this is easy to oversell: turning a section off saves
// HUNDREDS of tokens at most, not thousands.
//
// CORRECTED 2026-08-20 - the previous note said "~460 tokens across 8 sections"
// and was out of date by more than 4x. Measured from coder-modern.txt: 12
// sections, ~2,000 tokens total. Largest are tools (~469), change reporting
// (~207), honesty (~173), precedence (~160), output (~154). It matters because
// this is the exact comment someone reads while deciding what to switch off on
// a metered link, and a 4x understatement points them at the wrong lever.
//
// AND THE OUTPUT SECTION IS A TRAP: switching "# output" off saves ~154 tokens
// of upload and removes the instruction to keep replies SHORT. At a measured
// 377 bytes per output token, fifty extra tokens of reply costs ~18,850 bytes -
// about 30x what was saved. On a slow link it is the most valuable section in
// the prompt. Leave it on. The real bandwidth wins already shipped — prompt.env was 10k-30k
// before the shallow-summary refactor, and tool schemas are 200-850 each.
//
// The value here is BEHAVIOURAL control, not bandwidth: drop "# build discipline"
// on a non-build task, or "# output" when you want prose instead of terse replies.
package prompt

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
)

// Section is one "# heading" block of a prompt.
type Section struct {
	ID     string // "prompt.section.<slug>"
	Header string // "build discipline"; empty for the leading preamble
	Body   string // the section's lines, header included
	Tokens int    // len/4 estimate, replaced by calibration where possible
}

// sectionHeaderRe matches a markdown-ish section header at line start.
var sectionHeaderRe = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// GORILLA OVERRIDE: per-line tool gating.
//
// A prompt line may end with [[needs tool.fetch]] (or several ids, space
// separated). The line is kept only if every id is enabled in the loadout, and
// the marker is always stripped before the prompt is sent.
//
// Why this exists: sections are gated by SECTION, tools by TOOL, and nothing
// connected the two. Turning a tool off removed its schema and left the prompt
// still describing it, so the model was told it had a capability it did not
// have. The web_fetch line was the worst case - it says "never say you cannot
// reach a page", which at that moment is an instruction to fabricate, in the
// one codebase whose search tool exists because a cornered model fabricated
// citations on 2026-08-07. Low-bandwidth mode is a single keypress in /context
// and turns off tool.fetch, tool.websearch, tool.patch, tool.diagnostics and
// tool.agent at once, so this was reachable by accident, not only by an expert
// pruning the loadout deliberately.
//
// Line-level markers rather than more sections: /context lists one row per
// section with a "what you lose" line, and splitting these out would add
// one-line rows that duplicate the tool toggles already sitting in the same
// menu. The marker keeps the dependency next to the sentence it governs, where
// someone editing the prompt will see it.
var needsMarkerRe = regexp.MustCompile(`\s*\[\[needs\s+([^\]]+)\]\]`)

// applyToolGates drops the lines whose required loadout ids are off and strips
// the markers from the lines it keeps. Text with no markers is returned as-is,
// so a user's hand-edited prompt is unaffected.
func applyToolGates(body string) string {
	if !strings.Contains(body, "[[needs") {
		return body
	}
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		m := needsMarkerRe.FindStringSubmatch(line)
		if m == nil {
			kept = append(kept, line)
			continue
		}
		enabled := true
		for _, id := range strings.Fields(m[1]) {
			if !config.LoadoutEnabled(id) {
				enabled = false
				break
			}
		}
		if enabled {
			kept = append(kept, needsMarkerRe.ReplaceAllString(line, ""))
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// bodyIsOnlyHeader reports whether gating emptied a section down to its "# foo"
// line. A bare header is noise in the prompt and, worse, implies the section's
// content was meant to be there, so the whole section is dropped instead.
func bodyIsOnlyHeader(body string) bool {
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "#") {
			return false
		}
	}
	return true
}

const (
	sectionIDPrefix = "prompt.section."
	preambleSlug    = "preamble"
)

// SectionID builds the loadout id for a section slug. One function so the
// registration and the gate cannot drift.
func SectionID(slug string) string { return sectionIDPrefix + slug }

// slugify turns "build discipline" into "build-discipline".
func slugify(header string) string {
	s := strings.ToLower(strings.TrimSpace(header))
	s = strings.NewReplacer(" ", "-", "/", "-", "_", "-").Replace(s)
	return strings.Trim(s, "-")
}

// ParseSections splits text on "# " headers. Text before the first header is the
// preamble and gets its own section, because it carries the agent's identity and
// is the one part that should almost never be switched off.
func ParseSections(text string) []Section {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	locs := sectionHeaderRe.FindAllStringSubmatchIndex(text, -1)
	if len(locs) == 0 {
		// No headers at all — one implicit section, so a user who rewrites the
		// prompt as flat prose still gets a toggle rather than nothing.
		return []Section{{
			ID:     SectionID(preambleSlug),
			Body:   text,
			Tokens: len(text) / 4,
		}}
	}

	var out []Section

	if pre := strings.TrimSpace(text[:locs[0][0]]); pre != "" {
		out = append(out, Section{
			ID:     SectionID(preambleSlug),
			Body:   pre,
			Tokens: len(pre) / 4,
		})
	}

	for i, loc := range locs {
		start := loc[0]
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		header := strings.TrimSpace(text[loc[2]:loc[3]])
		body := strings.TrimSpace(text[start:end])
		slug := slugify(header)
		if slug == "" {
			slug = fmt.Sprintf("section-%d", i+1)
		}
		out = append(out, Section{
			ID:     SectionID(slug),
			Header: header,
			Body:   body,
			Tokens: len(body) / 4,
		})
	}
	return out
}

// Sections of the ACTIVE coder prompt, cached. Derived from the active text
// rather than the factory copy, so a user who edits the prompt and adds a
// section gets a toggle for it automatically.
var (
	sectionsMu    sync.RWMutex
	sectionsCache []Section
	sectionsRead  bool
)

func invalidateSections() {
	sectionsMu.Lock()
	sectionsCache = nil
	sectionsRead = false
	sectionsMu.Unlock()
}

// CoderSections returns the sections of the coder prompt currently in use.
func CoderSections() []Section {
	sectionsMu.RLock()
	if sectionsRead {
		defer sectionsMu.RUnlock()
		return sectionsCache
	}
	sectionsMu.RUnlock()

	sectionsMu.Lock()
	defer sectionsMu.Unlock()
	if sectionsRead {
		return sectionsCache
	}
	sectionsCache = ParseSections(Text(PromptCoder))
	sectionsRead = true
	return sectionsCache
}

// assembleCoderPrompt joins the sections the loadout leaves enabled.
//
// If every section is off, fall back to the whole factory prompt. Turning
// everything off is a lobotomy rather than a configuration, and an empty system
// prompt fails silently — the agent just gets worse with no error to explain it.
func assembleCoderPrompt() string {
	secs := CoderSections()
	if len(secs) == 0 {
		return applyToolGates(Factory(PromptCoder))
	}
	var keep []string
	for _, s := range secs {
		if !config.LoadoutEnabled(s.ID) {
			continue
		}
		// Gate individual lines on the tools they describe, then drop a section
		// that gating reduced to a bare header.
		body := applyToolGates(s.Body)
		if body == "" || bodyIsOnlyHeader(body) {
			continue
		}
		keep = append(keep, body)
	}
	if len(keep) == 0 {
		return applyToolGates(Factory(PromptCoder))
	}
	return strings.Join(keep, "\n\n")
}

// SectionTradeoff is the layman "what you lose" line for each shipped section.
// Written per section rather than generated, because "you lose the honesty
// rules" needs to say what that actually means.
var SectionTradeoff = map[string]string{
	SectionID(preambleSlug): "the AI is not told what it is or what it specialises in — expect generic answers",
	// GORILLA OVERRIDE: added 2026-08-06. This section does not add a rule; it
	// says which existing rule wins when two of them disagree. Switching it off
	// does not remove any instruction, so the honest tradeoff is about wasted
	// work rather than lost capability — measured once at ~10 re-derivations and
	// 33K input tokens in a single turn before the ordering existed.
	SectionID("precedence"):       "the AI loses the tie-breaker it uses when two of your rules disagree — it still follows them, but can spend a lot of a turn arguing with itself about which one applies",
	SectionID("method"):           "the AI may rewrite working code instead of making the smallest fix, and may not read files before editing them",
	SectionID("build-discipline"): "the AI may retry the same failed build repeatedly instead of stopping and telling you what is blocked",
	SectionID("verification"):     "the AI may report a job done without building or testing it first",
	SectionID("scope"):            "the AI may start changing code when you only asked a question, and may run restarts or deletions on a hunch",
	SectionID("delegation"):       "the AI stops handing independent work to helpers and does everything itself — slower on big jobs",
	SectionID("memory"):           "the AI stops treating your CLAUDE.md / OpenCode.md as authoritative, and will not offer to write down what it learned",
	// GORILLA OVERRIDE: this line used to say the AI "runs tool calls one at a
	// time instead of in parallel", which implied that leaving the section ON
	// bought parallelism. It never did — agent.go executes tool calls in a plain
	// sequential loop and the agent tool blocks on its helper. The real saving
	// from batching is round-trips to the model, not concurrency, so that is what
	// it now says. Measured 2026-07-31.
	SectionID("tools"):            "the AI stops batching independent tool calls into one turn — the same work, but more round-trips to the model, which is the slow part on a poor connection",
	SectionID("honesty"):          "the AI becomes MORE LIKELY to claim success it did not observe, and to invent file paths and flags",
	SectionID("change-reporting"): "the AI stops telling you what a change costs you before it makes it — no list of what stops working, and no note of which claims it did not verify",
	SectionID("output"):           "replies get longer and more padded, and the AI may add explanatory comments to your code",
	SectionID("conduct"):          "the AI may stop halfway to ask what to do next, and may take destructive actions without confirming",
}

// criticalSections are the ones whose loss materially degrades trustworthiness
// rather than merely style. The /context menu marks these with a warning.
var criticalSections = map[string]bool{
	SectionID(preambleSlug): true,
	SectionID("honesty"):    true,
}

// RegisterSectionComponents adds a /context row per section of the active coder
// prompt. Called from app startup; config cannot import prompt (prompt imports
// config), so the registration is pushed from the other direction.
func RegisterSectionComponents() {
	var comps []config.LoadoutComponent
	for _, s := range CoderSections() {
		name := "Prompt: " + s.Header
		if s.Header == "" {
			name = "Prompt: preamble (identity)"
		}
		tradeoff, ok := SectionTradeoff[s.ID]
		if !ok {
			// A user-added section we have no hand-written tradeoff for.
			tradeoff = "that part of your prompt is not sent"
		}
		comps = append(comps, config.LoadoutComponent{
			ID:       s.ID,
			Name:     name,
			Tradeoff: tradeoff,
			Tokens:   s.Tokens,
			Default:  true,
			Critical: criticalSections[s.ID],
		})
	}
	config.RegisterLoadoutComponents(comps)
}

// GatedLineTokens measures the prompt lines that carry [[needs <id>]], so a
// line-gated loadout row can report a MEASURED cost rather than a guess.
//
// GORILLA OVERRIDE (2026-08-19): rows gated per-SECTION already get measured
// by section. Rows gated per-LINE had nothing measuring them, so
// prompt.localtools would have shipped showing whatever number was typed into
// the registry — on the one screen whose entire purpose is not doing that.
// calibrate_test.go's sentinel caught it immediately.
func GatedLineTokens(id string) int {
	// The gates have already been applied and the markers stripped by the time
	// BaseCoderPrompt returns, so measure the RAW embedded source instead —
	// otherwise a row that is currently OFF would measure as costing nothing,
	// which is true and useless: the number has to say what turning it ON
	// would cost.
	raw := baseModernCoderPrompt
	total := 0
	for _, line := range strings.Split(raw, "\n") {
		m := needsMarkerRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, want := range strings.Fields(m[1]) {
			if want == id {
				total += len(needsMarkerRe.ReplaceAllString(line, "")) / 4
				break
			}
		}
	}
	return total
}
