package provider

import (
	"strings"
	"testing"
)

// A 404 from an OpenAI-compatible provider means the model is not available to
// this account. The user saw only `404 Not Found` and reasonably concluded the
// app was broken. The message must name the model and say what to do.
func TestFourOhFourExplainsItIsTheModelNotTheKey(t *testing.T) {
	got := explainAPIStatus(404, "Llama 3.3 70B")
	if got == "" {
		t.Fatal("404 produced no explanation; the user is back to a bare HTTP status")
	}
	for _, want := range []string{"Llama 3.3 70B", "isn't enabled", "/models", "404"} {
		if !strings.Contains(got, want) {
			t.Errorf("404 message is missing %q, so it does not say which model or what to do:\n%s", want, got)
		}
	}
	// The whole point: stop people blaming their key for an entitlement problem.
	if !strings.Contains(got, "your key is fine") {
		t.Errorf("404 message does not clear the API key of blame:\n%s", got)
	}
}

// 401 is the opposite diagnosis and must not be confused with 404, or the advice
// sends people to change the wrong thing.
func TestKeyProblemsAndModelProblemsGiveDifferentAdvice(t *testing.T) {
	unauthorized := explainAPIStatus(401, "Llama 3.3 70B")
	notFound := explainAPIStatus(404, "Llama 3.3 70B")

	if !strings.Contains(unauthorized, "/connect") {
		t.Errorf("401 should send the user to /connect to fix the key, got:\n%s", unauthorized)
	}
	if strings.Contains(unauthorized, "/models") {
		t.Errorf("401 is a key problem; sending the user to /models is wrong advice:\n%s", unauthorized)
	}
	if strings.Contains(notFound, "/connect") {
		t.Errorf("404 is a model problem; sending the user to /connect is wrong advice:\n%s", notFound)
	}
}

// A retired model (410) is not the same as one the account cannot reach (404):
// switching models fixes both, but only one is worth asking support about.
func TestRetiredModelSaysRetired(t *testing.T) {
	got := explainAPIStatus(410, "Qwen 2.5 Coder 32B")
	if !strings.Contains(got, "retired") {
		t.Errorf("410 should say the model is retired, got:\n%s", got)
	}
}

// Statuses we have nothing better to say about must fall through to the raw
// error rather than inventing a confident wrong explanation.
func TestUnknownStatusesStaySilent(t *testing.T) {
	for _, status := range []int{200, 400, 429, 500, 503} {
		if got := explainAPIStatus(status, "X"); got != "" {
			t.Errorf("status %d invented an explanation it cannot support: %q", status, got)
		}
	}
}

// The message must survive a model with no display name rather than emitting a
// sentence with a hole in it.
func TestMissingModelNameDegradesGracefully(t *testing.T) {
	got := explainAPIStatus(404, "")
	if strings.Contains(got, "  ") || strings.HasPrefix(got, " ") {
		t.Errorf("empty model name left a gap in the sentence: %q", got)
	}
	if !strings.Contains(got, "this model") {
		t.Errorf("expected a neutral stand-in for the model name, got: %q", got)
	}
}
