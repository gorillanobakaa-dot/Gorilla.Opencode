// GORILLA OVERRIDE: this file did not exist upstream. It records models the
// user never wants to see in the picker again.
//
// WHY A PERSISTED LIST AND NOT A DELETION. Nothing in this program has ever
// removed a model. SupportedModels is a plain global map seeded from eleven
// compiled-in provider maps and then merged into by every refresh with
// maps.Copy; the only deletions anywhere are the two local-endpoint teardown
// paths. RefreshOpenRouter even COMPUTES a Removed list and throws it away.
//
// So a purge that is only a deletion is undone by the next launch, because the
// compiled-in models come back with the binary, and by the next refresh,
// because it re-adds everything the provider still advertises. Someone would
// clear 300 entries on Monday and find them all back on Tuesday. The only thing
// that survives both is a persisted "I have seen this and I do not want it".
//
// HIDING IS A PICKER CONCERN, NOT A REGISTRY ONE. Hidden ids stay in
// SupportedModels on purpose: a session already running on a model must keep
// working if the user hides it mid-session, and a hidden id must still resolve
// to a name in the transcript. Only the lists the user browses are filtered.
//
// NOTHING HIDES ITSELF. There is no automatic removal on error, by design: a
// provider having a bad afternoon is not evidence a model is retired, and the
// benchmark run on 2026-08-20 had four models return unreachable that answered
// fine on a retry. Auto-hiding would have deleted all four.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const hiddenModelsFileName = "hidden-models.json"

var (
	hiddenMu     sync.RWMutex
	hiddenSet    map[string]bool
	hiddenLoaded bool
)

func hiddenModelsPath() string { return filepath.Join(ConfigBase(), hiddenModelsFileName) }

// hiddenModelsFile is the on-disk shape. A list rather than a map so the file
// stays readable and hand-editable, which is the point of a sidecar.
type hiddenModelsFile struct {
	Hidden []string `json:"hidden"`
}

func initHiddenModels() {
	hiddenMu.Lock()
	defer hiddenMu.Unlock()
	if hiddenLoaded {
		return
	}
	hiddenLoaded = true
	hiddenSet = map[string]bool{}
	data, err := os.ReadFile(hiddenModelsPath())
	if err != nil {
		return
	}
	var f hiddenModelsFile
	if json.Unmarshal(data, &f) != nil {
		// A corrupt file must not hide everything, and must not hide nothing
		// silently either — it leaves the set empty, which is the safe direction:
		// the user sees their models and can hide them again.
		return
	}
	for _, id := range f.Hidden {
		hiddenSet[id] = true
	}
}

// IsModelHidden reports whether the user has hidden this model id.
func IsModelHidden(id string) bool {
	initHiddenModels()
	hiddenMu.RLock()
	defer hiddenMu.RUnlock()
	return hiddenSet[id]
}

// HiddenModels returns the hidden ids, sorted, so the review screen is stable.
func HiddenModels() []string {
	initHiddenModels()
	hiddenMu.RLock()
	defer hiddenMu.RUnlock()
	out := make([]string, 0, len(hiddenSet))
	for id := range hiddenSet {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// HiddenCount is for the picker's status line.
func HiddenCount() int {
	initHiddenModels()
	hiddenMu.RLock()
	defer hiddenMu.RUnlock()
	return len(hiddenSet)
}

// HideModels adds ids to the hidden set and persists. Idempotent.
func HideModels(ids ...string) error { return mutateHidden(true, ids...) }

// UnhideModels removes ids from the hidden set and persists. Idempotent.
func UnhideModels(ids ...string) error { return mutateHidden(false, ids...) }

// UnhideAll clears the list entirely — the "give me everything back" escape
// hatch, so hiding can never become a one-way door.
func UnhideAll() error {
	initHiddenModels()
	hiddenMu.Lock()
	hiddenSet = map[string]bool{}
	hiddenMu.Unlock()
	return saveHiddenModels()
}

func mutateHidden(hide bool, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	initHiddenModels()
	hiddenMu.Lock()
	for _, id := range ids {
		if id == "" {
			continue
		}
		if hide {
			hiddenSet[id] = true
		} else {
			delete(hiddenSet, id)
		}
	}
	hiddenMu.Unlock()
	return saveHiddenModels()
}

func saveHiddenModels() error {
	data, err := json.MarshalIndent(hiddenModelsFile{Hidden: HiddenModels()}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ConfigBase(), 0o700); err != nil {
		return err
	}
	tmp := hiddenModelsPath() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	// Atomic replace, matching how the model caches are written: a truncated
	// hidden list is worse than none, because it hides an arbitrary subset.
	return os.Rename(tmp, hiddenModelsPath())
}

// ResetHiddenModelsForTest clears in-memory state so a test starts known.
func ResetHiddenModelsForTest() {
	hiddenMu.Lock()
	hiddenSet = map[string]bool{}
	hiddenLoaded = true
	hiddenMu.Unlock()
}
