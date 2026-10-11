package agent

// GORILLA OVERRIDE (2026-10-11): the evidence check. A lane's findings were
// checked for five headings by substring and nothing else: no claim, grade,
// URL or quote was ever read by code, and "never cite a source you did not
// open" was a sentence in the prompt. Everything a lane fetched is in its own
// session, so the rule is checkable, and here it is checked, by code, before
// the orchestrator writes a word of the answer.
//
// The rule names and the record format are the ones `fieldkit evidence`
// uses (Gorilla.Fieldkit, fieldkit/evidence/record.py), so a saved run can be
// re-checked there later, against snapshots of the pages.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

var (
	contractHeadings = []string{"ANSWER", "FINDINGS", "SOURCES TRIED", "CONFIDENCE", "NOT ESTABLISHED"}
	evidenceTiers    = []string{"primary_source", "config", "multiple_reports", "single_claim", "unsourced"}
	confidenceWords  = []string{"proven", "strong", "plausible", "speculative"}
	yardstickTerms   = []string{"remote chance", "highly unlikely", "unlikely", "realistic possibility", "likely",
		"probable", "highly likely", "almost certain"}

	headingLineRe = regexp.MustCompile(`^#{2,3}\s+(.+?)\s*#*\s*$`)
	claimFieldRe  = regexp.MustCompile(`(?i)\b(CLAIM|EVIDENCE|TIER|GRADE|ORIGIN|QUOTE|SOURCE)\s*:\s*`)
	gradeRe       = regexp.MustCompile(`^[A-F][1-6]$`)
	urlRe         = regexp.MustCompile(`(?i)https?://[^\s<>"'` + "`" + `|\])}]+`)
	doiRe         = regexp.MustCompile(`(?i)\b(10\.\d{4,9}/[^\s"'<>|\])}]+)`)
	quotedRe      = regexp.MustCompile(`["“”«»]([^"“”«»]{20,})["“”«»]`)
	trackingRe    = regexp.MustCompile(`(?i)^(utm_[a-z]+|fbclid|gclid|mc_cid|mc_eid|_hsenc|_hsmi|ref_src)$`)
	likelihoodRe  = regexp.MustCompile(`(?i)LIKELIHOOD\s*:\s*([^\n.;]+)`)
	levelRe       = regexp.MustCompile(`(?i)CONFIDENCE\s*:\s*(HIGH|MODERATE|LOW)\b`)
	fetchedLineRe = regexp.MustCompile(`(?m)^Fetched: (\S+)`)
)

// laneClaim is one FINDINGS bullet (or the ANSWER's BASIS line).
type laneClaim struct {
	N        int
	Claim    string
	Evidence string
	Tier     string
	Grade    string
	Quote    string
	Origin   string
	URLs     []string
}

// laneParse is a lane's reply, read the way the output contract writes it.
type laneParse struct {
	Missing      []string // contract headings absent as HEADINGS (a word in a sentence is not one)
	Basis        *laneClaim
	Claims       []laneClaim
	Confidence   string
	SourcesTried []string
}

// claimFields reads "CLAIM: a | EVIDENCE: b | TIER: c" into a map; bold
// markers and extra fields are tolerated, and a value runs to the next field.
func claimFields(line string) map[string]string {
	line = strings.NewReplacer("**", "", "__", "").Replace(line)
	line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*• "))
	marks := claimFieldRe.FindAllStringSubmatchIndex(line, -1)
	out := map[string]string{}
	for i, m := range marks {
		end := len(line)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		v := strings.TrimSpace(strings.Trim(strings.TrimSpace(line[m[1]:end]), "|"))
		out[strings.ToLower(line[m[2]:m[3]])] = v
	}
	return out
}

func newClaim(n int, f map[string]string) laneClaim {
	c := laneClaim{N: n, Claim: f["claim"], Evidence: f["evidence"], Origin: f["origin"],
		Tier:  strings.ToLower(strings.Trim(f["tier"], "` ")),
		Grade: strings.ToUpper(strings.Trim(f["grade"], "` ")),
		Quote: strings.Trim(f["quote"], "\"“”«» "),
		URLs:  extractEvidenceURLs(f["evidence"] + " " + f["source"])}
	if strings.EqualFold(c.Quote, "none") || c.Quote == "-" {
		c.Quote = ""
	}
	if c.Quote == "" {
		if m := quotedRe.FindStringSubmatch(f["evidence"]); m != nil {
			c.Quote = strings.TrimSpace(m[1])
		}
	}
	return c
}

func parseLane(reply string) laneParse {
	sections := map[string][]string{}
	cur := ""
	for _, line := range strings.Split(reply, "\n") {
		if m := headingLineRe.FindStringSubmatch(line); m != nil {
			h := strings.ToUpper(strings.TrimSpace(m[1]))
			if slicesContains(contractHeadings, h) {
				cur = h
				sections[cur] = []string{}
				continue
			}
		}
		if cur != "" {
			sections[cur] = append(sections[cur], line)
		}
	}
	var p laneParse
	for _, h := range contractHeadings {
		if _, ok := sections[h]; !ok {
			p.Missing = append(p.Missing, h)
		}
	}
	for _, line := range sections["ANSWER"] {
		t := strings.TrimSpace(strings.ReplaceAll(line, "**", ""))
		if len(t) >= 6 && strings.EqualFold(t[:6], "BASIS:") {
			b := newClaim(0, claimFields("EVIDENCE:"+t[6:]))
			p.Basis = &b
		}
	}
	n := 0
	for _, line := range sections["FINDINGS"] {
		if !regexp.MustCompile(`(?i)\bCLAIM\s*:`).MatchString(line) {
			continue
		}
		n++
		p.Claims = append(p.Claims, newClaim(n, claimFields(line)))
	}
	p.Confidence = strings.TrimSpace(strings.Join(sections["CONFIDENCE"], "\n"))
	for _, line := range sections["SOURCES TRIED"] {
		if t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*• ")); t != "" {
			p.SourcesTried = append(p.SourcesTried, t)
		}
	}
	return p
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// normalizeEvidenceURL gives one spelling per page: case of scheme and host,
// default port, fragment, tracking parameters, trailing slash; a DOI in any
// of its URL forms becomes doi:10.x/y. Same rules as fieldkit/evidence/urls.py.
func normalizeEvidenceURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), ".,;:")
	low := strings.ToLower(raw)
	for _, p := range []string{"doi:", "https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/"} {
		if strings.HasPrefix(low, p) && strings.HasPrefix(strings.TrimSpace(low[len(p):]), "10.") {
			return "doi:" + strings.TrimSpace(low[len(p):])
		}
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https" && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) || u.Hostname() == "" {
		return raw
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if port := u.Port(); port != "" && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		host += ":" + port
	}
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		if !trackingRe.MatchString(k) {
			keys = append(keys, k)
		}
	}
	var qs []string
	// Keep the original order of the query, minus tracking parameters.
	for _, part := range strings.Split(u.RawQuery, "&") {
		k, _, _ := strings.Cut(part, "=")
		if dk, err := url.QueryUnescape(k); err == nil && part != "" && slicesContains(keys, dk) {
			qs = append(qs, part)
		}
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	out := scheme + "://" + host + path
	if len(qs) > 0 {
		out += "?" + strings.Join(qs, "&")
	}
	return out
}

// extractEvidenceURLs lists every URL and DOI in a piece of text, normalized,
// in order, once each.
func extractEvidenceURLs(text string) []string {
	var out []string
	add := func(s string) {
		if !slicesContains(out, s) {
			out = append(out, s)
		}
	}
	for _, u := range urlRe.FindAllString(text, -1) {
		add(normalizeEvidenceURL(u))
	}
	for _, m := range doiRe.FindAllStringSubmatch(text, -1) {
		d := normalizeEvidenceURL("doi:" + m[1])
		inURL := false
		for _, o := range out {
			if !strings.HasPrefix(o, "doi:") && strings.Contains(strings.ToLower(o), d[4:]) {
				inURL = true
			}
		}
		if !inURL {
			add(d)
		}
	}
	return out
}

// foldText removes what does not make two passages different: Unicode form,
// case, curly quotes and dashes, invisible characters, runs of white space.
func foldText(s string) string {
	s = norm.NFKC.String(s)
	s = strings.NewReplacer("‘", "'", "’", "'", "‚", "'", "‛", "'", "“", `"`, "”", `"`, "„", `"`, "«", `"`, "»", `"`,
		"–", "-", "—", "-", "−", "-", "\u00ad", "", "\u200b", "", "\u200c", "", "\u200d", "", "\ufeff", "").Replace(s)
	s = strings.ToLower(s)
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

var hyphenBreakRe = regexp.MustCompile(`(\w)- (\w)`)

// quoteIn reports whether a quote is in a text, as words.
func quoteIn(text, quote string) bool {
	q, t := foldText(quote), foldText(text)
	if q == "" {
		return false
	}
	return strings.Contains(t, q) || strings.Contains(hyphenBreakRe.ReplaceAllString(t, "$1$2"), hyphenBreakRe.ReplaceAllString(q, "$1$2"))
}

// laneFetches is what a helper actually opened: every successful web_fetch in
// its session, by normalized URL (the one asked for, and the one the tool
// says it fetched after redirects), with the text the helper was given.
func laneFetches(ctx context.Context, messages message.Service, sessionID string) map[string]string {
	out := map[string]string{}
	if messages == nil {
		return out
	}
	msgs, err := messages.List(ctx, sessionID)
	if err != nil {
		return out
	}
	asked := map[string]string{} // tool call id -> url asked for
	for _, m := range msgs {
		for _, c := range m.ToolCalls() {
			if c.Name != tools.FetchToolName {
				continue
			}
			var p tools.FetchParams
			if json.Unmarshal([]byte(c.Input), &p) == nil && p.URL != "" {
				asked[c.ID] = p.URL
			}
		}
	}
	for _, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, r := range m.ToolResults() {
			u, ok := asked[r.ToolCallID]
			if !ok || r.IsError {
				continue
			}
			keys := []string{normalizeEvidenceURL(u)}
			if f := fetchedLineRe.FindStringSubmatch(r.Content); f != nil {
				keys = append(keys, normalizeEvidenceURL(f[1]))
			}
			for _, k := range keys {
				out[k] += "\n" + r.Content
			}
		}
	}
	return out
}

// evidenceFinding is one broken rule, named the way fieldkit names it.
type evidenceFinding struct {
	Rule    string
	Claim   int // 0 = the lane as a whole
	Message string
}

// openedPage finds the fetched text for a cited URL, a DOI cited as doi: being
// opened through any URL that carries it.
func openedPage(fetched map[string]string, u string) (string, bool) {
	if t, ok := fetched[u]; ok {
		return t, true
	}
	if strings.HasPrefix(u, "doi:") {
		for k, t := range fetched {
			if strings.Contains(strings.ToLower(k), u[4:]) {
				return t, true
			}
		}
	}
	return "", false
}

// auditLane checks a parsed lane against the contract and against what the
// helper fetched. fetched nil means "not known" (no session store): the
// opened-page rules are then skipped rather than failing every claim.
func auditLane(p laneParse, fetched map[string]string, doctrine string) []evidenceFinding {
	var out []evidenceFinding
	add := func(rule string, n int, format string, a ...any) {
		out = append(out, evidenceFinding{rule, n, fmt.Sprintf(format, a...)})
	}
	if len(p.Missing) > 0 {
		add("contract-headings", 0, "missing heading(s): %s", strings.Join(p.Missing, ", "))
	}
	if p.Basis == nil && !slicesContains(p.Missing, "ANSWER") {
		add("contract-basis", 0, "the ANSWER has no BASIS line")
	}
	if !slicesContains(p.Missing, "CONFIDENCE") {
		if doctrine == DoctrineDossier {
			if levelRe.FindStringSubmatch(p.Confidence) == nil {
				add("contract-confidence", 0, "CONFIDENCE must be HIGH, MODERATE or LOW")
			}
			if m := likelihoodRe.FindStringSubmatch(p.Confidence); m != nil {
				term := strings.TrimSpace(strings.Split(strings.ToLower(m[1]), "/")[0])
				if !slicesContains(yardstickTerms, term) {
					add("contract-likelihood", 0, "LIKELIHOOD %q is not one of the seven yardstick terms", clip(term, 40))
				}
			}
		} else if w := strings.Fields(p.Confidence); len(w) != 1 || !slicesContains(confidenceWords, strings.ToLower(strings.Trim(w[0], ".*"))) {
			add("contract-confidence", 0, "CONFIDENCE must be exactly one word: proven | strong | plausible | speculative")
		}
	}
	if len(p.SourcesTried) == 0 && !slicesContains(p.Missing, "SOURCES TRIED") {
		add("contract-sources", 0, "SOURCES TRIED is empty")
	}
	for _, c := range p.Claims {
		if c.Evidence == "" {
			add("claim-evidence", c.N, "no EVIDENCE")
		}
		if doctrine == DoctrineDossier {
			switch {
			case c.Tier != "" && c.Grade == "":
				add("claim-scale", c.N, "a TIER on a dossier run: dossier claims carry a GRADE")
			case c.Grade == "":
				add("claim-grade", c.N, "no GRADE")
			case !gradeRe.MatchString(c.Grade):
				add("claim-grade", c.N, "GRADE %q is not a letter A-F and a digit 1-6", clip(c.Grade, 20))
			}
		} else {
			switch {
			case c.Grade != "" && c.Tier == "":
				add("claim-scale", c.N, "a GRADE on a standard run: standard claims carry an evidence TIER")
			case c.Tier == "":
				add("claim-tier", c.N, "no TIER")
			case !slicesContains(evidenceTiers, c.Tier):
				add("claim-tier", c.N, "TIER %q is not one of the five", clip(c.Tier, 30))
			}
		}
		if fetched == nil {
			continue
		}
		for _, u := range c.URLs {
			text, opened := openedPage(fetched, u)
			if !opened {
				add("cited-not-opened", c.N, "cites %s, which this helper never fetched", clip(u, 120))
				continue
			}
			if c.Quote != "" && !quoteIn(text, c.Quote) {
				add("quote-not-found", c.N, "the quote is not in what this helper fetched from %s", clip(u, 120))
			}
		}
	}
	return out
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return cutRunes(s, n) + "..."
	}
	return s
}

func cutRunes(s string, n int) string {
	for n > 0 && n < len(s) && !utf8RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// writeEvidenceSection is what the orchestrator reads under a lane: the
// findings by claim, or one line saying every claim held.
func writeEvidenceSection(b *strings.Builder, p laneParse, findings []evidenceFinding, fetchedKnown bool) {
	if len(findings) == 0 {
		if fetchedKnown && len(p.Claims) > 0 {
			fmt.Fprintf(b, "**Evidence check (by code): all %d claim(s) hold** — every cited page was opened by this helper", len(p.Claims))
			b.WriteString(", and every quote is in the page it fetched.\n\n")
		}
		return
	}
	fmt.Fprintf(b, "**Evidence check (by code, not by a model): %d problem(s).** A claim named here is UNSUPPORTED until you check it yourself: do not repeat it as found.\n", len(findings))
	for _, f := range findings {
		where := "lane"
		if f.Claim > 0 {
			where = fmt.Sprintf("claim %d", f.Claim)
		}
		fmt.Fprintf(b, "- %s: %s [%s]\n", where, f.Message, f.Rule)
	}
	b.WriteString("\n")
}

// writeEvidenceRecord saves the run beside its findings file, in the record
// format `fieldkit evidence check` reads: run, lanes, claims, and what each
// lane fetched. Best effort, owner-only, like the findings file.
func writeEvidenceRecord(findingsPath, question, doctrine, callID string, roles []researchRole, parses []laneParse, fetched []map[string]string) string {
	if findingsPath == "" || !strings.HasSuffix(findingsPath, ".md") {
		return ""
	}
	path := strings.TrimSuffix(findingsPath, ".md") + ".jsonl"
	var b strings.Builder
	enc := func(v any) {
		j, _ := json.Marshal(v)
		b.Write(j)
		b.WriteString("\n")
	}
	enc(map[string]any{"type": "run", "question": question, "doctrine": doctrineName(doctrine), "call": callID})
	for i, role := range roles {
		if i >= len(parses) {
			break
		}
		p := parses[i]
		lane := map[string]any{"type": "lane", "lane": role.Title, "headings_missing": nonNil(p.Missing),
			"sources_tried": nonNil(p.SourcesTried), "audit": nil, "uncovered": false, "basis": nil, "confidence": nil}
		if p.Basis != nil {
			lane["basis"] = map[string]any{"evidence": p.Basis.Evidence, "tier": nilIfEmpty(p.Basis.Tier),
				"grade": nilIfEmpty(p.Basis.Grade), "urls": nonNil(p.Basis.URLs)}
		}
		if !slicesContains(p.Missing, "CONFIDENCE") {
			if doctrine == DoctrineDossier {
				conf := map[string]any{"likelihood": nil, "level": nil}
				if m := likelihoodRe.FindStringSubmatch(p.Confidence); m != nil {
					conf["likelihood"] = strings.ToLower(strings.TrimSpace(m[1]))
				}
				if m := levelRe.FindStringSubmatch(p.Confidence); m != nil {
					conf["level"] = strings.ToUpper(m[1])
				}
				lane["confidence"] = conf
			} else {
				w := strings.Fields(p.Confidence)
				word := ""
				if len(w) > 0 {
					word = strings.ToLower(strings.Trim(w[0], ".*"))
				}
				lane["confidence"] = map[string]any{"word": word, "alone": len(w) == 1}
			}
		}
		if len(p.Missing) == len(contractHeadings) && len(p.Claims) == 0 {
			lane["uncovered"] = true
		}
		enc(lane)
		for _, c := range p.Claims {
			rec := map[string]any{"type": "claim", "lane": role.Title, "n": c.N, "claim": c.Claim,
				"evidence": c.Evidence, "urls": nonNil(c.URLs)}
			for k, v := range map[string]string{"tier": c.Tier, "grade": c.Grade, "quote": c.Quote, "origin": c.Origin} {
				if v != "" {
					rec[k] = v
				}
			}
			enc(rec)
		}
		if i < len(fetched) && fetched[i] != nil {
			urls := make([]string, 0, len(fetched[i]))
			for u := range fetched[i] {
				urls = append(urls, u)
			}
			sort.Strings(urls)
			for _, u := range urls {
				enc(map[string]any{"type": "fetched", "lane": role.Title, "url": u})
			}
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, findingsFileMode)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		return ""
	}
	return path
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
