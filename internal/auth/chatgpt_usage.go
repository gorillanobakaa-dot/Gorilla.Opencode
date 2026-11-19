package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatGPTUsageWindow struct {
	UsedPercent *float64 `json:"used_percent"`
	Seconds     int64    `json:"limit_window_seconds"`
	ResetAt     int64    `json:"reset_at"`
}

func (w *chatGPTUsageWindow) window() *ChatGPTWindow {
	if w == nil || w.UsedPercent == nil || *w.UsedPercent < 0 || *w.UsedPercent > 100 {
		return nil
	}
	if *w.UsedPercent == 0 && w.Seconds <= 0 && w.ResetAt <= 0 {
		return nil
	}
	return &ChatGPTWindow{UsedPercent: *w.UsedPercent, WindowMinutes: w.Seconds / 60, ResetAt: w.ResetAt}
}

type chatGPTUsageLimit struct {
	Primary   *chatGPTUsageWindow `json:"primary_window"`
	Secondary *chatGPTUsageWindow `json:"secondary_window"`
}

func decodeChatGPTUsage(raw []byte) (*ChatGPTQuota, string, error) {
	var payload struct {
		AccountID  string            `json:"account_id"`
		PlanType   string            `json:"plan_type"`
		RateLimit  chatGPTUsageLimit `json:"rate_limit"`
		Credits    *ChatGPTCredits   `json:"credits"`
		Additional []struct {
			Name      string            `json:"limit_name"`
			Feature   string            `json:"metered_feature"`
			RateLimit chatGPTUsageLimit `json:"rate_limit"`
		} `json:"additional_rate_limits"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", fmt.Errorf("invalid ChatGPT usage response")
	}
	q := &ChatGPTQuota{
		PlanType: payload.PlanType, Credits: payload.Credits, SeenAt: time.Now().Unix(),
		Primary: payload.RateLimit.Primary.window(), Secondary: payload.RateLimit.Secondary.window(),
	}
	for _, limit := range payload.Additional {
		name := strings.TrimSpace(limit.Name)
		if name == "" {
			name = strings.TrimSpace(limit.Feature)
		}
		named := ChatGPTQuota{LimitName: name, PlanType: q.PlanType,
			Primary: limit.RateLimit.Primary.window(), Secondary: limit.RateLimit.Secondary.window()}
		if !named.Empty() && name != "" {
			q.Additional = append(q.Additional, named)
		}
	}
	if q.Empty() {
		return nil, "", fmt.Errorf("ChatGPT supplied no account limits or credits")
	}
	return q, payload.AccountID, nil
}

// RetrieveUsage reads the signed-in account's limits on explicit /usage.
func (c *ChatGPTCreds) RetrieveUsage(ctx context.Context) (*ChatGPTQuota, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	token, err := c.Ensure(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not authenticate ChatGPT usage")
	}
	url := strings.TrimSuffix(ChatGPTBackend, "/codex") + "/wham/usage"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range c.AuthHeaders(token) {
		req.Header.Set(key, value)
	}
	req.Header.Set("Accept", "application/json")
	client := authHTTP()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach ChatGPT account usage")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ChatGPT account usage returned HTTP %d", resp.StatusCode)
	}
	const maxBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil || len(raw) > maxBytes {
		return nil, fmt.Errorf("could not read ChatGPT account usage")
	}
	q, accountID, err := decodeChatGPTUsage(raw)
	if err != nil {
		return nil, err
	}
	if accountID != "" && c.AccountID != "" && accountID != c.AccountID {
		return nil, fmt.Errorf("ChatGPT returned usage for a different account")
	}
	if q.PlanType == "" {
		q.PlanType = c.PlanType
	}
	_ = q.save(c, true)
	return q, nil
}
