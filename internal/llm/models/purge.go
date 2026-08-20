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
//   - The hidden list. Purging is about VOLUME; hiding is about a specific
//     judgement the user made. Conflating them would mean a purge silently
//     un-hides everything the user rejected.
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
}

// PurgeFetchedCatalogues deletes the cached provider catalogues and drops the
// models they contributed. configDir is CacheBase().
func PurgeFetchedCatalogues(configDir string) PurgeResult {
	res := PurgeResult{}

	// Which providers came from a fetched file rather than the binary.
	fetched := map[ModelProvider]bool{
		ProviderOpenRouter:  true,
		ProviderAntigravity: true,
	}
	for id, m := range SupportedModels {
		if fetched[m.Provider] {
			delete(SupportedModels, id)
			res.RemovedModels++
		}
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
