// Package cost prices one model call from a dated, sourced price table.
//
// GORILLA (2026-10-21): the status bar used to multiply tokens by four flat
// numbers per model (models.Model.CostPer1M*), with no date, no source page,
// no long-prompt tier and two field names that meant the opposite of what they
// said. This package is the Go port of Fieldkit's session-cost kit
// (local/kits/TokenAudit/session_cost.py): the same prices.json, the same
// rules, the same test figures.
//
// prices.json is embedded at build time. There is no network call and no
// remote price list: a price changes when someone re-reads the provider's
// page, edits the file with the new date and sha256, and the next build
// carries it. A model the table does not know is UNPRICED and says so; it is
// never given a neighbour's price.
package cost

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed prices.json
var pricesJSON []byte

// PricesJSON is the embedded table, for the test that keeps it identical to
// the Fieldkit kit's copy.
func PricesJSON() []byte { return pricesJSON }

type tier struct {
	PromptTokens int64    `json:"prompt_tokens"`
	Input        *float64 `json:"input"`
	Output       *float64 `json:"output"`
	CacheRead    *float64 `json:"cache_read"`
	CacheWrite5m *float64 `json:"cache_write_5m"`
	CacheWrite1h *float64 `json:"cache_write_1h"`
}

type entry struct {
	From         string   `json:"from"`
	To           string   `json:"to"`
	Input        float64  `json:"input"`
	Output       float64  `json:"output"`
	CacheRead    float64  `json:"cache_read"`
	CacheWrite5m *float64 `json:"cache_write_5m"`
	CacheWrite1h *float64 `json:"cache_write_1h"`
	Over         *tier    `json:"over"`
	Fast         *tier    `json:"fast"`
}

// Table is one provider's prices and where they were read.
type Table struct {
	Label           string             `json:"label"`
	Source          string             `json:"source"`
	Fetched         string             `json:"fetched"`
	Sha256          string             `json:"sha256"`
	USGeoMultiplier float64            `json:"us_geo_multiplier"`
	Models          map[string][]entry `json:"models"`
}

// Prices is the whole embedded file.
type Prices struct {
	SuffixesStripped []string         `json:"suffixes_stripped"`
	Providers        map[string]Table `json:"providers"`
}

// Rates are US dollars per million tokens.
type Rates struct {
	Input, Output, CacheRead, CacheWrite5m, CacheWrite1h float64
}

var (
	loadOnce sync.Once
	loaded   *Prices
	loadErr  error
)

// Load parses the embedded table once.
func Load() (*Prices, error) {
	loadOnce.Do(func() {
		var p Prices
		loadErr = json.Unmarshal(pricesJSON, &p)
		loaded = &p
	})
	return loaded, loadErr
}

var (
	bracketSuffix = regexp.MustCompile(`\[[^\]]*\]$`)
	dateSuffix    = regexp.MustCompile(`-\d{8}$`)
)

// Canonical turns a provider's model id into the table's name:
// claude-opus-4-6-thinking -> claude-opus-4-6, claude-sonnet-4-5-20250929 ->
// claude-sonnet-4-5, models/gemini-3.8-flash -> gemini-3.8-flash. Only the
// effort/thinking suffixes listed in prices.json are stripped.
func (p *Prices) Canonical(name string) string {
	m := strings.ToLower(strings.TrimSpace(name))
	m = strings.TrimPrefix(m, "models/")
	m = bracketSuffix.ReplaceAllString(m, "")
	m = dateSuffix.ReplaceAllString(m, "")
	for changed := true; changed; {
		changed = false
		for _, s := range p.SuffixesStripped {
			if strings.HasSuffix(m, s) {
				m = strings.TrimSuffix(m, s)
				changed = true
			}
		}
	}
	return m
}

func pick(v *float64, fallback float64) float64 {
	if v != nil {
		return *v
	}
	return fallback
}

// Rate returns the prices for one call, or a reason it has none. day is the
// call's UTC date (YYYY-MM-DD); promptTokens is input + cache reads + cache
// writes, which decides the long-prompt tier.
func (p *Prices) Rate(table, model, day string, promptTokens int64, fast bool) (Rates, string) {
	t, ok := p.Providers[table]
	if !ok {
		return Rates{}, "no price table for provider " + table
	}
	entries := t.Models[model]
	if len(entries) == 0 {
		return Rates{}, "no published price for " + model
	}
	var e *entry
	for i := range entries {
		x := &entries[i]
		if (x.From != "" && day < x.From) || (x.To != "" && day > x.To) {
			continue
		}
		e = x
		break
	}
	if e == nil {
		return Rates{}, "no price for " + model + " on " + day
	}
	r := Rates{Input: e.Input, Output: e.Output, CacheRead: e.CacheRead,
		CacheWrite5m: pick(e.CacheWrite5m, e.Input), CacheWrite1h: pick(e.CacheWrite1h, e.Input)}
	if o := e.Over; o != nil && promptTokens > o.PromptTokens {
		in := pick(o.Input, r.Input)
		r = Rates{Input: in, Output: pick(o.Output, r.Output), CacheRead: pick(o.CacheRead, r.CacheRead),
			CacheWrite5m: pick(o.CacheWrite5m, in), CacheWrite1h: pick(o.CacheWrite1h, in)}
	}
	if fast {
		if e.Fast == nil || e.Fast.Input == nil || e.Input == 0 {
			return Rates{}, "no fast-mode price for " + model
		}
		// The page: caching multipliers apply on top of fast-mode prices.
		f := *e.Fast.Input / e.Input
		r = Rates{Input: *e.Fast.Input, Output: pick(e.Fast.Output, r.Output*f), CacheRead: r.CacheRead * f,
			CacheWrite5m: r.CacheWrite5m * f, CacheWrite1h: r.CacheWrite1h * f}
	}
	return r, ""
}

// Call is one model call's usage. Input excludes cache reads and writes.
type Call struct {
	Table        string // a key of Prices.Providers ("anthropic", "gemini-api", ...)
	Model        string // the provider's model id
	Time         time.Time
	Input        int64
	CacheWrite5m int64
	CacheWrite1h int64
	CacheRead    int64
	Output       int64 // thinking included
	Fast         bool
	USGeo        bool
}

// Result is what one call cost.
type Result struct {
	USD      float64 // the whole call
	Reread   float64 // the part paid for re-sending old context (cache reads)
	Priced   bool
	Reason   string // why not priced
	PricedAs string // the table's model name used
	Context  int64  // tokens sent with the call
}

// Price prices one call.
func (p *Prices) Price(c Call) Result {
	model := p.Canonical(c.Model)
	res := Result{PricedAs: model, Context: c.Input + c.CacheWrite5m + c.CacheWrite1h + c.CacheRead}
	r, why := p.Rate(c.Table, model, c.Time.UTC().Format("2006-01-02"), res.Context, c.Fast)
	if why != "" {
		res.Reason = why
		return res
	}
	mult := 1.0
	if c.USGeo && p.Providers[c.Table].USGeoMultiplier > 0 {
		mult = p.Providers[c.Table].USGeoMultiplier
	}
	reread := float64(c.CacheRead) * r.CacheRead
	usd := float64(c.Input)*r.Input + float64(c.CacheWrite5m)*r.CacheWrite5m +
		float64(c.CacheWrite1h)*r.CacheWrite1h + reread + float64(c.Output)*r.Output
	res.USD = usd * mult / 1e6
	res.Reread = reread * mult / 1e6
	res.Priced = true
	return res
}

// TableFor maps an OpenCode provider and API model id to a price table, or ""
// when the provider has no sourced table (the caller then uses the model
// catalogue's estimate, as before).
//
// antigravity bills nothing per call (a subscription), but what the same calls
// would cost on API credits is the number that shows a user why a long session
// is expensive; Claude there runs on Google Cloud (API_PROVIDER_ANTHROPIC_VERTEX
// in Antigravity's own descriptor), so it is priced from Google's table.
func TableFor(provider, apiModel string) string {
	switch provider {
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini-api"
	case "antigravity":
		if strings.HasPrefix(strings.ToLower(apiModel), "claude") {
			return "google-cloud-claude"
		}
		return "gemini-api"
	}
	return ""
}

// Source says where a table's prices come from, for the status line.
func (p *Prices) Source(table string) (url, fetched string) {
	t := p.Providers[table]
	return t.Source, t.Fetched
}

// ListRates returns the sourced input and output price per million tokens for
// a model BILLED per token, on day now, at the base tier. ok is false for a
// provider with no sourced table, for a subscription (antigravity: nothing is
// billed per call, so a forecast of a bill must stay zero there), and for a
// model the table does not know; the caller then keeps its own numbers.
//
// GORILLA (2026-10-21): the forecasts on the /context screen and the
// model-change dialog used the catalogue's typed numbers while the status bar
// used this table, so the two could disagree about the same model.
func ListRates(provider, apiModel string, now time.Time) (in, out float64, ok bool) {
	var table string
	switch provider {
	case "anthropic":
		table = "anthropic"
	case "gemini":
		table = "gemini-api"
	default:
		return 0, 0, false
	}
	p, err := Load()
	if err != nil {
		return 0, 0, false
	}
	r, why := p.Rate(table, p.Canonical(apiModel), now.UTC().Format("2006-01-02"), 0, false)
	if why != "" {
		return 0, 0, false
	}
	return r.Input, r.Output, true
}
