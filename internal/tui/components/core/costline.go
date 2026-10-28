package core

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/cost"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/session"
)

// paidForecast is what the NEXT message can cost and this session's spending
// pace, for a model with a price in the dated table: "next up to $0.055,
// $0.60/h". Empty for a model with no list price (free providers included).
//
// GORILLA (2026-10-28, owner): "each message costs more than the last" means
// nothing until "more" is a number. This is that number, from the size of the
// conversation now. The pace is the session's spend over its age, shown once a
// minute has passed so one call does not read as a rate.
func paidForecast(s session.Session, model models.Model, contextTokens int64, now time.Time) string {
	in, out, ok := cost.ListRates(string(model.Provider), model.APIModel, now)
	if !ok || in <= 0 {
		return ""
	}
	ctx := max(contextTokens, int64(cost.ShortConversationTokens))
	parts := []string{"next up to " + dollars(cost.MessageCost(in, out, ctx))}
	if age := now.Sub(time.Unix(s.CreatedAt, 0)); s.Cost > 0 && s.CreatedAt > 0 && age >= time.Minute {
		parts = append(parts, dollars(s.Cost/age.Hours())+"/h")
	}
	return strings.Join(parts, ", ")
}

// dollars prints a figure with the precision its size needs.
func dollars(v float64) string {
	switch {
	case v < 0.1:
		return fmt.Sprintf("$%.3f", v)
	case v < 10:
		return fmt.Sprintf("$%.2f", v)
	}
	return fmt.Sprintf("$%.0f", v)
}

// costDetail is the "why" beside the session's cost: how much of it was paid
// for re-sending old context, what the newest call cost, and how many calls no
// price table knows.
//
// GORILLA (2026-10-21): every call sends the whole conversation again. In a
// measured 64-hour session 72% of a $303 bill was that re-reading, and calls
// carrying 500K+ tokens cost $0.19 each against $0.05 at 100-200K. A total
// alone never tells the user that starting fresh would bring the cost per call
// down; these two numbers do. Unpriced calls are said, because a total that
// silently leaves calls out reads lower than it is.
func costDetail(s session.Session) string {
	var parts []string
	// GORILLA FIX (2026-10-21, same day): this said "N% re-reading" from the
	// cost of CACHE reads, and read "2%" on an Antigravity session where nine
	// tenths of the input was the conversation sent again: the cache caught a
	// fifth of the repeats and the rest were billed as fresh input. The share
	// of input that was repetition is what tells the user to start fresh.
	if share := resentShare(s); share > 0 {
		parts = append(parts, fmt.Sprintf("%d%% re-sent", share))
	}
	if s.LastCallCost > 0 {
		parts = append(parts, fmt.Sprintf("last call $%.3f", s.LastCallCost))
	}
	if s.CallsUnpriced > 0 {
		parts = append(parts, fmt.Sprintf("%d call(s) unpriced", s.CallsUnpriced))
	}
	return strings.Join(parts, ", ")
}

// resentShare is the percentage of this session's input that was the
// conversation sent again, 0 when nothing is known yet.
func resentShare(s session.Session) int {
	return int(math.Round(100 * s.ResentShare()))
}

// costWarning returns the warning for a session whose cost has passed a new
// multiple of threshold, and the step now reached. step is the last multiple
// already warned about, so each $5 (or whatever threshold) is announced once.
// threshold <= 0 switches the warning off.
func costWarning(s session.Session, cost, threshold float64, step int) (string, int) {
	if threshold <= 0 || cost < threshold*float64(step+1) {
		return "", step
	}
	now := int(cost / threshold)
	msg := fmt.Sprintf("This session has passed $%.2f (now ~$%.2f at API prices).", threshold*float64(now), cost)
	if s.LastCallCost > 0 {
		msg += fmt.Sprintf(" Every call re-sends %s tokens and costs about $%.3f.",
			humanTokens(s.PromptTokens), s.LastCallCost)
	}
	return msg + " Finish the job and start a fresh session: it sends far less with every call.", now
}

// overCostThreshold says whether the cost chip should turn red.
func overCostThreshold(cost, threshold float64) bool {
	return threshold > 0 && cost >= threshold
}
