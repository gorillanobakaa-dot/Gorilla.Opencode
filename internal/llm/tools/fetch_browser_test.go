package tools

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/politehttp"
)

// These tests fake the fieldkit binary (runFieldkitFetch / fieldkitLookPath):
// no real browser, no network beyond a loopback test server. They do not use
// t.Parallel because they swap package-level seams.

const jsOnlyURL = "http://js-only.test/app"

const emptyShell = `<html><head><script>document.body.innerHTML="x"</script></head><body></body></html>`

type fkCounts struct{ looks, runs int }

// withFieldkit installs fakes and returns the call counters.
func withFieldkit(t *testing.T, look func(string) (string, error),
	run func(ctx context.Context, path, target string) ([]byte, error)) *fkCounts {
	t.Helper()
	c := &fkCounts{}
	oldLook, oldRun, oldClient := fieldkitLookPath, runFieldkitFetch, newFetchClient
	fieldkitOnce = sync.Once{}
	fieldkitLookPath = func(n string) (string, error) { c.looks++; return look(n) }
	runFieldkitFetch = func(ctx context.Context, p, target string) ([]byte, error) {
		c.runs++
		return run(ctx, p, target)
	}
	t.Cleanup(func() {
		fieldkitLookPath, runFieldkitFetch, newFetchClient = oldLook, oldRun, oldClient
		fieldkitOnce = sync.Once{}
	})
	return c
}

// serveHTML makes jsOnlyURL reach a loopback server serving body. Only the
// dialer is redirected; the URL check and redirect guard stay in the path.
func serveHTML(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	addr := srv.Listener.Addr().String()
	newFetchClient = func(timeout time.Duration) *http.Client {
		c := newSafeClient(timeout)
		polite := c.Transport.(*politehttp.Transport)
		tr := polite.Base.(*http.Transport).Clone()
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		}
		polite.Base = tr
		return c
	}
}

func fetchJS(t *testing.T) ToolResponse {
	t.Helper()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	resp, err := NewFetchTool(&recordingPermissions{grant: true}).Run(ctx, ToolCall{
		Name: FetchToolName, Input: `{"url":"` + jsOnlyURL + `"}`})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func okJSON(text string) []byte {
	return []byte(`{"ok":true,"via":"browser","browser":"firefox","title":"t","final_url":"` +
		jsOnlyURL + `","text":"` + text + `"}`)
}

func fkFound(string) (string, error) { return "/bin/fieldkit", nil }

func TestBrowserFallbackUsedOnEmptyPage(t *testing.T) {
	serveHTML(t, emptyShell)
	var gotTarget string
	withFieldkit(t, fkFound, func(_ context.Context, _, target string) ([]byte, error) {
		gotTarget = target
		return okJSON("Rendered by script"), nil
	})
	resp := fetchJS(t)
	if resp.IsError {
		t.Fatalf("unexpected error: %s", resp.Content)
	}
	for _, want := range []string{"Rendered by script",
		"rendered by a local browser (firefox) via fieldkit web fetch: the HTTP page had only 0 characters of readable text"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("missing %q in %s", want, resp.Content)
		}
	}
	if gotTarget != jsOnlyURL {
		t.Errorf("subprocess target = %q", gotTarget)
	}
}

func TestBrowserFallbackAbsentKeepsOldError(t *testing.T) {
	serveHTML(t, emptyShell)
	c := withFieldkit(t, func(string) (string, error) { return "", errors.New("not found") },
		func(context.Context, string, string) ([]byte, error) { return nil, nil })
	resp := fetchJS(t)
	if !resp.IsError || !strings.Contains(resp.Content, "no readable text could be extracted") {
		t.Fatalf("old error missing: %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "fieldkit is not installed") {
		t.Errorf("reason missing: %s", resp.Content)
	}
	if c.runs != 0 {
		t.Errorf("runner called %d times", c.runs)
	}
}

func TestBrowserFallbackFailures(t *testing.T) {
	type runFn = func(context.Context, string, string) ([]byte, error)
	cases := map[string]struct {
		run  runFn
		want string
	}{
		"exit code": {func(context.Context, string, string) ([]byte, error) { return nil, errors.New("exit code 3") }, "exit code 3"},
		"timeout":   {func(context.Context, string, string) ([]byte, error) { return nil, context.DeadlineExceeded }, "timeout"},
		"ok false": {func(context.Context, string, string) ([]byte, error) {
			return []byte(`{"ok":false,"reason":"no browser drivable"}`), nil
		}, "ok=false: no browser drivable"},
		"empty text": {func(context.Context, string, string) ([]byte, error) { return okJSON("  "), nil }, "no text either"},
		"bad json":   {func(context.Context, string, string) ([]byte, error) { return []byte("zzz"), nil }, "unreadable output"},
		"private final url": {func(context.Context, string, string) ([]byte, error) {
			return []byte(`{"ok":true,"final_url":"http://169.254.169.254/x","text":"secret"}`), nil
		}, "refused address"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			serveHTML(t, emptyShell)
			withFieldkit(t, fkFound, c.run)
			resp := fetchJS(t)
			if !resp.IsError || !strings.Contains(resp.Content, "no readable text could be extracted") {
				t.Fatalf("old error missing: %s", resp.Content)
			}
			if !strings.Contains(resp.Content, "browser fallback") || !strings.Contains(resp.Content, c.want) {
				t.Errorf("want reason %q in: %s", c.want, resp.Content)
			}
			if strings.Contains(resp.Content, "secret") {
				t.Error("refused text leaked")
			}
		})
	}
}

func TestBrowserFallbackSwitchOff(t *testing.T) {
	t.Setenv(browserFallbackEnvVar, "0")
	serveHTML(t, emptyShell)
	c := withFieldkit(t, fkFound,
		func(context.Context, string, string) ([]byte, error) { return okJSON("x"), nil })
	resp := fetchJS(t)
	if !resp.IsError || strings.Contains(resp.Content, "browser fallback") {
		t.Fatalf("expected the plain old error: %s", resp.Content)
	}
	if c.runs != 0 || c.looks != 0 {
		t.Errorf("switch off but looks=%d runs=%d", c.looks, c.runs)
	}
}

func TestBrowserFallbackLookPathOnce(t *testing.T) {
	serveHTML(t, emptyShell)
	c := withFieldkit(t, fkFound,
		func(context.Context, string, string) ([]byte, error) { return okJSON("hello"), nil })
	for i := 0; i < 5; i++ {
		if resp := fetchJS(t); resp.IsError {
			t.Fatal(resp.Content)
		}
	}
	if c.looks != 1 || c.runs != 5 {
		t.Errorf("looks=%d runs=%d, want 1 and 5", c.looks, c.runs)
	}
}

func TestBrowserFallbackNotUsedWhenPageHasText(t *testing.T) {
	serveHTML(t, `<html><body><p>`+strings.Repeat("Plain readable words here. ", 10)+`</p></body></html>`)
	c := withFieldkit(t, fkFound,
		func(context.Context, string, string) ([]byte, error) { return okJSON("x"), nil })
	resp := fetchJS(t)
	if resp.IsError || c.runs != 0 || c.looks != 0 {
		t.Fatalf("err=%v looks=%d runs=%d: %s", resp.IsError, c.looks, c.runs, resp.Content)
	}
}

// Measured 2026-10-31: discord.com/app over HTTP is its <title> and nothing
// else. A shell with a scrap of text must reach the browser too.
const titleShell = `<html><head><title>Discord</title><script>x()</script></head><body><div id="app"></div></body></html>`

func TestBrowserFallbackUsedOnTitleOnlyShell(t *testing.T) {
	serveHTML(t, titleShell)
	c := withFieldkit(t, fkFound, func(context.Context, string, string) ([]byte, error) {
		return okJSON("Rendered by script, a much longer page than the title alone"), nil
	})
	resp := fetchJS(t)
	if resp.IsError || c.runs != 1 {
		t.Fatalf("err=%v runs=%d: %s", resp.IsError, c.runs, resp.Content)
	}
	if !strings.Contains(resp.Content, "Rendered by script") ||
		!strings.Contains(resp.Content, "the HTTP page had only 7 characters of readable text") {
		t.Errorf("browser text or note missing: %s", resp.Content)
	}
}

func TestBrowserFallbackFailureKeepsShortHTTPText(t *testing.T) {
	serveHTML(t, titleShell)
	withFieldkit(t, fkFound, func(context.Context, string, string) ([]byte, error) {
		return nil, errors.New("exit code 3")
	})
	resp := fetchJS(t)
	if resp.IsError {
		t.Fatalf("a short page must not become an error: %s", resp.Content)
	}
	for _, want := range []string{"Discord", "only 7 characters of text; the browser fallback", "exit code 3"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("missing %q in %s", want, resp.Content)
		}
	}
}
