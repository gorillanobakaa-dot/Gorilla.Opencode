package tools

// GORILLA: tests for the four registry gold-seam backends. Each stubs its API
// with the recorded wire shape and asserts the mapping into searchHit — the
// same discipline as searxng_test.go. The wire shapes were taken from live
// responses during the 2026-08-17 triage; if an API drifts, the LIVE smoke
// (scripts side) catches it, and these keep the parser honest.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stubAPI(t *testing.T, target *string, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	old := *target
	*target = srv.URL
	t.Cleanup(func() { *target = old })
}

// stubTool returns a tool whose HTTP client accepts loopback, because httptest
// binds 127.0.0.1 and the production client's SSRF guard (correctly) refuses
// it. Production keeps newSafeClient; only the parser is under test here.
func stubTool() *webSearchTool {
	tool := newTestTool()
	tool.client = &http.Client{}
	return tool
}

func TestGDELTParsesArticles(t *testing.T) {
	stubAPI(t, &gdeltAPI, `{"articles":[
	  {"url":"https://example.org/a","title":"Border clash reported","seendate":"20260815T120000Z","domain":"example.org","language":"english"},
	  {"url":"https://andina.pe/b","title":"Informe regional","seendate":"20260816T090000Z","domain":"andina.pe","language":"spanish"}]}`)

	hits, err := stubTool().searchGDELT(context.Background(), "border clash", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d", len(hits))
	}
	if hits[0].Year != "2026" || hits[0].Venue != "example.org" {
		t.Errorf("first hit mapped wrong: %+v", hits[0])
	}
	// Non-English sources are labelled, not hidden — GDELT's reach beyond
	// English media is the point of the backend.
	if !strings.Contains(hits[1].Venue, "spanish") {
		t.Errorf("language not surfaced: %+v", hits[1])
	}
}

func TestWorldBankParsesDocumentsMap(t *testing.T) {
	// The WDS API returns documents as a keyed OBJECT (D1, D2…) with a
	// "facets" sibling that must be skipped — not an array.
	stubAPI(t, &worldBankAPI, `{"documents":{
	  "D1":{"display_title":"Kenya Economic Update","pdfurl":"https://documents.worldbank.org/x.pdf","docdt":"2023-06-30T04:00:00Z","docty":"Report"},
	  "facets":{}}}`)

	hits, err := stubTool().searchWorldBank(context.Background(), "kenya", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit (facets skipped), got %d", len(hits))
	}
	h := hits[0]
	if h.Title != "Kenya Economic Update" || h.Year != "2023" || h.FreePDF == "" {
		t.Errorf("mapping wrong: %+v", h)
	}
}

func TestHDXParsesDatasets(t *testing.T) {
	// CKAN wraps everything in {success, result:{results:[…]}}; the dataset URL
	// is reconstructed from the slug. This source replaced ReliefWeb's API,
	// which wanted a registered appname (see the GORILLA OVERRIDE in
	// searchHDX for the coverage analysis behind the swap).
	stubAPI(t, &hdxAPI, `{"success":true,"result":{"results":[{
	  "name":"sudan-displacement-idps-dtm","title":"Sudan: Displacement Situation - IDPs [IOM DTM]",
	  "last_modified":"2026-08-04T10:00:00","organization":{"title":"International Organization for Migration"}}]}}`)

	hits, err := stubTool().searchHDX(context.Background(), "sudan", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	h := hits[0]
	if h.Venue != "International Organization for Migration" || h.Year != "2026" {
		t.Errorf("mapping wrong: %+v", h)
	}
	if h.URL != "https://data.humdata.org/dataset/sudan-displacement-idps-dtm" {
		t.Errorf("dataset URL not reconstructed from slug: %q", h.URL)
	}
}

func TestSECEDGARBuildsArchiveURLs(t *testing.T) {
	stubAPI(t, &secEdgarAPI, `{"hits":{"hits":[{
	  "_id":"0001628280-24-000123:tsla-10k.htm",
	  "_source":{"display_names":["Tesla Inc"],"file_date":"2024-01-30","file_type":"10-K","ciks":["0001318605"]}}]}}`)

	hits, err := stubTool().searchSECEDGAR(context.Background(), "battery recall", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	h := hits[0]
	// CIK left-zeros stripped, accession dashes removed — the SEC archive layout.
	want := "https://www.sec.gov/Archives/edgar/data/1318605/000162828024000123/tsla-10k.htm"
	if h.URL != want {
		t.Errorf("archive URL wrong:\n got %s\nwant %s", h.URL, want)
	}
	if !strings.Contains(h.Title, "Tesla") || !strings.Contains(h.Title, "10-K") {
		t.Errorf("title mapping wrong: %q", h.Title)
	}
}

// The enum, the dispatch and the description must agree — a source listed in
// one and missing in another is a model-visible lie.
func TestNewSourcesAreDeclaredEverywhere(t *testing.T) {
	info := NewWebSearchTool(nil).Info()
	enum := info.Parameters["source"].(map[string]any)["enum"].([]string)
	set := map[string]bool{}
	for _, e := range enum {
		set[e] = true
	}
	for _, want := range []string{"news", "worldbank", "humanitarian", "sec", "preprints"} {
		if !set[want] {
			t.Errorf("source %q missing from the schema enum", want)
		}
		if !strings.Contains(info.Description, want) {
			t.Errorf("source %q missing from the description", want)
		}
	}
}
