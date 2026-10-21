package cost

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The dollar figures are the ones Fieldkit's test_session_cost.py checks, worked
// out by hand from prices.json (fetched 2026-10-21).
func TestPriceCases(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	oct := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		call   Call
		usd    float64
		reread float64
		priced bool
	}{
		{"opus 5.5: 1M in x4 + 1M 5-min write x5 + 1M 1-hour write x8 + 1M out x20",
			Call{Table: "anthropic", Model: "claude-opus-5-5", Time: oct, Input: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6, Output: 1e6},
			37, 0, true},
		{"opus 5.5 US-only inference: 10M cache read x0.20 x1.1",
			Call{Table: "anthropic", Model: "claude-opus-5-5", Time: oct, CacheRead: 10e6, USGeo: true},
			2.2, 2.2, true},
		{"opus 5.5 fast mode: 1M in x8 + 1M out x40",
			Call{Table: "anthropic", Model: "claude-opus-5-5", Time: oct, Input: 1e6, Output: 1e6, Fast: true},
			48, 0, true},
		{"haiku 5.5, 50k prompt: 50k x0.10 + 1M out x0.50",
			Call{Table: "anthropic", Model: "claude-haiku-5-5", Time: oct, Input: 50_000, Output: 1e6},
			0.505, 0, true},
		{"haiku 5.5, prompt over 100k: 1M x0.50 + 1M out x2.50",
			Call{Table: "anthropic", Model: "claude-haiku-5-5", Time: oct, Input: 1e6, Output: 1e6},
			3.0, 0, true},
		{"gemini 3.8 flash on 2026-12-31 (promotion): 1M x0.75 + 1M out x3.75 + 10M read x0.075",
			Call{Table: "gemini-api", Model: "gemini-3.8-flash", Time: time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC),
				Input: 1e6, Output: 1e6, CacheRead: 10e6},
			5.25, 0.75, true},
		{"gemini 3.8 flash on 2027-01-01: 1M x1.50 + 1M out x7.50 + 10M read x0.15",
			Call{Table: "gemini-api", Model: "models/gemini-3.8-flash", Time: time.Date(2027, 1, 1, 0, 0, 1, 0, time.UTC),
				Input: 1e6, Output: 1e6, CacheRead: 10e6},
			10.5, 1.5, true},
		{"claude on google cloud, effort suffix stripped: 1M x4 + 1M write x5 + 1M read x0.20 + 1M out x20",
			Call{Table: "google-cloud-claude", Model: "claude-opus-5-5-medium", Time: oct,
				Input: 1e6, CacheWrite5m: 1e6, CacheRead: 1e6, Output: 1e6},
			29.2, 0.2, true},
		{"gemini 3.1 pro preview over 200k: 300k x4 + 100k out x18",
			Call{Table: "gemini-api", Model: "gemini-3.1-pro-preview", Time: oct, Input: 300_000, Output: 100_000},
			3.0, 0, true},
		{"an internal name stays unpriced",
			Call{Table: "gemini-api", Model: "gemini-3.1-pro-low", Time: oct, Input: 300_000, Output: 100_000},
			0, 0, false},
		{"a provider without a table",
			Call{Table: "openrouter", Model: "x", Time: oct, Input: 1},
			0, 0, false},
	}
	for _, c := range cases {
		r := p.Price(c.call)
		if r.Priced != c.priced || math.Abs(r.USD-c.usd) > 1e-9 || math.Abs(r.Reread-c.reread) > 1e-9 {
			t.Errorf("%s: got priced=%v $%.6f reread $%.6f (%s), want priced=%v $%.6f reread $%.6f",
				c.name, r.Priced, r.USD, r.Reread, r.Reason, c.priced, c.usd, c.reread)
		}
	}
}

func TestCanonicalAndTables(t *testing.T) {
	p, _ := Load()
	for in, want := range map[string]string{
		"claude-opus-4-6-thinking":   "claude-opus-4-6",
		"claude-sonnet-5-5-20261001": "claude-sonnet-5-5",
		"claude-opus-5-5[1m]":        "claude-opus-5-5",
		"gemini-3.8-flash-tiered":    "gemini-3.8-flash-tiered",
	} {
		if got := p.Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
	for _, c := range [][3]string{
		{"anthropic", "claude-opus-5-5", "anthropic"},
		{"gemini", "gemini-3.8-flash", "gemini-api"},
		{"antigravity", "claude-opus-4-6-thinking", "google-cloud-claude"},
		{"antigravity", "gemini-3.8-flash", "gemini-api"},
		{"openrouter", "anthropic/claude-opus-5-5", ""},
	} {
		if got := TableFor(c[0], c[1]); got != c[2] {
			t.Errorf("TableFor(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
	for name, tb := range p.Providers {
		if len(tb.Fetched) != 10 || len(tb.Sha256) != 64 || tb.Source == "" {
			t.Errorf("table %s has no source, date or page hash", name)
		}
	}
}

// The embedded table must be the Fieldkit kit's table, byte for byte (line
// endings aside), so the two meters never disagree. Skipped where the kit is
// not installed (a fresh clone, CI).
func TestPricesMatchFieldkitKit(t *testing.T) {
	home, _ := os.UserHomeDir()
	kit := filepath.Join(home, "Documents", "Scripts", "Fieldkit", "local", "kits", "TokenAudit", "prices.json")
	b, err := os.ReadFile(kit)
	if err != nil {
		t.Skip("Fieldkit kit not on this machine: " + kit)
	}
	norm := func(x []byte) []byte { return bytes.ReplaceAll(x, []byte("\r\n"), []byte("\n")) }
	if !bytes.Equal(norm(b), norm(PricesJSON())) {
		t.Fatalf("internal/llm/cost/prices.json differs from %s: copy the newer one over", kit)
	}
}
