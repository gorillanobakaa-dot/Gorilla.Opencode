// GORILLA (2026-09-02): structured biology and chemistry records.
//
// /research answers "what has been WRITTEN about this". This answers "what IS
// this" — a protein's length and organism, a variant's clinical significance, a
// compound's formula, a structure's resolution. Those are records, not papers,
// and no amount of searching literature produces them.
//
// ONE tool with a database parameter, not one tool per database. The schema of
// every declared tool is paid for on every turn, measured by
// TestDefaultToolSchemaCostIsCounted; eleven separate tools here would have
// cost roughly eight thousand tokens a turn to give most users something they
// never ask for. This costs one schema and is OFF by default.
//
// Every endpoint below was probed keyless and returned 200 before being
// listed. Nothing here needs an API key, and Gorilla sends none: the keys
// these services offer buy higher rate limits, and the politeness limiter in
// internal/politehttp is the cheaper way to stay inside the free budget.
//
// OpenTargets is deliberately absent. Its API is GraphQL-over-POST, and a
// half-supported entry that returns 400 would be worse than an honest gap.
package tools

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/permission"
)

const (
	BioDataToolName = "bio_lookup"

	// Records are small; the timeout covers a slow upstream, not a big body.
	bioTimeout = 45 * time.Second

	// Reactome answered one probe with 256 KB. Whole records are not the point
	// and would evict the conversation that asked for them, so the projection
	// below is capped and the source URL is always returned for the full text.
	bioMaxChars = 6000
)

type BioDataParams struct {
	// Database to query.
	Database string `json:"database"`
	// Query: a gene symbol, accession, compound name, PDB ID or free text,
	// depending on the database.
	Query string `json:"query"`
	// Limit bounds how many records come back. 0 means a sensible default.
	Limit int `json:"limit"`
}

// bioSource is one database: how to build its URL, and which fields are worth
// pulling out of whatever it returns.
type bioSource struct {
	label string
	// build returns the request URL for a query.
	build func(q string, limit int) string
	// recordPath names the JSON key holding the list of records, or "" when
	// the response IS the record.
	recordPath string
	// fields are preferred keys, in display order. Anything not found is
	// skipped rather than reported as empty.
	fields []string
	// note is appended to every answer from this source: a caveat the model
	// would otherwise have to know already.
	note string
}

var bioSources = map[string]bioSource{
	"uniprot": {
		label: "UniProt",
		build: func(q string, n int) string {
			return fmt.Sprintf("https://rest.uniprot.org/uniprotkb/search?query=%s&format=json&size=%d",
				url.QueryEscape(q), n)
		},
		recordPath: "results",
		fields: []string{"primaryAccession", "uniProtkbId", "proteinDescription",
			"organism", "genes", "sequence", "comments"},
	},
	"ensembl": {
		label: "Ensembl",
		build: func(q string, _ int) string {
			return "https://rest.ensembl.org/lookup/symbol/homo_sapiens/" +
				url.PathEscape(q) + "?content-type=application/json;expand=0"
		},
		fields: []string{"id", "display_name", "description", "biotype",
			"seq_region_name", "start", "end", "strand", "assembly_name"},
		note: "Human (GRCh38) by symbol. Coordinates are 1-based inclusive.",
	},
	"pdb": {
		label: "RCSB PDB",
		build: func(q string, _ int) string {
			return "https://data.rcsb.org/rest/v1/core/entry/" + url.PathEscape(strings.ToUpper(q))
		},
		fields: []string{"struct", "rcsb_entry_info", "exptl", "rcsb_accession_info",
			"refine", "citation"},
		note: "Takes a 4-character PDB ID (e.g. 2PGH), not a keyword.",
	},
	"alphafold": {
		label: "AlphaFold DB",
		build: func(q string, _ int) string {
			return "https://alphafold.ebi.ac.uk/api/prediction/" + url.PathEscape(strings.ToUpper(q))
		},
		fields: []string{"entryId", "uniprotAccession", "uniprotDescription",
			"organismScientificName", "uniprotStart", "uniprotEnd", "pdbUrl", "cifUrl"},
		note: "PREDICTED structure, not experimental. Confidence is per-residue " +
			"pLDDT: above 90 is very high, below 50 usually means disordered — " +
			"a low-confidence region is not evidence of a shape.",
	},
	"pubchem": {
		label: "PubChem",
		build: func(q string, _ int) string {
			return "https://pubchem.ncbi.nlm.nih.gov/rest/pug/compound/name/" +
				url.PathEscape(q) +
				"/property/MolecularFormula,MolecularWeight,CanonicalSMILES,InChIKey,IUPACName/JSON"
		},
		recordPath: "PropertyTable.Properties",
		fields:     []string{"CID", "IUPACName", "MolecularFormula", "MolecularWeight", "CanonicalSMILES", "InChIKey"},
	},
	"chembl": {
		label: "ChEMBL",
		build: func(q string, n int) string {
			return fmt.Sprintf("https://www.ebi.ac.uk/chembl/api/data/molecule/search?q=%s&format=json&limit=%d",
				url.QueryEscape(q), n)
		},
		recordPath: "molecules",
		fields: []string{"molecule_chembl_id", "pref_name", "max_phase",
			"molecule_type", "first_approval", "oral", "molecule_properties"},
		note: "max_phase is how far it got in trials: 4 means an approved drug, " +
			"null means preclinical or unknown — not zero.",
	},
	"interpro": {
		label: "InterPro",
		build: func(q string, n int) string {
			return fmt.Sprintf("https://www.ebi.ac.uk/interpro/api/entry/interpro/protein/uniprot/%s/?page_size=%d",
				url.PathEscape(strings.ToUpper(q)), n)
		},
		recordPath: "results",
		fields:     []string{"metadata", "accession", "name", "source_database", "type"},
		note:       "Takes a UniProt accession (e.g. P38398), and returns the domains and families found in it.",
	},
	"clinvar": {
		label: "ClinVar",
		build: func(q string, n int) string {
			return fmt.Sprintf("https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi?db=clinvar&term=%s&retmode=json&retmax=%d",
				url.QueryEscape(q), n)
		},
		recordPath: "esearchresult",
		fields:     []string{"count", "idlist", "querytranslation"},
		note: "This is the SEARCH step: it returns ClinVar record IDs and a total " +
			"count, not the clinical interpretations themselves. Fetch a specific " +
			"ID at ncbi.nlm.nih.gov/clinvar/variation/<id>/ before stating what a " +
			"variant means. Never infer pathogenicity from a hit count.",
	},
	"openfda": {
		label: "openFDA",
		build: func(q string, n int) string {
			return fmt.Sprintf("https://api.fda.gov/drug/label.json?search=openfda.generic_name:%s&limit=%d",
				url.QueryEscape(q), n)
		},
		recordPath: "results",
		fields: []string{"openfda", "indications_and_usage", "warnings",
			"boxed_warning", "dosage_and_administration", "effective_time"},
		note: "US drug LABELS as submitted to the FDA. This is regulatory text, " +
			"not medical advice, and label content differs between countries.",
	},
	"string": {
		label: "STRING",
		build: func(q string, _ int) string {
			return "https://string-db.org/api/json/get_string_ids?identifiers=" +
				url.QueryEscape(q) + "&species=9606"
		},
		fields: []string{"stringId", "preferredName", "annotation", "taxonName", "ncbiTaxonId"},
		note: "Human (taxon 9606). STRING scores combine experimental and PREDICTED " +
			"evidence, so a high score is not by itself proof of a physical interaction.",
	},
	"reactome": {
		label: "Reactome",
		build: func(q string, n int) string {
			return fmt.Sprintf("https://reactome.org/ContentService/search/query?query=%s&species=Homo%%20sapiens&cluster=true&rows=%d",
				url.QueryEscape(q), n)
		},
		recordPath: "results",
		fields:     []string{"name", "entries", "typeName"},
		note:       "Human pathways.",
	},
}

type bioDataTool struct {
	permissions permission.Service
}

func NewBioDataTool(permissions permission.Service) BaseTool {
	return &bioDataTool{permissions: permissions}
}

func bioSourceNames() []string {
	names := make([]string, 0, len(bioSources))
	for k := range bioSources {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func (b *bioDataTool) Info() ToolInfo {
	return ToolInfo{
		Name: BioDataToolName,
		Description: `Look up structured biology and chemistry RECORDS — not papers about them.

Use this when the question is what something IS, rather than what has been written about it: a protein's length and organism, a gene's coordinates, a compound's formula, a structure's resolution, what a drug's label warns about. For literature use web_search instead.

  uniprot    proteins: accession, name, organism, sequence, function
  ensembl    human genes by symbol: ID, coordinates, biotype (GRCh38)
  pdb        experimental structures by 4-character PDB ID
  alphafold  PREDICTED structures by UniProt accession
  pubchem    compounds by name: formula, weight, SMILES, InChIKey
  chembl     drugs and bioactivity by name
  interpro   domains and families within a UniProt accession
  clinvar    searches clinical variants; returns IDs and a count, NOT interpretations
  openfda    US drug labels: indications, warnings, boxed warnings
  string     protein interaction partners (human)
  reactome   human pathways

Records are returned as a COMPACT PROJECTION with the source URL. When a detail matters, fetch that URL with web_fetch rather than assuming the projection carried it.

Two standing cautions. Anything from alphafold is a prediction, and pLDDT below 50 usually means disorder rather than a shape. Anything from clinvar or openfda is clinical or regulatory data: report what the record says and attribute it, never restate it as advice about a person.`,
		Parameters: map[string]any{
			"database": map[string]any{
				"type":        "string",
				"description": "Which database to query.",
				"enum":        bioSourceNames(),
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Gene symbol, accession, PDB ID, compound or drug name — whichever that database keys on.",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Maximum records (default 3, max 20). Ignored by databases that return one record.",
			},
		},
		Required: []string{"database", "query"},
	}
}

func (b *bioDataTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params BioDataParams
	if call.Input != "" {
		if err := UnmarshalToolInput(call.Input, &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("could not read the parameters: %s", err)), nil
		}
	}
	name := strings.ToLower(strings.TrimSpace(params.Database))
	src, ok := bioSources[name]
	if !ok {
		return NewTextErrorResponse(fmt.Sprintf(
			"unknown database %q. Available: %s",
			params.Database, strings.Join(bioSourceNames(), ", "))), nil
	}
	q := strings.TrimSpace(params.Query)
	if q == "" {
		return NewTextErrorResponse("query is empty; give a gene symbol, accession, PDB ID or compound name"), nil
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 3
	}
	if limit > 20 {
		limit = 20
	}

	target := src.build(q, limit)

	// newSafeClient, so this inherits the SSRF guards and — the reason it
	// matters here — the per-host politeness limiter. These are public
	// scientific APIs with real budgets, and a research run that fires eleven
	// helpers at NCBI is exactly what earns a block.
	client := newSafeClient(bioTimeout)
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("could not build the request: %s", err)), nil
	}
	// The same contact string web_search sends. Several of these services ask
	// callers to identify themselves in their fair-access policy, and one that
	// does not ask still deserves it.
	req.Header.Set("User-Agent", politeContact)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("%s did not answer: %s", src.label, err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("could not read %s's answer: %s", src.label, err)), nil
	}
	body = gunzipIfNeeded(body)
	if resp.StatusCode == 404 {
		return NewTextResponse(fmt.Sprintf(
			"%s has no record for %q.\n\nThat is an answer, not a failure — say so plainly "+
				"rather than trying other spellings indefinitely.\n\n%s", src.label, q, target)), nil
	}
	if resp.StatusCode != 200 {
		return NewTextResponse(fmt.Sprintf(
			"%s returned HTTP %d for %q. Nothing was retrieved, so report that rather "+
				"than describing the record from memory.\n\n%s",
			src.label, resp.StatusCode, q, target)), nil
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return NewTextErrorResponse(fmt.Sprintf(
			"%s returned something that is not JSON: %s", src.label, err)), nil
	}
	return NewTextResponse(renderBioRecords(src, q, target, parsed, limit)), nil
}

// gunzipIfNeeded unwraps a body that is STILL gzip after net/http has already
// done its own decompression.
//
// UniProt double-compresses: Go's transport adds Accept-Encoding, sees
// Content-Encoding: gzip, decompresses one layer, sets Uncompressed=true and
// removes the header -- and what it hands over begins 1f 8b, because the
// payload underneath was gzip too. Measured 2026-09-02; a plain http.Client
// with no custom transport behaves identically, so this is the server's
// doing, not the politeness wrapper's.
//
// Detected by magic bytes rather than by the header, because the header is
// gone by this point precisely when the problem occurs. Anything that is not
// gzip, or that fails to decompress, is returned untouched: the JSON parser
// upstream gives a better error than a decompression failure would.
func gunzipIfNeeded(body []byte) []byte {
	if len(body) < 2 || body[0] != 0x1f || body[1] != 0x8b {
		return body
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return body
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, 8*1024*1024))
	if err != nil || len(out) == 0 {
		return body
	}
	return out
}

// renderBioRecords projects whatever came back onto the fields that matter.
//
// Deliberately generic rather than eleven hand-written parsers. A bespoke
// parser per database is eleven things to silently break when an upstream
// renames a field, and the breakage looks like "no data" — which reads
// exactly like "no such record". Projecting named fields and saying what was
// found keeps a shape change visible instead.
func renderBioRecords(src bioSource, query, source string, parsed any, limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %q\n%s\n\n", src.label, query, source)
	if src.note != "" {
		fmt.Fprintf(&b, "NOTE: %s\n\n", src.note)
	}

	records := extractRecords(parsed, src.recordPath)
	if len(records) == 0 {
		b.WriteString("No records matched. That is an answer: report it rather than guessing.\n")
		return b.String()
	}
	if len(records) > limit {
		records = records[:limit]
	}

	for i, rec := range records {
		if len(records) > 1 {
			fmt.Fprintf(&b, "--- record %d of %d ---\n", i+1, len(records))
		}
		m, ok := rec.(map[string]any)
		if !ok {
			fmt.Fprintf(&b, "  %s\n", oneLineOf(fmt.Sprint(rec), 300))
			continue
		}
		wrote := false
		for _, f := range src.fields {
			v, present := m[f]
			if !present || v == nil {
				continue
			}
			fmt.Fprintf(&b, "  %s: %s\n", f, oneLineOf(flattenJSON(v), 400))
			wrote = true
		}
		// Nothing matched: the shape changed, or this database returns keys
		// nobody predicted. Show something real rather than an empty block.
		if !wrote {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 12 {
				keys = keys[:12]
			}
			fmt.Fprintf(&b, "  (none of the expected fields were present; keys returned: %s)\n",
				strings.Join(keys, ", "))
			for _, k := range keys {
				fmt.Fprintf(&b, "  %s: %s\n", k, oneLineOf(flattenJSON(m[k]), 200))
			}
		}
		if b.Len() > bioMaxChars {
			fmt.Fprintf(&b, "\n... truncated at %d characters. Fetch the source URL above for the rest.\n", bioMaxChars)
			break
		}
	}
	return b.String()
}

// extractRecords finds the list of records, following a dotted path when the
// source names one. A single object becomes a one-element list so callers do
// not need two code paths.
func extractRecords(parsed any, path string) []any {
	cur := parsed
	if path != "" {
		for _, part := range strings.Split(path, ".") {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur, ok = m[part]
			if !ok {
				return nil
			}
		}
	}
	switch v := cur.(type) {
	case []any:
		return v
	case map[string]any:
		return []any{v}
	case nil:
		return nil
	default:
		return []any{v}
	}
}

// flattenJSON renders a nested value on one line, keeping the parts a reader
// would want and dropping the punctuation they would not.
func flattenJSON(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			if s := flattenJSON(e); s != "" {
				parts = append(parts, s)
			}
			if len(parts) >= 8 {
				parts = append(parts, "...")
				break
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			s := flattenJSON(x[k])
			if s == "" {
				continue
			}
			parts = append(parts, k+"="+s)
			if len(parts) >= 8 {
				parts = append(parts, "...")
				break
			}
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprint(x)
	}
}
