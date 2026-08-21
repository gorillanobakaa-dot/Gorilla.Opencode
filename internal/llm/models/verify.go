package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// GORILLA OVERRIDE: check a provider's shipped model list against what it
// actually offers today.
//
// WHY THIS IS A CHECK AND NOT A REFRESH
//
// OpenRouter publishes a rich catalogue - prices, context windows, tool support
// - so its list can be rebuilt outright (see refresh.go). Every other provider
// here answers /v1/models with bare identifiers:
//
//	{"id": "llama-3.3-70b", "object": "model", "owned_by": "meta"}
//
// No price, no context window, no capability flags. Rebuilding a list from that
// would REPLACE curated entries carrying real numbers with names and guesses -
// a downgrade dressed as an update. So for these providers the honest operation
// is verification: which of the models we advertise no longer exist?
//
// That is the exact failure this project shipped. The hand-written OpenRouter
// list had nine models that had been retired, two of them the defaults for every
// agent, and nothing said so - a retired model does not fail politely, it errors
// the moment someone picks it. Being able to ask "is my list still true?" is
// most of the value; rebuilding it is the smaller half.

// CatalogueEndpoint describes where a provider publishes its model list, for the
// `models verify` command.
//
// GORILLA OVERRIDE (2026-08-21): this used to carry its OWN table of provider
// URLs beside the one in catalogue_fetch.go. Two lists of the same six endpoints
// is precisely the shape that drifts — the estate has a launcher-in-three-places
// incident on record — so the table is now DERIVED from LiveCatalogues, which is
// the one place a provider's listing URL is written down.
type CatalogueEndpoint struct {
	Name     string // human label
	URL      string // the provider's model listing
	NeedsKey bool
	// Free marks providers with a usable free tier. Directive §8: this audience
	// mostly has no card, so these are listed first and paid ones last.
	Free bool
	// KeyHint tells someone where to get a key when they have none.
	KeyHint string
}

// keyHints are the "where do I get one" lines. Separate from LiveCatalogues
// because they are advice for a human, not wire configuration.
var keyHints = map[ModelProvider]string{
	ProviderGROQ:      "free key at console.groq.com",
	ProviderCerebras:  "free key at cloud.cerebras.ai",
	ProviderAnthropic: "console.anthropic.com — paid",
	ProviderOpenAI:    "platform.openai.com — paid",
	ProviderXAI:       "console.x.ai — paid",
	ProviderDeepSeek:  "platform.deepseek.com — paid",
}

// CatalogueEndpoints is every provider that publishes a machine-readable list.
// Providers absent from it cannot be checked, and saying so is better than
// pretending — Antigravity and Google Code Assist ship fixed lists inside their
// clients, so there is nothing to check them against.
var CatalogueEndpoints = func() map[ModelProvider]CatalogueEndpoint {
	out := make(map[ModelProvider]CatalogueEndpoint, len(LiveCatalogues))
	for p, cat := range LiveCatalogues {
		out[p] = CatalogueEndpoint{
			Name: cat.Label, URL: cat.URL, NeedsKey: true,
			Free: cat.FreeTier, KeyHint: keyHints[p],
		}
	}
	return out
}()

// VerifyResult is what a check found.
type VerifyResult struct {
	Provider ModelProvider
	Name     string
	Listed   int      // models this program advertises for the provider
	Upstream int      // models the provider actually offers
	Missing  []string // advertised here, absent upstream — these error when picked
	NewThere []string // offered upstream, absent here — you could add them
	Err      error
}

// VerifyProvider asks a provider what it offers and compares. It never mutates
// the registry: a curated entry carrying a real context window and price is
// worth more than a bare id, so the answer is a report, not a replacement.
func VerifyProvider(p ModelProvider, apiKey string) VerifyResult {
	res := VerifyResult{Provider: p}
	ep, ok := CatalogueEndpoints[p]
	if !ok {
		res.Err = fmt.Errorf("%s does not publish a model list, so it cannot be checked", p)
		return res
	}
	res.Name = ep.Name
	if ep.NeedsKey && strings.TrimSpace(apiKey) == "" {
		res.Err = fmt.Errorf("no API key configured — %s", ep.KeyHint)
		return res
	}

	req, err := http.NewRequest("GET", ep.URL, nil)
	if err != nil {
		res.Err = err
		return res
	}
	// Auth style has to match the fetcher's, or verify reports a false failure
	// on the one provider that does not take a bearer token.
	if apiKey != "" {
		if LiveCatalogues[p].Auth == authAnthropicKey {
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		res.Err = fmt.Errorf("could not reach %s: %w", ep.Name, err)
		return res
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Never echo the response wholesale: an auth failure from some providers
		// repeats the key back in the error body. (House rule §7.)
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			res.Err = fmt.Errorf("%s rejected the API key (HTTP %d)", ep.Name, resp.StatusCode)
		default:
			res.Err = fmt.Errorf("%s returned HTTP %d", ep.Name, resp.StatusCode)
		}
		return res
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		res.Err = fmt.Errorf("%s sent something that is not a model list: %w", ep.Name, err)
		return res
	}
	if len(payload.Data) == 0 {
		// An empty list is far more likely to be a changed API than a provider
		// with no models, and treating it as truth would report every model as
		// retired. Refuse to draw a conclusion.
		res.Err = fmt.Errorf("%s returned an empty list — treating that as a fault, not as 'no models'", ep.Name)
		return res
	}

	upstream := map[string]bool{}
	for _, m := range payload.Data {
		upstream[m.ID] = true
	}
	res.Upstream = len(upstream)

	listed := map[string]bool{}
	for _, m := range SupportedModels {
		if m.Provider != p || m.APIModel == "" {
			continue
		}
		listed[m.APIModel] = true
	}
	res.Listed = len(listed)

	for api := range listed {
		if !upstream[api] {
			res.Missing = append(res.Missing, api)
		}
	}
	for api := range upstream {
		if !listed[api] {
			res.NewThere = append(res.NewThere, api)
		}
	}
	sort.Strings(res.Missing)
	sort.Strings(res.NewThere)
	return res
}
