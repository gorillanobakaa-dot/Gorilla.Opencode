package provider

// GORILLA FIX (2026-10-23): MacBook Air, 16:37:36, Gemini 3.8 Flash through
// Antigravity: "Antigravity: Internal error encountered (HTTP 500)" ended the
// turn and the run sat idle for 40 minutes. A transient Google server error is
// now retried up to googleServerRetries times, with a line on screen.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
	"google.golang.org/genai"
)

// cannedTransport answers every request with the next status in its list; a
// 200 carries one streamed chunk saying "PONG".
type cannedTransport struct {
	mu       sync.Mutex
	statuses []int
	calls    int
}

func (c *cannedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	status := c.statuses[len(c.statuses)-1]
	if c.calls < len(c.statuses) {
		status = c.statuses[c.calls]
	}
	c.calls++
	var body string
	switch {
	case status == 200:
		body = "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"PONG\"}]},\"finishReason\":\"STOP\"}]}}\n\n"
	default:
		body = `{"error":{"code":` + strconv.Itoa(status) + `,"message":"Internal error encountered.","status":"INTERNAL"}}`
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
}

// withCannedGoogle points http.DefaultClient at a canned transport, makes the
// retry wait instant and records what was put on screen.
func withCannedGoogle(t *testing.T, statuses ...int) (*cannedTransport, *[]string) {
	t.Helper()
	ct := &cannedTransport{statuses: statuses}
	prevT := http.DefaultClient.Transport
	http.DefaultClient.Transport = ct
	t.Cleanup(func() { http.DefaultClient.Transport = prevT })
	prevWait, prevNotify := serverRetryWait, notifyServerRetry
	serverRetryWait = func(int) time.Duration { return time.Millisecond }
	var said []string
	notifyServerRetry = func(msg string, _ time.Duration) { said = append(said, msg) }
	t.Cleanup(func() { serverRetryWait, notifyServerRetry = prevWait, prevNotify })
	return ct, &said
}

func antigravityForTest() *antigravityClient {
	return &antigravityClient{
		providerOptions: providerClientOptions{model: models.Model{APIModel: "gemini-3.8-flash"}, maxTokens: 100},
		creds:           &auth.AntigravityCreds{AccessToken: "t", ProjectID: "p", Expiry: time.Now().Add(time.Hour)},
	}
}

func streamAll(c *antigravityClient) (text string, errs []error) {
	msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}}
	for ev := range c.stream(context.Background(), msgs, nil) {
		switch ev.Type {
		case EventError:
			errs = append(errs, ev.Error)
		case EventComplete:
			text = ev.Response.Content
		}
	}
	return text, errs
}

func TestAGoogleServerErrorIsRetriedAndTheReplyArrives(t *testing.T) {
	ct, said := withCannedGoogle(t, 500, 503, 200)
	text, errs := streamAll(antigravityForTest())
	if len(errs) > 0 || text != "PONG" {
		t.Fatalf("after two server errors the reply did not arrive: text=%q errs=%v", text, errs)
	}
	if ct.calls != 3 {
		t.Errorf("%d requests, want 3", ct.calls)
	}
	want := []string{"Google server error (HTTP 500), retry 1 of 4", "Google server error (HTTP 503), retry 2 of 4"}
	if strings.Join(*said, "|") != strings.Join(want, "|") {
		t.Errorf("on screen: %q, want %q", *said, want)
	}
}

func TestAServerThatStaysDownIsReportedAfterFourRetries(t *testing.T) {
	ct, said := withCannedGoogle(t, 500)
	_, errs := streamAll(antigravityForTest())
	if ct.calls != 1+googleServerRetries || len(*said) != googleServerRetries {
		t.Errorf("%d requests and %d notices, want %d and %d", ct.calls, len(*said), 1+googleServerRetries, googleServerRetries)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "HTTP 500") {
		t.Errorf("the final error was not reported: %v", errs)
	}
}

func TestAClientErrorIsNotRetried(t *testing.T) {
	ct, said := withCannedGoogle(t, 400)
	streamAll(antigravityForTest())
	if ct.calls != 1 || len(*said) != 0 {
		t.Errorf("a 400 was retried: %d requests, %d notices", ct.calls, len(*said))
	}
}

func TestTheGeminiClientClassifiesServerErrors(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{genai.APIError{Code: 500, Message: "Internal error encountered."}, true},
		{genai.APIError{Code: 503, Status: "UNAVAILABLE"}, true},
		{genai.APIError{Code: 400, Message: "bad request"}, false},
		{genai.APIError{Code: 429, Message: "quota exceeded"}, false},
		{errors.New("Error 502, Message: bad gateway"), true},
	} {
		if _, got := geminiServerError(c.err); got != c.want {
			t.Errorf("geminiServerError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
	g := &geminiClient{}
	retry, _, err := g.shouldRetry(1, genai.APIError{Code: 500, Message: "Internal error encountered."})
	if !retry || err != nil {
		t.Errorf("the Gemini client did not retry a 500: retry=%v err=%v", retry, err)
	}
	if _, _, err := g.shouldRetry(googleServerRetries+1, genai.APIError{Code: 500}); err == nil {
		t.Error("the Gemini client retried a 500 past the cap")
	}
	if !strings.Contains(geminiRetryNotice(genai.APIError{Code: 500}, 2), "Google server error (HTTP 500), retry 2 of 4") {
		t.Errorf("notice = %q", geminiRetryNotice(genai.APIError{Code: 500}, 2))
	}
}
