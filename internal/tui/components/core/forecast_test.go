package core

import (
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/session"
)

// The Air's session (2026-10-28): Sonnet 5, 19.4K tokens, 5 cents spent.
// The chip says what the next message can cost and, after a minute, the pace.
func TestPaidForecastOnTheAirsSession(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	m := models.Model{Provider: models.ProviderAnthropic, APIModel: "claude-sonnet-5"}
	s := session.Session{Cost: 0.05, CreatedAt: now.Add(-30 * time.Minute).Unix()}
	got := paidForecast(s, m, 19_400, now)
	if got != "next up to $0.055, $0.10/h" {
		t.Errorf("forecast = %q", got)
	}
	s.CreatedAt = now.Add(-20 * time.Second).Unix()
	if got := paidForecast(s, m, 19_400, now); strings.Contains(got, "/h") {
		t.Errorf("a pace after 20 seconds is noise: %q", got)
	}
}

// A model with no list price (free providers) shows nothing.
func TestNoForecastWithoutAPrice(t *testing.T) {
	m := models.Model{Provider: models.ProviderLocal, APIModel: "llama"}
	if got := paidForecast(session.Session{Cost: 1}, m, 50_000, time.Now()); got != "" {
		t.Errorf("unpriced model got %q", got)
	}
}
