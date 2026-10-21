package usage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/quota"
	"github.com/opencode-ai/opencode/internal/session"
)

// stub replaces the readers for one test and counts the network reads.
func stub(t *testing.T, p models.ModelProvider) *int {
	t.Helper()
	reads := 0
	oldP, oldA, oldNow := activeProvider, antigravityMeters, now
	clock := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	activeProvider = func() models.ModelProvider { return p }
	antigravityMeters = func(context.Context) ([]quota.Meter, string, error) {
		reads++
		return []quota.Meter{{Provider: "Gemini Models", Account: "someone@example.org", Kind: quota.KindWindowQuota,
			Bars: []quota.Bar{{Label: "Weekly Limit", Remaining: 0.96, Reset: "resets in 5d"}}}}, "Google AI Pro", nil
	}
	now = func() time.Time { return clock }
	cacheMu.Lock()
	cacheText = nil
	cacheMu.Unlock()
	t.Cleanup(func() { activeProvider, antigravityMeters, now = oldP, oldA, oldNow })
	return &reads
}

// The measured session, answered in one call: cost, re-sent share, what is left.
func TestReportAnswersTheBananasQuestion(t *testing.T) {
	stub(t, models.ProviderAntigravity)
	s := session.Session{Cost: 0.39, CostReread: 0.008, CallsPriced: 33, LastCallCost: 0.016,
		PromptTokens: 20_774, CompletionTokens: 33, CumulativePromptTokens: 596_340, ResentTokens: 536_706}
	got := Report(context.Background(), s)
	for _, want := range []string{"$0.39", "33 priced", "90% of this session's input", "2% of the cost",
		"Gemini Models — Weekly Limit: 96% left (resets in 5d)", "Google AI Pro", "Do not search files"} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "someone@example.org") {
		t.Errorf("the account e-mail must not be handed to the model:\n%s", got)
	}
}

// A model calling in a loop must not turn the tool into traffic.
func TestAllowanceIsCachedForAMinute(t *testing.T) {
	reads := stub(t, models.ProviderAntigravity)
	ctx := context.Background()
	AllowanceLines(ctx)
	AllowanceLines(ctx)
	if *reads != 1 {
		t.Fatalf("two calls inside a minute made %d reads, want 1", *reads)
	}
	base := now()
	now = func() time.Time { return base.Add(CacheFor + time.Second) }
	AllowanceLines(ctx)
	if *reads != 2 {
		t.Fatalf("after a minute the reading must be refreshed: %d reads", *reads)
	}
}

// Only the provider in use is read; others are not contacted.
func TestOnlyTheActiveProviderIsRead(t *testing.T) {
	reads := stub(t, models.ProviderAnthropic)
	got := strings.Join(AllowanceLines(context.Background()), "\n")
	if *reads != 0 {
		t.Fatalf("an Anthropic session read the Antigravity quota %d time(s)", *reads)
	}
	if !strings.Contains(got, "bills per call") {
		t.Errorf("anthropic allowance = %q", got)
	}
}

func TestAReadFailureIsSaidNotHidden(t *testing.T) {
	stub(t, models.ProviderAntigravity)
	antigravityMeters = func(context.Context) ([]quota.Meter, string, error) { return nil, "", errors.New("offline") }
	if got := strings.Join(AllowanceLines(context.Background()), "\n"); !strings.Contains(got, "could not be read (offline)") {
		t.Errorf("failure line = %q", got)
	}
}

func TestEmptySession(t *testing.T) {
	if got := SessionLines(session.Session{}); len(got) != 1 || !strings.Contains(got[0], "No model call") {
		t.Errorf("empty session = %q", got)
	}
}

func TestResetTimesAreWorded(t *testing.T) {
	stub(t, models.ProviderAntigravity)
	for in, want := range map[string]string{
		"2026-10-24T09:42:32Z": "resets in 2d 21h",
		"2026-10-21T20:20:12Z": "resets in 8h 20m",
		"2026-10-21T12:05:00Z": "resets in 5m",
		"2026-10-20T00:00:00Z": "resets now",
		"resets in 23d":        "resets in 23d",
	} {
		if got := resetPhrase(in); got != want {
			t.Errorf("resetPhrase(%q) = %q, want %q", in, got, want)
		}
	}
}
