package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
)

// GORILLA OVERRIDE: this tool exists because the agent had NO way to search.
//
// On 2026-08-07 a model was asked for papers outside PubMed and arXiv. With no
// search tool it hand-constructed query URLs for IEEE, ACM, Springer, CORE and
// Semantic Scholar from memory - every one returned 403, 404 or 429 - and then,
// cornered, fabricated a citation table and a "How I found them" narrative
// describing searches that had failed. Four of its six links resolved to real
// pages containing entirely different papers, which is worse than a dead link
// because it survives a spot-check.
//
// The fabrication was the model's choice. Being cornered was the harness's
// fault, and this is that half of the fix.
//
// Backends are chosen on measured reliability, not on brand:
//
//	DuckDuckGo's HTML endpoint is DELIBERATELY ABSENT. It answers 200 with a
//	block page ("anomaly" markers, ~14KB) on every attempt from this machine. A
//	search tool that returns 200-with-garbage is worse than none, because the
//	model cannot tell the difference and will summarise the block page.
//
// The three below were verified to return real, parseable content and need no
// API key. If a general-web backend is added later it must be behind a key and
// must fail loudly rather than return a block page.

const (
	WebSearchToolName = "web_search"

	openAlexAPI  = "https://api.openalex.org/works"
	crossrefAPI  = "https://api.crossref.org/works"
	europePMCAPI = "https://www.ebi.ac.uk/europepmc/webservices/rest/search"

	// OpenAlex and Crossref give better rate limits to requests that identify a
	// contact ("the polite pool").
	politeContact = "gorilla-opencode (+https://github.com/gorillanobakaa-dot/Gorilla.Opencode)"

	webSearchDescription = `Search the literature and the web for sources, by keyword.

USE THIS BEFORE GUESSING A URL. If you need papers, articles or references and
do not already have an exact address, search here first. Do not hand-construct
search URLs for publisher sites - most of them block automated access and you
will get 403s.

SOURCES (pick with the "source" argument):
- scholar (default) — OpenAlex. All disciplines, ~250M works, includes
  preprints, conference papers and journal articles. Best general starting point.
- medical — Europe PMC. Indexes MEDLINE/PubMed plus preprints. Use for
  clinical, biomedical and life-sciences questions.
- crossref — Crossref. DOI metadata across publishers; good for pinning down a
  citation you already half-know, or finding the publisher of record.
- all — queries all three and merges, de-duplicated by DOI.

RETURNS: title, authors, year, venue, DOI and a URL for each result, plus an
abstract where the source provides one.

IF A SEARCH RETURNS NOTHING, SAY SO. Do not fall back on remembered citations:
a plausible-looking reference that turns out to be a different paper is worse
than telling the user the search came up empty.

Note: general web search (Google/Bing/DuckDuckGo style) is NOT available here.
These three cover published literature. For a specific page you already know the
address of, use web_fetch.`
)

type WebSearchParams struct {
	Query      string `json:"query"`
	Source     string `json:"source,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type searchHit struct {
	Title    string
	Authors  string
	Year     string
	Venue    string
	DOI      string
	URL      string
	Abstract string
	Backend  string
}

type webSearchTool struct {
	permissions permission.Service
	client      *http.Client
}

func NewWebSearchTool(permissions permission.Service) BaseTool {
	return &webSearchTool{
		permissions: permissions,
		// Reuses the SSRF-safe client: these are fixed public hosts, but the
		// guard costs nothing and stops a future edit pointing this at a
		// private address.
		client: newSafeClient(30 * time.Second),
	}
}

func (t *webSearchTool) Info() ToolInfo {
	return ToolInfo{
		Name:        WebSearchToolName,
		Description: webSearchDescription,
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Keywords to search for.",
			},
			"source": map[string]any{
				"type":        "string",
				"description": "Which index to search. Defaults to scholar.",
				"enum":        []string{"scholar", "medical", "crossref", "all"},
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "How many results to return (default 8, max 25).",
			},
		},
		Required: []string{"query"},
	}
}

func (t *webSearchTool) getJSON(ctx context.Context, raw string, into any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", politeContact)
	req.Header.Set("Accept", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode,
			strings.SplitN(strings.TrimPrefix(raw, "https://"), "/", 2)[0],
			strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(into)
}

// openAlexAbstract rebuilds prose from OpenAlex's inverted index, which stores
// {word: [positions]} rather than the abstract itself.
func openAlexAbstract(idx map[string][]int) string {
	if len(idx) == 0 {
		return ""
	}
	type wp struct {
		pos  int
		word string
	}
	var words []wp
	for w, positions := range idx {
		for _, p := range positions {
			words = append(words, wp{p, w})
		}
	}
	sort.Slice(words, func(i, j int) bool { return words[i].pos < words[j].pos })
	parts := make([]string, 0, len(words))
	for _, w := range words {
		parts = append(parts, w.word)
	}
	return strings.Join(parts, " ")
}

func (t *webSearchTool) searchOpenAlex(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?search=%s&per-page=%d&mailto=%s",
		openAlexAPI, url.QueryEscape(q), n, url.QueryEscape("gorilla-opencode"))
	var out struct {
		Results []struct {
			Title           string           `json:"title"`
			PublicationYear int              `json:"publication_year"`
			DOI             string           `json:"doi"`
			AbstractIdx     map[string][]int `json:"abstract_inverted_index"`
			Authorships     []struct {
				Author struct {
					DisplayName string `json:"display_name"`
				} `json:"author"`
			} `json:"authorships"`
			PrimaryLocation struct {
				LandingPageURL string `json:"landing_page_url"`
				Source         struct {
					DisplayName string `json:"display_name"`
				} `json:"source"`
			} `json:"primary_location"`
		} `json:"results"`
	}
	if err := t.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0, len(out.Results))
	for _, r := range out.Results {
		var names []string
		for i, a := range r.Authorships {
			if i == 4 {
				names = append(names, "et al.")
				break
			}
			names = append(names, a.Author.DisplayName)
		}
		link := r.PrimaryLocation.LandingPageURL
		if link == "" {
			link = r.DOI
		}
		hits = append(hits, searchHit{
			Title: r.Title, Authors: strings.Join(names, ", "),
			Year: fmt.Sprint(r.PublicationYear), Venue: r.PrimaryLocation.Source.DisplayName,
			DOI: r.DOI, URL: link,
			Abstract: openAlexAbstract(r.AbstractIdx), Backend: "OpenAlex",
		})
	}
	return hits, nil
}

func (t *webSearchTool) searchEuropePMC(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?query=%s&format=json&pageSize=%d&resultType=core",
		europePMCAPI, url.QueryEscape(q), n)
	var out struct {
		ResultList struct {
			Result []struct {
				Title        string `json:"title"`
				AuthorString string `json:"authorString"`
				PubYear      string `json:"pubYear"`
				JournalTitle string `json:"journalTitle"`
				DOI          string `json:"doi"`
				PMID         string `json:"pmid"`
				PMCID        string `json:"pmcid"`
				AbstractText string `json:"abstractText"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := t.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0)
	for _, r := range out.ResultList.Result {
		link := ""
		switch {
		case r.PMCID != "":
			link = "https://pmc.ncbi.nlm.nih.gov/articles/" + r.PMCID + "/"
		case r.PMID != "":
			link = "https://pubmed.ncbi.nlm.nih.gov/" + r.PMID + "/"
		case r.DOI != "":
			link = "https://doi.org/" + r.DOI
		}
		hits = append(hits, searchHit{
			Title: r.Title, Authors: r.AuthorString, Year: r.PubYear,
			Venue: r.JournalTitle, DOI: r.DOI, URL: link,
			Abstract: r.AbstractText, Backend: "Europe PMC",
		})
	}
	return hits, nil
}

func (t *webSearchTool) searchCrossref(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?query=%s&rows=%d&mailto=%s",
		crossrefAPI, url.QueryEscape(q), n, url.QueryEscape("gorilla-opencode"))
	var out struct {
		Message struct {
			Items []struct {
				Title     []string `json:"title"`
				DOI       string   `json:"DOI"`
				URL       string   `json:"URL"`
				Container []string `json:"container-title"`
				Abstract  string   `json:"abstract"`
				Author    []struct {
					Given  string `json:"given"`
					Family string `json:"family"`
				} `json:"author"`
				Issued struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"issued"`
			} `json:"items"`
		} `json:"message"`
	}
	if err := t.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0)
	for _, r := range out.Message.Items {
		title, venue := "", ""
		if len(r.Title) > 0 {
			title = r.Title[0]
		}
		if len(r.Container) > 0 {
			venue = r.Container[0]
		}
		year := ""
		if len(r.Issued.DateParts) > 0 && len(r.Issued.DateParts[0]) > 0 {
			year = fmt.Sprint(r.Issued.DateParts[0][0])
		}
		var names []string
		for i, a := range r.Author {
			if i == 4 {
				names = append(names, "et al.")
				break
			}
			names = append(names, strings.TrimSpace(a.Given+" "+a.Family))
		}
		hits = append(hits, searchHit{
			Title: title, Authors: strings.Join(names, ", "), Year: year,
			Venue: venue, DOI: r.DOI, URL: r.URL,
			Abstract: stripJATS(r.Abstract), Backend: "Crossref",
		})
	}
	return hits, nil
}

// Crossref abstracts arrive as JATS XML fragments.
func stripJATS(s string) string {
	for {
		i := strings.Index(s, "<")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], ">")
		if j < 0 {
			break
		}
		s = s[:i] + " " + s[i+j+1:]
	}
	return strings.Join(strings.Fields(s), " ")
}

func (t *webSearchTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params WebSearchParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("Failed to parse web_search parameters: " + err.Error()), nil
	}
	if strings.TrimSpace(params.Query) == "" {
		return NewTextErrorResponse("query parameter is required"), nil
	}
	source := strings.ToLower(strings.TrimSpace(params.Source))
	if source == "" {
		source = "scholar"
	}
	n := params.MaxResults
	if n <= 0 {
		n = 8
	}
	if n > 25 {
		n = 25
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for web_search")
	}
	if !t.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        config.WorkingDirectory(),
		ToolName:    WebSearchToolName,
		Action:      "search",
		Description: fmt.Sprintf("Search %s for: %s", source, params.Query),
		Params:      params,
	}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	type backend struct {
		name string
		fn   func(context.Context, string, int) ([]searchHit, error)
	}
	var chosen []backend
	switch source {
	case "scholar":
		chosen = []backend{{"OpenAlex", t.searchOpenAlex}}
	case "medical":
		chosen = []backend{{"Europe PMC", t.searchEuropePMC}}
	case "crossref":
		chosen = []backend{{"Crossref", t.searchCrossref}}
	case "all":
		chosen = []backend{{"OpenAlex", t.searchOpenAlex}, {"Europe PMC", t.searchEuropePMC}, {"Crossref", t.searchCrossref}}
	default:
		return NewTextErrorResponse("source must be one of: scholar, medical, crossref, all"), nil
	}

	var hits []searchHit
	var failures []string
	seenDOI := map[string]bool{}
	for _, b := range chosen {
		got, err := b.fn(reqCtx, params.Query, n)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", b.name, err))
			continue
		}
		for _, h := range got {
			key := strings.ToLower(strings.TrimPrefix(h.DOI, "https://doi.org/"))
			if key != "" && seenDOI[key] {
				continue
			}
			if key != "" {
				seenDOI[key] = true
			}
			hits = append(hits, h)
		}
	}

	// Every backend failing must read as a failure, not as "no results".
	// Silence and success must never look alike.
	if len(hits) == 0 {
		msg := fmt.Sprintf("No results for %q.", params.Query)
		if len(failures) > 0 {
			msg = fmt.Sprintf("Search FAILED for %q - no backend answered:\n  %s\n\n"+
				"Nothing was retrieved. Tell the user the search failed; do not "+
				"substitute remembered citations.", params.Query, strings.Join(failures, "\n  "))
		}
		return NewTextErrorResponse(msg), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Searched %s for %q — %d result(s)\n", source, params.Query, len(hits))
	if len(failures) > 0 {
		fmt.Fprintf(&sb, "PARTIAL: some backends failed, results below are incomplete:\n  %s\n",
			strings.Join(failures, "\n  "))
	}
	for i, h := range hits {
		if i >= n && source != "all" {
			break
		}
		fmt.Fprintf(&sb, "\n%d. %s\n", i+1, strings.TrimSpace(h.Title))
		if h.Authors != "" {
			fmt.Fprintf(&sb, "   authors: %s\n", h.Authors)
		}
		meta := []string{}
		if h.Year != "" && h.Year != "0" {
			meta = append(meta, h.Year)
		}
		if h.Venue != "" {
			meta = append(meta, h.Venue)
		}
		if len(meta) > 0 {
			fmt.Fprintf(&sb, "   %s\n", strings.Join(meta, " · "))
		}
		if h.URL != "" {
			fmt.Fprintf(&sb, "   url: %s\n", h.URL)
		}
		if h.DOI != "" {
			fmt.Fprintf(&sb, "   doi: %s\n", h.DOI)
		}
		if a := strings.TrimSpace(h.Abstract); a != "" {
			if len(a) > 700 {
				a = a[:700] + "… [abstract truncated; web_fetch the url for the full text]"
			}
			fmt.Fprintf(&sb, "   abstract: %s\n", a)
		}
		fmt.Fprintf(&sb, "   via: %s\n", h.Backend)
	}
	return NewTextResponse(sb.String()), nil
}
