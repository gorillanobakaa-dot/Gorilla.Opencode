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

// CuratedVerdict returns our own judgement of a model, if we have earned one.
//
// GORILLA OVERRIDE (2026-08-09): OpenRouter lists 274 usable models and we have
// tested NONE of them — every one of the 77 eval runs in
// Scripts.For.Work/model-eval was against NVIDIA NIM. Twenty-three of the 274
// are nonetheless the SAME underlying model as an entry already curated in
// metadata/nim.json, so the verdict already written for it applies: DeepSeek V4
// Pro is 1.6T MoE at 80.6% SWE-bench whichever door you reach it through.
//
// Matching is on the model name with the vendor prefix and punctuation removed,
// because the same model is published as "deepseek-ai/deepseek-v4-pro" on NIM
// and "deepseek/deepseek-v4-pro" on OpenRouter.
//
// Everything else gets the vendor's own description, MARKED as such. That
// distinction is the whole point: a curated line is a judgement someone earned
// and can defend, a vendor line is marketing copy nobody here has checked, and
// presenting the second as the first is how a picker starts lying quietly.
func CuratedVerdict(apiModel string) (string, bool) {
	key := normaliseModelKey(apiModel)
	for id, meta := range modelMetaByID {
		if normaliseModelKey(id) == key && strings.TrimSpace(meta.Description) != "" {
			return meta.Description, true
		}
	}
	return "", false
}

// normaliseModelKey strips the vendor prefix and every non-alphanumeric
// character, so "deepseek-ai/deepseek-v4-pro" and "deepseek/deepseek-v4-pro"
// compare equal.
func normaliseModelKey(id string) string {
	s := strings.ToLower(id)
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DescribeForPicker builds the line shown in the model picker, in the order
// this project trusts things: what we found ourselves, then a judgement already
// written for the same model, then the vendor's own claim with the word that
// triggered its label, then an admission that nobody has any idea.
//
// The label leads because it is the decision. Someone learning to code should
// not have to read a paragraph of marketing to discover that a model is a
// roleplay bot.
func DescribeForPicker(apiModel, vendorDesc string, ctx int64, in, out float64) string {
	// 1. Earned here, with evidence on file.
	if v, ok := EarnedVerdict(apiModel); ok {
		return CleanCatalogueDescription(v.Verdict, ctx, in, out)
	}
	// 2. Already judged for the same underlying model.
	if v, ok := CuratedVerdict(apiModel); ok {
		return CleanCatalogueDescription(v, ctx, in, out)
	}
	// 3. The vendor's own claim, labelled and quotable.
	label, trigger := ClassifyForCoding(vendorDesc)
	body := label
	if trigger != "" {
		body += ` (vendor: "` + trigger + `")`
	}
	if d := strings.TrimSpace(vendorDesc); d != "" && trigger != "" {
		body += " — " + d
	}
	return CleanCatalogueDescription(body, ctx, in, out)
}
