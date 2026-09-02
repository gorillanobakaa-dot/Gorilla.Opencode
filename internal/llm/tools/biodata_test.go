package tools

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// ── offline: the logic that can rot without anyone noticing ──────────────────

func TestExtractRecordsHandlesEveryShapeTheseAPIsReturn(t *testing.T) {
	// A list under a key: UniProt, ChEMBL, openFDA.
	got := extractRecords(map[string]any{"results": []any{
		map[string]any{"a": 1}, map[string]any{"a": 2},
	}}, "results")
	if len(got) != 2 {
		t.Errorf("list under a key: got %d records, want 2", len(got))
	}

	// A dotted path: PubChem's PropertyTable.Properties.
	got = extractRecords(map[string]any{
		"PropertyTable": map[string]any{"Properties": []any{map[string]any{"CID": 2244}}},
	}, "PropertyTable.Properties")
	if len(got) != 1 {
		t.Errorf("dotted path: got %d records, want 1", len(got))
	}

	// The whole response IS the record: Ensembl, PDB, AlphaFold.
	got = extractRecords(map[string]any{"id": "ENSG00000012048"}, "")
	if len(got) != 1 {
		t.Errorf("bare object: got %d records, want 1", len(got))
	}

	// A bare top-level list: STRING.
	got = extractRecords([]any{map[string]any{"preferredName": "TP53"}}, "")
	if len(got) != 1 {
		t.Errorf("bare list: got %d records, want 1", len(got))
	}

	// Missing path must be empty, not a panic.
	if got = extractRecords(map[string]any{"x": 1}, "nope.deeper"); len(got) != 0 {
		t.Errorf("missing path: got %d records, want 0", len(got))
	}
}

func TestFlattenJSONKeepsWhatAReaderWants(t *testing.T) {
	// Integers must not render as 2244.000000 — an ID is not a measurement.
	if got := flattenJSON(float64(2244)); got != "2244" {
		t.Errorf("integer rendered as %q", got)
	}
	if got := flattenJSON(180.16); got != "180.16" {
		t.Errorf("float rendered as %q", got)
	}
	if got := flattenJSON(nil); got != "" {
		t.Errorf("nil rendered as %q, want empty", got)
	}
	if got := flattenJSON([]any{"a", "b"}); got != "a; b" {
		t.Errorf("list rendered as %q", got)
	}
	if got := flattenJSON(map[string]any{"b": "2", "a": "1"}); got != "a=1 b=2" {
		t.Errorf("map rendered as %q; keys must be sorted so output is stable", got)
	}
	// Long collections are capped rather than dumped.
	long := make([]any, 40)
	for i := range long {
		long[i] = "x"
	}
	if got := flattenJSON(long); !strings.Contains(got, "...") {
		t.Errorf("a 40-element list was not capped: %q", got)
	}
}

// UniProt double-gzips. This is the guard for that, and it is written against
// the shape of the bug rather than against UniProt: any server doing the same
// thing is handled.
func TestGunzipIfNeeded(t *testing.T) {
	plain := []byte(`{"results":[]}`)
	if got := gunzipIfNeeded(plain); string(got) != string(plain) {
		t.Error("plain JSON was altered")
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(plain)
	zw.Close()
	if got := gunzipIfNeeded(buf.Bytes()); string(got) != string(plain) {
		t.Errorf("gzip body not decompressed: %q", got)
	}

	// Something that merely starts like gzip must come back untouched rather
	// than being lost — the JSON parser gives the better error.
	broken := []byte{0x1f, 0x8b, 'n', 'o', 't', 'g', 'z'}
	if got := gunzipIfNeeded(broken); len(got) != len(broken) {
		t.Error("a malformed gzip body was not returned untouched")
	}
	if got := gunzipIfNeeded(nil); got != nil {
		t.Error("nil body was altered")
	}
}

func TestRenderBioRecordsAlwaysCitesItsSource(t *testing.T) {
	src := bioSources["ensembl"]
	out := renderBioRecords(src, "BRCA1", "https://example.test/x",
		map[string]any{"id": "ENSG00000012048", "biotype": "protein_coding"}, 3)

	if !strings.Contains(out, "https://example.test/x") {
		t.Error("the source URL is missing; a record with no provenance is unciteable")
	}
	if !strings.Contains(out, "ENSG00000012048") {
		t.Error("the projected field is missing")
	}
	if src.note != "" && !strings.Contains(out, src.note) {
		t.Error("the database's caveat was dropped")
	}
}

// The caveats are the point for these two. A model that reports a ClinVar hit
// COUNT as evidence of pathogenicity, or an AlphaFold prediction as a measured
// structure, is doing real harm, and the warning has to travel with the data.
func TestDangerousDatabasesCarryTheirWarning(t *testing.T) {
	if n := bioSources["clinvar"].note; !strings.Contains(n, "Never infer pathogenicity") {
		t.Error("clinvar must warn against reading pathogenicity from a hit count")
	}
	if n := bioSources["alphafold"].note; !strings.Contains(n, "PREDICTED") {
		t.Error("alphafold must say its structures are predictions")
	}
	if n := bioSources["openfda"].note; !strings.Contains(n, "not medical advice") {
		t.Error("openfda must not read as medical advice")
	}
}

func TestUnknownDatabaseIsRefusedWithTheList(t *testing.T) {
	resp, err := NewBioDataTool(nil).Run(context.Background(),
		ToolCall{Input: `{"database":"genbank","query":"x"}`})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(resp.Content, "uniprot") {
		t.Errorf("refusal should list what IS available, got: %s", resp.Content)
	}
}

func TestEveryDatabaseIsReachableAndDescribed(t *testing.T) {
	info := NewBioDataTool(nil).Info()
	enum := info.Parameters["database"].(map[string]any)["enum"].([]string)
	if len(enum) != len(bioSources) {
		t.Errorf("enum lists %d databases, the map has %d", len(enum), len(bioSources))
	}
	for _, name := range enum {
		src, ok := bioSources[name]
		if !ok {
			t.Errorf("%q is in the enum but not implemented", name)
			continue
		}
		if src.build == nil {
			t.Errorf("%q has no URL builder", name)
		}
		if !strings.Contains(info.Description, name) {
			t.Errorf("%q is selectable but undescribed; the model cannot know when to use it", name)
		}
	}
}

// ── live: run with GORILLA_LIVE_SEARCH=1 ─────────────────────────────────────

func TestLiveBioLookupEveryDatabase(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("needs network")
	}
	cases := []struct{ db, query, want string }{
		{"uniprot", "BRCA1", "primaryAccession"},
		{"ensembl", "BRCA1", "ENSG"},
		{"pdb", "2PGH", "rcsb_entry_info"},
		{"alphafold", "P38398", "entryId"},
		{"pubchem", "aspirin", "MolecularFormula"},
		{"chembl", "aspirin", "molecule_chembl_id"},
		{"interpro", "P38398", "accession"},
		{"clinvar", "BRCA1", "count"},
		{"openfda", "aspirin", "openfda"},
		{"string", "TP53", "preferredName"},
		{"reactome", "TP53", "name"},
	}
	tool := NewBioDataTool(nil)
	for _, tc := range cases {
		t.Run(tc.db, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			resp, err := tool.Run(ctx, ToolCall{
				Input: `{"database":"` + tc.db + `","query":"` + tc.query + `","limit":2}`,
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !strings.Contains(resp.Content, tc.want) {
				t.Errorf("no %q in the projection:\n%s", tc.want, resp.Content)
			}
			if strings.Contains(resp.Content, "none of the expected fields") {
				t.Errorf("%s changed shape; the projection fell back to raw keys", tc.db)
			}
		})
	}
}
