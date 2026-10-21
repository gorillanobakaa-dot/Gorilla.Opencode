package core

import (
	"fmt"
	"math"
	"strings"

	"github.com/opencode-ai/opencode/internal/session"
)

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
	if s.Cost > 0 && s.CostReread > 0 {
		parts = append(parts, fmt.Sprintf("%d%% re-reading", int(math.Round(100*s.CostReread/s.Cost))))
	}
	if s.LastCallCost > 0 {
		parts = append(parts, fmt.Sprintf("last call $%.3f", s.LastCallCost))
	}
	if s.CallsUnpriced > 0 {
		parts = append(parts, fmt.Sprintf("%d call(s) unpriced", s.CallsUnpriced))
	}
	return strings.Join(parts, ", ")
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
