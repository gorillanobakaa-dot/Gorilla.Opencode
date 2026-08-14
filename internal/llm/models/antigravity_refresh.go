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

	for id := range built {
		if _, existed := AntigravityModels[id]; !existed {
			res.Added = append(res.Added, strings.TrimPrefix(string(id), "antigravity."))
		}
	}
	for id := range AntigravityModels {
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
	applyAntigravity(cached.Models)
	return len(cached.Models), nil
}

// applyAntigravity registers the refreshed models. Entries are ADDED and
// UPDATED, never removed: a model the user has configured must not vanish from
// under them because one refresh did not list it.
func applyAntigravity(built map[ModelID]Model) {
	for id, m := range built {
		AntigravityModels[id] = m
		SupportedModels[id] = m
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
