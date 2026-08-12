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
//
// # WHY IT ALSO FETCHES EVERY MODEL'S WEB PAGE
//
// OpenRouter's list API truncates descriptions server-side: 354 of 406 ended
// in "..." when measured on 2026-08-12, cut around 215 characters. The full
// text exists only in each model's public page. A detail page built from the
// truncated copy tells the user nothing — so this generator, which runs on the
// developer's machine at release time, fetches the ~280 pages (~300 KB each)
// and embeds the full text. The runtime refresh on user machines never does
// this (§8: that is ~85 MB, hours on a metered link); it keeps the bundled
// full text instead via PreferFullerDetail.
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
	full := fetchFullDescriptions(usable)
	emit(usable, rank, full, len(catalogue), len(free))
}

// fetchFullDescriptions pulls each model's public page and extracts the
// untruncated description. Returns id → full text; an absent entry means the
// page could not be fetched or matched and the API's truncated text stands.
// Variants of one model (":free") share their base model's page, so pages are
// fetched once per base id.
func fetchFullDescriptions(usable []orModel) map[string]string {
	client := &http.Client{Timeout: 30 * time.Second}

	baseID := func(id string) string {
		if i := strings.Index(id, ":"); i > 0 {
			return id[:i]
		}
		return id
	}

	// One representative API description per base page, to anchor extraction.
	anchors := map[string]string{}
	var bases []string
	for _, m := range usable {
		b := baseID(m.ID)
		if _, seen := anchors[b]; !seen {
			anchors[b] = m.Description
			bases = append(bases, b)
		}
	}

	type result struct{ base, text string }
	jobs := make(chan string)
	results := make(chan result)
	const workers = 4
	for w := 0; w < workers; w++ {
		go func() {
			for b := range jobs {
				text := fetchPageDescription(client, b, anchors[b])
				if text == "" {
					// One retry: measured across runs, a single pass loses
					// ~10 pages to transient fetch failures.
					time.Sleep(2 * time.Second)
					text = fetchPageDescription(client, b, anchors[b])
				}
				results <- result{b, text}
			}
		}()
	}
	go func() {
		for _, b := range bases {
			jobs <- b
		}
		close(jobs)
	}()

	full := map[string]string{}
	byBase := map[string]string{}
	done, missed := 0, 0
	for range bases {
		r := <-results
		done++
		if r.text == "" {
			missed++
		} else {
			byBase[r.base] = r.text
		}
		if done%50 == 0 {
			fmt.Fprintf(os.Stderr, "openrouter-models: %d/%d pages fetched\n", done, len(bases))
		}
	}
	for _, m := range usable {
		if t := byBase[baseID(m.ID)]; t != "" {
			full[m.ID] = t
		}
	}
	fmt.Fprintf(os.Stderr, "openrouter-models: full descriptions for %d/%d models (%d pages unmatched — those keep the API text)\n",
		len(full), len(usable), missed)
	return full
}

// fetchPageDescription downloads one model page and returns the untruncated
// description, or "" if it cannot be found. The page embeds several models'
// descriptions (related-model sections), so the right one is the LONGEST field
// that starts with the API text's stem — the API truncation is a prefix cut.
func fetchPageDescription(client *http.Client, id, apiDesc string) string {
	req, err := http.NewRequest("GET", "https://openrouter.ai/"+id, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) gorilla-opencode-generator")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return ""
	}

	stem := strings.TrimRight(strings.TrimSpace(apiDesc), ".…")
	if len(stem) > 100 {
		stem = stem[:100]
	}
	if stem == "" {
		return ""
	}
	html := string(page)
	best := ""
	const marker = `\"description\":\"`
	for i := 0; ; {
		j := strings.Index(html[i:], marker)
		if j < 0 {
			break
		}
		txt, next := decodeFlightString(html, i+j+len(marker))
		if strings.HasPrefix(txt, stem) && len(txt) > len(best) {
			best = txt
		}
		i = next
	}
	// Anything not meaningfully longer than the API copy is not worth the
	// bytes it would add to the binary.
	if len(best) <= len(apiDesc) {
		return ""
	}
	return best
}

// decodeFlightString reads a backslash-escaped JSON string embedded in the
// page's script payload, starting just after its opening quote. Returns the
// decoded text and the index after the closing quote.
func decodeFlightString(s string, start int) (string, int) {
	var b strings.Builder
	i := start
	for i < len(s)-1 {
		switch {
		// A newline in the text is JSON-escaped twice (once by the model
		// object, once by the script payload wrapping it), arriving as the
		// three characters \\n. Decoding the pair alone leaves a literal
		// "\n" in the description, which the picker then prints verbatim.
		case strings.HasPrefix(s[i:], `\\n`):
			b.WriteByte('\n')
			i += 3
		case s[i] == '\\' && s[i+1] == '\\':
			b.WriteByte('\\')
			i += 2
		case s[i] == '\\' && s[i+1] == '"':
			// The closing quote of the field is followed by a JSON separator;
			// an inner escaped quote is not.
			if i+2 < len(s) && (s[i+2] == ',' || s[i+2] == '}' || s[i+2] == ']') {
				return b.String(), i + 2
			}
			b.WriteByte('"')
			i += 2
		case s[i] == '\\' && s[i+1] == 'n':
			b.WriteByte('\n')
			i += 2
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String(), i
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

func emit(usable []orModel, rank map[string]int, full map[string]string, total, freeCount int) {
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
		// The full page text feeds BOTH fields: the one-line label benefits
		// too, because classification (CAN CODE / roleplay / …) reads words
		// the API's truncation may have cut off.
		desc := m.Description
		if f := full[m.ID]; f != "" {
			desc = f
		}
		fmt.Fprintf(w, "\t%s: {\n", goIdent(m.ID))
		fmt.Fprintf(w, "\t\tID:                  %s,\n", goIdent(m.ID))
		fmt.Fprintf(w, "\t\tName:                %q,\n", m.Name)
		fmt.Fprintf(w, "\t\tDescription:         %q,\n", models.DescribeForPicker(m.ID, desc, m.ContextLength, perMillion(m.Pricing.Prompt), perMillion(m.Pricing.Completion)))
		if d := models.DetailForPicker(m.ID, desc); d != "" {
			fmt.Fprintf(w, "\t\tDetail:              %q,\n", d)
		}
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
