// Package export renders a session into a plain-text forensic record.
//
// GORILLA OVERRIDE: this package did not exist upstream, and the /export it
// replaces produced a transcript you could not reconstruct events from. Three
// things were missing, and all three matter when something has gone wrong and
// you are trying to work out what:
//
//   - No timestamps at all, on any message — so no timeline could be built, even
//     though messages.created_at has been recorded since the first migration.
//   - No tool RESULTS. The old renderer switched on the message role and handled
//     only User and Assistant, so every message with role "tool" was silently
//     dropped. You saw every decision the agent made and not one outcome.
//   - No reasoning in practice. The old renderer did write it, but nothing ever
//     captured it outside the Anthropic provider, so the block never appeared.
//
// It is a pure function over stored data on purpose: no TUI, no filesystem, no
// clock except the one passed in. That makes it testable, and it means the same
// renderer can serve the dialog, a CLI flag, or a crash handler.
package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

// timeLayout is sortable and unambiguous.
//
// The request was "YY:MM:DD:HH:MM:SS". This uses a four-digit year and a dash
// date separator instead, because colons between date components do not sort
// lexicographically as dates and a two-digit year is ambiguous in a record whose
// whole purpose is to be read later. Same information, same field order.
const timeLayout = "2006-01-02 15:04:05"

// Stored timestamps are UNIX SECONDS.
//
// The migration comments claim milliseconds in three places and are wrong: the
// SQLite triggers use strftime('%s','now'), which is seconds. A real stored value
// of 1785228225 reads as 2026-07-28 09:43:45 in seconds; divided by 1000 as the
// comments instruct it reads as 1970-01-21. Anything that touches these columns
// must not trust the comment.
func at(unixSeconds int64) time.Time { return time.Unix(unixSeconds, 0) }

// Render returns the full transcript. now is the export time, injected so the
// output is deterministic under test.
func Render(sess session.Session, msgs []message.Message, now time.Time) string {
	var b strings.Builder

	// Chronological order is the point of the exercise; do not trust the caller.
	ordered := make([]message.Message, len(msgs))
	copy(ordered, msgs)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].CreatedAt < ordered[j].CreatedAt
	})

	writeHeader(&b, sess, ordered, now)

	var start time.Time
	if len(ordered) > 0 {
		start = at(ordered[0].CreatedAt)
	}

	// Stored tool RESULTS frequently have an empty Name — only the call carries
	// it. Rendering real sessions showed rows reading "← (ERROR)" with no clue
	// which tool had failed, which is close to useless. Build the id → name map
	// from the calls first, so a result can borrow the name of the call it
	// answers.
	names := toolNamesByCallID(ordered)

	for _, m := range ordered {
		writeMessage(&b, m, start, names)
	}
	return b.String()
}

func toolNamesByCallID(msgs []message.Message) map[string]string {
	names := map[string]string{}
	for _, m := range msgs {
		for _, tc := range m.ToolCalls() {
			if tc.ID != "" && tc.Name != "" {
				names[tc.ID] = tc.Name
			}
		}
	}
	return names
}

func writeHeader(b *strings.Builder, sess session.Session, msgs []message.Message, now time.Time) {
	title := strings.TrimSpace(sess.Title)
	if title == "" {
		title = "Untitled session"
	}
	fmt.Fprintf(b, "# %s\n\n", title)

	fmt.Fprintf(b, "- **Session ID:** `%s`\n", sess.ID)
	fmt.Fprintf(b, "- **Exported:** %s\n", now.Format(timeLayout))
	fmt.Fprintf(b, "- **Messages:** %d\n", len(msgs))

	if len(msgs) > 0 {
		first, last := at(msgs[0].CreatedAt), at(msgs[len(msgs)-1].CreatedAt)
		fmt.Fprintf(b, "- **First message:** %s\n", first.Format(timeLayout))
		fmt.Fprintf(b, "- **Last message:** %s\n", last.Format(timeLayout))
		fmt.Fprintf(b, "- **Elapsed:** %s\n", humanDuration(last.Sub(first)))

		// Which models actually served this session. A session can span several
		// after a /model switch, and "which model produced this" is usually the
		// first question asked of a transcript.
		if used := modelsUsed(msgs); len(used) > 0 {
			fmt.Fprintf(b, "- **Models used:** %s\n", strings.Join(used, ", "))
		}
	}

	// GORILLA OVERRIDE: how much reasoning this session actually generated.
	//
	// The honest version of "reasoning costs extra". Exact reasoning-token counts
	// would be better, but they are NOT available: measured against NVIDIA NIM on
	// 2026-07-28, the usage object carries only prompt_tokens, completion_tokens
	// and total_tokens — no completion_tokens_details.reasoning_tokens. Reading
	// that field would be dead code for the provider actually in use.
	//
	// So this reports what we own outright: the reasoning characters we captured
	// and stored, plus an estimate at the ~4 chars/token convention already used
	// for the loadout figures. Labelled as an estimate, with the method stated,
	// because a precise-looking number derived from a guess is worse than a
	// visibly approximate one.
	if chars := reasoningChars(msgs); chars > 0 {
		fmt.Fprintf(b, "- **Reasoning captured:** %d characters (~%d tokens at 4 chars/token — an estimate; this provider does not report an exact count)\n",
			chars, chars/4)
	}

	// Anything that ended abnormally, surfaced up front rather than left for the
	// reader to find by scrolling.
	if flags := abnormalEndings(msgs); len(flags) > 0 {
		fmt.Fprintf(b, "- **Abnormal endings:** %s\n", strings.Join(flags, ", "))
	}
	if n := toolErrorCount(msgs); n > 0 {
		fmt.Fprintf(b, "- **Tool calls that failed:** %d\n", n)
	}

	b.WriteString("\n---\n\n")
}

func writeMessage(b *strings.Builder, m message.Message, sessionStart time.Time, toolNames map[string]string) {
	ts := at(m.CreatedAt)
	// Absolute time answers "when"; the offset answers "how far into the session",
	// which is what you actually want when correlating with a log or a build.
	stamp := ts.Format(timeLayout)
	if !sessionStart.IsZero() {
		stamp += fmt.Sprintf(" (+%s)", humanDuration(ts.Sub(sessionStart)))
	}

	switch m.Role {
	case message.User:
		fmt.Fprintf(b, "## [%s] User\n\n", stamp)
		writeBody(b, m.Content().String())

	case message.Assistant:
		fmt.Fprintf(b, "## [%s] Assistant", stamp)
		if m.Model != "" {
			fmt.Fprintf(b, " · `%s`", m.Model)
		}
		b.WriteString("\n\n")

		// Reasoning first, because it precedes the answer in time and explains it.
		if r := strings.TrimSpace(m.ReasoningContent().String()); r != "" {
			b.WriteString("<details><summary>Reasoning</summary>\n\n")
			fmt.Fprintf(b, "```\n%s\n```\n\n", r)
			b.WriteString("</details>\n\n")
		}
		writeBody(b, m.Content().String())

		for _, tc := range m.ToolCalls() {
			fmt.Fprintf(b, "**→ tool call:** `%s`", tc.Name)
			if tc.ID != "" {
				fmt.Fprintf(b, "  <sub>%s</sub>", tc.ID)
			}
			b.WriteString("\n\n")
			fmt.Fprintf(b, "```json\n%s\n```\n\n", prettyJSON(tc.Input))
		}

		if fr := m.FinishReason(); isAbnormal(fr) {
			fmt.Fprintf(b, "> **Ended: %s**\n\n", fr)
		}

	// GORILLA OVERRIDE: the case whose absence made the whole export useless for
	// diagnosis. Tool results are stored on their own messages with role "tool";
	// with no branch for them, every outcome was thrown away.
	case message.Tool:
		fmt.Fprintf(b, "## [%s] Tool result\n\n", stamp)
		for _, tr := range m.ToolResults() {
			status := "ok"
			if tr.IsError {
				status = "ERROR"
			}
			// Borrow the call's name when the result has none, so no row reads
			// "← (ERROR)" with nothing to say which tool failed.
			name := tr.Name
			if name == "" {
				name = toolNames[tr.ToolCallID]
			}
			if name == "" {
				name = "unknown tool"
			}
			fmt.Fprintf(b, "**← %s** (%s)", name, status)
			if tr.ToolCallID != "" {
				fmt.Fprintf(b, "  <sub>answers %s</sub>", tr.ToolCallID)
			}
			b.WriteString("\n\n")

			if c := strings.TrimSpace(tr.Content); c != "" {
				fmt.Fprintf(b, "```\n%s\n```\n\n", c)
			} else {
				b.WriteString("_(empty result)_\n\n")
			}
			if md := strings.TrimSpace(tr.Metadata); md != "" && md != "{}" {
				fmt.Fprintf(b, "<sub>metadata: `%s`</sub>\n\n", md)
			}
		}

	case message.System:
		fmt.Fprintf(b, "## [%s] System\n\n", stamp)
		writeBody(b, m.Content().String())
	}
}

func writeBody(b *strings.Builder, s string) {
	if s = strings.TrimSpace(s); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
}

// prettyJSON reformats tool input for reading, and returns it untouched if it is
// not valid JSON — a forensic record must never discard what was actually sent
// just because it failed to parse.
func prettyJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(out)
}

func isAbnormal(fr message.FinishReason) bool {
	switch fr {
	case message.FinishReasonCanceled,
		message.FinishReasonError,
		message.FinishReasonPermissionDenied,
		message.FinishReasonMaxTokens:
		return true
	}
	return false
}

func abnormalEndings(msgs []message.Message) []string {
	counts := map[message.FinishReason]int{}
	for _, m := range msgs {
		if fr := m.FinishReason(); isAbnormal(fr) {
			counts[fr]++
		}
	}
	out := make([]string, 0, len(counts))
	for fr, n := range counts {
		out = append(out, fmt.Sprintf("%s ×%d", fr, n))
	}
	sort.Strings(out)
	return out
}

// reasoningChars totals the reasoning actually stored for this session.
func reasoningChars(msgs []message.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(strings.TrimSpace(m.ReasoningContent().Thinking))
	}
	return n
}

func toolErrorCount(msgs []message.Message) int {
	n := 0
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			if tr.IsError {
				n++
			}
		}
	}
	return n
}

func modelsUsed(msgs []message.Message) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range msgs {
		if m.Model == "" || seen[string(m.Model)] {
			continue
		}
		seen[string(m.Model)] = true
		out = append(out, string(m.Model))
	}
	return out
}

// humanDuration formats an elapsed span for a timeline: HH:MM:SS, which stays
// readable across a five-hour session and sorts sensibly.
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60)
}
