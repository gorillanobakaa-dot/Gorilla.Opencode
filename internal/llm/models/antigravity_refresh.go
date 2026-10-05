package models

// GORILLA OVERRIDE: refresh the Antigravity catalogue from the backend.
//
// The built-in AntigravityModels list is a snapshot and goes stale the moment
// Google ships anything. On 2026-08-14 it held 5 models while the backend served
// 20, and Gemini 3.7 was unreachable purely because nobody had retyped it.
//
// Shape follows refresh.go deliberately: a user-invoked command, a cache next to
// the config, a merge at startup, and a failure that leaves the built-in list
// working. This one costs about 40 KB rather than 650 KB, and needs the user's
// Antigravity login — so it is offered only when they have one.
//
// This file does NOT import internal/auth. The caller passes the fetched rows
// in, the same way refresh.go avoids importing config: models is imported by
// nearly everything, and it must not drag an OAuth stack behind it.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AntigravityRow is one row as fetched from the backend. It mirrors
// auth.AntigravityModelInfo without importing it.
type AntigravityRow struct {
	ID               string
	DisplayName      string
	APIProvider      string
	MaxTokens        int64
	MaxOutputTokens  int64
	SupportsImages   bool
	SupportsThinking bool
	IsInternal       bool
}

const antigravityCacheSchema = 1

type cachedAntigravity struct {
	Schema    int               `json:"schema"`
	Refreshed time.Time         `json:"refreshed"`
	Models    map[ModelID]Model `json:"models"`
}

func antigravityCachePath(configDir string) string {
	return filepath.Join(configDir, "antigravity-models.json")
}

// usable decides whether a fetched row belongs in a chat model picker.
//
// Measured against the live response 2026-08-14: 25 rows in, 20 usable. The
// rejects are internal scaffolding (chat_20706, chat_23310), editor
// tab-completion models (tab_flash_lite_preview, tab_jump_flash_lite_preview)
// and an image-generation endpoint (gemini-3.1-flash-image).
//
// NOTE the filter deliberately does NOT require a DisplayName. The newest model
// on that day — gemini-3.7-flash-tiered, the only id that actually serves Gemini
// 3.7 — has no displayName at all. Filtering on it is the obvious rule and it
// silently drops exactly the model the user came for.
func (r AntigravityRow) usable() bool {
	switch {
	case r.IsInternal:
		return false
	case r.APIProvider == "API_PROVIDER_INTERNAL":
		return false
	case strings.HasPrefix(r.ID, "tab_"), strings.HasPrefix(r.ID, "chat_"):
		return false
	case r.MaxOutputTokens <= 0:
		// No output budget means it is not a chat endpoint. This is what
		// excludes the image-generation model without naming it.
		return false
	}
	return true
}

// displayNameFor produces a human label. The backend's own displayName is used
// when present and trusted even when it disagrees with the id — Google ships
// gemini-2.5-flash labelled "Gemini 3.1 Flash Lite", and second-guessing the
// vendor is how a picker starts lying in a different direction.
func displayNameFor(r AntigravityRow) string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	// Derive something honest from the id: "gemini-3.7-flash-tiered" ->
	// "Gemini 3.7 Flash (Tiered)". Never invent a tier the backend did not
	// offer; the suffix is reported, not chosen.
	parts := strings.Split(r.ID, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	if n := len(parts); n > 1 {
		return strings.Join(parts[:n-1], " ") + " (" + parts[n-1] + ")"
	}
	return strings.Join(parts, " ")
}

// BuildAntigravityModels converts fetched rows into catalogue entries.
func BuildAntigravityModels(rows []AntigravityRow) map[ModelID]Model {
	out := make(map[ModelID]Model, len(rows))
	for _, r := range rows {
		if !r.usable() {
			continue
		}
		id := ModelID("antigravity." + r.ID)
		out[id] = Model{
			ID:       id,
			Name:     displayNameFor(r) + " (Antigravity free)",
			Provider: ProviderAntigravity,
			APIModel: r.ID,
			Description: fmt.Sprintf("%s via your free Google Antigravity tier. Reported by the backend on %s.",
				displayNameFor(r), time.Now().Format("2006-01-02")),
			ContextWindow:       r.MaxTokens,
			DefaultMaxTokens:    r.MaxOutputTokens,
			CanReason:           r.SupportsThinking,
			SupportsAttachments: r.SupportsImages,
			// The entitlement is the user's own free tier: no per-token charge.
			CostPer1MIn:  0,
			CostPer1MOut: 0,
		}
	}
	// Order and unique names: see antigravity_order.go.
	finishAntigravity(out)
	return out
}

// AntigravityRefreshResult reports what changed, for telling the user.
type AntigravityRefreshResult struct {
	Fetched int
	Usable  int
	Skipped int
	Added   []string
	Removed []string
}

// RefreshAntigravity converts rows, writes the cache and merges into the live
// catalogue. It reports what changed relative to what was already registered.
func RefreshAntigravity(configDir string, rows []AntigravityRow) (*AntigravityRefreshResult, error) {
	built := BuildAntigravityModels(rows)
	if len(built) == 0 {
		return nil, fmt.Errorf("no usable models in the backend response")
	}

	res := &AntigravityRefreshResult{Fetched: len(rows), Usable: len(built)}
	res.Skipped = res.Fetched - res.Usable

	// Compared with what is REGISTERED now, not with the built-in map: the
	// built-in map never changes, so measuring against it reported the same
	// "added" models on every refresh for ever.
	for id := range built {
		if m, existed := SupportedModels[id]; !existed || m.Provider != ProviderAntigravity {
			res.Added = append(res.Added, strings.TrimPrefix(string(id), "antigravity."))
		}
	}
	for id, m := range SupportedModels {
		if m.Provider != ProviderAntigravity {
			continue
		}
		if _, still := built[id]; !still {
			res.Removed = append(res.Removed, strings.TrimPrefix(string(id), "antigravity."))
		}
	}
	sort.Strings(res.Added)
	sort.Strings(res.Removed)

	blob, err := json.MarshalIndent(cachedAntigravity{
		Schema:    antigravityCacheSchema,
		Refreshed: time.Now().UTC(),
		Models:    built,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}
	// Write-then-rename: a half-written cache read at startup is worse than no
	// cache, because it looks like a catalogue.
	tmp := antigravityCachePath(configDir) + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, antigravityCachePath(configDir)); err != nil {
		return nil, err
	}

	applyAntigravity(built)
	return res, nil
}

// LoadRefreshedAntigravity merges a previously refreshed catalogue over the
// built-in list at startup. A missing, unreadable, corrupt or wrong-schema cache
// is not an error: the built-in list keeps working.
func LoadRefreshedAntigravity(configDir string) (int, error) {
	blob, err := os.ReadFile(antigravityCachePath(configDir))
	if err != nil {
		return 0, nil
	}
	var cached cachedAntigravity
	if err := json.Unmarshal(blob, &cached); err != nil {
		return 0, fmt.Errorf("antigravity cache unreadable, using built-in list: %w", err)
	}
	if cached.Schema != antigravityCacheSchema || len(cached.Models) == 0 {
		return 0, nil
	}
	// A cache written before 2026-10-05 has no ranks and repeats names. Repair
	// it here so the picker is right at once, not after the next refresh.
	finishAntigravity(cached.Models)
	applyAntigravity(cached.Models)
	return len(cached.Models), nil
}

// applyAntigravity REPLACES this provider's registered models with the fetched
// set.
//
// GORILLA OVERRIDE (2026-10-05): this used to add and update and never remove,
// on the argument that "a model the user has configured must not vanish from
// under them because one refresh did not list it". Measured outcome of that
// rule: Google stopped listing claude-sonnet-4-6 and claude-opus-4-6-thinking,
// the picker went on showing both at the top of the list, and the portal went
// on selecting the first of them as the coder at every sign-in. A model that is
// offered and does not answer is worse than one that is gone. RefreshChatGPT
// already replaces, for the same reason.
//
// The concern in the old rule is real and is met another way: every id that
// disappears is mapped in LegacyModelIDs to the best current model of the same
// family, so a config or a running session that names a retired model is moved
// to its successor (validateAgent, and the /update handler) instead of being
// dropped onto an unrelated default.
//
// AntigravityModels, the built-in map, is deliberately left alone: it is the
// offline first-run list and nothing else.
func applyAntigravity(built map[ModelID]Model) {
	retired := map[ModelID]string{}
	func() {
		SupportedModels, _, commit := beginRegistryEdit()
		defer commit()
		for id, m := range SupportedModels {
			if m.Provider != ProviderAntigravity {
				continue
			}
			if _, still := built[id]; !still {
				retired[id] = agBaseLabel(m)
			}
			delete(SupportedModels, id)
		}
		for id, m := range built {
			SupportedModels[id] = m
			delete(LegacyModelIDs, id) // listed again: it is not retired
		}
	}()
	// The successors are looked up AFTER the new list is published.
	for id, label := range retired {
		if to := AntigravityReplacementFor(id, label); to != "" {
			LegacyModelIDs[id] = to
		}
	}
}

// AntigravityCatalogueAge reports when the Antigravity list was last refreshed.
func AntigravityCatalogueAge(configDir string) (age time.Duration, ok bool) {
	blob, err := os.ReadFile(antigravityCachePath(configDir))
	if err != nil {
		return 0, false
	}
	var cached cachedAntigravity
	if err := json.Unmarshal(blob, &cached); err != nil || cached.Refreshed.IsZero() {
		return 0, false
	}
	return time.Since(cached.Refreshed), true
}
