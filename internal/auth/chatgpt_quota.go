package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GORILLA OVERRIDE (2026-08-23): the ChatGPT sign-in DOES publish a usage meter,
// and we were showing nothing.
//
// WHY THIS EXISTS. /usage on a ChatGPT session used to print the ANTIGRAVITY
// weekly quota plus a Google account email — the wrong barrel entirely. Gating
// that off (same day) removed the lie but left a blank, while the user sat at
// 20% of a monthly limit with no way to see it from inside the program.
//
// THERE IS NO ENDPOINT TO CALL. Established from the Codex reference client at
// ~/Downloads/codex-rust-v0.147.0 (codex-rs/codex-api/src/rate_limits.rs): the
// numbers ride the responses we already make, as `x-codex-*` headers. So this
// costs ZERO extra requests, which is the §8-correct outcome on a metered link.
//
// TWO TRAPS, both encoded below:
//  1. The wire field is `used_percent`, NOT remaining. This project's panel
//     reports REMAINING on purpose (see the GORILLA OVERRIDE at
//     internal/tui/quota_panel.go:16, written so a user cannot confuse 96% left
//     with 96% spent). Inverting it recreates exactly the error that override
//     exists to prevent, so the conversion lives in ONE place: Remaining().
//  2. The window name and length come from the WIRE, never a constant. Codex
//     renders "Monthly limit" on a free plan, not the weekly pair a paid plan
//     shows. Hardcoding "weekly" mislabels every tier it was not written
//     against, and looks correct on whichever account the developer used.
//
// The reading is persisted because it is traffic-driven: there is nothing to
// show before the session's first request, and "not known yet" must be
// distinguishable from "no meter exists".

// ChatGPTWindow is one rate-limit window as the backend reports it.
type ChatGPTWindow struct {
	// UsedPercent is the wire value: percent CONSUMED, 0-100.
	UsedPercent float64 `json:"used_percent"`
	// WindowMinutes is the window length the backend declares. 0 when absent.
	WindowMinutes int64 `json:"window_minutes,omitempty"`
	// ResetAt is a unix timestamp; 0 when the backend did not say.
	ResetAt int64 `json:"reset_at,omitempty"`
}

// Remaining converts the wire's used-percent into the REMAINING fraction the
// banana renderer expects (0..1). The single conversion point on purpose.
func (w ChatGPTWindow) Remaining() float64 {
	// (100-used)/100, NOT 1-used/100. The second form is off by one ULP at every
	// round percentage: 1-80/100 is 0.19999999999999996, which falls BELOW the
	// banana ladder's 0.20 boundary and reports "Banana emergency" to someone
	// with exactly a fifth of their allowance left. Every alert boundary (80,
	// 85, 90, 95% used) sat one tier too alarming. Caught by
	// TestChatGPTMeterRendersWithBananaLadder, 2026-08-23.
	r := (100 - w.UsedPercent) / 100
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// Window-length buckets, ported verbatim from the Codex reference client
// (codex-rs/tui/src/chatwidget/rate_limits.rs, get_limits_duration).
//
// The first version of this function was INVENTED — threshold ranges with an
// "Hourly" bucket that does not exist, and no 5h bucket, which is the single
// most common window a ChatGPT plan actually reports. It would have labelled a
// 5-hour limit "Hourly" and a 200-minute window "Daily". Read the reference
// instead of guessing; the tree is at ~/Downloads/codex-rust-v0.147.0.
const (
	minutesPerHour    = 60
	minutesPer5Hours  = 5 * minutesPerHour
	minutesPerDay     = 24 * minutesPerHour
	minutesPerWeek    = 7 * minutesPerDay
	minutesPerMonth   = 30 * minutesPerDay
	minutesPerYear    = 365 * minutesPerDay
	primaryFallback   = "usage"
	secondaryFallback = "secondary usage"
)

// isApproximateWindow matches Codex's tolerance exactly: within 5% either way.
// A window that matches nothing gets the fallback word, never a nearest guess.
func isApproximateWindow(minutes, expected int64) bool {
	m, e := float64(minutes), float64(expected)
	return m >= e*0.95 && m <= e*1.05
}

// durationWord returns Codex's own word for the window length, or "" when the
// declared length matches no known bucket.
func durationWord(minutes int64) string {
	if minutes < 0 {
		minutes = 0
	}
	switch {
	case isApproximateWindow(minutes, minutesPer5Hours):
		return "5h"
	case isApproximateWindow(minutes, minutesPerDay):
		return "daily"
	case isApproximateWindow(minutes, minutesPerWeek):
		return "weekly"
	case isApproximateWindow(minutes, minutesPerMonth):
		return "monthly"
	case isApproximateWindow(minutes, minutesPerYear):
		return "annual"
	default:
		return ""
	}
}

// Label names the window in the backend's own terms. secondary selects the
// fallback wording for the second window, matching Codex.
func (w ChatGPTWindow) Label(secondary bool) string {
	word := durationWord(w.WindowMinutes)
	if word == "" {
		word = primaryFallback
		if secondary {
			word = secondaryFallback
		}
	}
	// Codex capitalises the first letter and appends "limit".
	return strings.ToUpper(word[:1]) + word[1:] + " limit"
}

// ChatGPTQuota is the last reading seen on a response from the ChatGPT backend.
type ChatGPTQuota struct {
	Primary   *ChatGPTWindow `json:"primary,omitempty"`
	Secondary *ChatGPTWindow `json:"secondary,omitempty"`
	// LimitName is the backend's own name for the limit, when it sends one.
	LimitName string `json:"limit_name,omitempty"`
	// PlanType is shown beside the account, as Codex does ("(Free)").
	PlanType string `json:"plan_type,omitempty"`
	// SeenAt records when this reading was taken, so a stale number can say so.
	SeenAt int64 `json:"seen_at,omitempty"`
}

// Empty reports whether the reading carries no usable window.
func (q *ChatGPTQuota) Empty() bool {
	return q == nil || (q.Primary == nil && q.Secondary == nil)
}

func headerFloat(h http.Header, name string) (float64, bool) {
	v := strings.TrimSpace(h.Get(name))
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil
}

func headerInt(h http.Header, name string) int64 {
	v := strings.TrimSpace(h.Get(name))
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func parseWindow(h http.Header, prefix string) *ChatGPTWindow {
	used, ok := headerFloat(h, prefix+"-used-percent")
	if !ok {
		return nil
	}
	return &ChatGPTWindow{
		UsedPercent:   used,
		WindowMinutes: headerInt(h, prefix+"-window-minutes"),
		ResetAt:       headerInt(h, prefix+"-reset-at"),
	}
}

// ParseChatGPTQuota reads the x-codex-* rate-limit family off a response.
// Returns nil when the backend sent no usage headers at all.
func ParseChatGPTQuota(h http.Header) *ChatGPTQuota {
	q := &ChatGPTQuota{
		Primary:   parseWindow(h, "x-codex-primary"),
		Secondary: parseWindow(h, "x-codex-secondary"),
		LimitName: strings.TrimSpace(h.Get("x-codex-limit-name")),
	}
	if q.Empty() {
		return nil
	}
	q.SeenAt = time.Now().Unix()
	return q
}

// ChatGPTQuotaPath returns ~/.config/gorilla-opencode/chatgpt-quota.json.
func ChatGPTQuotaPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gorilla-opencode", "chatgpt-quota.json")
}

// LoadChatGPTQuota reads the last stored reading, or (nil, nil) if none.
func LoadChatGPTQuota() (*ChatGPTQuota, error) {
	data, err := os.ReadFile(ChatGPTQuotaPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var q ChatGPTQuota
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// Save writes the reading atomically. Best-effort by design: failing to record
// a usage number must never break the request that carried it.
func (q *ChatGPTQuota) Save() error {
	path := ChatGPTQuotaPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RecordChatGPTQuota parses a response's usage headers and stores them when
// present. Silent on every failure path: this is observability, not plumbing
// the user asked for.
func RecordChatGPTQuota(h http.Header) {
	if q := ParseChatGPTQuota(h); q != nil {
		_ = q.Save()
	}
}
