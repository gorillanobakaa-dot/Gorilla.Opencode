package cmd

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// The Anthropic warning prices every current model from the dated table.
func TestAnthropicCostWarningPricesTheModels(t *testing.T) {
	w := costWarning(models.ProviderAnthropic)
	t.Log("\n" + w)
	for _, want := range []string{"YOUR CARD PAYS FOR EVERY MESSAGE", "Claude Sonnet 5 ", "Claude Fable 5.1", "$0.055", "Free options"} {
		if !strings.Contains(w, want) {
			t.Errorf("warning lacks %q", want)
		}
	}
}

// A provider with no prices still gets the warning, and says it has no prices
// instead of inventing them.
func TestUnpricedProviderWarnsWithoutNumbers(t *testing.T) {
	w := costWarning(models.ProviderOpenAI)
	if !strings.Contains(w, "no price list") || strings.Contains(w, "Up to, at list price") {
		t.Errorf("OpenAI warning = %q", w)
	}
}
