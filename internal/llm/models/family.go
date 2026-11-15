package models

// One catalogue, grouped for the picker: family first, effort second.
//
// The Antigravity backend serves one id per effort ("claude-opus-5-5-high",
// "claude-opus-5-5-low"), so a flat picker shows the same model three times.
// The family and the efforts are DERIVED from the ids the backend reported:
// the trailing id segment, when it is an effort word, is the effort; the rest
// is the family. Nothing is typed per model, so a new family or a new effort
// level ("xhigh", a future "max") appears by itself. The effort the owner picks
// IS a backend id, so it is sent in the request as the model id (proven by
// TestChosenEffortIsTheModelIDSent).

import (
	"regexp"
	"sort"
	"strings"
)

// effortOrder ranks the effort words; unknown words are not efforts.
var effortOrder = map[string]int{"none": 0, "minimal": 1, "low": 2, "medium": 3, "high": 4, "xhigh": 5, "max": 6}

var effortNameRe = regexp.MustCompile(`(?i)\s*\((none|minimal|low|medium|high|xhigh|max)\)`)

// FamilyOf returns the family key, the family's display name and the effort of
// a model. effort is "" when the id carries none (the model has no effort
// choice). The display name drops the effort and the provider suffix.
func FamilyOf(m Model) (family, name, effort string) {
	api := m.APIModel
	if api == "" {
		api = string(m.ID)
	}
	family = api
	if i := strings.LastIndex(api, "-"); i > 0 {
		if _, ok := effortOrder[strings.ToLower(api[i+1:])]; ok {
			family, effort = api[:i], strings.ToLower(api[i+1:])
		}
	}
	name = trimAntigravitySuffix(m.Name)
	name = strings.TrimSpace(effortNameRe.ReplaceAllString(name, ""))
	return string(m.Provider) + "/" + family, name, effort
}

// EffortsOf lists the efforts a family offers, lowest first.
func EffortsOf(variants []Model) []string {
	var out []string
	for _, v := range variants {
		if _, _, e := FamilyOf(v); e != "" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return effortOrder[out[i]] < effortOrder[out[j]] })
	return out
}

// FamilyGroup is one picker row: the variants of one model family.
type FamilyGroup struct {
	Key      string
	Name     string
	Variants []Model // sorted lowest effort first; one entry when there is no choice
}

// Efforts is the choice this group offers (empty: none).
func (g FamilyGroup) Efforts() []string { return EffortsOf(g.Variants) }

// Pick returns the variant for an effort, or the default when effort is empty
// or not offered: "high" if offered, else the highest below it, else the first.
func (g FamilyGroup) Pick(effort string) Model {
	for _, v := range g.Variants {
		if _, _, e := FamilyOf(v); e != "" && e == effort {
			return v
		}
	}
	var best Model
	found := false
	for _, v := range g.Variants {
		_, _, e := FamilyOf(v)
		if e == "" {
			continue
		}
		if !found || (effortOrder[e] <= effortOrder["high"] && effortOrder[e] >= effortRank(best)) {
			best, found = v, true
		}
	}
	if found {
		return best
	}
	return g.Variants[0]
}

func effortRank(m Model) int { _, _, e := FamilyOf(m); return effortOrder[e] }

// GroupFamilies collapses variants of one family into one group, keeping the
// order in which each family first appears. A model with no effort suffix is a
// group of one.
func GroupFamilies(list []Model) []FamilyGroup {
	idx := map[string]int{}
	var out []FamilyGroup
	for _, m := range list {
		key, name, _ := FamilyOf(m)
		i, ok := idx[key]
		if !ok {
			idx[key] = len(out)
			out = append(out, FamilyGroup{Key: key, Name: name})
			i = len(out) - 1
		}
		out[i].Variants = append(out[i].Variants, m)
	}
	for i := range out {
		sort.SliceStable(out[i].Variants, func(a, b int) bool {
			return effortRank(out[i].Variants[a]) < effortRank(out[i].Variants[b])
		})
	}
	return out
}
