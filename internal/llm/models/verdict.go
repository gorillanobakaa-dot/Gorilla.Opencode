package models

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

// GORILLA OVERRIDE: what this project says a model is good for, in three layers.
//
// The picker offers hundreds of models to people who mostly want to learn to
// code, on a connection where researching a single unfamiliar name — a search
// plus a heavy vendor page — is not slow but impossible. So the description has
// to do that work, and it has to be honest about where it came from:
//
//	1. EARNED VERDICT   (verdicts.json)  — we used it; here is what happened,
//	                                       with a citation. Overrides everything.
//	2. CURATED          (nim.json)       — a judgement already written for the
//	                                       same underlying model.
//	3. VENDOR'S OWN TEXT, CLASSIFIED     — what THEY claim, with the word that
//	                                       triggered the label quoted so anyone
//	                                       can check it.
//	4. NO CLAIM AT ALL                   — say so plainly. Do not guess.
//
// Layer 3 is inference, not invention: if a vendor's own copy calls a model a
// roleplaying model, concluding it will not build a kernel is reading. What is
// forbidden is a verdict with nothing behind it — that would be worse than the
// marketing it replaced, because it would carry this project's name.

//go:embed metadata/verdicts.json
var verdictsJSON []byte

type earnedVerdict struct {
	Verdict  string `json:"verdict"`
	Evidence string `json:"evidence"`
}

var earnedVerdicts = loadVerdicts()

func loadVerdicts() map[string]earnedVerdict {
	var raw map[string]json.RawMessage
	out := map[string]earnedVerdict{}
	if json.Unmarshal(verdictsJSON, &raw) != nil {
		return out
	}
	for k, v := range raw {
		if strings.HasPrefix(k, "_") {
			continue
		}
		var e earnedVerdict
		if json.Unmarshal(v, &e) == nil && e.Verdict != "" {
			out[strings.ToLower(k)] = e
		}
	}
	return out
}

// EarnedVerdict returns this project's own finding for a model, if one exists.
// Matched on id PREFIX, because a family behaves like a family: every
// google/gemini-* shares the behaviour the forensics recorded.
func EarnedVerdict(apiModel string) (earnedVerdict, bool) {
	id := strings.ToLower(apiModel)
	for prefix, v := range earnedVerdicts {
		if strings.HasPrefix(id, prefix) {
			return v, true
		}
	}
	return earnedVerdict{}, false
}

// codingSignals classify a vendor's own description. Order matters: a claim to
// code outranks a mention of vision, because a multimodal coder is still a
// coder; roleplay outranks everything else, because it is a statement of
// purpose rather than a passing mention.
var codingSignals = []struct {
	label string
	re    *regexp.Regexp
}{
	{"CAN CODE", regexp.MustCompile(`(?i)SWE-?bench|agentic coding|coding-focused|code generation|software engineering|for developers`)},
	{"shit tier for code — vendor calls it roleplay", regexp.MustCompile(`(?i)role ?play|companion|persona|storytelling`)},
	{"shit tier for code — vision/image model", regexp.MustCompile(`(?i)vision-language|image generation|text-to-image|\bOCR\b`)},
	{"mentions coding", regexp.MustCompile(`(?i)coding|\bcode\b`)},
	{"research/admin work", regexp.MustCompile(`(?i)reasoning|agentic|long-horizon|orchestration|analysis`)},
}

// ClassifyForCoding labels a model from the vendor's OWN words and returns the
// phrase that triggered the label, so the claim can be checked rather than
// trusted. A description making no relevant claim is reported as exactly that.
func ClassifyForCoding(vendorDesc string) (label, trigger string) {
	for _, s := range codingSignals {
		if m := s.re.FindString(vendorDesc); m != "" {
			return s.label, m
		}
	}
	return "UNTESTED for coding work — use at your own risk", ""
}
