package models

import (
	"fmt"
	"regexp"
	"strings"
)

// GORILLA OVERRIDE: shared cleanup for provider-published catalogues.
//
// Lives here rather than in the generator because BOTH the build-time generator
// (cmd/openrouter-models) and the runtime refresh (refresh.go) need it, and two
// copies of "which models are usable" is how they drift apart. This project has
// been bitten by a duplicated launcher once already.

var markdownLink = regexp.MustCompile(`\[([^\]]+)\]\(<?[^)]*>?\)`)

// IsBatchVariant reports whether a model id is an asynchronous batch endpoint.
//
// OpenRouter lists 59 of these among its tool-capable models. They are for bulk
// jobs submitted and collected later - pointing an interactive agent at one
// means typing a question and waiting indefinitely for a reply. Excluding them
// is the same judgement as excluding models that cannot call tools: not
// curation, but removing entries that cannot do this job.
func IsBatchVariant(id string) bool {
	l := strings.ToLower(id)
	return strings.HasSuffix(l, ":batch") || strings.Contains(l, ":batch:")
}

// CleanCatalogueDescription turns a provider's marketing paragraph into one
// line the picker can show.
//
// Providers write these as markdown, so links arrive intact - "from
// [Poolside](<https://poolside.ai/>)" renders as exactly that in a terminal,
// spending characters on a URL nobody can click. The link text is kept and the
// address dropped.
func CleanCatalogueDescription(desc string, ctx int64, inPer1M, outPer1M float64) string {
	s := markdownLink.ReplaceAllString(desc, "$1")
	s = strings.ReplaceAll(s, "\n", " ")
	if i := strings.IndexAny(s, ".!"); i > 30 {
		s = s[:i]
	}
	s = strings.Join(strings.Fields(s), " ")
	// GORILLA OVERRIDE (2026-08-09): 90 characters cut almost every description
	// mid-sentence — "…is an instruction-tuned Mixture-of-Experts (MoE) model
	// from Google Deep…" — which defeats the point. The description is the ONE
	// thing standing between someone and a web search per model name, and a
	// search per model is impossible on a single-digit-KB/s line.
	//
	// The cap is now generous rather than tight, and the picker truncates each
	// ROW to the terminal width anyway. Wide terminals therefore show the whole
	// thing; narrow ones cut to fit, which is the only place cutting belongs.
	if len(s) > 220 {
		s = strings.TrimSpace(s[:220]) + "…"
	}
	// GORILLA OVERRIDE (2026-08-09): price goes FIRST, on every entry.
	//
	// Free models were marked and paid ones were not, so telling them apart
	// meant knowing that absence means paid — and 262 of 274 entries were
	// silent. Reported plainly: "I am still picking up the wrong models because
	// I have no way of knowing which one is free and which one requires a
	// subscription."
	//
	// Leading with it makes the column scannable: every row starts with either
	// FREE or a number. No probing needed - OpenRouter publishes exact prices
	// and they are already stored on the model.
	prefix := ""
	switch {
	case inPer1M == 0 && outPer1M == 0:
		prefix = "FREE — "
	case inPer1M < 1 && outPer1M < 1:
		prefix = fmt.Sprintf("$%.2f/$%.2f per 1M — ", inPer1M, outPer1M)
	default:
		prefix = fmt.Sprintf("$%.1f/$%.1f per 1M — ", inPer1M, outPer1M)
	}
	if ctx > 0 {
		return fmt.Sprintf("%s%s (%dK ctx)", prefix, s, ctx/1000)
	}
	return prefix + s
}
