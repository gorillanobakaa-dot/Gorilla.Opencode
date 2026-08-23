// Version: 1.0.0 · updated 26-08-23-15-30
//
// GORILLA OVERRIDE (2026-08-23): the ChatGPT sign-in list is FETCHED, not typed.
//
// chatgpt.go shipped two models typed out by hand from a reading of the live
// list on 2026-08-17, and carved this provider out of the fetch-it doctrine that
// catalogue_fetch.go had already written for everybody else. The carve-out was
// justified there as "small, tied to an OAuth flow, and updated with the
// binary". Six days later the owner had Codex running gpt-5.6-luna on the same
// free Google account while this program offered him GPT-5.4-Mini, a model
// OpenAI retires on 31 Aug 2026. That is the carve-out failing exactly the way
// catalogue_fetch.go predicted a hand-typed list always fails: not loudly, just
// quietly stopping being true.
//
// The endpoint costs nothing extra. GET /models is the same call ProbeBackend
// already makes to check the sign-in works, so the list arrives on a request the
// program was making anyway.
//
// This file does NOT import internal/auth. The caller passes the raw body in,
// the same bargain antigravity_refresh.go makes: package models is imported by
// nearly everything and must not drag an OAuth stack behind it.
package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const chatgptCacheSchema = 1

// ChatGPTCatalogueEntry is one model as the backend publishes it. The live
// payload carries about forty fields per model (including the full Codex system
// prompt); only these are read, and every one of them was checked against the
// real response on 2026-08-23.
type ChatGPTCatalogueEntry struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	// Visibility is "list" or "hide". The backend uses "hide" for models that
	// are not chat models at all: codex-auto-review is an approval-review
	// endpoint, and offering it in a picker would be the same class of mistake
	// as offering whisper or an embedding model.
	Visibility string `json:"visibility"`
	// ToolMode is "code_mode_only" or absent. See the note on toolModeCodeOnly.
	ToolMode string `json:"tool_mode"`
	// ContextWindow is the operating window. MaxContextWindow is the ceiling the
	// model can reach, which is larger for the 5.6 family (872K against 272K)
	// but is NOT what an ordinary request gets, so it is deliberately not used:
	// a context window set too high sends a request the backend rejects, and the
	// user cannot tell that failure from a broken sign-in.
	ContextWindow    int64 `json:"context_window"`
	MaxContextWindow int64 `json:"max_context_window"`
	// Priority is the backend's own ordering, LOWER being more prominent.
	// Measured 2026-08-23: terra 2, luna 3, gpt-5.5 7, gpt-5.4-mini 23,
	// codex-auto-review 43. This is the provider's opinion of its own catalogue
	// and is used instead of a hand-written order, which is the entire point of
	// fetching the list.
	Priority int `json:"priority"`
	// AvailablePlans lists the ChatGPT plans entitled to the model. Recorded for
	// the log line only: the endpoint is already scoped to the caller's account,
	// so filtering on it again would be second-guessing the server.
	AvailablePlans []string `json:"available_in_plans"`
	// SupportedReasoningLevels is present on every model that thinks. Used to
	// set CanReason from the wire rather than from a guess, because claiming it
	// falsely makes the client send a reasoning field the model rejects.
	SupportedReasoningLevels []struct {
		Effort string `json:"effort"`
	} `json:"supported_reasoning_levels"`
}

type chatgptCatalogueResponse struct {
	Models []ChatGPTCatalogueEntry `json:"models"`
}

type cachedChatGPT struct {
	Schema    int               `json:"schema"`
	Refreshed time.Time         `json:"refreshed"`
	Models    map[ModelID]Model `json:"models"`
}

func chatgptCachePath(configDir string) string {
	return filepath.Join(configDir, "chatgpt-models.json")
}

// usable decides whether a fetched entry belongs in a chat model picker.
//
// The only filter is the backend's own visibility flag. Measured against the
// live response 2026-08-23: 5 entries in, 4 usable, the reject being
// codex-auto-review (visibility "hide").
//
// NOTE what is deliberately NOT filtered: tool_mode. See toolModeCodeOnly.
func (e ChatGPTCatalogueEntry) usable() bool {
	return e.Slug != "" && !strings.EqualFold(e.Visibility, "hide")
}

// toolModeCodeOnly is the wire's "code_mode_only" marker.
//
// CORRECTION, MEASURED 2026-08-23. chatgpt.go argued that a code_mode_only model
// could not be offered because "code_mode_only means the model expects its tools
// presented as a code sandbox rather than as a function schema list", and that
// registering the 5.6 pair "would put two rows in the picker that sign in fine
// and then fail on the first tool call". That reasoning was an inference from
// the name and it was never tested. It is wrong, and the original wording is
// left in place at the top of chatgpt.go so the change is visible.
//
// What the flag actually is, from the Codex source at
// ~/Downloads/codex-rust-v0.147.0 (codex-rs/features/src/lib.rs, the CodeModeOnly
// variant): "Restrict model-visible tools to code mode entrypoints (exec, wait)".
// It is a CLIENT-SIDE choice about which tools to put in the tools array, not a
// wire protocol and not a backend requirement. Codex drives these models that
// way; nothing obliges anyone else to.
//
// Verified by sending each model an ordinary Responses request carrying one
// ordinary function schema. All three answered HTTP 200 and emitted a correct
// function_call with well-formed arguments:
//
//	gpt-5.5        HTTP 200  get_weather({"city":"Bucharest"})
//	gpt-5.6-luna   HTTP 200  get_weather({"city":"Bucharest"})
//	gpt-5.6-terra  HTTP 200  get_weather({"city":"Bucharest"})
//
// So the flag is recorded and shown to the user, and does not exclude anything.
const toolModeCodeOnly = "code_mode_only"

// chatgptRankFor turns the backend's priority order into this program's Rank,
// where HIGHER is better. Ranks are assigned by position rather than by
// arithmetic on the priority number, because the numbers are sparse (2, 3, 7,
// 23) and any formula over them would produce ties or negatives the moment
// OpenAI renumbers.
func chatgptRankFor(position int) int {
	rank := chatgptTopRank - position
	if rank < 1 {
		return 1
	}
	return rank
}

// chatgptTopRank keeps this provider's best model level with the other free
// sign-ins rather than sorting it to the bottom of the picker as "unranked".
// The previous hand-written list gave GPT-5.5 rank 9; the top of the fetched
// list inherits that so the picker does not reshuffle for existing users.
const chatgptTopRank = 9

// BuildChatGPTModels converts the fetched entries into registry models, best
// first. Exported for the tests, which build from a recorded payload rather than
// from the network.
func BuildChatGPTModels(entries []ChatGPTCatalogueEntry) []Model {
	kept := make([]ChatGPTCatalogueEntry, 0, len(entries))
	for _, e := range entries {
		if e.usable() {
			kept = append(kept, e)
		}
	}
	// The backend's own order, best first. sort.SliceStable with a slug
	// tiebreak so two models sharing a priority cannot swap places between
	// launches and make the picker look unstable.
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].Priority != kept[j].Priority {
			return kept[i].Priority < kept[j].Priority
		}
		return kept[i].Slug < kept[j].Slug
	})

	out := make([]Model, 0, len(kept))
	for i, e := range kept {
		name := e.DisplayName
		if name == "" {
			name = e.Slug
		}

		ctx := e.ContextWindow
		if ctx == 0 {
			// Conservative floor rather than a guess, matching catalogueModel.
			ctx = 272_000
		}

		detail := fmt.Sprintf(
			"Reached through the Codex backend using your ChatGPT login rather than an API key, "+
				"so it costs nothing beyond the plan you already have (including the free one). "+
				"%s context. Usage counts against your ChatGPT plan's limits, so heavy use on a "+
				"free plan hits a cooldown rather than a bill.",
			humanTokens(ctx))
		if e.ToolMode == toolModeCodeOnly {
			// Said plainly rather than hidden: OpenAI's own client drives these
			// models with a code sandbox instead of function schemas. Verified
			// working here with ordinary function calls, but the user is told
			// the model is tuned for a different harness.
			detail += " OpenAI's own client runs this model with a code sandbox instead of a tool " +
				"list; ordinary tool calls were verified working here on 2026-08-23, but it is " +
				"tuned for that other shape."
		}

		out = append(out, Model{
			ID:          ModelID(string(ProviderChatGPT) + "." + e.Slug),
			Name:        name + " (ChatGPT sign-in)",
			Description: e.Description,
			Detail:      detail,
			Provider:    ProviderChatGPT,
			APIModel:    e.Slug,

			ContextWindow: ctx,
			// Half the window, bounded. Not a capability claim, a per-request cap.
			DefaultMaxTokens: min(ctx/2, 32_000),
			// From the wire, not from a guess: every model that thinks publishes
			// its reasoning levels.
			CanReason:           len(e.SupportedReasoningLevels) > 0,
			SupportsAttachments: true,
			Rank:                chatgptRankFor(i),

			// Zero in every cost field because nothing here is billed per token.
			// The ChatGPT plan is, and a free plan is not billed at all. Do not
			// populate these with API prices: they are shown to the user as
			// money this path never charges.
			CostPer1MIn:  0,
			CostPer1MOut: 0,
		})
	}
	return out
}

// humanTokens renders a context window the way the picker text reads best.
func humanTokens(n int64) string {
	if n >= 1000 && n%1000 == 0 {
		return fmt.Sprintf("%dK", n/1000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// ChatGPTRefreshResult reports what changed, in the units a user cares about.
type ChatGPTRefreshResult struct {
	Fetched int
	Usable  int
	Skipped int
	Added   []string
	Removed []string
}

// ParseChatGPTCatalogue reads the GET /models body. Exported so the caller can
// parse without knowing the shape, and so a test can feed it a recorded payload.
func ParseChatGPTCatalogue(raw []byte) ([]ChatGPTCatalogueEntry, error) {
	var resp chatgptCatalogueResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("ChatGPT model list was unreadable: %w", err)
	}
	return resp.Models, nil
}

// RefreshChatGPT parses a fetched body, registers what it found and caches it.
//
// It REPLACES this provider's entries rather than merging them, deliberately and
// unlike RefreshAntigravity: merging keeps a model the provider has retired, and
// a retirement is exactly what is coming (GPT-5.4 and 5.4-mini stop being served
// to ChatGPT sign-ins on 31 Aug 2026). A merge would leave that entry sitting in
// the picker looking selectable and returning a 400 to whoever chose it.
func RefreshChatGPT(configDir string, raw []byte) (*ChatGPTRefreshResult, error) {
	entries, err := ParseChatGPTCatalogue(raw)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		// An empty list is not success. Registering it would silently empty the
		// picker for this provider and read exactly like a working refresh.
		return nil, fmt.Errorf("the ChatGPT model list came back empty")
	}

	built := BuildChatGPTModels(entries)
	if len(built) == 0 {
		return nil, fmt.Errorf("the ChatGPT model list held %d entries, none of them chat models", len(entries))
	}

	res := &ChatGPTRefreshResult{Fetched: len(entries), Usable: len(built)}
	res.Skipped = res.Fetched - res.Usable

	before := idsFor(ProviderChatGPT)
	byID := make(map[ModelID]Model, len(built))
	for _, m := range built {
		byID[m.ID] = m
	}

	if err := writeChatGPTCache(configDir, byID); err != nil {
		// The models are not registered yet, so failing here loses nothing that
		// was working. Report it rather than registering a list we cannot keep.
		return nil, err
	}

	applyChatGPT(built)

	added, removed := diffIDs(before, idsFor(ProviderChatGPT))
	res.Added, res.Removed = trimChatGPTPrefix(added), trimChatGPTPrefix(removed)
	sort.Strings(res.Added)
	sort.Strings(res.Removed)
	return res, nil
}

func trimChatGPTPrefix(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strings.TrimPrefix(id, string(ProviderChatGPT)+"."))
	}
	return out
}

func writeChatGPTCache(configDir string, built map[ModelID]Model) error {
	blob, err := json.MarshalIndent(cachedChatGPT{
		Schema:    chatgptCacheSchema,
		Refreshed: time.Now().UTC(),
		Models:    built,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	// Write-then-rename: a half-written cache read at startup is worse than no
	// cache, because it looks like a catalogue.
	tmp := chatgptCachePath(configDir) + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, chatgptCachePath(configDir))
}

// applyChatGPT replaces every registered entry for this provider.
func applyChatGPT(built []Model) {
	for id, m := range SupportedModels {
		if m.Provider == ProviderChatGPT {
			delete(SupportedModels, id)
		}
	}
	for _, m := range built {
		SupportedModels[m.ID] = m
	}
}

// LoadRefreshedChatGPT applies a previously fetched list at startup. Reads the
// disk only, never the network, so a slow or absent connection cannot delay
// launch. A missing, unreadable, corrupt or wrong-schema cache is not an error:
// the built-in list in chatgpt.go keeps working.
func LoadRefreshedChatGPT(configDir string) (int, error) {
	blob, err := os.ReadFile(chatgptCachePath(configDir))
	if err != nil {
		return 0, nil
	}
	var cached cachedChatGPT
	if err := json.Unmarshal(blob, &cached); err != nil {
		return 0, fmt.Errorf("ChatGPT model cache unreadable, using built-in list: %w", err)
	}
	if cached.Schema != chatgptCacheSchema || len(cached.Models) == 0 {
		return 0, nil
	}
	built := make([]Model, 0, len(cached.Models))
	for _, m := range cached.Models {
		built = append(built, m)
	}
	applyChatGPT(built)
	return len(built), nil
}

// ChatGPTCatalogueAge reports when the list was last fetched. Used to tell the
// user how old the list they are looking at is: a list with no date is not a
// measurement.
func ChatGPTCatalogueAge(configDir string) (age time.Duration, ok bool) {
	blob, err := os.ReadFile(chatgptCachePath(configDir))
	if err != nil {
		return 0, false
	}
	var cached cachedChatGPT
	if err := json.Unmarshal(blob, &cached); err != nil || cached.Refreshed.IsZero() {
		return 0, false
	}
	return time.Since(cached.Refreshed), true
}

// PreferredChatGPTModels reports which registered ChatGPT model to put the coder
// on and which to put the background agents (title, summariser, task) on.
//
// GORILLA OVERRIDE (2026-08-23): these used to be two constants at the sign-in
// site, ChatGPT55 and ChatGPT54Mini. That is a hand-written preference in the
// one place a hand-written preference is most expensive, because it runs once,
// at sign-in, and then silently decides what the user is on for the rest of the
// session. It also pinned the background agents to a model OpenAI retires on
// 31 Aug 2026.
//
// best is the highest-ranked model, which is the backend's own first choice
// (see chatgptRankFor). cheap is the lowest-ranked, deliberately: on a free plan
// the COOLDOWN is the scarce resource, not money, so the strong model should not
// be spent generating conversation titles.
//
// Returns ("", "") when nothing is registered, which the caller must treat as
// "leave the current selection alone" rather than as a model id.
func PreferredChatGPTModels() (best, cheap ModelID) {
	bestRank, cheapRank := -1, -1
	for id, m := range SupportedModels {
		if m.Provider != ProviderChatGPT {
			continue
		}
		if bestRank < 0 || m.Rank > bestRank {
			best, bestRank = id, m.Rank
		}
		if cheapRank < 0 || m.Rank < cheapRank {
			cheap, cheapRank = id, m.Rank
		}
	}
	// One model registered is a legitimate state (it is what 1 Sep 2026 looks
	// like if OpenAI retires the rest). Both roles then land on it rather than
	// one of them landing on "".
	if best != "" && cheap == "" {
		cheap = best
	}
	return best, cheap
}
