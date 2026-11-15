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
	"fmt"
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
	covered := map[string]bool{}
	for _, g := range q.Groups {
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
		if ids := groupModelIDs(groupFamilySpec(g), live); len(ids) > 0 {
			for _, id := range ids {
				covered[id] = true
			}
			m.Note = strings.TrimSpace(m.Note + "\nModel ids: " + strings.Join(ids, ", "))
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
	if len(live) > 0 {
		var other []string
		for id := range live {
			if !covered[id] {
				other = append(other, id)
			}
		}
		if len(other) > 0 {
			sort.Strings(other)
			out = append(out, quota.Meter{
				Provider: "other / bucket unknown",
				Account:  account,
				Kind:     quota.KindWindowQuota,
				Note:     "No quota reading from the backend for this plan.\nModel ids: " + strings.Join(other, ", "),
			})
		}
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
	// ROUND UP, not down. Truncating renders a window with 29m59s left as
	// "resets in 29m", which is EARLIER than the truth, so a user who waits
	// exactly that long retries into a limit that has not lifted yet. Rounding
	// up errs the safe way: the number is an upper bound, and the allowance is
	// back by then or sooner. It also removes an ugly off-by-one where a reset
	// set exactly 30 minutes out displays as 29.
	ceilDiv := func(a, b time.Duration) int { return int((a + b - 1) / b) }
	switch {
	case d < time.Hour:
		return fmt.Sprintf("resets in %dm", ceilDiv(d, time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("resets in %dh", ceilDiv(d, time.Hour))
	default:
		return fmt.Sprintf("resets in %dd", ceilDiv(d, 24*time.Hour))
	}
}

// groupModelIDs returns the live model ids whose display name contains one of
// the family names listed in a group description after the first colon.
func groupModelIDs(desc string, live map[string]AntigravityModelInfo) []string {
	i := strings.Index(desc, ":")
	if i < 0 || len(live) == 0 {
		return nil
	}
	var fams []string
	for _, f := range strings.Split(desc[i+1:], ",") {
		if f = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(f), "."))); f != "" {
			fams = append(fams, f)
		}
	}
	var ids []string
	for id, info := range live {
		name := strings.ToLower(info.DisplayName)
		for _, f := range fams {
			if strings.Contains(name, f) {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// groupFamilies is the family list a group's description names (the text after
// its first colon), or "" when it names none. The group's own heading is added
// by the caller, so a group such as "Gemini 4 Argon" with no list still claims
// the models whose display name carries its heading.
func groupFamilies(g QuotaGroup) string {
	if i := strings.Index(g.Description, ":"); i >= 0 {
		return g.Description[i+1:]
	}
	return ""
}

// groupFamilySpec is the ": a, b" text groupModelIDs reads: the description's
// own list when it has one, else the group's heading (without a trailing
// "models"), so a group that lists nothing still claims the models named like
// it. The heading is NOT added when a list exists: "Gemini Models" would claim
// every Gemini model, including another group's.
func groupFamilySpec(g QuotaGroup) string {
	if fams := strings.TrimSpace(groupFamilies(g)); fams != "" {
		return ": " + fams
	}
	h := strings.TrimSpace(g.DisplayName)
	if len(h) > 7 && strings.EqualFold(h[len(h)-7:], " models") {
		h = h[:len(h)-7]
	}
	return ": " + h
}
