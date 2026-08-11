// Package quota reads "how much do I have left" from providers that expose a
// balance endpoint on an ordinary API key. Antigravity's weekly quota lives in
// internal/auth (it rides the OAuth credentials); this package covers the paid
// providers a user brings their own key for.
//
// Only providers with a REAL endpoint are here. Anthropic, OpenAI, xAI and
// Groq expose nothing wallet-shaped to a plain key (rate-limit headers only),
// and a meter that guesses is worse than no meter — silence and success must
// never look alike, and neither may "unknown" and "plenty left".
package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Reading is one provider's answer, normalised just enough to render.
type Reading struct {
	Provider string  // display name: "DeepSeek", "OpenRouter"
	Text     string  // human line: "110.00 CNY available"
	Fraction float64 // remaining fraction 0..1, or FractionUnknown
	// FreeTier marks an account with nothing purchased: there is no barrel to
	// run down, which is a different fact from "barrel full" and from "barrel
	// empty" — the renderer words it accordingly.
	FreeTier bool
	Err      string // non-empty when the fetch failed; Text is then empty
}

// FractionUnknown marks a balance with no denominator: DeepSeek reports money
// left but not what you started with, so a percentage bar cannot be drawn
// honestly. Render the amount, not an invented percentage.
const FractionUnknown = -1

// fetchTimeout bounds each balance call. Generous because the reference
// connection is high-latency (§8), bounded because /usage must never hang the
// goroutine forever.
const fetchTimeout = 10 * time.Second

// ---- DeepSeek --------------------------------------------------------------

// Wire shape from the DeepSeek API docs (GET /user/balance). NOTE: the numbers
// are STRINGS on the wire. Shape from vendor documentation, not yet confirmed
// against a live capture — if the panel ever shows a blank DeepSeek section
// with a valid key, capture the real body and fix the tags (the antigravity
// quota test records how).
type deepSeekBalance struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency     string `json:"currency"`
		TotalBalance string `json:"total_balance"`
	} `json:"balance_infos"`
}

func parseDeepSeek(body []byte) (*Reading, error) {
	var b deepSeekBalance
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	r := &Reading{Provider: "DeepSeek", Fraction: FractionUnknown}
	if !b.IsAvailable {
		r.Fraction = 0
		r.Text = "balance exhausted or account unavailable"
		return r, nil
	}
	if len(b.BalanceInfos) == 0 {
		return nil, fmt.Errorf("deepseek: is_available with no balance_infos")
	}
	for i, bi := range b.BalanceInfos {
		if i > 0 {
			r.Text += " + "
		}
		r.Text += bi.TotalBalance + " " + bi.Currency
		if v, err := strconv.ParseFloat(bi.TotalBalance, 64); err == nil && v <= 0 {
			r.Fraction = 0
		}
	}
	r.Text += " available"
	return r, nil
}

// ---- OpenRouter ------------------------------------------------------------

// Wire shape from the OpenRouter API docs (GET /api/v1/credits). Confirmed
// against the live API 2026-08-11 with a real key (free-tier response:
// total_credits 0, total_usage 0 — the paid path is doc-only, like DeepSeek).
type openRouterCredits struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

func parseOpenRouter(body []byte) (*Reading, error) {
	var c openRouterCredits
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, err
	}
	r := &Reading{Provider: "OpenRouter"}
	total, used := c.Data.TotalCredits, c.Data.TotalUsage
	remaining := total - used
	if remaining < 0 {
		remaining = 0
	}
	switch {
	case total <= 0:
		// Free-tier key: nothing purchased, free models only. That is a state,
		// not an empty tank — no red bar for a wallet that was never filled.
		r.Fraction = FractionUnknown
		r.FreeTier = true
		r.Text = "no credits purchased — free models only"
	default:
		r.Fraction = remaining / total
		r.Text = fmt.Sprintf("$%.2f of $%.2f credits left", remaining, total)
	}
	return r, nil
}

// ---- fetch plumbing --------------------------------------------------------

type endpoint struct {
	provider string
	url      string
	parse    func([]byte) (*Reading, error)
}

var endpoints = map[string]endpoint{
	"deepseek":   {"DeepSeek", "https://api.deepseek.com/user/balance", parseDeepSeek},
	"openrouter": {"OpenRouter", "https://openrouter.ai/api/v1/credits", parseOpenRouter},
}

// Fetch queries one provider's balance endpoint with its API key. The error is
// folded into the Reading (Err field) rather than returned: /usage renders
// every configured provider, and a failed fetch must appear as a failed fetch,
// not vanish from the panel.
func Fetch(ctx context.Context, providerID, apiKey string) Reading {
	ep, ok := endpoints[providerID]
	if !ok {
		return Reading{Provider: providerID, Err: "no balance endpoint known"}
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.url, nil)
	if err != nil {
		return Reading{Provider: ep.provider, Err: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Reading{Provider: ep.provider, Err: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Reading{Provider: ep.provider, Err: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		// Status line only — the body of an auth error can echo the key back.
		return Reading{Provider: ep.provider, Err: "HTTP " + resp.Status}
	}
	r, err := ep.parse(body)
	if err != nil {
		return Reading{Provider: ep.provider, Err: err.Error()}
	}
	return *r
}

// Supported lists the provider IDs this package can meter, in render order.
func Supported() []string {
	return []string{"deepseek", "openrouter"}
}
