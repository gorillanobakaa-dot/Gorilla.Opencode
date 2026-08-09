package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GORILLA OVERRIDE: user-refreshable model catalogues.
//
// THE BUG THIS EXISTS TO PREVENT
//
// The OpenRouter list was typed out by hand. Checked against the live catalogue
// on 2026-08-09, NINE of its 22 models no longer existed and a tenth could not
// call tools - and two of the dead ones were the DEFAULTS for every agent, so
// configuring OpenRouter produced something that could not answer at all.
//
// None of that failed loudly. A hand-maintained mirror of someone else's
// catalogue does not break when upstream moves; it quietly stops being true. The
// build-time generator (cmd/openrouter-models) slowed that decay to one release
// cycle. This closes it: the person running the program can refresh it
// themselves, without waiting for anyone to cut a release.
//
// WHY IT IS A COMMAND AND NOT AUTOMATIC
//
// Directive §8 - this ships to people on single-digit-KB/s links:
//
//   - No fetch at launch. Nothing here runs unless someone asks for it.
//   - The cost is stated up front in bytes and in seconds at 8 KB/s, because
//     "it's only a few hundred KB" is a sentence written by someone on fibre.
//   - Offline, corrupt or half-written cache falls back to the built-in list.
//     A refresh that fails must leave a working program behind.
//   - It reports what CHANGED. Prices drive the cost display; a refresh that
//     silently altered them would make the spend figure lie.

const openRouterCatalogueURL = "https://openrouter.ai/api/v1/models"

// cacheFileName lives beside config.json. Named for what it holds.
const cacheFileName = "openrouter-models.json"

// catalogueSchema is the version of the RULES used to build a cache — which
// models are excluded, how descriptions are cleaned. Bump it whenever those
// change.
//
// GORILLA OVERRIDE (2026-08-09): added after a stale cache silently undid a
// fix. Batch endpoints were removed from the catalogue and descriptions stopped
// being cut mid-sentence, both verified in the generated file — and the running
// program still showed 333 models with 59 batch entries and truncated text,
// because a cache written an hour earlier was merged over the corrected list at
// startup. The improvement was real and invisible.
//
// A cache built under different rules is now discarded rather than trusted. It
// costs one refresh; trusting it costs a fix nobody can see.
const catalogueSchema = 4

// cachedCatalogue is what gets written to disk. The Model values are stored
// whole rather than re-derived, so a future change to the conversion cannot
// silently reinterpret an old cache.
type cachedCatalogue struct {
	Schema    int               `json:"schema"`
	Refreshed time.Time         `json:"refreshed"`
	Source    string            `json:"source"`
	Models    map[ModelID]Model `json:"models"`
}

// RefreshResult describes what a refresh did, for reporting to the user.
type RefreshResult struct {
	Fetched int
	Usable  int
	Free    int
	// Skipped, split by REASON. Reporting one number for both would tell
	// someone that 126 models "cannot call tools" when 59 of them can — they
	// are simply asynchronous batch endpoints.
	NoTools      int
	Batch        int
	Added        []string
	Removed      []string
	PriceChanged []string
	Bytes        int64
}

// catalogueCachePath returns where the refreshed list is stored. It takes the
// config dir as an argument rather than importing config, because config
// imports models and the reverse would be a cycle.
func catalogueCachePath(configDir string) string {
	return filepath.Join(configDir, cacheFileName)
}

// LoadRefreshedCatalogue merges a previously refreshed catalogue over the
// built-in one. Any failure - missing, unreadable, malformed, empty - leaves the
// built-in list untouched and reports why, because a broken cache must never
// leave someone with no models at all.
func LoadRefreshedCatalogue(configDir string) (int, error) {
	data, err := os.ReadFile(catalogueCachePath(configDir))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // never refreshed; built-in list stands
		}
		return 0, err
	}
	var c cachedCatalogue
	if err := json.Unmarshal(data, &c); err != nil {
		return 0, fmt.Errorf("refreshed model list is unreadable, using the built-in one: %w", err)
	}
	if len(c.Models) == 0 {
		return 0, fmt.Errorf("refreshed model list is empty, using the built-in one")
	}
	if c.Schema != catalogueSchema {
		// Built under older rules — it may contain models since excluded, or
		// text cleaned the old way. Ignoring it costs one refresh; trusting it
		// silently reverts whatever those rules were changed to fix.
		return 0, fmt.Errorf(
			"saved model list was built by an older version (schema %d, now %d) — using the built-in list; run `gorilla-opencode models refresh` to rebuild it",
			c.Schema, catalogueSchema)
	}
	n := 0
	for id, m := range c.Models {
		if m.Provider != ProviderOpenRouter || m.APIModel == "" {
			continue // refuse entries that could not work
		}
		SupportedModels[id] = m
		n++
	}
	return n, nil
}

// RefreshOpenRouter fetches the catalogue, writes the cache and reports the
// difference against what is currently registered.
func RefreshOpenRouter(configDir string) (*RefreshResult, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequest("GET", openRouterCatalogueURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach openrouter.ai: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("openrouter returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("reading catalogue: %w", err)
	}

	var payload struct {
		Data []struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			Description   string   `json:"description"`
			ContextLength int64    `json:"context_length"`
			SupportedArgs []string `json:"supported_parameters"`
			Pricing       struct {
				Prompt         string `json:"prompt"`
				Completion     string `json:"completion"`
				InputCacheRead string `json:"input_cache_read"`
			} `json:"pricing"`
			TopProvider struct {
				MaxCompletionTokens int64 `json:"max_completion_tokens"`
			} `json:"top_provider"`
			Architecture struct {
				InputModalities []string `json:"input_modalities"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("openrouter sent something that is not a catalogue: %w", err)
	}

	res := &RefreshResult{Fetched: len(payload.Data), Bytes: int64(len(raw))}
	fresh := map[ModelID]Model{}

	for _, m := range payload.Data {
		hasTools := false
		canReason := false
		for _, p := range m.SupportedArgs {
			switch p {
			case "tools":
				hasTools = true
			case "reasoning":
				canReason = true
			}
		}
		// A model that cannot call tools cannot do this job: the agent would
		// describe edits it is unable to make. Dropping them is not curation.
		if !hasTools {
			res.NoTools++
			continue
		}
		// Batch endpoints are asynchronous: an interactive agent pointed at one
		// waits indefinitely for a reply.
		if IsBatchVariant(m.ID) {
			res.Batch++
			continue
		}
		attach := false
		for _, mod := range m.Architecture.InputModalities {
			if mod == "image" {
				attach = true
			}
		}
		maxTok := m.TopProvider.MaxCompletionTokens
		if maxTok == 0 {
			maxTok = 4096
		}
		free := m.Pricing.Prompt == "0" && m.Pricing.Completion == "0"
		id := ModelID("openrouter." + m.ID)
		fresh[id] = Model{
			ID:                  id,
			Name:                m.Name,
			Description:         describeCatalogueModel(m.ID, m.Description, m.ContextLength, perMillionPrice(m.Pricing.Prompt), perMillionPrice(m.Pricing.Completion)),
			Provider:            ProviderOpenRouter,
			APIModel:            m.ID,
			CostPer1MIn:         perMillionPrice(m.Pricing.Prompt),
			CostPer1MOut:        perMillionPrice(m.Pricing.Completion),
			CostPer1MInCached:   perMillionPrice(m.Pricing.InputCacheRead),
			ContextWindow:       m.ContextLength,
			DefaultMaxTokens:    maxTok,
			CanReason:           canReason,
			SupportsAttachments: attach,
		}
		res.Usable++
		if free {
			res.Free++
		}
	}

	if res.Usable == 0 {
		// Refuse rather than overwrite a working list with nothing.
		return nil, fmt.Errorf("catalogue contained no tool-capable models — refusing to replace the existing list")
	}

	// Rank the free ones, same rule as the generator: this is the top of the
	// picker, and for this audience free is what makes a model usable at all.
	var freeIDs []ModelID
	for id, m := range fresh {
		if m.CostPer1MIn == 0 && m.CostPer1MOut == 0 {
			freeIDs = append(freeIDs, id)
		}
	}
	sort.Slice(freeIDs, func(i, j int) bool {
		a, b := fresh[freeIDs[i]], fresh[freeIDs[j]]
		if a.ContextWindow != b.ContextWindow {
			return a.ContextWindow > b.ContextWindow
		}
		return a.ID < b.ID
	})
	for i, id := range freeIDs {
		if i >= 12 {
			break
		}
		m := fresh[id]
		m.Rank = i + 1
		fresh[id] = m
	}

	// Diff against what is registered now, so the user is told what moved.
	for id, m := range fresh {
		old, existed := SupportedModels[id]
		switch {
		case !existed:
			res.Added = append(res.Added, string(id))
		case old.CostPer1MIn != m.CostPer1MIn || old.CostPer1MOut != m.CostPer1MOut:
			res.PriceChanged = append(res.PriceChanged, fmt.Sprintf("%s (%.2f→%.2f in, %.2f→%.2f out per 1M)",
				id, old.CostPer1MIn, m.CostPer1MIn, old.CostPer1MOut, m.CostPer1MOut))
		}
	}
	for id, m := range SupportedModels {
		if m.Provider != ProviderOpenRouter {
			continue
		}
		if _, still := fresh[id]; !still {
			res.Removed = append(res.Removed, string(id))
		}
	}
	sort.Strings(res.Added)
	sort.Strings(res.Removed)
	sort.Strings(res.PriceChanged)

	// Write atomically: a half-written cache read at next launch is exactly the
	// "corrupt file leaves you with no models" case the loader guards against,
	// and it is cheaper to not create it than to recover from it.
	out := cachedCatalogue{Schema: catalogueSchema, Refreshed: time.Now().UTC(), Source: openRouterCatalogueURL, Models: fresh}
	blob, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}
	path := catalogueCachePath(configDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}

	for id, m := range fresh {
		SupportedModels[id] = m
	}
	return res, nil
}

func perMillionPrice(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f * 1_000_000
}

// CatalogueAge reports when the model list was last refreshed. ok is false if it
// never has been. Reads one file's mtime - no parsing, no network - so it is
// safe to call from the startup screen's render path.
func CatalogueAge(configDir string) (age time.Duration, ok bool) {
	fi, err := os.Stat(catalogueCachePath(configDir))
	if err != nil {
		return 0, false
	}
	return time.Since(fi.ModTime()), true
}

// StaleAfter is how old a refreshed list gets before the startup screen
// mentions it. Providers retire models continuously, and a retired model does
// not fail politely - it errors when picked. A month is long enough that the
// notice stays rare and short enough that it appears before the list rots.
const StaleAfter = 30 * 24 * time.Hour

// describeCatalogueModel prefers a verdict we have earned over the vendor's own
// description of itself. See CuratedVerdict.
func describeCatalogueModel(apiModel, vendorDesc string, ctx int64, in, out float64) string {
	if v, ok := CuratedVerdict(apiModel); ok {
		return CleanCatalogueDescription(v, ctx, in, out, true)
	}
	return CleanCatalogueDescription(vendorDesc, ctx, in, out, false)
}
