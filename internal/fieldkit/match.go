// Package fieldkit matches a person's request against the Fieldkit tool
// catalogue, in-process, so the fitting tool names can be put in front of the
// model before it answers.
//
// GORILLA (2026-10-19). Measured that day with Gemma 4 E2B through the
// Fieldkit MCP server: asked about the TV, an Office file and the cooling
// profile, it read the server's ten entry points and answered "no such tool"
// without ever calling discover; a "START HERE, search first" description did
// not change that. A model that never takes the search path cannot be talked
// into it (see the note on DeferredCatalogueBlock in llm/tools/toolsearch.go).
// So the search is done for it: here, in milliseconds, with no server start.
//
// catalogue.json is generated, never typed: `fieldkit cards export` writes it
// from Fieldkit's reviewed public cards (drafts and the owner's local cards are
// left out) and stamps the Fieldkit commit it came from.
package fieldkit

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed catalogue.json
var catalogueJSON []byte

// Tool is one catalogue entry: what a model needs to pick and call a tool.
type Tool struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Trust  string `json:"trust"`
	Safety string `json:"safety"`
	// Online: the tool goes off the machine (its card has the network effect).
	Online bool `json:"online"`
	Inputs []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	} `json:"inputs"`
	// Recipe marks a recipe offered beside the tools (never from the file:
	// Match sets it). A recipe is read with fieldkit_describe, never run.
	Recipe bool `json:"-"`
}

// Recipe is how something was done before, kept so a model does not have to
// work it out again (Fieldkit desk/recipes.yaml, published ones only). The
// steps stay behind fieldkit_describe; the catalogue carries what a match
// needs. Schema 2, 2026-10-20.
type Recipe struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Problem   string   `json:"problem"`
	Tags      []string `json:"tags"`
	Platforms []string `json:"platforms"`
}

// Catalogue is the embedded file.
type Catalogue struct {
	Schema         int      `json:"schema"`
	FieldkitCommit string   `json:"fieldkit_commit"`
	Tools          []Tool   `json:"tools"`
	Recipes        []Recipe `json:"recipes"`
}

var (
	loadOnce sync.Once
	loaded   Catalogue
	loadErr  error
)

// Load parses the embedded catalogue once.
func Load() (Catalogue, error) {
	loadOnce.Do(func() { loadErr = json.Unmarshal(catalogueJSON, &loaded) })
	return loaded, loadErr
}

var word = regexp.MustCompile(`[a-z0-9]+`)

// stopWords say nothing about a tool. The same list as Fieldkit's discover:
// two-letter words such as "tv", "pc", "os" and "ip" are kept.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true, "in": true, "on": true, "at": true, "is": true,
	"it": true, "my": true, "me": true, "do": true, "be": true, "by": true, "or": true, "and": true, "for": true,
	"can": true, "how": true, "what": true, "with": true, "from": true, "this": true, "that": true, "you": true,
	"your": true, "i": true, "we": true, "up": true, "as": true, "if": true, "so": true, "no": true, "not": true,
	"all": true, "any": true, "use": true, "tool": true, "tools": true, "fieldkit": true, "check": true,
	"right": true, "now": true, "please": true,
	// Question words carry no subject (2026-10-19: "which" matched "Every
	// profile and which version..." and outranked the cooling tool).
	"which": true, "where": true, "when": true, "who": true, "why": true, "does": true, "are": true, "was": true,
	"current": true, "currently": true, "there": true, "about": true, "tell": true, "show": true,
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range word.FindAllString(strings.ToLower(s), -1) {
		out[w] = true
	}
	return out
}

var trustRank = map[string]int{"verified": 3, "tested": 2, "carded": 1}

// Match returns up to limit tools for a request, best first: the same scoring
// as Fieldkit's discover (a word in the title 3, in the id 2, in an input 1),
// trust breaking ties. A request that matches nothing returns nothing.
func Match(c Catalogue, request string, limit int) []Tool {
	var q []string
	for _, w := range word.FindAllString(strings.ToLower(request), -1) {
		if len(w) > 1 && !stopWords[w] {
			q = append(q, w)
		}
	}
	if len(q) == 0 {
		return nil
	}
	type scored struct {
		t     Tool
		score float64
	}
	// Tools and recipes are searched together (the same as Fieldkit's
	// discover): what was done before is as findable as what can be run. A
	// recipe's problem and tags count as a tool's inputs do.
	type entry struct {
		t     Tool
		extra map[string]bool
	}
	pool := make([]entry, 0, len(c.Tools)+len(c.Recipes))
	for _, t := range c.Tools {
		in := map[string]bool{}
		for _, i := range t.Inputs {
			in[i.Name] = true
		}
		pool = append(pool, entry{t, in})
	}
	for _, r := range c.Recipes {
		pool = append(pool, entry{Tool{ID: r.ID, Title: r.Title, Trust: "recipe", Recipe: true},
			words(r.Problem + " " + strings.Join(r.Tags, " "))})
	}
	// A rare word says more than a common one (measured 2026-10-19: "cooling
	// profile" put the Firefox profile tools first, because "profile" is in a
	// dozen titles and "cooling" in one). Each word is weighted by how few
	// entries mention it: log(N / entries mentioning it).
	df := map[string]int{}
	for _, e := range pool {
		d := words(e.t.Title + " " + e.t.ID)
		for w := range d {
			df[w]++
		}
	}
	n := float64(len(pool))
	var hits []scored
	for _, e := range pool {
		t := e.t
		title, id := words(t.Title), words(t.ID)
		in := e.extra
		s := 0.0
		for _, w := range q {
			weight := 1.0
			if df[w] > 0 {
				weight = math.Log(n/float64(df[w])) + 0.1
			}
			if title[w] {
				s += 3 * weight
			}
			if id[w] {
				s += 2 * weight
			}
			if in[w] {
				s += weight
			}
		}
		if s > 0 {
			// A tool that goes online ranks after an offline one of the same fit
			// (2026-10-19: Gemma picked hdmi-diag, which Fieldkit refused without
			// network=true, and gave up; hdmi-status answers offline).
			if t.Online {
				s *= 0.9
			}
			// A tool that answers ranks before a recipe that explains, at the
			// same fit (the factor Fieldkit's discover uses).
			if t.Recipe {
				s *= 0.85
			}
			hits = append(hits, scored{t, s + 0.01*float64(trustRank[t.Trust])})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].t.ID < hits[j].t.ID
	})
	out := make([]Tool, 0, limit)
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.t)
	}
	return out
}

// Hint is the short block put beside the person's message: the matching tools
// by exact name, and how to call them. Empty when nothing matches.
func Hint(c Catalogue, request string, limit int) string {
	hits := Match(c, request, limit)
	if len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[Gorilla: Fieldkit tools that match this request. Prefer one of these to guessing or doing it by " +
		"hand: call fieldkit_describe with the exact name, then fieldkit_run. A read-only tool changes nothing. " +
		"A RECIPE is how this was done before on this computer: read it with fieldkit_describe and follow it; " +
		"never run it.]\n")
	for _, t := range hits {
		if t.Recipe {
			fmt.Fprintf(&b, "- %s [RECIPE: read it with fieldkit_describe, never run it] %s\n", t.ID, t.Title)
			continue
		}
		var ins []string
		for _, i := range t.Inputs {
			n := i.Name
			if !i.Required {
				n += "?"
			}
			ins = append(ins, n)
		}
		inputs := "none"
		if len(ins) > 0 {
			inputs = strings.Join(ins, ", ")
		}
		online := ""
		if t.Online {
			online = ", goes online: run it with network=true"
		}
		fmt.Fprintf(&b, "- %s [%s, %s%s] %s (inputs: %s)\n", t.ID, t.Trust, t.Safety, online, t.Title, inputs)
	}
	return b.String()
}
