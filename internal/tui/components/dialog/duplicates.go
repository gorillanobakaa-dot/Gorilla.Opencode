// Version: 1.0.0 · updated 26-08-21-12-30
//
// GORILLA OVERRIDE (2026-08-21): one row per model in the all-provider search.
//
// The same model is commonly reachable through several providers — GPT-OSS 120B
// is served by NVIDIA NIM, Groq, Cerebras and OpenRouter, and the free NIM key
// and the free OpenRouter tier both offer it. A search for "gpt-oss" therefore
// returned four rows that look identical and are not: they differ in what they
// cost the user in QUOTA, which is the only currency this audience has.
//
// So the rows collapse to one, and the survivor is the route with the most
// usable free allowance. The others are named on the row rather than hidden, so
// the choice is still visible.
package dialog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// providerQuotaRank orders providers by how much work a free user can actually
// get out of them. Lower is better. This is a JUDGEMENT, so it is dated, sourced
// and coarse on purpose — see the note below on why it is not finer than this.
//
// CHECKED 2026-08-21:
//   - OpenRouter free tier: 20 requests/minute and **50 requests per DAY**;
//     1000/day only after buying at least 10 credits, i.e. with a card.
//     (openrouter.ai/docs/api-reference/limits)
//   - NVIDIA NIM: no monthly credit cap; a soft ~40 requests per MINUTE, and
//     NVIDIA staff describe the limit as varying by model and current traffic.
//     (NVIDIA Developer Forums, build.nvidia.com free tier thread)
//   - Groq, Cerebras, Google AI Studio: free tiers exist but the vendors have
//     stopped publishing the numbers — their docs now point at a per-account
//     dashboard.
//
// A per-minute limit throttles how fast you work. A per-day limit tells you to
// come back tomorrow. That distinction is the whole ranking, and it is why
// OpenRouter sorts below the free-key providers here despite being the most
// convenient.
//
// It is deliberately NOT a table of exact quotas. ratelimit.go already settled
// that question for this estate: free-tier limits are "undocumented, variable,
// and load/time-of-day dependent", so "no hard-coded value is right across
// accounts and times". A coarse order can be right for years; a table of numbers
// would be wrong within weeks, and wrong silently.
var providerQuotaRank = map[models.ModelProvider]int{
	// Signed in, no key, no card, generous.
	models.ProviderGeminiCA:    1,
	models.ProviderAntigravity: 1,
	models.ProviderChatGPT:     2,
	// Your own endpoint: whatever you configured, usually a per-minute cap.
	models.ProviderLocal: 3,
	// Free keys, per-minute limits.
	models.ProviderGROQ:     4,
	models.ProviderCerebras: 4,
	models.ProviderGemini:   5,
	// Free tier is 50 requests A DAY without a card.
	models.ProviderOpenRouter: 6,
	// Paid keys: no free allowance to compare, so they lose to anything free.
	models.ProviderAnthropic: 8,
	models.ProviderOpenAI:    8,
	models.ProviderXAI:       8,
	models.ProviderDeepSeek:  8,
}

func quotaRank(p models.ModelProvider) int {
	if r, ok := providerQuotaRank[p]; ok {
		return r
	}
	return 9 // unknown provider: below everything ranked
}

// modelFingerprint is what makes two rows "the same model".
//
// It is the api id with the provider's own packaging stripped: the vendor prefix
// ("meta/llama-3.3-70b" and "meta-llama/llama-3.3-70b" are one model), and
// OpenRouter's ":free" / ":nitro" routing suffixes. Deliberately conservative —
// it collapses spellings of one name, never two different models that look
// similar. A wrong collapse HIDES a model, which is worse than a duplicate row.
func modelFingerprint(m models.Model) string {
	id := strings.ToLower(m.APIModel)
	if id == "" {
		id = strings.ToLower(string(m.ID))
	}
	if i := strings.Index(id, ":"); i > 0 {
		id = id[:i]
	}
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	return id
}

// collapseDuplicates keeps one row per model — the best free route — and names
// the alternatives on the surviving row. Input order is preserved for whatever
// survives, so an existing ranking is not disturbed.
func collapseDuplicates(in []models.Model) []models.Model {
	type group struct {
		best     models.Model
		bestPos  int
		bestRank int
		others   map[string]bool
	}
	groups := map[string]*group{}
	order := []string{}

	for i, m := range in {
		fp := modelFingerprint(m)
		if fp == "" {
			continue
		}
		g, seen := groups[fp]
		if !seen {
			groups[fp] = &group{best: m, bestPos: i, bestRank: quotaRank(m.Provider), others: map[string]bool{}}
			order = append(order, fp)
			continue
		}
		r := quotaRank(m.Provider)
		if r < g.bestRank {
			// A better route wins the row; the previous winner becomes an
			// alternative. It keeps the earlier position so the list does not
			// reshuffle around a collapse.
			g.others[providerLabel(g.best)] = true
			g.best, g.bestRank = m, r
		} else {
			g.others[providerLabel(m)] = true
		}
	}

	out := make([]models.Model, 0, len(order))
	type placed struct {
		m   models.Model
		pos int
	}
	var all []placed
	for _, fp := range order {
		g := groups[fp]
		m := g.best
		if len(g.others) > 0 {
			names := make([]string, 0, len(g.others))
			for n := range g.others {
				names = append(names, n)
			}
			sort.Strings(names)
			// Say it on the row. A collapsed alternative that leaves no trace is
			// a model the user can no longer find, and finding models is what
			// this screen is for.
			note := fmt.Sprintf("also on %s", strings.Join(names, ", "))
			if m.Description == "" {
				m.Description = note
			} else {
				m.Description = m.Description + " — " + note
			}
		}
		all = append(all, placed{m, g.bestPos})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].pos < all[j].pos })
	for _, p := range all {
		out = append(out, p.m)
	}
	return out
}

// providerLabel is how an alternative route is named on the row: the user's own
// endpoint name where there is one ("Gorilla.FREE.NVIDIA.NIM"), otherwise the
// provider. A row saying "also on local" would not tell anyone which machine.
func providerLabel(m models.Model) string {
	if ep := models.LocalEndpointFor(m.ID); ep != "" {
		return ep
	}
	return string(m.Provider)
}
