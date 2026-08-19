// GORILLA OVERRIDE: this file did not exist upstream. It sanitises the
// generated session title.
//
// The title is produced by whatever small, cheap model the title agent points
// at, and small models do not obey "output only the title". A real observed
// result, rendered verbatim into the sidebar:
//
//	Session: Here's a possible title, keeping the constraints in
//	         mind:  **Short and Concise:**  **Title:**  Your
//	         Business Brief
//
// The old code did `TrimSpace(ReplaceAll(content, "\n", " "))` and stored the
// result, so every word of the model's thinking-out-loud became the title.
//
// Tightening the prompt alone cannot fix this — the prompt already says "no
// meta-text like 'Title:'" and "entire output becomes title", and the model
// ignored both. The output has to be treated as untrusted text and reduced to
// the part that is actually a title, with the user's own first line as the
// fallback when nothing usable survives.
package agent

import (
	"regexp"
	"strings"
	"unicode"
)

const maxTitleChars = 50

var (
	// Fenced code blocks, which some models wrap the answer in.
	titleFenceRe = regexp.MustCompile("(?s)```[a-zA-Z0-9]*\n?(.*?)```")
	// A meta label the model emits before the real title. The last one wins,
	// because the title follows the final label ("... **Title:** Your Brief").
	titleLabelRe = regexp.MustCompile(`(?i)\b(?:title|session title|summary|suggestion|answer|response|here'?s?[^:]*)\s*:\s*`)
	// Leading list/heading markers: "- ", "* ", "1. ", "#", "> ".
	titleBulletRe = regexp.MustCompile(`^\s*(?:[-*+>]|#{1,6}|\d+[.)])\s*`)
	// Markdown emphasis and code ticks. Removed rather than parsed: the goal is
	// plain text, and a stray unmatched ** is more likely than real markup.
	titleEmphasisRe = regexp.MustCompile(`[*_` + "`" + `~]+`)
	titleSpaceRe    = regexp.MustCompile(`\s+`)
	// Phrases that mean the model narrated instead of answering. If this is all
	// that is left, the title is worthless and the user's own words are better.
	titleChatterRe = regexp.MustCompile(`(?i)^(?:sure|okay|ok|here|here'?s|certainly|of course|understood|got it|based on|as requested|i'?ve|i will|let me)\b`)
)

// sanitiseTitle reduces a model's reply to a single short plain-text line.
// fallback is the user's first message, used when nothing usable survives —
// a truncated version of what the user actually said always beats the model's
// commentary about the request.
func sanitiseTitle(raw, fallback string) string {
	s := raw
	// Prefer the contents of a code fence when one is present: models that wrap
	// the answer put ONLY the answer inside it.
	if m := titleFenceRe.FindStringSubmatch(s); m != nil && strings.TrimSpace(m[1]) != "" {
		s = m[1]
	}

	// Take the text after the LAST meta label, then the last non-empty line.
	// Both handle the same failure from different directions: the label case
	// ("**Title:** Your Brief") and the preamble-then-answer case, where the
	// model explains itself first and answers on the final line.
	if locs := titleLabelRe.FindAllStringIndex(s, -1); len(locs) > 0 {
		if tail := strings.TrimSpace(s[locs[len(locs)-1][1]:]); tail != "" {
			s = tail
		}
	}
	if line := lastMeaningfulLine(s); line != "" {
		s = line
	}

	s = titleBulletRe.ReplaceAllString(s, "")
	s = titleEmphasisRe.ReplaceAllString(s, "")
	s = titleSpaceRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	// Surrounding quotes, then trailing sentence punctuation. A title is not a
	// sentence, and the prompt asks for no quotes.
	s = strings.Trim(s, `"'“”‘’`)
	s = strings.TrimRight(s, ".,;:!—- ")
	s = strings.TrimSpace(s)

	if s == "" || titleChatterRe.MatchString(s) {
		s = titleSpaceRe.ReplaceAllString(strings.TrimSpace(fallback), " ")
		s = strings.TrimSpace(s)
	}
	return truncateTitle(s, maxTitleChars)
}

// lastMeaningfulLine returns the final line that carries actual content. Models
// that narrate put the answer last; a line that is only a label ("Title:") or
// only punctuation is skipped.
func lastMeaningfulLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		bare := strings.TrimSpace(titleEmphasisRe.ReplaceAllString(l, ""))
		bare = strings.TrimSpace(titleBulletRe.ReplaceAllString(bare, ""))
		if bare == "" || strings.HasSuffix(bare, ":") {
			continue // a label with its value on the next line, or decoration
		}
		if strings.ContainsAny(bare, ":") && titleLabelRe.MatchString(bare) {
			// "**Title:** Your Brief" on one line — keep only what follows.
			if locs := titleLabelRe.FindAllStringIndex(bare, -1); len(locs) > 0 {
				if tail := strings.TrimSpace(bare[locs[len(locs)-1][1]:]); tail != "" {
					return tail
				}
			}
			continue
		}
		return l
	}
	return ""
}

// truncateTitle cuts at a word boundary where it can, so a clipped title reads
// as shortened rather than corrupted.
func truncateTitle(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	// GORILLA FIX (2026-08-19): the marker has to fit INSIDE the limit.
	// This cut to `limit` and then appended, so the result was always over by
	// the marker's length — invisible at one character, obvious the moment the
	// marker became "..." at three. A budget that the thing announcing the
	// budget does not fit inside is not a budget.
	const marker = "..."
	room := limit - len(marker)
	if room < 1 {
		room = 1
	}
	cut := string(r[:room])
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > room/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(strings.TrimSpace(cut), ".,;:!—- ") + marker
}
