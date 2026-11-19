package auth

// GORILLA OVERRIDE (2026-08-23): adapt this package's two OAuth quota shapes
// into the neutral quota.Meter, so the TUI never has to import an OAuth stack
// to draw a bar.
//
// The direction is deliberate and one-way: auth imports quota, never the
// reverse. internal/quota stays free of tokens, refresh flows and browsers,
// which is what lets a renderer depend on it without dragging half a sign-in
// implementation into a view.
//
// Each adapter's job is to attach the OWNING ACCOUNT to the numbers. That is the
// whole point of the refactor: `/usage` on a ChatGPT session used to print the
// Antigravity weekly quota under a Google email, and it was possible because the
// figures and the name they belonged to travelled as separate arguments that
// nothing forced to agree. A Meter carries both or fails Validate().

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/quota"
)

// AntigravityProviderName and ChatGPTProviderName are the panel headings. Kept
// here beside the adapters so a rename cannot leave the two out of step.
const (
	AntigravityProviderName = "ANTIGRAVITY"
	ChatGPTProviderName     = "CHATGPT"
)

// ToMeter converts the Antigravity weekly summary into neutral meters, one per
// model group, each stamped with the Google account it was fetched for.
//
// KindWindowQuota: these are genuine allowances inside a rolling window, which
// is the one kind that honestly earns a bar.
func (q *QuotaSummary) ToMeter(account string) []quota.Meter {
	return q.ToMeterWithModels(account, nil)
}

// ToMeterWithModels is ToMeter that also names, under each group, the model ids
// it covers. The provider says the family names ("Models within this group:
// Gemini Flash, Gemini Pro") in the group description; the live model list
// (id -> display name, from fetchAvailableModels) turns them into ids. Nothing
// is hardcoded: a model the backend does not list is not named.
func (q *QuotaSummary) ToMeterWithModels(account string, live map[string]AntigravityModelInfo) []quota.Meter {
	if q == nil {
		return nil
	}
	out := make([]quota.Meter, 0, len(q.Groups))
	membership := make(map[int][]string)
	var other []string
	for id, model := range live {
		index := quotaGroupForModel(q.Groups, model.DisplayName)
		if index < 0 {
			other = append(other, id)
		} else {
			membership[index] = append(membership[index], id)
		}
	}
	for i, g := range q.Groups {
		// The GROUP name is the heading, not "ANTIGRAVITY". A Google sign-in
		// reports several model families under one account ("Gemini Models",
		// "Claude and GPT Models") and each is its own barrel with its own
		// number, so collapsing them under one provider heading would print two
		// identical titles over two different meters.
		heading := g.DisplayName
		if heading == "" && len(g.Buckets) > 0 {
			// A group the backend left unnamed (a plan-only bucket) still
			// gets its own meter, titled by the backend's bucket id.
			heading = g.Buckets[0].BucketID
		}
		if heading == "" {
			heading = AntigravityProviderName
		}
		m := quota.Meter{
			Provider: heading,
			Account:  account,
			Kind:     quota.KindWindowQuota,
			Note:     g.Description,
		}
		if ids := membership[i]; len(ids) > 0 {
			m.Note = appendQuotaModels(m.Note, "Models sharing this quota", ids, live)
		}
		for _, b := range g.Buckets {
			label := b.DisplayName
			if label == "" {
				label = b.Window
			}
			if label == "" {
				label = b.BucketID
			}
			rem := b.RemainingFraction
			if b.NoReading {
				rem = quota.FractionUnknown
			}
			m.Bars = append(m.Bars, quota.Bar{
				Label:     label,
				Remaining: rem,
				// ResetTime is carried as the provider's raw string; the panel
				// owns the phrasing, because "resets in 6d" is presentation.
				Reset: b.ResetTime,
			})
		}
		out = append(out, m)
	}
	// Every live model sits under a group or under "other": one the group
	// descriptions do not name (Argon, Orchestrator, a new variant) is shown,
	// never silently dropped. Nothing here is a hard-coded model name.
	if len(other) > 0 {
		out = append(out, quota.Meter{
			Provider: "other / bucket unknown",
			Account:  account,
			Kind:     quota.KindWindowQuota,
			Note:     appendQuotaModels("No quota reading from the backend for this plan.", "Quota group unavailable", other, live),
		})
	}
	// The summary-level description explains the shared weekly limit and is
	// written once, under the groups it applies to. Attaching it to the last
	// meter keeps that placement without repeating it per group.
	if len(out) > 0 && q.Description != "" {
		out[len(out)-1].Footer = q.Description
	}
	return out
}

// ToMeter converts the ChatGPT reading into a neutral meter.
//
// Two things this must not lose, both learned the hard way:
//
//   - The window LABEL comes from the wire. Codex renders "Monthly limit" on a
//     free plan and a weekly pair on a paid one, so any constant here mislabels
//     every tier it was not written against.
//   - Remaining comes from ChatGPTWindow.Remaining(), the single conversion
//     point, because the wire field is used_percent and this panel reports what
//     is LEFT. Inverting it anywhere else recreates the exact confusion the
//     panel was built to prevent, and the obvious arithmetic is off by one ULP
//     at every tier boundary.
func (q *ChatGPTQuota) ToMeter(account string) quota.Meter {
	m := quota.Meter{
		Provider: ChatGPTProviderName,
		Account:  account,
		Kind:     quota.KindWindowQuota,
	}
	// Nothing read yet is a REAL third state, not an error and not zero. This
	// backend reports usage on its replies, so before the session's first
	// request there is genuinely nothing to show, and saying "100%" there would
	// be a lie in the expensive direction.
	if q.Empty() {
		m.Pending = true
		return m
	}
	m.Plan = q.PlanType

	add := func(w *ChatGPTWindow, secondary bool) {
		if w == nil {
			return
		}
		m.Bars = append(m.Bars, quota.Bar{
			Label:     w.Label(secondary),
			Remaining: w.Remaining(),
			Reset:     chatGPTResetPhrase(w.ResetAt),
		})
	}
	add(q.Primary, false)
	add(q.Secondary, true)
	if q.Credits != nil {
		switch {
		case q.Credits.Unlimited:
			m.Note = "Credits: unlimited"
		case q.Credits.Balance != nil && strings.TrimSpace(*q.Credits.Balance) != "":
			m.Note = "Credits: " + *q.Credits.Balance
		case q.Credits.HasCredits != nil && !*q.Credits.HasCredits:
			m.Note = "Credits: 0"
		default:
			m.Note = "Credits: balance unavailable"
		}
	}
	return m
}

// chatGPTResetPhrase turns a unix reset timestamp into the panel's phrasing.
// Empty when the backend did not say: unknown must not be dressed as a number.
func chatGPTResetPhrase(resetAt int64) string {
	if resetAt <= 0 {
		return ""
	}
	d := time.Until(time.Unix(resetAt, 0))
	if d <= 0 {
		return "resets now"
	}
	return quota.ResetIn(d)
}

var quotaEffortSuffix = regexp.MustCompile(`(?i)\s+\((none|minimal|low|medium|high|xhigh|max|thinking)\)$`)

func quotaModelName(name string) string {
	return quotaEffortSuffix.ReplaceAllString(strings.Join(strings.Fields(name), " "), "")
}

func appendQuotaModels(note, label string, ids []string, live map[string]AntigravityModelInfo) string {
	sort.Strings(ids)
	var names []string
	seen := make(map[string]bool)
	for _, id := range ids {
		name := quotaModelName(live[id].DisplayName)
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		sort.Strings(names)
		note += "\n" + label + ": " + strings.Join(names, ", ")
	}
	return strings.TrimSpace(note + "\nModel ids: " + strings.Join(ids, ", "))
}

// Prefer an exact family name. A generic family label may also cover a model
// with a numeric version. Distinct named routes need an explicit match.
func quotaGroupForModel(groups []QuotaGroup, name string) int {
	name = strings.ToLower(quotaModelName(name))
	best, bestScore := -1, 0
	for i, group := range groups {
		families := group.DisplayName
		const prefix = "models within this group:"
		if desc := strings.TrimSpace(group.Description); strings.HasPrefix(strings.ToLower(desc), prefix) {
			families = desc[len(prefix):]
		}
		for _, family := range strings.Split(families, ",") {
			family = strings.ToLower(quotaModelName(strings.TrimSuffix(strings.TrimSpace(family), ".")))
			if family == "" {
				continue
			}
			score := 0
			if name == family {
				score = len(name) + 1
			} else if tail, ok := strings.CutPrefix(name, family+" "); ok && tail != "" && tail[0] >= '0' && tail[0] <= '9' && !strings.Contains(tail, "(") {
				score = len(family)
			}
			if score > bestScore {
				best, bestScore = i, score
			} else if score > 0 && score == bestScore && best != i {
				best = -1
			}
		}
	}
	return best
}
