package provider

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// nimNotFound builds the error the SDK actually hands us for a model the
// account cannot reach: status only, empty body (measured 2026-08-04).
func nimNotFound(t *testing.T, status int) *openai.Error {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		"https://integrate.api.nvidia.com/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	return &openai.Error{
		StatusCode: status,
		Request:    req,
		Response:   &http.Response{StatusCode: status},
	}
}

func clientFor(name string) *openaiClient {
	return &openaiClient{
		providerOptions: providerClientOptions{
			model: models.Model{Name: name},
		},
	}
}

// A translated error must ADD an explanation, never replace the evidence.
//
// The first version of this fix returned only the friendly sentence and pushed
// the raw error to the log file. That is diagnosis by deletion: when the
// translation is right it saves a minute, and when it is wrong it destroys the
// URL and status code you need to work out why. Both, always.
func TestTranslatedErrorKeepsTheRawDetail(t *testing.T) {
	o := clientFor("Llama 3.3 70B")
	retry, _, got := o.shouldRetry(1, nimNotFound(t, 404), false)

	if retry {
		t.Fatal("a 404 is not retryable; retrying it just delays the explanation")
	}
	msg := got.Error()

	// The explanation, and the evidence behind it.
	if !strings.Contains(msg, "Llama 3.3 70B isn't enabled") {
		t.Errorf("the plain explanation is missing:\n%s", msg)
	}
	if !strings.Contains(msg, "404") {
		t.Errorf("the status code was swallowed; the user cannot see what happened:\n%s", msg)
	}
	if !strings.Contains(msg, "integrate.api.nvidia.com") {
		t.Errorf("the endpoint was swallowed; there is no way to tell which provider "+
			"refused the request:\n%s", msg)
	}

	// The explanation must come FIRST: the status bar truncates the tail, so
	// whatever leads is the part that always survives on a narrow terminal.
	plainAt := strings.Index(msg, "Llama 3.3 70B isn't enabled")
	rawAt := strings.Index(msg, "integrate.api.nvidia.com")
	if plainAt > rawAt {
		t.Errorf("the raw error leads, so a truncated status bar shows the jargon "+
			"and hides the advice:\n%s", msg)
	}
}

// Wrapping with %w rather than %s keeps the typed error reachable, so any
// upstream code that inspects the status still works.
func TestTranslatedErrorStaysInspectable(t *testing.T) {
	o := clientFor("Llama 3.3 70B")
	_, _, got := o.shouldRetry(1, nimNotFound(t, 404), false)

	var apierr *openai.Error
	if !errors.As(got, &apierr) {
		t.Fatalf("the *openai.Error no longer survives errors.As, so callers cannot "+
			"read the status: %v", got)
	}
	if apierr.StatusCode != 404 {
		t.Errorf("status came back as %d, want 404", apierr.StatusCode)
	}
}

// A status with no translation must pass the raw error through untouched rather
// than being dressed up in a sentence we cannot support.
func TestUntranslatedStatusPassesThroughUnchanged(t *testing.T) {
	o := clientFor("Llama 3.3 70B")
	raw := nimNotFound(t, 418)
	_, _, got := o.shouldRetry(1, raw, false)

	if got.Error() != raw.Error() {
		t.Errorf("an untranslated status was modified:\n got: %s\nwant: %s", got, raw)
	}
}
