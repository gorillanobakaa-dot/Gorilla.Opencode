package auth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Measured 2026-10-19: loadCodeAssist on daily-cloudcode-pa for a Google AI Pro
// account (trimmed: privacy notice text and URLs left out). currentTier is
// "free-tier" exactly as for a free account; only paidTier says Pro. The build
// before this read currentTier alone and told the paying owner he was free.
const loadProAccount = `{
  "currentTier": {"id": "free-tier", "name": "Antigravity"},
  "allowedTiers": [
    {"id": "free-tier", "name": "Antigravity", "isDefault": true},
    {"id": "standard-tier", "name": "Antigravity", "userDefinedCloudaicompanionProject": true}
  ],
  "cloudaicompanionProject": "aicode-consumers",
  "gcpManaged": false,
  "paidTier": {"id": "g1-pro-tier", "name": "Google AI Pro", "description": "Google AI Pro"}
}`

// The same answer with no paid plan: what the free-tier users this program is
// built for get. It must come out exactly as before: free tier, same project.
const loadFreeAccount = `{
  "currentTier": {"id": "free-tier", "name": "Antigravity"},
  "allowedTiers": [{"id": "free-tier", "name": "Antigravity", "isDefault": true}],
  "cloudaicompanionProject": "aicode-consumers",
  "gcpManaged": false
}`

func planOf(t *testing.T, raw string) *AntigravityCreds {
	t.Helper()
	var load loadCodeAssistResp
	if err := json.Unmarshal([]byte(raw), &load); err != nil {
		t.Fatal(err)
	}
	c := &AntigravityCreds{ProjectID: load.CloudaicompanionProject}
	c.applyPlan(load, time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC))
	return c
}

func TestAProAccountIsCalledPro(t *testing.T) {
	c := planOf(t, loadProAccount)
	if c.PlanID != "g1-pro-tier" || c.PlanName != "Google AI Pro" || c.PlanLabel() != "Google AI Pro" {
		t.Fatalf("a Google AI Pro account read as %q / %q / label %q", c.PlanID, c.PlanName, c.PlanLabel())
	}
	if c.Tier != "free-tier" || c.ProjectID != "aicode-consumers" {
		t.Fatalf("the tier and project must be what Google sent: %q %q", c.Tier, c.ProjectID)
	}
}

func TestAFreeAccountStaysFree(t *testing.T) {
	c := planOf(t, loadFreeAccount)
	if c.PlanID != "" || c.PlanName != "" || c.PlanLabel() != "free tier" {
		t.Fatalf("a free account read as plan %q / %q / label %q", c.PlanID, c.PlanName, c.PlanLabel())
	}
	if c.Tier != "free-tier" || c.ProjectID != "aicode-consumers" {
		t.Fatalf("free path changed: tier %q project %q", c.Tier, c.ProjectID)
	}
}

// A plan that ends is forgotten at the next reading, never kept from before.
func TestACancelledPlanIsForgotten(t *testing.T) {
	c := planOf(t, loadProAccount)
	var load loadCodeAssistResp
	_ = json.Unmarshal([]byte(loadFreeAccount), &load)
	c.applyPlan(load, time.Now())
	if c.PlanLabel() != "free tier" {
		t.Fatalf("a cancelled plan is still shown: %q", c.PlanLabel())
	}
}

// A login saved by an older build has a project but no plan reading: it must be
// read once, and again after a day, so a bought or cancelled plan shows up.
func TestThePlanIsReadForOldLoginsAndRecheckedDaily(t *testing.T) {
	now := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	old := &AntigravityCreds{ProjectID: "aicode-consumers"}
	if !old.NeedsSetup(now) {
		t.Fatal("a login with no plan reading must be set up again")
	}
	fresh := &AntigravityCreds{ProjectID: "aicode-consumers", Tier: "free-tier", PlanChecked: now.Add(-time.Hour)}
	if fresh.NeedsSetup(now) {
		t.Fatal("a reading an hour old must not be repeated")
	}
	stale := &AntigravityCreds{ProjectID: "aicode-consumers", Tier: "free-tier", PlanChecked: now.Add(-25 * time.Hour)}
	if !stale.NeedsSetup(now) {
		t.Fatal("a reading over a day old must be repeated")
	}
}

func TestTheUsageLineNamesThePlan(t *testing.T) {
	q := &QuotaSummary{Groups: []QuotaGroup{{DisplayName: "Gemini Models",
		Buckets: []QuotaBucket{{RemainingFraction: 0.76}}}}}
	got := FormatQuotaLineFor(q, time.Now(), "Google AI Pro")
	if !strings.HasPrefix(got, "Antigravity (Google AI Pro) weekly quota — Gemini Models: 76%") {
		t.Fatalf("usage line: %q", got)
	}
	if got := FormatQuotaLine(q, time.Now()); !strings.HasPrefix(got, "Antigravity weekly quota — ") {
		t.Fatalf("the plan-less line changed: %q", got)
	}
}
