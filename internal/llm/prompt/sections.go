// GORILLA OVERRIDE: this file did not exist upstream. It splits the coder prompt
// into its "# " sections so each can be switched off individually in /context.
//
// Honest scoping, because this is easy to oversell: the whole base prompt is
// ~460 tokens across 8 sections. Turning one off saves TENS of tokens, not
// thousands. The real bandwidth wins already shipped — prompt.env was 10k-30k
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
		return Factory(PromptCoder)
	}
	var keep []string
	for _, s := range secs {
		if config.LoadoutEnabled(s.ID) {
			keep = append(keep, s.Body)
		}
	}
	if len(keep) == 0 {
		return Factory(PromptCoder)
	}
	return strings.Join(keep, "\n\n")
}

// SectionTradeoff is the layman "what you lose" line for each shipped section.
// Written per section rather than generated, because "you lose the honesty
// rules" needs to say what that actually means.
var SectionTradeoff = map[string]string{
	SectionID(preambleSlug):       "the AI is not told what it is or what it specialises in — expect generic answers",
	SectionID("method"):           "the AI may rewrite working code instead of making the smallest fix, and may not read files before editing them",
	SectionID("build-discipline"): "the AI may retry the same failed build repeatedly instead of stopping and telling you what is blocked",
	SectionID("verification"):     "the AI may report a job done without building or testing it first",
	SectionID("tools"):            "the AI runs tool calls one at a time instead of in parallel — slower, more requests",
	SectionID("honesty"):          "the AI becomes MORE LIKELY to claim success it did not observe, and to invent file paths and flags",
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
