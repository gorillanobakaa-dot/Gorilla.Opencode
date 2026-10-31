package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA OVERRIDE (2026-10-30, plan P3c): a page whose HTTP body has no
// readable text is usually built by JavaScript. When `fieldkit` is installed,
// `fieldkit web fetch URL --json` renders it in a browser already on this
// machine (throwaway profile, no telemetry, no downloads).
//
// Threat model: the subprocess is handed the URL as one argv element and
// nothing else (no cookies, headers or session data from this program). What
// it prints is a fetched page: it goes through the same fence as the HTTP path.

const (
	browserFallbackTimeout = 45 // seconds, passed to fieldkit and enforced here
	browserFallbackGrace   = 10 * time.Second
	browserFallbackEnvVar  = "GORILLA_OPENCODE_WEB_FETCH_BROWSER"
	browserFallbackMaxOut  = 8 << 20
	// Under this many characters of HTTP text the page is treated as a
	// script-only shell (fieldkit/web/rule.py MIN_TEXT_CHARS, the same number).
	browserFallbackMinChars = 200
)

type browserFetchResult struct {
	OK       bool   `json:"ok"`
	Via      string `json:"via"`
	Reason   string `json:"reason"`
	Error    string `json:"error"`
	Browser  string `json:"browser"`
	Title    string `json:"title"`
	FinalURL string `json:"final_url"`
	Text     string `json:"text"`
}

// Seams for tests. Production code never reassigns them.
var (
	newFetchClient   = newSafeClient
	fieldkitLookPath = exec.LookPath
	// runFieldkitFetch runs the fieldkit binary at path and returns stdout.
	runFieldkitFetch = func(ctx context.Context, path, target string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, path, "web", "fetch", target, "--json",
			"--timeout", fmt.Sprint(browserFallbackTimeout))
		var out bytes.Buffer
		cmd.Stdout = &limitedWriter{w: &out, left: browserFallbackMaxOut}
		var errb bytes.Buffer
		cmd.Stderr = &limitedWriter{w: &errb, left: 4096}
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return out.Bytes(), fmt.Errorf("exit code %d", ee.ExitCode())
			}
			return nil, err
		}
		return out.Bytes(), nil
	}

	fieldkitOnce sync.Once
	fieldkitPath string
	fieldkitErr  error
)

// limitedWriter keeps at most `left` bytes and silently drops the rest.
type limitedWriter struct {
	w    *bytes.Buffer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if l.left <= 0 {
		return n, nil
	}
	if len(p) > l.left {
		p = p[:l.left]
	}
	l.w.Write(p)
	l.left -= len(p)
	return n, nil
}

// browserFallbackEnabled reports whether the user left the fallback on.
func browserFallbackEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(browserFallbackEnvVar))) {
	case "0", "off", "false", "no":
		return false
	}
	cfg := config.Get()
	return cfg == nil || !cfg.WebFetchBrowserOff
}

// fieldkitBinary looks fieldkit up on PATH once per process.
func fieldkitBinary() (string, error) {
	fieldkitOnce.Do(func() { fieldkitPath, fieldkitErr = fieldkitLookPath("fieldkit") })
	return fieldkitPath, fieldkitErr
}

// browserFetch renders target in a local browser. On success it returns the
// text and the browser name; otherwise a one-line reason it failed.
func browserFetch(ctx context.Context, target string) (text, browser string, reason string) {
	if !browserFallbackEnabled() {
		return "", "", "" // switched off: nothing was tried, nothing to say
	}
	path, err := fieldkitBinary()
	if err != nil {
		return "", "", "fieldkit is not installed"
	}
	ctx, cancel := context.WithTimeout(ctx, browserFallbackTimeout*time.Second+browserFallbackGrace)
	defer cancel()
	raw, err := runFieldkitFetch(ctx, path, target)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", "", "timeout"
		}
		if errors.Is(err, context.Canceled) {
			return "", "", "cancelled"
		}
		// fieldkit prints its refusal as JSON before exiting non-zero.
		var res browserFetchResult
		if json.Unmarshal(raw, &res) == nil {
			if d := strings.TrimSpace(res.Error + res.Reason); d != "" {
				return "", "", err.Error() + ": " + d
			}
		}
		return "", "", err.Error()
	}
	var res browserFetchResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", "", "unreadable output: " + err.Error()
	}
	if !res.OK {
		r := strings.TrimSpace(res.Error + res.Reason)
		if r == "" {
			r = "no reason given"
		}
		return "", "", "ok=false: " + r
	}
	// The browser follows redirects without this program's address guard.
	if res.FinalURL != "" {
		if why := blockedFetchTarget(res.FinalURL); why != "" {
			return "", "", "the browser ended on a refused address: " + why
		}
	}
	if strings.TrimSpace(res.Text) == "" {
		return "", "", "ok=true but the browser found no text either"
	}
	return res.Text, res.Browser, ""
}
