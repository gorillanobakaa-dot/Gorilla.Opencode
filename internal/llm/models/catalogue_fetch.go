// Version: 1.0.0 · updated 26-08-21-12-10
//
// GORILLA OVERRIDE (2026-08-21): provider model lists are FETCHED, not typed.
//
// Six providers used to ship a hand-written map of models compiled into the
// binary. Checked against the providers on 2026-08-21, sixteen of those entries
// were dead:
//
//   - Groq: ALL FIVE decommissioned (qwen-qwq-32b 2025-07-14, deepseek-r1-distill
//     2025-10-02, llama-4-maverick 2026-03-09, llama-4-scout 2026-07-17, and
//     llama-3.3-70b-versatile on 2026-08-16 — five days before this was written).
//   - Anthropic: ALL SEVEN retired, the last of them 2026-06-15. The "-latest"
//     aliases do not help; they resolve to retired models and 404.
//   - xAI: ALL FOUR grok-3 betas gone, superseded by the grok-4 line.
//
// A dead entry does not fail politely. It sits in the picker looking selectable
// and returns a 400 the moment somebody chooses it — which is exactly how the
// owner met this bug, twice in three minutes.
//
// openrouter.go already wrote the conclusion in August: "that is what a
// hand-maintained mirror of someone else's catalogue does: it does not fail
// loudly when upstream moves, it just quietly stops being true." This file
// applies that to the other providers. Presence now comes from the provider;
// only the NUMBERS (context window, price) stay curated, because a bare id
// carries none and a wrong context window degrades gracefully where a missing
// model does not.
package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/logging"
)

// authStyle is how a provider wants its key presented. Anthropic is the odd one
// out — everything else here is OpenAI-compatible and takes a bearer token.
type authStyle int

const (
	authBearer authStyle = iota
	authAnthropicKey
)

// LiveCatalogue describes where a provider publishes what it currently serves.
type LiveCatalogue struct {
	Label string
	URL   string
	Auth  authStyle
	// Prefer is the default-pick order, best first, matched against the raw api
	// id. A curated order rather than "whatever came back first": NVIDIA returns
	// its catalogue in id order, so position 0 was "01-ai/yi-large" — chosen
	// purely for starting with a digit, and not even entitled on that account.
	// See preferredChatModel in local.go, which learned this the same way.
	Prefer []string
	// Skip drops ids that are not chat models. OpenAI's list is mostly not chat
	// (whisper, tts, dall-e, embeddings, moderation); Groq's carries whisper and
	// guard models. Selecting one produces a baffling failure — the safety
	// classifiers answer a chat request with a bare HTTP 400 — so they are
	// filtered at the source rather than left for the user to discover.
	Skip []string
	// FreeTier marks providers usable without a card. Directive §8.
	FreeTier bool
}

// LiveCatalogues is the whole set. A provider is either here (its list is
// fetched and can never rot) or it ships a fixed list the app itself controls
// (Antigravity, ChatGPT sign-in, Gemini Code Assist — small, tied to an OAuth
// flow, and updated with the binary). There is deliberately no third category.
var LiveCatalogues = map[ModelProvider]LiveCatalogue{
	ProviderGROQ: {
		Label: "Groq", URL: "https://api.groq.com/openai/v1/models", FreeTier: true,
		Prefer: []string{"openai/gpt-oss-120b", "qwen/qwen3.6-27b", "openai/gpt-oss-20b"},
		Skip:   []string{"whisper", "tts", "guard", "prompt-guard", "distil-whisper"},
	},
	ProviderCerebras: {
		Label: "Cerebras", URL: "https://api.cerebras.ai/v1/models", FreeTier: true,
		Prefer: []string{"zai-glm-4.7", "gpt-oss-120b", "gemma-4-31b"},
	},
	ProviderAnthropic: {
		Label: "Anthropic", URL: "https://api.anthropic.com/v1/models", Auth: authAnthropicKey,
		Prefer: []string{"claude-sonnet-5", "claude-opus-5", "claude-haiku-4-5"},
	},
	ProviderOpenAI: {
		Label: "OpenAI", URL: "https://api.openai.com/v1/models",
		Prefer: []string{"gpt-5.5", "gpt-5.4-mini"},
		Skip: []string{
			"whisper", "tts", "dall-e", "embedding", "moderation", "audio",
			"realtime", "transcribe", "image", "davinci", "babbage", "sora",
		},
	},
	ProviderXAI: {
		Label: "xAI", URL: "https://api.x.ai/v1/models",
		Prefer: []string{"grok-4.6", "grok-4.5", "grok-4.3"},
		Skip:   []string{"imagine", "image", "voice", "video"},
	},
	ProviderDeepSeek: {
		Label: "DeepSeek", URL: "https://api.deepseek.com/v1/models",
		Prefer: []string{"deepseek-chat", "deepseek-reasoner"},
	},
}

// catalogueCacheFile is where a fetched list is kept so the next launch does not
// need the network. Named per provider: one refresh must never invalidate
// another, the same reason OpenRouter and Antigravity have separate files.
func catalogueCacheFile(dir string, p ModelProvider) string {
	return filepath.Join(dir, fmt.Sprintf("%s-models.json", p))
}

type catalogueCache struct {
	Provider  ModelProvider `json:"provider"`
	FetchedAt time.Time     `json:"fetched_at"`
	Models    []Model       `json:"models"`
}

// listEntry is the union of the two list shapes in play. OpenAI-compatible
// endpoints return {id}; Anthropic additionally returns a display name and, since
// March 2026, the real context and output limits — which is why Anthropic models
// need no curated numbers at all.
type listEntry struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	MaxInputTokens int64  `json:"max_input_tokens"`
	MaxTokens      int64  `json:"max_tokens"`
}

type listResponse struct {
	Data []listEntry `json:"data"`
}

// CatalogueResult reports what a fetch did, in the units a user cares about.
type CatalogueResult struct {
	Provider ModelProvider
	Label    string
	Usable   int      // models registered
	Skipped  int      // non-chat entries filtered out
	Added    []string // present now, absent from the previous list
	Removed  []string // gone upstream — these would have 400'd if picked
}

// FetchProviderCatalogue asks a provider what it serves right now, replaces that
// provider's entries in the registry, and caches the answer.
//
// It REPLACES rather than merges, deliberately: merging would keep a model the
// provider has retired, which is the failure this whole file exists to end.
func FetchProviderCatalogue(p ModelProvider, apiKey, cacheDir string) (CatalogueResult, error) {
	cat, ok := LiveCatalogues[p]
	if !ok {
		return CatalogueResult{Provider: p}, fmt.Errorf("%s does not publish a model list", p)
	}
	res := CatalogueResult{Provider: p, Label: cat.Label}

	entries, err := fetchCatalogueList(cat, apiKey)
	if err != nil {
		return res, err
	}
	if len(entries) == 0 {
		// An empty list is not success. Registering it would silently empty the
		// picker for that provider and read exactly like a working refresh.
		return res, fmt.Errorf("%s returned no models", cat.Label)
	}

	before := idsFor(p)
	built := make([]Model, 0, len(entries))
	for _, e := range entries {
		if skipCatalogueID(cat, e.ID) {
			res.Skipped++
			continue
		}
		built = append(built, catalogueModel(p, cat, e))
	}
	if len(built) == 0 {
		return res, fmt.Errorf("%s returned %d models, none of them chat models", cat.Label, len(entries))
	}

	registerCatalogue(p, built)
	res.Usable = len(built)
	res.Added, res.Removed = diffIDs(before, idsFor(p))

	if cacheDir != "" {
		if err := writeCatalogueCache(cacheDir, p, built); err != nil {
			// The models are registered and usable; only the next cold start
			// loses out. Say so in the log rather than failing the fetch.
			logging.Warn("Could not cache provider catalogue", "provider", p, "error", err)
		}
	}
	return res, nil
}

func fetchCatalogueList(cat LiveCatalogue, apiKey string) ([]listEntry, error) {
	req, err := http.NewRequest(http.MethodGet, cat.URL, nil)
	if err != nil {
		return nil, err
	}
	switch cat.Auth {
	case authAnthropicKey:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}
	// A bounded timeout because this runs on the startup path and on /update.
	// The target machine may be on a high-latency link; a hung listing must not
	// hold the session open (CLAUDE.md: assume a possibly high-latency link).
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 401 is the common one and deserves its own words: the user pasted a
		// key and it was refused, which is not the same as "provider is down".
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("%s refused the key (HTTP %d)", cat.Label, resp.StatusCode)
		}
		return nil, fmt.Errorf("%s listing failed (HTTP %d)", cat.Label, resp.StatusCode)
	}
	var out listResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("%s listing was unreadable: %w", cat.Label, err)
	}
	return out.Data, nil
}

// skipCatalogueID drops entries that are not chat models — the provider's own
// filter list plus the shared non-chat patterns already used for local endpoints.
func skipCatalogueID(cat LiveCatalogue, id string) bool {
	low := strings.ToLower(id)
	for _, pat := range cat.Skip {
		if strings.Contains(low, pat) {
			return true
		}
	}
	for _, pat := range cannotChat {
		if strings.Contains(low, pat) {
			return true
		}
	}
	return false
}

// catalogueModel turns one listed id into a registry entry.
//
// The id is prefixed with the provider so two providers serving the same model
// (Groq and Cerebras both serve gpt-oss-120b) do not collide in one flat map.
// APIModel keeps the raw id — that is what goes on the wire.
func catalogueModel(p ModelProvider, cat LiveCatalogue, e listEntry) Model {
	name := e.DisplayName
	if name == "" {
		name = friendlyModelName(e.ID)
	}
	// Label the provider in the name. A picker listing "GPT-OSS 120B" three
	// times, once per provider, is not a choice anyone can make.
	if !strings.Contains(name, cat.Label) {
		name = fmt.Sprintf("%s (%s)", name, cat.Label)
	}

	var description string
	var ctx int64
	var costIn, costInCached, costOut, costOutCached float64
	rank := 0
	if meta, ok := lookupModelMeta(e.ID); ok {
		if meta.Name != "" && e.DisplayName == "" {
			name = fmt.Sprintf("%s (%s)", meta.Name, cat.Label)
		}
		description = meta.Description
		ctx = meta.ContextWindow
		rank = meta.Rank
		costIn, costInCached = meta.CostIn, meta.CostInCached
		costOut, costOutCached = meta.CostOut, meta.CostOutCached
	}
	// The provider's own figure wins when it publishes one — it is the only
	// source that cannot be stale. Anthropic publishes it; nobody else here does.
	if e.MaxInputTokens > 0 {
		ctx = e.MaxInputTokens
	}
	if ctx == 0 {
		// Conservative floor rather than a guess. Too LOW costs an early
		// compaction; too HIGH sends a request the provider rejects, and the
		// user cannot tell that failure from a broken key.
		ctx = 32768
	}
	maxOut := e.MaxTokens
	if maxOut == 0 {
		maxOut = min(ctx/2, 8192)
	}

	return Model{
		ID:            ModelID(string(p) + "." + e.ID),
		Name:          name,
		Description:   description,
		Rank:          rank,
		Provider:      p,
		APIModel:      e.ID,
		ContextWindow: ctx,
		// GORILLA OVERRIDE: DefaultMaxTokens is not a capability claim, it is a
		// per-request cap, so half the window (bounded) is safe everywhere.
		DefaultMaxTokens: maxOut,
		// CanReason stays false unless curated metadata says otherwise: a bare
		// listing does not say, and claiming it makes the OpenAI-compatible
		// client send reasoning_effort, which non-thinking models reject with a
		// 400 (learned from Ollama — see convertLocalModel).
		CanReason:           false,
		SupportsAttachments: true,
		CostPer1MIn:         costIn,
		CostPer1MOut:        costOut,
		CostPer1MInCached:   costInCached,
		CostPer1MOutCached:  costOutCached,
	}
}

// registerCatalogue replaces every entry for one provider.
func registerCatalogue(p ModelProvider, built []Model) {
	for id, m := range SupportedModels {
		if m.Provider == p {
			delete(SupportedModels, id)
		}
	}
	for _, m := range built {
		SupportedModels[m.ID] = m
	}
}

func idsFor(p ModelProvider) map[string]bool {
	out := map[string]bool{}
	for id, m := range SupportedModels {
		if m.Provider == p {
			out[string(id)] = true
		}
	}
	return out
}

func diffIDs(before, after map[string]bool) (added, removed []string) {
	for id := range after {
		if !before[id] {
			added = append(added, id)
		}
	}
	for id := range before {
		if !after[id] {
			removed = append(removed, id)
		}
	}
	return added, removed
}

func writeCatalogueCache(dir string, p ModelProvider, built []Model) error {
	data, err := json.MarshalIndent(catalogueCache{
		Provider: p, FetchedAt: time.Now(), Models: built,
	}, "", "  ")
	if err != nil {
		return err
	}
	// 0o600: the file is a model list, not a credential, but it lives in the
	// same directory as things that are, and one mode for the whole directory is
	// one fewer thing to get wrong.
	return os.WriteFile(catalogueCacheFile(dir, p), data, 0o600)
}

// LoadCachedCatalogues applies every cached provider list at startup. Reads the
// disk only — never the network — so a slow or absent connection cannot delay
// launch, the same bargain LoadRefreshedCatalogue makes.
//
// Returns how many models were applied. A corrupt or missing file is logged and
// skipped: a bad cache must never leave someone with no models at all.
func LoadCachedCatalogues(dir string) int {
	n := 0
	for p := range LiveCatalogues {
		data, err := os.ReadFile(catalogueCacheFile(dir, p))
		if err != nil {
			continue // not fetched yet; that is the normal first-run state
		}
		var cache catalogueCache
		if err := json.Unmarshal(data, &cache); err != nil {
			logging.Warn("Ignoring unreadable catalogue cache", "provider", p, "error", err)
			continue
		}
		if len(cache.Models) == 0 {
			continue
		}
		registerCatalogue(p, cache.Models)
		n += len(cache.Models)
	}
	return n
}

// CatalogueAgeFor reports when a provider's list was last fetched. Zero time
// means never. Used to tell the user how old the list they are looking at is —
// a list with no date is not a measurement.
func CatalogueAgeFor(dir string, p ModelProvider) time.Time {
	data, err := os.ReadFile(catalogueCacheFile(dir, p))
	if err != nil {
		return time.Time{}
	}
	var cache catalogueCache
	if json.Unmarshal(data, &cache) != nil {
		return time.Time{}
	}
	return cache.FetchedAt
}

// PreferredCatalogueModel picks the model to start on for a fetched provider:
// the curated preference order first, then anything registered, then nothing.
// Never the first id returned — see the Prefer field.
func PreferredCatalogueModel(p ModelProvider) ModelID {
	cat, ok := LiveCatalogues[p]
	if !ok {
		return ""
	}
	for _, want := range cat.Prefer {
		id := ModelID(string(p) + "." + want)
		if _, ok := SupportedModels[id]; ok {
			return id
		}
	}
	// Fall back to the highest-ranked registered model, then any of them, so a
	// provider that renamed everything still yields a working default.
	best, bestRank := ModelID(""), 0
	for id, m := range SupportedModels {
		if m.Provider != p {
			continue
		}
		if best == "" || (m.Rank > 0 && (bestRank == 0 || m.Rank < bestRank)) {
			best, bestRank = id, m.Rank
		}
	}
	return best
}

// PurgeCatalogue drops one fetched provider's models and its cache file, so
// /purge treats these exactly like the other downloaded lists. keep protects the
// models an agent is currently pointed at.
func PurgeCatalogue(dir string, p ModelProvider, keep map[ModelID]bool) int {
	n := 0
	for id, m := range SupportedModels {
		if m.Provider == p && !keep[id] {
			delete(SupportedModels, id)
			n++
		}
	}
	if dir != "" {
		_ = os.Remove(catalogueCacheFile(dir, p))
	}
	return n
}
