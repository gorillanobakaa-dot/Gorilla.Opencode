package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// GORILLA: the websearch description must tell the model which world it is in.
// The old static text warned on every turn that source: web was probably not
// configured — and the session database showed the result: TWO web searches
// ever, while a configured SearXNG sat answering in 0.7s. A tool that
// introduces itself as unavailable does not get called.

func TestWebSearchDescriptionEncouragesWhenConfigured(t *testing.T) {
	t.Setenv("SEARXNG_URL", "http://127.0.0.1:8888")
	d := webSearchDescription()

	assert.Contains(t, d, "IS CONFIGURED", "a configured machine must say so")
	assert.Contains(t, d, "free", "the model must know searches cost nothing extra")
	// The coder triggers — without them the tool reads as academic-only and a
	// coding model never maps "look up this flag" onto it.
	for _, trigger := range []string{"error message", "flag", "release notes", "documentation"} {
		assert.Contains(t, d, trigger, "description must name the %q trigger", trigger)
	}
	assert.NotContains(t, d, "NOT configured", "no defeatist text on a working machine")
}

func TestWebSearchDescriptionStaysHonestWhenNotConfigured(t *testing.T) {
	t.Setenv("SEARXNG_URL", "")
	d := webSearchDescription()

	assert.Contains(t, d, "NOT configured")
	assert.Contains(t, d, "ask the user")
	assert.Contains(t, strings.ToLower(d), "scholar", "the always-working sources must still be offered")
	assert.NotContains(t, d, "IS CONFIGURED", "must not claim a web search that cannot happen")
}
