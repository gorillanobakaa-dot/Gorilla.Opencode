package agent

// GORILLA (2026-10-24): say it when the always-on instructions and tools take
// a large share of the model's window, at the start of a session, where the
// person can still act on it.
//
// config.LoadoutCrowdsTheContext existed and was called by nothing; the advice
// lived only on the /context screen, which nobody opens before they know there
// is a problem. Measured that day on Gemma 4 12B (20,224-token window, LM
// Studio): 12,592 tokens of prompt before the first word, read for 4m17s, and
// a 4-turn conversation that ran out of room.

import (
	"fmt"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
)

var crowdWarned sync.Map // session ID → struct{}

// crowdedLoadoutNotice is the notice for a session's first message, or "" when
// the loadout fits comfortably, the model declares no window, or this session
// was already told.
func crowdedLoadoutNotice(sessionID string) string {
	if !config.LoadoutCrowdsTheContext() {
		return ""
	}
	if _, told := crowdWarned.LoadOrStore(sessionID, struct{}{}); told {
		return ""
	}
	tokens, window, name, share := config.LoadoutContextShare()
	return crowdedText(tokens, window, name, share, cutsText())
}

func cutsText() string {
	cuts, saved := config.LoadoutBiggestCuts(3)
	if saved <= 0 {
		return ""
	}
	names := make([]string, 0, len(cuts))
	for _, c := range cuts {
		names = append(names, fmt.Sprintf("%s ~%s", c.Name, humanTokens(int64(c.Tokens))))
	}
	return fmt.Sprintf(" Biggest you could switch off: %s (~%s back).", strings.Join(names, ", "), humanTokens(int64(saved)))
}

func crowdedText(tokens, window int, name string, share float64, cuts string) string {
	return fmt.Sprintf(
		"The instructions and tools switched on take about %s of %s's %s window (%.0f%%). "+
			"The model reads them before its first answer and they stay in the window, "+
			"leaving less room for the conversation.%s Change it in /context.",
		humanTokens(int64(tokens)), name, humanTokens(int64(window)), share*100, cuts)
}
