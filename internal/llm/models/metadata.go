// GORILLA OVERRIDE: this file did not exist upstream. It bundles curated
// metadata (short name, capability description, context window) for known
// models so that a provider exposing 100+ discovered models — e.g. NVIDIA
// NIM — is navigable: the picker can show "DeepSeek V4 Pro — 1.6T MoE, 1M
// ctx, 80.6% SWE-bench" instead of a bare, auto-generated name. The data
// is embedded at build time from metadata/*.json and keyed by the raw
// provider model id (the part after "local.").
package models

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed metadata/*.json
var metadataFS embed.FS

// ModelMeta is one curated metadata entry.
type ModelMeta struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	ContextWindow int64  `json:"context_window"`
}

var modelMetaByID = loadModelMeta()

func loadModelMeta() map[string]ModelMeta {
	out := map[string]ModelMeta{}
	entries, err := metadataFS.ReadDir("metadata")
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := metadataFS.ReadFile("metadata/" + e.Name())
		if err != nil {
			continue
		}
		var m map[string]ModelMeta
		if json.Unmarshal(data, &m) == nil {
			for k, v := range m {
				out[k] = v
			}
		}
	}
	return out
}

// lookupModelMeta finds curated metadata for a raw model id, tolerating
// the "local." prefix that discovered ids carry.
func lookupModelMeta(id string) (ModelMeta, bool) {
	id = strings.TrimPrefix(id, "local.")
	m, ok := modelMetaByID[id]
	return m, ok
}
