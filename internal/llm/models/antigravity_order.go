package models

// GORILLA OVERRIDE (2026-10-05): order, names and defaults for the FETCHED
// Antigravity list, derived from what the backend said rather than typed.
//
// What the owner saw, in one screenshot of his own picker:
//
//	 9. Claude Sonnet 4.6 ...      <- not in the backend's list any more
//	10. Claude Opus 4.6 Thinking   <- not in the backend's list any more
//	GPT-OSS 120B (Medium) ...
//	Claude Opus 5.5 (High) ...     <- the current models, unranked, below
//	...
//	Gemini 3.5 Flash Lite ...      <- four rows, letter for letter identical
//	Gemini 3.5 Flash Lite ...
//	Gemini 3.5 Flash Lite ...
//	Gemini 3.5 Flash Lite ...
//
// Three separate faults, all from the same habit of typing a fact once:
//
//  1. The five built-in entries carried hand-written ranks and were never
//     removed, so two retired models sat at the top numbered 9 and 10 under a
//     heading that says "1 = best", and the coder was PUT on one of them at
//     every sign-in by a constant in the portal.
//  2. Fetched models had no rank at all, so everything current sorted below.
//  3. The backend gives four different ids the same display name. The name was
//     trusted (rightly) and printed alone (wrongly): four rows nobody can tell
//     apart are three rows nobody can choose.
//
// The fixes are rules over the backend's own labels, not a new list: a new
// version or a new effort level sorts into place without anyone retyping it.

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const antigravityNameSuffix = " (Antigravity free)"

var agVersionRe = regexp.MustCompile(`\d+(?:\.\d+)?`)

// Families, best first for coding work. The label is what is matched, not the
// id: the backend ships id "gemini-2.5-flash" labelled "Gemini 3.5 Flash Lite",
// and the label is the one that says what answers.
const (
	agFamilyOpus = iota
	agFamilySonnet
	agFamilyOtherClaude
	agFamilyGeminiPro
	agFamilyGeminiFlash
	agFamilyOther
	agFamilyLite
)

type agKey struct {
	family  int
	version float64
	effort  int
}

func antigravityKey(label string) agKey {
	s := strings.ToLower(strings.TrimSuffix(label, antigravityNameSuffix))
	k := agKey{family: agFamilyOther, effort: 1}
	switch {
	case strings.Contains(s, "lite"):
		k.family = agFamilyLite
	case strings.Contains(s, "opus"):
		k.family = agFamilyOpus
	case strings.Contains(s, "sonnet"):
		k.family = agFamilySonnet
	case strings.Contains(s, "claude"):
		k.family = agFamilyOtherClaude
	case strings.Contains(s, "gemini") && strings.Contains(s, "pro"):
		k.family = agFamilyGeminiPro
	case strings.Contains(s, "gemini") && strings.Contains(s, "flash"):
		k.family = agFamilyGeminiFlash
	}
	if v := agVersionRe.FindString(s); v != "" {
		k.version, _ = strconv.ParseFloat(v, 64)
	}
	// Effort is the backend's own bracket. No bracket and "medium" are the same
	// middle setting; "tiered" lets the backend choose, so it sits below a
	// stated medium and above a stated low.
	switch {
	case strings.Contains(s, "(high)"), strings.Contains(s, "thinking"):
		k.effort = 0
	case strings.Contains(s, "(tiered)"):
		k.effort = 2
	case strings.Contains(s, "(low)"):
		k.effort = 3
	}
	return k
}

// finishAntigravity gives a fetched set its picker order and makes every name
// unique. It works on the stored models, not on the wire rows, so a cache
// written by an older build is repaired when it is read instead of waiting for
// the next refresh.
func finishAntigravity(set map[ModelID]Model) {
	ids := make([]ModelID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}

	// Names first: a label shared by several ids gets each id appended.
	byLabel := map[string][]ModelID{}
	for _, id := range ids {
		byLabel[agBaseLabel(set[id])] = append(byLabel[agBaseLabel(set[id])], id)
	}
	for label, same := range byLabel {
		if len(same) < 2 {
			continue
		}
		for _, id := range same {
			m := set[id]
			m.Name = label + " [" + m.APIModel + "]" + antigravityNameSuffix
			set[id] = m
		}
	}

	sort.Slice(ids, func(i, j int) bool {
		a, b := antigravityKey(agBaseLabel(set[ids[i]])), antigravityKey(agBaseLabel(set[ids[j]]))
		switch {
		case a.family != b.family:
			return a.family < b.family
		case a.version != b.version:
			return a.version > b.version // newer first
		case a.effort != b.effort:
			return a.effort < b.effort
		}
		return ids[i] < ids[j]
	})
	for i, id := range ids {
		m := set[id]
		m.Rank = i + 1 // 1 = best, as everywhere else
		set[id] = m
	}
}

// agBaseLabel is the backend's label with nothing of ours on it.
func agBaseLabel(m Model) string {
	s := strings.TrimSuffix(m.Name, antigravityNameSuffix)
	if i := strings.LastIndex(s, " ["); i >= 0 && strings.HasSuffix(s, "]") {
		s = s[:i]
	}
	return s
}

// PreferredAntigravityModels reports which registered Antigravity model to put
// the coder on and which to put the background agents (title, summariser, task)
// on. It replaces two constants at the sign-in site, AGClaudeSonnet46 and
// AGGemini36Flash: the first named a model Google had stopped listing, and the
// portal went on selecting it at every sign-in.
//
// coder is the best Claude Sonnet: the same choice the constants made, for the
// same reason — Opus shares the Claude weekly allowance and spends it faster.
// background is the best mid-effort Gemini Flash, which draws the SEPARATE
// Gemini weekly allowance and so leaves the Claude one for the work the user
// watches. Each falls back to the best model of any kind rather than to "".
func PreferredAntigravityModels() (coder, background ModelID) {
	type cand struct {
		id   ModelID
		rank int
		key  agKey
	}
	var all []cand
	for id, m := range SupportedModels {
		if m.Provider != ProviderAntigravity {
			continue
		}
		rank := m.Rank
		if rank <= 0 {
			rank = 1 << 20 // unranked loses to anything ranked
		}
		all = append(all, cand{id, rank, antigravityKey(agBaseLabel(m))})
	}
	if len(all) == 0 {
		return "", ""
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].rank != all[j].rank {
			return all[i].rank < all[j].rank
		}
		return all[i].id < all[j].id
	})
	pick := func(ok func(agKey) bool) ModelID {
		for _, c := range all {
			if ok(c.key) {
				return c.id
			}
		}
		return ""
	}
	coder = pick(func(k agKey) bool { return k.family == agFamilySonnet })
	if coder == "" {
		coder = all[0].id
	}
	background = pick(func(k agKey) bool { return k.family == agFamilyGeminiFlash && k.effort == 1 })
	if background == "" {
		background = pick(func(k agKey) bool { return k.family == agFamilyGeminiFlash })
	}
	if background == "" {
		background = coder
	}
	return coder, background
}

// AntigravityReplacementFor names the model to move to when id is no longer
// offered: the best current model of the same family, else the default coder.
// Returns "" when id is still offered or nothing is registered.
func AntigravityReplacementFor(id ModelID, retiredLabel string) ModelID {
	if _, still := SupportedModels[id]; still {
		return ""
	}
	want := antigravityKey(retiredLabel).family
	best, bestRank := ModelID(""), 0
	for cid, m := range SupportedModels {
		if m.Provider != ProviderAntigravity || m.Rank <= 0 {
			continue
		}
		if antigravityKey(agBaseLabel(m)).family != want {
			continue
		}
		if best == "" || m.Rank < bestRank {
			best, bestRank = cid, m.Rank
		}
	}
	if best != "" {
		return best
	}
	coder, _ := PreferredAntigravityModels()
	return coder
}
