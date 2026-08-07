package tools

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// TextRank: extractive summarisation, in-process, no dependencies.
//
// GORILLA OVERRIDE: this exists so the token sieve can reach full-text
// documents without shipping a Python runtime. The tempting pipeline was
// Python + sumy + a numeric stack; on 2026-08-07 none of xmllint, pup,
// html2text or sumy were installed on this project's own development machine,
// and on a 2GB laptop that stack is a larger install than this entire
// application. About 150 lines of Go costs nothing and ships in the binary that
// is already there.
//
// TWO RULES, both of which this file enforces rather than documents:
//
//  1. NEVER SUMMARISE WHAT IS ALREADY SMALL. An abstract is ~330 tokens.
//     Compressing it saves perhaps 100 tokens and can delete the finding. The
//     source ladder already captured ~93% of the available saving before this
//     runs. Summarise(...) refuses below minSummaryChars and returns the input
//     untouched, saying so.
//
//  2. ALWAYS DECLARE WHAT WAS DROPPED. Centrality is not importance: a paper's
//     caveats, limitations and negative results are frequently the LEAST
//     central sentences in the graph and the first to be cut. A model handed a
//     silent extract reasons about the fragment as though it were the whole
//     document - the truncation bug one layer up. Every summary carries its
//     compression ratio and the word "extractive".
const (
	// Below this, summarising costs more in lost meaning than it saves in
	// tokens. ~8000 chars is roughly 2000 tokens.
	minSummaryChars = 8000
	prDamping       = 0.85
	prIterations    = 30
)

var sentenceSplit = regexp.MustCompile(`(?m)([.!?])\s+`)

// English closed-class words carry no topic signal; leaving them in makes every
// sentence look similar to every other and flattens the ranking.
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "been": true, "but": true, "by": true, "can": true, "for": true,
	"from": true, "has": true, "have": true, "in": true, "is": true, "it": true,
	"its": true, "of": true, "on": true, "or": true, "that": true, "the": true,
	"their": true, "there": true, "these": true, "this": true, "to": true,
	"was": true, "were": true, "which": true, "with": true, "we": true,
	"our": true, "they": true, "than": true, "then": true, "such": true,
	"also": true, "however": true, "using": true, "used": true, "into": true,
}

func splitSentences(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ")
	marked := sentenceSplit.ReplaceAllString(text, "$1\x00")
	var out []string
	for _, s := range strings.Split(marked, "\x00") {
		s = strings.TrimSpace(s)
		// Very short fragments are headings, list bullets and page furniture.
		if len([]rune(s)) >= 40 {
			out = append(out, s)
		}
	}
	return out
}

func contentWords(s string) map[string]int {
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	m := make(map[string]int, len(words))
	for _, w := range words {
		if len(w) > 2 && !stopWords[w] {
			m[w]++
		}
	}
	return m
}

// similarity is the standard TextRank overlap: shared words normalised by
// sentence length, so a long sentence is not central merely for being long.
func similarity(a, b map[string]int) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for w := range a {
		if b[w] > 0 {
			shared++
		}
	}
	if shared == 0 {
		return 0
	}
	norm := math.Log(float64(len(a))) + math.Log(float64(len(b)))
	if norm == 0 {
		return 0
	}
	return float64(shared) / norm
}

// SummaryResult keeps the provenance with the text, so a caller cannot pass the
// summary on without also being able to say what it is.
type SummaryResult struct {
	Text             string
	Summarised       bool
	OriginalChars    int
	SummaryChars     int
	SentencesKept    int
	SentencesTotal   int
	CompressionRatio float64
}

// Header is the line that must accompany the text wherever it goes.
func (r SummaryResult) Header() string {
	if !r.Summarised {
		return ""
	}
	return fmt.Sprintf(
		"[EXTRACTIVE SUMMARY — %d of %d sentences kept, %.0f%% of the original text. "+
			"Sentences were selected by graph centrality, which is NOT the same as "+
			"importance: caveats, limitations and negative results are often the least "+
			"central and may have been dropped. Fetch the source for anything load-bearing.]",
		r.SentencesKept, r.SentencesTotal, r.CompressionRatio*100)
}

// Summarise reduces long text to its most central sentences, preserving
// original order. It refuses on short input and always reports what it did.
func Summarise(text string, targetSentences int) SummaryResult {
	orig := len(text)
	res := SummaryResult{Text: text, OriginalChars: orig, SummaryChars: orig, CompressionRatio: 1}

	if orig < minSummaryChars {
		return res // rule 1: too small to be worth the risk
	}
	sentences := splitSentences(text)
	res.SentencesTotal = len(sentences)
	if targetSentences <= 0 {
		targetSentences = 12
	}
	if len(sentences) <= targetSentences {
		return res
	}

	bags := make([]map[string]int, len(sentences))
	for i, s := range sentences {
		bags[i] = contentWords(s)
	}

	n := len(sentences)
	adj := make([][]float64, n)
	degree := make([]float64, n)
	for i := range adj {
		adj[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			w := similarity(bags[i], bags[j])
			adj[i][j], adj[j][i] = w, w
			degree[i] += w
			degree[j] += w
		}
	}

	// PageRank over the sentence graph.
	score := make([]float64, n)
	next := make([]float64, n)
	for i := range score {
		score[i] = 1.0 / float64(n)
	}
	for iter := 0; iter < prIterations; iter++ {
		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				if i != j && adj[j][i] > 0 && degree[j] > 0 {
					sum += adj[j][i] / degree[j] * score[j]
				}
			}
			next[i] = (1-prDamping)/float64(n) + prDamping*sum
		}
		copy(score, next)
	}

	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return score[idx[a]] > score[idx[b]] })
	keep := idx[:targetSentences]
	sort.Ints(keep) // restore reading order

	parts := make([]string, 0, len(keep))
	for _, i := range keep {
		parts = append(parts, sentences[i])
	}
	summary := strings.Join(parts, " ")

	res.Text = summary
	res.Summarised = true
	res.SummaryChars = len(summary)
	res.SentencesKept = len(keep)
	res.CompressionRatio = float64(len(summary)) / float64(orig)
	return res
}
