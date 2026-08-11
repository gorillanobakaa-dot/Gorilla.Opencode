package quota

import (
	"strings"
	"testing"
)

// Shapes below are from the vendors' API documentation (DeepSeek
// /user/balance, OpenRouter /api/v1/credits), NOT yet live-captured. If a live
// response ever disagrees, capture it and replace the fixture — the struct
// tags must match the wire, or the panel goes blank with a valid key.

func TestParseDeepSeekBalance(t *testing.T) {
	t.Parallel()
	body := []byte(`{
	  "is_available": true,
	  "balance_infos": [
	    {"currency": "CNY", "total_balance": "110.00",
	     "granted_balance": "10.00", "topped_up_balance": "100.00"}
	  ]}`)
	r, err := parseDeepSeek(body)
	if err != nil {
		t.Fatalf("documented shape failed to parse: %v", err)
	}
	if r.Provider != "DeepSeek" || !strings.Contains(r.Text, "110.00 CNY") {
		t.Errorf("bad reading: %+v", r)
	}
	if r.Fraction != FractionUnknown {
		t.Errorf("DeepSeek reports no denominator; fraction must be unknown, got %v", r.Fraction)
	}
}

func TestParseDeepSeekExhausted(t *testing.T) {
	t.Parallel()
	r, err := parseDeepSeek([]byte(`{"is_available": false, "balance_infos": []}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Fraction != 0 {
		t.Errorf("unavailable account must read as empty (fraction 0), got %v", r.Fraction)
	}
	if r.Text == "" {
		t.Error("empty text on an exhausted account — silence and success must never look alike")
	}
}

func TestParseDeepSeekZeroBalance(t *testing.T) {
	t.Parallel()
	body := []byte(`{"is_available": true,
	  "balance_infos": [{"currency": "USD", "total_balance": "0.00"}]}`)
	r, err := parseDeepSeek(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Fraction != 0 {
		t.Errorf("zero balance must read as empty, got fraction %v", r.Fraction)
	}
}

func TestParseOpenRouterCredits(t *testing.T) {
	t.Parallel()
	r, err := parseOpenRouter([]byte(`{"data": {"total_credits": 10.0, "total_usage": 6.5}}`))
	if err != nil {
		t.Fatalf("documented shape failed to parse: %v", err)
	}
	if r.Fraction < 0.3499 || r.Fraction > 0.3501 {
		t.Errorf("6.5 used of 10 should leave fraction 0.35, got %v", r.Fraction)
	}
	if !strings.Contains(r.Text, "$3.50 of $10.00") {
		t.Errorf("text should show remaining of total: %q", r.Text)
	}
}

func TestParseOpenRouterFreeTier(t *testing.T) {
	t.Parallel()
	r, err := parseOpenRouter([]byte(`{"data": {"total_credits": 0, "total_usage": 0}}`))
	if err != nil {
		t.Fatal(err)
	}
	// A wallet that was never filled is not an empty tank: no red bar.
	if r.Fraction != FractionUnknown {
		t.Errorf("free tier must not render as exhausted, got fraction %v", r.Fraction)
	}
	if !r.FreeTier {
		t.Error("free tier must be marked so the panel words it as no-barrel, not full-barrel")
	}
	if !strings.Contains(r.Text, "free models") {
		t.Errorf("free tier should say what it means: %q", r.Text)
	}
}

func TestParseOpenRouterOverspend(t *testing.T) {
	t.Parallel()
	// Usage can exceed credits (negative balance). Clamp at empty, never a
	// negative-width bar.
	r, err := parseOpenRouter([]byte(`{"data": {"total_credits": 10.0, "total_usage": 11.2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Fraction != 0 {
		t.Errorf("overspent account must clamp to fraction 0, got %v", r.Fraction)
	}
}

func TestFetchUnknownProviderRefuses(t *testing.T) {
	t.Parallel()
	r := Fetch(t.Context(), "grok", "key")
	if r.Err == "" {
		t.Error("unknown provider must return an error reading, not invent a meter")
	}
}
