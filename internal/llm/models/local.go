package models

import (
	"cmp"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/opencode-ai/opencode/internal/logging"
)

const (
	ProviderLocal ModelProvider = "local"

	localModelsPath        = "v1/models"
	lmStudioBetaModelsPath = "api/v0/models"
)

// GORILLA OVERRIDE: multiple OpenAI-compatible endpoints (NIM, Ollama, Kilo,
// LM Studio, ...) can be registered and coexist. Each model routes to the
// endpoint it came from via its own baseURL + apiKey, recorded here.
type localRouteInfo struct {
	BaseURL string
	APIKey  string
	// Endpoint is the user's name for the connection ("Gorilla.FREE.NVIDIA.NIM",
	// "ollama"). Every OpenAI-compatible endpoint lands under the single "local"
	// provider, so without this a picker showing 104 models cannot say which of
	// them is NVIDIA and which is the local Ollama.
	Endpoint string
}

var localRoute = map[ModelID]localRouteInfo{}

// LocalRouteFor returns the endpoint baseURL + apiKey a given local model
// should be reached with. ok is false for non-local / unknown models.
func LocalRouteFor(id ModelID) (baseURL, apiKey string, ok bool) {
	r, ok := localRoute[id]
	return r.BaseURL, r.APIKey, ok
}

// RegisterLocalEndpoint fetches an OpenAI-compatible endpoint's model list and
// registers each model into SupportedModels, routing every one to this
// endpoint's baseURL + apiKey. Returns how many were discovered and the first
// registered model id. IDs stay flat ("local.<rawID>") so existing config
// references keep resolving; only a cross-endpoint id collision is namespaced
// as "local.<name>/<rawID>". Safe to call at runtime (e.g. from /connect).
func RegisterLocalEndpoint(name, baseURL, apiKey string) (int, ModelID) {
	raw := fetchLocalModels(baseURL, apiKey)
	if len(raw) == 0 {
		logging.Debug("No local models found", "endpoint", baseURL)
		return 0, ""
	}
	// Sort local first. It was 0, and getEnabledProviders maps 0 to 999 ("not
	// ranked, show last") — so a deliberately configured endpoint sorted BELOW
	// every provider the user has never touched. An endpoint someone added by
	// hand is the one they are looking for.
	ProviderPopularity[ProviderLocal] = 1
	registered := make([]ModelID, 0, len(raw))
	n := 0
	for _, m := range raw {
		model := convertLocalModel(m)
		if existing, dup := SupportedModels[model.ID]; dup {
			if r, routed := localRoute[model.ID]; routed && r.BaseURL != baseURL {
				// same model id from a different endpoint -> namespace newcomer
				model.ID = ModelID("local." + name + "/" + m.ID)
			}
			_ = existing
		}
		SupportedModels[model.ID] = model
		localRoute[model.ID] = localRouteInfo{BaseURL: baseURL, APIKey: apiKey, Endpoint: name}
		registered = append(registered, model.ID)
		n++
	}
	return n, preferredChatModel(registered)
}

// cannotChat matches ids that are not chat models at all — embedders,
// rerankers, retrievers and safety classifiers. Selecting one as an agent
// produces an immediate, baffling failure (the safety classifiers answer a chat
// request with a bare HTTP 400).
var cannotChat = []string{
	"embed", "embedding", "rerank", "retriever", "bge-", "guard", "safety", "moderation",
}

// preferredChatModel picks the model to start on when a local endpoint is first
// configured.
//
// GORILLA FIX: this used to be whatever the provider listed FIRST.
//
// NVIDIA returns its catalogue in id order, so position 0 is "01-ai/yi-large"
// — it sorts first purely because it begins with "01". Every re-run of the
// startup portal therefore pinned all four agents to that one model, which on
// this account is not even entitled (HTTP 404). Positions 2 and 5 are a vision
// model and an EMBEDDING model, so list order is not merely unlucky here; it
// carries no information about whether the model can hold a conversation.
//
// Preference order, then anything that is not obviously a non-chat model, then
// (only if everything looks unusable) the first id, because returning nothing
// would leave the endpoint configured with no model at all.
func preferredChatModel(ids []ModelID) ModelID {
	if len(ids) == 0 {
		return ""
	}
	have := make(map[ModelID]bool, len(ids))
	for _, id := range ids {
		have[id] = true
	}
	for _, want := range []string{
		"local.meta/llama-3.3-70b-instruct",
		"local.meta/llama-3.1-70b-instruct",
		"local.qwen/qwen2.5-coder-32b-instruct",
		"local.meta/llama-3.1-8b-instruct",
	} {
		if have[ModelID(want)] {
			return ModelID(want)
		}
	}
	for _, id := range ids {
		low := strings.ToLower(string(id))
		bad := false
		for _, pat := range cannotChat {
			if strings.Contains(low, pat) {
				bad = true
				break
			}
		}
		if !bad {
			return id
		}
	}
	return ids[0]
}

// UnregisterLocalEndpoint drops every model routed to baseURL (used when a
// connection is disabled/removed so its models vanish from the picker).
func UnregisterLocalEndpoint(baseURL string) {
	for id, r := range localRoute {
		if r.BaseURL == baseURL {
			delete(localRoute, id)
			delete(SupportedModels, id)
		}
	}
}

// UnregisterLocalEndpointByName drops the models registered BY A NAMED endpoint.
//
// GORILLA OVERRIDE: the baseURL variant above is wrong for removal. Several
// configured endpoints may share one baseURL (the duplicate-NVIDIA case), and
// after collapsing, exactly one of them owns the registered models. Deleting by
// URL would therefore take the surviving endpoint's models down with the
// redundant entry being removed. The route records which endpoint registered it,
// so match on that.
func UnregisterLocalEndpointByName(name string) int {
	n := 0
	for id, r := range localRoute {
		if r.Endpoint == name {
			delete(localRoute, id)
			delete(SupportedModels, id)
			n++
		}
	}
	return n
}

// PurgeLocalModels drops every model that came from an OpenAI-compatible
// endpoint (NVIDIA NIM, Ollama, LM Studio, anything added with /connect) and
// forgets their routes. Returns how many left the picker.
//
// GORILLA FIX (2026-08-21): /purge missed these entirely. It cleared OpenRouter
// and Antigravity because those arrive as cache FILES, and local models arrive
// over the wire at startup instead — so a purge left the picker holding NVIDIA
// NIM's ~100 entries, which on this account is the largest single block in it.
// From the user's side "purge the downloaded model lists" plainly includes the
// list downloaded from their own endpoint; the storage mechanism is an
// implementation detail they are not required to know.
//
// The endpoint itself is NOT touched. It stays configured, keeps its key, and
// re-registers on the next launch or /update — the same reversible bargain the
// fetched catalogues get. Removing it for good is /connection, which is a
// different decision and asks first.
// keep names the models an agent is currently pointed at; those survive, so a
// purge cannot blank out the model you are talking to right now.
func PurgeLocalModels(keep map[ModelID]bool) int {
	n := 0
	for id, m := range SupportedModels {
		if m.Provider != ProviderLocal || keep[id] {
			continue
		}
		delete(SupportedModels, id)
		delete(localRoute, id)
		n++
	}
	// A route with no model behind it routes nothing; drop the leftovers rather
	// than keep a map that outlives the registry it describes.
	for id := range localRoute {
		if _, live := SupportedModels[id]; !live {
			delete(localRoute, id)
		}
	}
	return n
}

// fetchLocalModels tries the LM Studio beta path first, then the standard
// OpenAI /v1/models path, against baseURL (authenticating with apiKey).
func fetchLocalModels(baseURL, apiKey string) []localModel {
	base, err := url.Parse(baseURL)
	if err != nil {
		logging.Debug("Failed to parse local endpoint", "error", err, "endpoint", baseURL)
		return nil
	}
	try := func(path string) []localModel {
		u := *base
		u.Path = path
		return listLocalModels(u.String(), apiKey)
	}
	models := try(lmStudioBetaModelsPath)
	if len(models) == 0 {
		models = try(localModelsPath)
	}
	if len(models) == 0 {
		// GORILLA OVERRIDE: a provider-specific fallback for endpoints that are
		// OpenAI-compatible for CHAT but not for LISTING.
		//
		// Cloudflare Workers AI is the case that forced this: its
		// /v1/chat/completions is a drop-in, but GET /v1/models answers
		// "405 GET not supported for requested URI" (measured 2026-08-05). Without
		// a fallback the endpoint looks completely broken — discovery finds zero
		// models and the portal reports "no models found ... is the key valid?",
		// which points the user at their key when the key is fine.
		models = fetchCloudflareModels(base, apiKey)
	}
	return models
}

// cfModelSearch is Cloudflare's own model listing. Only the fields needed to
// decide "can this hold a conversation?" are decoded.
type cfModelSearch struct {
	Success bool `json:"success"`
	Result  []struct {
		Name string `json:"name"`
		Task struct {
			Name string `json:"name"`
		} `json:"task"`
	} `json:"result"`
}

// fetchCloudflareModels lists Workers AI models via Cloudflare's native
// endpoint and returns only the text-generation ones.
//
// The chat base URL is .../accounts/<id>/ai/v1; the listing lives one level up
// at .../accounts/<id>/ai/models/search, so the path is derived rather than
// requiring the user to supply a second URL.
//
// Of 61 models the account exposes, 26 are Text Generation — the rest are image,
// speech, embedding and classification models that cannot act as an agent at
// all. Filtering here means the picker never offers one.
func fetchCloudflareModels(base *url.URL, apiKey string) []localModel {
	if base == nil || !strings.Contains(base.Host, "api.cloudflare.com") {
		return nil
	}
	u := *base
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/v1") + "/models/search"
	u.RawQuery = "per_page=100"

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Debug("Cloudflare model listing failed", "error", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logging.Debug("Cloudflare model listing rejected", "status", resp.StatusCode)
		return nil
	}
	var out cfModelSearch
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Success {
		return nil
	}
	models := make([]localModel, 0, len(out.Result))
	for _, m := range out.Result {
		if m.Task.Name != "Text Generation" {
			continue
		}
		models = append(models, localModel{ID: m.Name})
	}
	return models
}

type localModelList struct {
	Data []localModel `json:"data"`
}

type localModel struct {
	ID                  string `json:"id"`
	Object              string `json:"object"`
	Type                string `json:"type"`
	Publisher           string `json:"publisher"`
	Arch                string `json:"arch"`
	CompatibilityType   string `json:"compatibility_type"`
	Quantization        string `json:"quantization"`
	State               string `json:"state"`
	MaxContextLength    int64  `json:"max_context_length"`
	LoadedContextLength int64  `json:"loaded_context_length"`
}

func listLocalModels(modelsEndpoint, apiKey string) []localModel {
	req, err := http.NewRequest(http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		logging.Debug("Failed to build local models request",
			"error", err,
			"endpoint", modelsEndpoint,
		)
		return []localModel{}
	}
	// GORILLA OVERRIDE: allow authenticated OpenAI-compatible endpoints
	// (e.g. NVIDIA NIM) to act as the "local" provider — the original
	// unauthenticated http.Get gets 401 from any keyed endpoint.
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Debug("Failed to list local models",
			"error", err,
			"endpoint", modelsEndpoint,
		)
		return []localModel{}
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		logging.Debug("Failed to list local models",
			"status", res.StatusCode,
			"endpoint", modelsEndpoint,
		)
		return []localModel{}
	}

	var modelList localModelList
	if err = json.NewDecoder(res.Body).Decode(&modelList); err != nil {
		logging.Debug("Failed to list local models",
			"error", err,
			"endpoint", modelsEndpoint,
		)
		return []localModel{}
	}

	var supportedModels []localModel
	for _, model := range modelList.Data {
		if strings.HasSuffix(modelsEndpoint, lmStudioBetaModelsPath) {
			if model.Object != "model" || model.Type != "llm" {
				logging.Debug("Skipping unsupported LMStudio model",
					"endpoint", modelsEndpoint,
					"id", model.ID,
					"object", model.Object,
					"type", model.Type,
				)

				continue
			}
		}

		supportedModels = append(supportedModels, model)
	}

	return supportedModels
}

func convertLocalModel(model localModel) Model {
	// GORILLA OVERRIDE: enrich discovered models with bundled metadata
	// (curated name, capability description, real context window, pricing) so a
	// user facing 100+ NVIDIA NIM models can tell them apart. Falls
	// back to the auto-generated name when a model isn't in the bundle.
	name := friendlyModelName(model.ID)
	description := ""
	var metaCtx int64
	rank := 0
	var costIn, costInCached, costOut, costOutCached float64
	if meta, ok := lookupModelMeta(model.ID); ok {
		if meta.Name != "" {
			name = meta.Name
		}
		description = meta.Description
		metaCtx = meta.ContextWindow
		rank = meta.Rank
		costIn = meta.CostIn
		costInCached = meta.CostInCached
		costOut = meta.CostOut
		costOutCached = meta.CostOutCached
	}
	ctx := cmp.Or(model.LoadedContextLength, metaCtx, 32768)
	return Model{
		ID:          ModelID("local." + model.ID),
		Name:        name,
		Description: description,
		Rank:        rank,
		Provider:    ProviderLocal,
		APIModel:    model.ID,
		// GORILLA OVERRIDE: prefer the endpoint's reported length, then
		// bundled metadata, then a conservative 32K floor. 4096 (the
		// original) crippled endpoints that report nothing (Ollama
		// /v1/models, NVIDIA NIM).
		ContextWindow:    ctx,
		DefaultMaxTokens: min(ctx/2, 8192),
		// GORILLA OVERRIDE: local models must not be assumed reasoning-capable.
		// CanReason=true made the OpenAI-compat client send reasoning_effort,
		// which Ollama (2026) rejects with 400 "does not support thinking"
		// for non-thinking models like qwen2.5-coder.
		CanReason:           false,
		SupportsAttachments: true,
		// GORILLA OVERRIDE: copy token pricing from curated metadata
		// so the cost meter moves for NIM/Ollama/LM Studio models.
		CostPer1MIn:        costIn,
		CostPer1MOut:       costOut,
		CostPer1MInCached:  costInCached,
		CostPer1MOutCached: costOutCached,
	}
}

var modelInfoRegex = regexp.MustCompile(`(?i)^([a-z0-9]+)(?:[-_]?([rv]?\d[\.\d]*))?(?:[-_]?([a-z]+))?.*`)

func friendlyModelName(modelID string) string {
	mainID := modelID
	tag := ""

	if slash := strings.LastIndex(mainID, "/"); slash != -1 {
		mainID = mainID[slash+1:]
	}

	if at := strings.Index(modelID, "@"); at != -1 {
		mainID = modelID[:at]
		tag = modelID[at+1:]
	}

	match := modelInfoRegex.FindStringSubmatch(mainID)
	if match == nil {
		return modelID
	}

	capitalize := func(s string) string {
		if s == "" {
			return ""
		}
		runes := []rune(s)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}

	family := capitalize(match[1])
	version := ""
	label := ""

	if len(match) > 2 && match[2] != "" {
		version = strings.ToUpper(match[2])
	}

	if len(match) > 3 && match[3] != "" {
		label = capitalize(match[3])
	}

	var parts []string
	if family != "" {
		parts = append(parts, family)
	}
	if version != "" {
		parts = append(parts, version)
	}
	if label != "" {
		parts = append(parts, label)
	}
	if tag != "" {
		parts = append(parts, tag)
	}

	return strings.Join(parts, " ")
}

// RegisterLocalRouteForTest and ClearLocalRouteForTest expose the route table to
// tests in other packages. localRoute is intentionally unexported — a route is
// only legitimately created by RegisterLocalEndpoint after a successful /v1/models
// listing — but config's validateAgent needs to be testable against a routed
// model without standing up a real endpoint. GORILLA OVERRIDE.
func RegisterLocalRouteForTest(id ModelID, baseURL, apiKey string) {
	localRoute[id] = localRouteInfo{BaseURL: baseURL, APIKey: apiKey, Endpoint: "test"}
}

func ClearLocalRouteForTest(id ModelID) { delete(localRoute, id) }

// RegisterLocalRouteForTestNamed is the same but records the OWNING endpoint,
// which matters for any test about removal: routes are dropped by endpoint name,
// not by baseURL, precisely because several endpoints can share a URL.
func RegisterLocalRouteForTestNamed(id ModelID, baseURL, apiKey, endpoint string) {
	localRoute[id] = localRouteInfo{BaseURL: baseURL, APIKey: apiKey, Endpoint: endpoint}
}

// LocalEndpointFor returns the user's name for the connection a local model is
// served by, or "" if the model is not local.
func LocalEndpointFor(id ModelID) string { return localRoute[id].Endpoint }

// HasLocalModels reports whether any OpenAI-compatible endpoint successfully
// registered models.
//
// GORILLA OVERRIDE: the /model picker builds its provider list from
// cfg.Providers plus the *_API_KEY environment variables. ProviderLocal appears
// in neither — local endpoints are configured as localEndpoints entries and
// authenticate with their own per-endpoint key — so every discovered local
// model was invisible in the picker. With an NVIDIA NIM key added through
// /connect that meant 102 working models registered and none selectable, which
// looks exactly like the key having been rejected.
func HasLocalModels() bool { return len(localRoute) > 0 }
