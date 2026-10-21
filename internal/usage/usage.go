// Package usage answers "how much has this cost, and how much is left?" in one
// place, for the person (/usage) and for the AI (the usage tool) alike.
//
// GORILLA (2026-10-21): asked "how many bananas have I got left?", Gemini 3.8
// Flash spent 30 tool calls and most of a $0.39 session hunting for the answer:
// it guessed shell commands that do not exist, searched the whole home folder,
// opened the Antigravity sign-in file, then wrote and ran SQL against the live
// session database. Every one of those calls re-sent the whole conversation. The
// answer was one /usage away, and /usage is a keyboard command the AI cannot
// type. This package is that answer as data, so one call returns it.
//
// Doors. The session part is local (the ledger the program already keeps). The
// allowance part reads ONLY the provider the session is already using, through
// the same request /usage makes, and caches it for a minute so a model calling
// in a loop cannot turn it into traffic. No account name or e-mail is put in
// the answer: the AI does not need to know whose allowance it is.
package usage

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/quota"
	"github.com/opencode-ai/opencode/internal/session"
)

// SessionLines describes this session's ledger in plain words.
func SessionLines(s session.Session) []string {
	if s.CallsPriced == 0 && s.CallsUnpriced == 0 && s.Cost == 0 {
		return []string{"No model call has been recorded in this session yet."}
	}
	out := []string{fmt.Sprintf("Cost so far: $%.2f at API prices, over %d priced call(s).", s.Cost, s.CallsPriced)}
	if s.LastCallCost > 0 {
		out = append(out, fmt.Sprintf("Last call: $%.4f. The conversation now holds %s tokens, sent again with every call.",
			s.LastCallCost, humanTokens(s.PromptTokens+s.CompletionTokens)))
	}
	if share := s.ResentShare(); share > 0 {
		line := fmt.Sprintf("%d%% of this session's input (%s of %s tokens) was the conversation sent again.",
			pct(share), humanTokens(s.ResentTokens), humanTokens(s.CumulativePromptTokens))
		if s.Cost > 0 && s.CostReread > 0 {
			line += fmt.Sprintf(" The provider's cache made %d%% of the cost cheaper re-reads; the rest of the repeats were paid as new input.",
				pct(s.CostReread/s.Cost))
		}
		out = append(out, line)
	}
	if s.CallsUnpriced > 0 {
		out = append(out, fmt.Sprintf("%d call(s) were on a model with no published price; they are not in the cost.", s.CallsUnpriced))
	}
	return out
}

func pct(f float64) int { return int(math.Round(100 * f)) }

func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

// The readers are variables so a test can stand in for the network.
var (
	activeProvider = func() models.ModelProvider {
		c := config.Get()
		if c == nil {
			return ""
		}
		return models.SupportedModels[c.Agents[config.AgentCoder].Model].Provider
	}
	antigravityMeters = func(ctx context.Context) ([]quota.Meter, string, error) {
		creds, _ := auth.LoadAntigravityCreds()
		if creds == nil || creds.AccessToken == "" {
			return nil, "", errNotSignedIn
		}
		q, err := creds.RetrieveQuota(ctx)
		if err != nil {
			return nil, "", err
		}
		return q.ToMeter(""), creds.PlanLabel(), nil
	}
	chatGPTMeters = func() []quota.Meter {
		q, _ := auth.LoadChatGPTQuota()
		if q == nil {
			q = &auth.ChatGPTQuota{}
		}
		return []quota.Meter{q.ToMeter("")}
	}
	balance = func(ctx context.Context, id string) (quota.Reading, bool) {
		key := ""
		if c := config.Get(); c != nil {
			if p, ok := c.Providers[models.ModelProvider(id)]; ok && !p.Disabled {
				key = p.APIKey
			}
		}
		if key == "" {
			key = os.Getenv(strings.ToUpper(id) + "_API_KEY")
		}
		if key == "" {
			return quota.Reading{}, false
		}
		return quota.Fetch(ctx, id, key), true
	}
	now = time.Now
)

var errNotSignedIn = fmt.Errorf("not signed in to Antigravity")

// CacheFor is how long an allowance reading is reused.
const CacheFor = time.Minute

var (
	cacheMu   sync.Mutex
	cacheAt   time.Time
	cacheFor  models.ModelProvider
	cacheText []string
)

// AllowanceLines says what is left for the provider this session uses, from a
// reading at most CacheFor old.
func AllowanceLines(ctx context.Context) []string {
	p := activeProvider()
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheText != nil && cacheFor == p && now().Sub(cacheAt) < CacheFor {
		return cacheText
	}
	lines := allowance(ctx, p)
	cacheAt, cacheFor, cacheText = now(), p, lines
	return lines
}

func allowance(ctx context.Context, p models.ModelProvider) []string {
	switch {
	case p == models.ProviderAntigravity || p == "":
		meters, plan, err := antigravityMeters(ctx)
		if err != nil {
			return []string{"Antigravity allowance: could not be read (" + err.Error() + ")."}
		}
		head := "Antigravity allowance"
		if plan != "" {
			head += " (" + plan + ")"
		}
		return append([]string{head + ":"}, meterLines(meters)...)
	case p == models.ProviderChatGPT:
		return append([]string{"ChatGPT allowance (from its last reply):"}, meterLines(chatGPTMeters())...)
	}
	for _, id := range quota.Supported() {
		if models.ModelProvider(id) != p {
			continue
		}
		if r, ok := balance(ctx, id); ok {
			if r.Err != "" {
				return []string{r.Provider + " balance: could not be read (" + r.Err + ")."}
			}
			return []string{r.Provider + " balance: " + r.Text + "."}
		}
	}
	return []string{fmt.Sprintf("The %s provider reports no remaining allowance to Gorilla OpenCode; it bills per call (see the cost above).", p)}
}

func meterLines(ms []quota.Meter) []string {
	var out []string
	for _, m := range ms {
		for _, b := range m.Bars {
			// Antigravity names the model family in Provider ("Gemini Models"),
			// one meter per family; it is the part that says WHICH barrel.
			line := "  " + m.Provider + " — " + b.Label + ": "
			switch {
			case b.Text != "":
				line += b.Text
			case b.Remaining == quota.FractionUnknown:
				line += "amount not reported"
			default:
				line += fmt.Sprintf("%d%% left", pct(b.Remaining))
			}
			if r := resetPhrase(b.Reset); r != "" {
				line += " (" + r + ")"
			}
			out = append(out, line)
		}
		if len(m.Bars) == 0 {
			out = append(out, "  "+m.Provider+": no reading yet")
		}
	}
	return out
}

// resetPhrase words an RFC3339 reset time ("resets in 2d 14h"); an already
// worded phrase (ChatGPT's adapter writes one) is passed through.
func resetPhrase(r string) string {
	t, err := time.Parse(time.RFC3339, r)
	if err != nil {
		return r
	}
	d := t.Sub(now())
	switch {
	case d <= 0:
		return "resets now"
	case d >= 24*time.Hour:
		return fmt.Sprintf("resets in %dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("resets in %dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("resets in %dm", int(d.Minutes()))
}

// Report is the whole answer: this session, then what is left.
func Report(ctx context.Context, s session.Session) string {
	var b strings.Builder
	b.WriteString("THIS SESSION (Gorilla OpenCode's own ledger on this computer):\n")
	for _, l := range SessionLines(s) {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("WHAT IS LEFT:\n")
	for _, l := range AllowanceLines(ctx) {
		b.WriteString("  " + strings.TrimPrefix(l, "  ") + "\n")
	}
	b.WriteString("This is everything Gorilla OpenCode knows about usage. Do not search files, databases or settings for more.")
	return b.String()
}
