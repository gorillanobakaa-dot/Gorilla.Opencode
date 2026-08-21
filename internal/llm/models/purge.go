// GORILLA OVERRIDE: this file did not exist upstream. It empties the FETCHED
// model catalogues, so a picker clogged with hundreds of provider entries can
// be reset to the ones that ship with the binary.
//
// WHAT IT DELIBERATELY DOES NOT TOUCH:
//
//   - The eleven compiled-in provider maps (Anthropic, OpenAI, Gemini, Groq,
//     Cerebras, Azure, xAI, VertexAI, DeepSeek, Copilot, ChatGPT). Those are the
//     models that work, several of them free with a Gmail sign-in, and deleting
//     them would throw away access someone completed an OAuth flow to get.
//     They also come back with the binary, so "deleting" them at runtime would
//     look like it worked and silently undo itself on the next launch.
//   - Bookmarks. The whole point of a personal list is that a clean slate does
//     not touch it. An id that disappears from the registry still renders in the
//     bookmarks column, marked unavailable, so nothing vanishes silently.
//   - Models an agent is currently pointed at (passed in by the caller). See
//     the keep parameter below.
//   - The hidden list. Purging is about VOLUME; hiding is about a specific
//     judgement the user made. Conflating them would mean a purge silently
//     un-hides everything the user rejected.
//
// LOCAL ENDPOINTS (added 2026-08-21): models served by a configured
// OpenAI-compatible endpoint ARE purged — they are a fetched list like any
// other, they are simply fetched over the wire at startup rather than from a
// cache file. The endpoint entry itself, its name and its key are untouched, so
// the list comes back on the next launch or /update. Removing an endpoint for
// good is /connection.
//
// So this is the reversible half of the pair: /purge models clears the fetched
// lists, /update models fetches them again. The irreversible-feeling half —
// "never show me this again" — is hidden-models.json, which is honoured
// separately and survives both.
package models

import (
	"os"
	"path/filepath"
)

// PurgeResult reports what a purge actually did, in the units the user cares
// about: how many entries left the picker and which files went.
type PurgeResult struct {
	RemovedModels int
	FilesDeleted  []string
	Kept          int
	// RemovedCompiled is how many of RemovedModels ship inside the binary and
	// will therefore be back on the next launch. Reported separately because a
	// purge that says "284 removed" without saying "279 of those return when you
	// restart" is a number the user cannot act on.
	RemovedCompiled int
	// RemovedLocal is how many of RemovedModels came from a configured
	// OpenAI-compatible endpoint rather than a cache file. Reported separately
	// because those come BACK by themselves on the next launch — the endpoint is
	// still configured — and a purge that says "removed 391" without saying
	// which ones return is a report the user cannot act on.
	RemovedLocal int
}

// compiledInIDs is the set of model ids that genuinely ship inside the binary,
// SNAPSHOTTED at init.
//
// GORILLA FIX (2026-08-21, same day, caught by a screenshot): this was a live
// lookup into OpenRouterGeneratedModels and AntigravityModels. That is wrong,
// because applyAntigravity WRITES FETCHED MODELS BACK INTO AntigravityModels —
// so after one /update the "compiled-in" map held 20 entries instead of 5, and
// /purge reported "38 of them ship with the app" when the true figure was 23.
//
// The irony is the lesson: the fix for an overstated count shipped with an
// overstated count, because it asked a mutable variable what the binary
// contains. A binary's contents are fixed at build time, so the answer must be
// taken at init and never re-derived.
//
// Package-level variables are fully initialised before any init() runs (Go
// spec), so this sees the literals and nothing else.
var compiledInIDs = map[ModelID]bool{}

func init() {
	for id := range OpenRouterGeneratedModels {
		compiledInIDs[id] = true
	}
	for id := range AntigravityModels {
		compiledInIDs[id] = true
	}
}

// compiledIn reports whether a model ships in the binary — i.e. it comes back on
// the next launch whatever a purge does.
func compiledIn(m Model) bool {
	return compiledInIDs[m.ID]
}

// PurgeFetchedCatalogues deletes the cached provider catalogues and drops the
// models they contributed. configDir is CacheBase().
// keep names models that must survive whatever their provider — in practice the
// ones the agents are currently pointed at. Purging the model you are talking to
// leaves the footer naming a model the registry no longer has, and the picker
// with nothing selected; the volume problem /purge exists to solve is not solved
// any better by including the one entry in use. Variadic so a caller with no
// live session (a test, a first run) can just omit it.
func PurgeFetchedCatalogues(configDir string, keep ...ModelID) PurgeResult {
	res := PurgeResult{}
	inUse := make(map[ModelID]bool, len(keep))
	for _, id := range keep {
		if id != "" {
			inUse[id] = true
		}
	}

	// Which providers came from a fetched file rather than the binary.
	fetched := map[ModelProvider]bool{
		ProviderOpenRouter:  true,
		ProviderAntigravity: true,
	}
	for id, m := range SupportedModels {
		if !fetched[m.Provider] || inUse[id] {
			continue
		}
		delete(SupportedModels, id)
		res.RemovedModels++
		// GORILLA FIX (2026-08-21): count what SHIPS separately from what was
		// downloaded. OpenRouter and Antigravity each have compiled-in models
		// registered by an init() as well as fetched ones, and purging by
		// PROVIDER cannot tell them apart — so /purge reported 284 removed when
		// 279 of those were built into the binary and silently returned on the
		// next launch. The number was true for about as long as the session was.
		if compiledIn(m) {
			res.RemovedCompiled++
		}
	}
	// GORILLA FIX (2026-08-21): local endpoint models were being left behind.
	// They are fetched too, just over the wire at startup instead of from a
	// cache file, and that distinction means nothing to whoever typed /purge.
	// See PurgeLocalModels for why the endpoint itself survives.
	res.RemovedLocal = PurgeLocalModels(inUse)
	res.RemovedModels += res.RemovedLocal

	// GORILLA OVERRIDE (2026-08-21): the fetched provider catalogues are
	// downloaded lists too — Groq, Cerebras, Anthropic, OpenAI, xAI, DeepSeek.
	// They come back on the next connect or /update, same reversible bargain as
	// OpenRouter's.
	for p := range LiveCatalogues {
		res.RemovedModels += PurgeCatalogue(configDir, p, inUse)
	}

	res.Kept = len(SupportedModels)

	for _, name := range []string{cacheFileName, "antigravity-models.json"} {
		p := filepath.Join(configDir, name)
		if err := os.Remove(p); err == nil {
			res.FilesDeleted = append(res.FilesDeleted, name)
		}
	}
	return res
}
