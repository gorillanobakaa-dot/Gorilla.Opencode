package tools

// GORILLA OVERRIDE (2026-08-17): four data-source backends from the 900-source
// registry triage (docs/source-registry.json) — the keyless gold seams that
// earn real adapters because they answer question classes the scholarly
// backends cannot: world events (GDELT), development economics and official
// reports (World Bank), humanitarian data (HDX), and corporate
// filings (SEC EDGAR). All four are free, keyless, and A/B-reliability
// primary sources; each returns structured hits instead of scraped pages.
//
// Endpoints are vars, not consts, solely so tests can stub them with httptest.

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var (
	gdeltAPI     = "https://api.gdeltproject.org/api/v2/doc/doc"
	worldBankAPI = "https://search.worldbank.org/api/v3/wds"
	hdxAPI       = "https://data.humdata.org/api/3/action/package_search"
	secEdgarAPI  = "https://efts.sec.gov/LATEST/search-index"
)

// searchGDELT queries the GDELT DOC 2.0 API: global news coverage, updated
// every 15 minutes, reaching far beyond English-language media. Reliability
// prior is B — GDELT indexes the world's news, it does not vet it; the
// per-article source domain is surfaced so vetting can grade each origin.
func (t *webSearchTool) searchGDELT(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?query=%s&mode=artlist&maxrecords=%d&format=json&sort=hybridrel",
		gdeltAPI, url.QueryEscape(q), n)
	var out struct {
		Articles []struct {
			URL      string `json:"url"`
			Title    string `json:"title"`
			SeenDate string `json:"seendate"` // 20260817T120000Z
			Domain   string `json:"domain"`
			Language string `json:"language"`
		} `json:"articles"`
	}
	if err := t.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0, len(out.Articles))
	for _, a := range out.Articles {
		year := ""
		if len(a.SeenDate) >= 4 {
			year = a.SeenDate[:4]
		}
		venue := a.Domain
		if a.Language != "" && !strings.EqualFold(a.Language, "english") {
			venue += " (" + a.Language + ")"
		}
		hits = append(hits, searchHit{
			Title:   a.Title,
			Year:    year,
			Venue:   venue,
			URL:     a.URL,
			Backend: "GDELT",
		})
	}
	return hits, nil
}

// searchWorldBank queries the World Bank Documents & Reports API: country
// analyses, project reports, working papers — the primary literature of
// development economics, all open access.
func (t *webSearchTool) searchWorldBank(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?format=json&qterm=%s&rows=%d&fl=display_title,pdfurl,txturl,docdt,docty,count",
		worldBankAPI, url.QueryEscape(q), n)
	var out struct {
		Documents map[string]struct {
			DisplayTitle string `json:"display_title"`
			PDFURL       string `json:"pdfurl"`
			TxtURL       string `json:"txturl"`
			DocDate      string `json:"docdt"` // 2023-06-30T04:00:00Z
			DocType      string `json:"docty"`
		} `json:"documents"`
	}
	if err := t.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0, len(out.Documents))
	for key, d := range out.Documents {
		if key == "facets" || d.DisplayTitle == "" {
			continue
		}
		link := d.PDFURL
		if link == "" {
			link = d.TxtURL
		}
		year := ""
		if len(d.DocDate) >= 4 {
			year = d.DocDate[:4]
		}
		hits = append(hits, searchHit{
			Title:   d.DisplayTitle,
			Year:    year,
			Venue:   strings.TrimSpace("World Bank " + d.DocType),
			URL:     link,
			FreePDF: d.PDFURL, // World Bank documents are open access by policy
			Backend: "World Bank",
		})
	}
	return hits, nil
}

// searchHDX queries the Humanitarian Data Exchange — OCHA's own data portal —
// via its CKAN API: datasets from IOM DTM, UNHCR, WFP, ACLED and the whole
// humanitarian cluster, each carrying its owning organisation and update date.
//
// GORILLA OVERRIDE (2026-08-17): this source was ReliefWeb's API, which asks
// for a registered appname — a signup dependency. Coverage analysis showed the
// registration buys nothing unique: HDX is the same organisation's structured
// data portal and answers keyless (verified live: 123 datasets for one query,
// freshest updated three days prior), ReliefWeb's report PAGES stay reachable
// through source=web and web_fetch, and UNHCR's population API is keyless for
// refugee statistics. Swapped rather than registered: no free capability was
// given up, and the tool keeps working on machines where nobody signed up for
// anything — which is the §8 promise.
func (t *webSearchTool) searchHDX(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?q=%s&rows=%d", hdxAPI, url.QueryEscape(q), n)
	var out struct {
		Success bool `json:"success"`
		Result  struct {
			Results []struct {
				Name         string `json:"name"`
				Title        string `json:"title"`
				LastModified string `json:"last_modified"`
				Organization struct {
					Title string `json:"title"`
				} `json:"organization"`
			} `json:"results"`
		} `json:"result"`
	}
	if err := t.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0, len(out.Result.Results))
	for _, d := range out.Result.Results {
		year := ""
		if len(d.LastModified) >= 4 {
			year = d.LastModified[:4]
		}
		venue := "HDX"
		if d.Organization.Title != "" {
			venue = d.Organization.Title
		}
		hits = append(hits, searchHit{
			Title:   d.Title,
			Year:    year,
			Venue:   venue,
			URL:     "https://data.humdata.org/dataset/" + d.Name,
			Backend: "HDX",
		})
	}
	return hits, nil
}

// searchSECEDGAR queries the SEC's full-text search over corporate filings —
// what a company told its regulator under penalty, as opposed to what its
// press release said. Filing URLs are reconstructed from CIK + accession
// number, the SEC's documented archive layout.
func (t *webSearchTool) searchSECEDGAR(ctx context.Context, q string, n int) ([]searchHit, error) {
	u := fmt.Sprintf("%s?q=%s", secEdgarAPI, url.QueryEscape(q))
	var out struct {
		Hits struct {
			Hits []struct {
				ID     string `json:"_id"` // "0001628280-24-000123:filename.htm"
				Source struct {
					DisplayNames []string `json:"display_names"`
					FileDate     string   `json:"file_date"` // 2024-01-30
					FileType     string   `json:"file_type"`
					CIKs         []string `json:"ciks"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	// SEC's fair-access policy requires an email-form User-Agent and 403s
	// anything else (measured live 2026-08-17). The address is the project's
	// public git-author contact; owner-approved, one line to change.
	if err := t.getJSONUA(ctx, u, "gorilla-opencode research tool (gorillanobakaa@gmail.com)", &out); err != nil {
		return nil, err
	}
	hits := make([]searchHit, 0)
	for _, h := range out.Hits.Hits {
		if len(hits) >= n {
			break
		}
		name := ""
		if len(h.Source.DisplayNames) > 0 {
			name = h.Source.DisplayNames[0]
		}
		year := ""
		if len(h.Source.FileDate) >= 4 {
			year = h.Source.FileDate[:4]
		}
		link := ""
		if parts := strings.SplitN(h.ID, ":", 2); len(parts) == 2 && len(h.Source.CIKs) > 0 {
			accession := strings.ReplaceAll(parts[0], "-", "")
			cik := strings.TrimLeft(h.Source.CIKs[0], "0")
			link = fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s", cik, accession, parts[1])
		}
		hits = append(hits, searchHit{
			Title:   strings.TrimSpace(name + " — " + h.Source.FileType),
			Year:    year,
			Venue:   "SEC EDGAR",
			URL:     link,
			Backend: "SEC EDGAR",
		})
	}
	return hits, nil
}
