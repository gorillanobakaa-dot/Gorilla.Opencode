// GORILLA OVERRIDE: generates internal/llm/models/openrouter_generated.go from
// OpenRouter's public catalogue.
//
//	go run ./cmd/openrouter-models > internal/llm/models/openrouter_generated.go
//
// # WHY GENERATED AND NOT FETCHED AT RUNTIME
//
// Directive §8: this ships to people on single-digit-KB/s links. A runtime fetch
// makes every launch wait on openrouter.ai, fails offline, and silently changes
// the prices the cost display is based on. Generated instead: no network at
// startup, prices pinned and reviewable in a diff, and regenerating is a
// deliberate act with a visible commit.
//
// # WHY IT DROPS MODELS
//
// Only models advertising the "tools" parameter are emitted. OpenRouter lists
// ~400; a third of them cannot call tools at all, and a coding agent handed one
// of those is simply broken - it will describe edits it cannot make. Filtering
// them out is not curation, it is removing entries that cannot work here.
//
// # RANKING
//
// The picker shows every model and ranks the good ones first (see
// getModelsForProvider). Rank 1..N is assigned to FREE models that can call
// tools, cheapest-capable first, because for this project's audience "free" is
// the difference between usable and not. Everything else is emitted unranked and
// falls to the codingRank heuristic, which is guidance rather than a gate.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const catalogueURL = "https://openrouter.ai/api/v1/models"

// maxFreeRanked caps how many free models get an explicit Rank. The picker
// prints ranked models as "N. Name — description" and the reference screen is
// 1600x900; a ranked list longer than a screenful stops being a
// recommendation and becomes another wall of text.
const maxFreeRanked = 12

type orModel struct {
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
}

func (m orModel) supports(param string) bool {
	for _, p := range m.SupportedArgs {
		if p == param {
			return true
		}
	}
	return false
}

// perMillion converts OpenRouter's per-token string price. Absent or malformed
// prices become 0, which the picker renders as free - so a parse failure would
// tell someone a paid model costs nothing. Callers must treat "" as unknown,
// which is why isFree() checks the raw string rather than the float.
func perMillion(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f * 1_000_000
}

func (m orModel) isFree() bool {
	return m.Pricing.Prompt == "0" && m.Pricing.Completion == "0"
}

var idSanitiser = regexp.MustCompile(`[^A-Za-z0-9]+`)

// goIdent turns "meta-llama/llama-3.3-70b-instruct:free" into
// "OpenRouterMetaLlamaLlama3370bInstructFree".
func goIdent(id string) string {
	parts := idSanitiser.Split(id, -1)
	var b strings.Builder
	b.WriteString("OpenRouter")
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

func main() {
	catalogue, err := fetch()
	if err != nil {
		fmt.Fprintln(os.Stderr, "openrouter-models:", err)
		os.Exit(1)
	}

	var usable []orModel
	for _, m := range catalogue {
		// Batch endpoints are asynchronous bulk jobs - an interactive agent
		// pointed at one waits indefinitely. Same judgement as the tools filter:
		// removing entries that cannot do this job, not curating taste.
		if m.supports("tools") && !models.IsBatchVariant(m.ID) {
			usable = append(usable, m)
		}
	}
	if len(usable) == 0 {
		// Refuse rather than emit an empty file: a generator that silently
		// produces nothing is how a provider quietly loses all its models.
		fmt.Fprintln(os.Stderr, "openrouter-models: catalogue returned no tool-capable models — refusing to write an empty registry")
		os.Exit(1)
	}

	// Free, tool-capable models get the explicit ranks. Larger context first as
	// a rough proxy for usefulness when nothing costs anything.
	var free []orModel
	for _, m := range usable {
		if m.isFree() {
			free = append(free, m)
		}
	}
	sort.Slice(free, func(i, j int) bool {
		if free[i].ContextLength != free[j].ContextLength {
			return free[i].ContextLength > free[j].ContextLength
		}
		return free[i].ID < free[j].ID
	})
	rank := map[string]int{}
	for i, m := range free {
		if i >= maxFreeRanked {
			break
		}
		rank[m.ID] = i + 1
	}

	sort.Slice(usable, func(i, j int) bool { return usable[i].ID < usable[j].ID })
	emit(usable, rank, len(catalogue), len(free))
}

func fetch() ([]orModel, error) {
	req, err := http.NewRequest("GET", catalogueURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", catalogueURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return nil, fmt.Errorf("HTTP %d from openrouter: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Data []orModel `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("parsing catalogue: %w", err)
	}
	return out.Data, nil
}

func emit(usable []orModel, rank map[string]int, total, freeCount int) {
	w := os.Stdout
	fmt.Fprintf(w, `// Code generated by "go run ./cmd/openrouter-models". DO NOT EDIT.
//
// Source: %s
// Generated: %s
// Catalogue held %d models; %d advertise tool support and are listed here;
// %d of those are free. Models that cannot call tools are omitted - a coding
// agent handed one describes edits it cannot make.
//
// Regenerate deliberately and review the diff: these prices drive the cost
// display, and a silent change to them is a silent change to what someone is
// told they are spending.

package models

const (
`, catalogueURL, time.Now().UTC().Format("2006-01-02"), total, len(usable), freeCount)

	for _, m := range usable {
		fmt.Fprintf(w, "\t%s ModelID = %q\n", goIdent(m.ID), "openrouter."+m.ID)
	}
	fmt.Fprintf(w, ")\n\nvar OpenRouterGeneratedModels = map[ModelID]Model{\n")

	for _, m := range usable {
		maxTok := m.TopProvider.MaxCompletionTokens
		if maxTok == 0 {
			maxTok = 4096
		}
		attach := false
		for _, mod := range m.Architecture.InputModalities {
			if mod == "image" {
				attach = true
			}
		}
		fmt.Fprintf(w, "\t%s: {\n", goIdent(m.ID))
		fmt.Fprintf(w, "\t\tID:                  %s,\n", goIdent(m.ID))
		fmt.Fprintf(w, "\t\tName:                %q,\n", m.Name)
		fmt.Fprintf(w, "\t\tDescription:         %q,\n", describeModel(m))
		if r := rank[m.ID]; r > 0 {
			fmt.Fprintf(w, "\t\tRank:                %d,\n", r)
		}
		fmt.Fprintf(w, "\t\tProvider:            ProviderOpenRouter,\n")
		fmt.Fprintf(w, "\t\tAPIModel:            %q,\n", m.ID)
		fmt.Fprintf(w, "\t\tCostPer1MIn:         %g,\n", perMillion(m.Pricing.Prompt))
		fmt.Fprintf(w, "\t\tCostPer1MOut:        %g,\n", perMillion(m.Pricing.Completion))
		fmt.Fprintf(w, "\t\tCostPer1MInCached:   %g,\n", perMillion(m.Pricing.InputCacheRead))
		fmt.Fprintf(w, "\t\tContextWindow:       %d,\n", m.ContextLength)
		fmt.Fprintf(w, "\t\tDefaultMaxTokens:    %d,\n", maxTok)
		fmt.Fprintf(w, "\t\tCanReason:           %t,\n", m.supports("reasoning"))
		fmt.Fprintf(w, "\t\tSupportsAttachments: %t,\n", attach)
		fmt.Fprintf(w, "\t},\n")
	}
	fmt.Fprintf(w, "}\n")
}

// describeModel prefers a verdict already earned in metadata/nim.json over the
// vendor's own marketing. Twenty-three of OpenRouter's models are the same
// underlying model as one already judged there; the rest are marked unverified,
// because none of them has ever been tested here.
func describeModel(m orModel) string {
	in, out := perMillion(m.Pricing.Prompt), perMillion(m.Pricing.Completion)
	if v, ok := models.CuratedVerdict(m.ID); ok {
		return models.CleanCatalogueDescription(v, m.ContextLength, in, out, true)
	}
	return models.CleanCatalogueDescription(m.Description, m.ContextLength, in, out, false)
}
