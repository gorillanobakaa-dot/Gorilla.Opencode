package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type okTransport struct{ called bool }

func (t *okTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.called = true
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

// A first attempt bigger than the whole budget must be refused WITHOUT sending,
// and must not be described as a connection failure.
func TestFirstAttemptTooLargeIsNotBlamedOnTheLink(t *testing.T) {
	base := &okTransport{}
	tr := newBudgetTransport(base)
	b := NewUploadBudget(1000)
	req, _ := http.NewRequestWithContext(WithUploadBudget(context.Background(), b),
		"POST", "http://example.invalid/", strings.NewReader(strings.Repeat("x", 5000)))
	req.ContentLength = 5000

	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected refusal")
	}
	if base.called {
		t.Error("bytes were put on the link despite exceeding the budget")
	}
	var tooLarge *ErrTurnTooLarge
	if !asErr(err, &tooLarge) {
		t.Fatalf("wrong error type: %T (%v)", err, err)
	}
	if strings.Contains(err.Error(), "connection kept failing") {
		t.Error("a too-big message must not be reported as a connection fault")
	}
	if !strings.Contains(err.Error(), "connection is fine") {
		t.Errorf("error should say the link is fine: %v", err)
	}
}

func asErr[T any](err error, target *T) bool {
	if v, ok := err.(T); ok {
		*target = v
		return true
	}
	return false
}
