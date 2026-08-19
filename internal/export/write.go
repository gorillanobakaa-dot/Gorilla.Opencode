package export

// GORILLA OVERRIDE (2026-08-18): writing an export, for ANY session.
//
// `/export` could only ever save the conversation you were currently in. That
// is the wrong shape for the machines this is built for, and the owner said why:
//
//   "the electricity isn't guaranteed either. 15 year old laptops have very poor
//    battery life, measured in minutes maybe, and some of them seconds only, so
//    they have to be plugged in all the time. If the electricity drops, the whole
//    session goes as well. Sometimes it might not appear for days on end."
//
// A session you can only export while you are inside it is a session you cannot
// export after the power cut — which is precisely when you need it. So the
// naming and writing moved here, out of the TUI, taking a session and its
// messages as arguments rather than reading whatever is on screen.
//
// The transcript deliberately keeps the model's REASONING and every tool call
// with its input and its result. Again in the owner's words: those are
//
//   "very important because they enable you to understand how the model ended up
//    either posting or leaking your private messages on GitHub in the happiest
//    case, or erasing half of your hard drive with your long lost memories."
//
// An export that keeps only the visible chat cannot answer either question.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

// WriteSession renders a session and writes it into dir, returning the path.
//
// It never overwrites: an export is a record, and silently replacing one is the
// quiet data loss this whole feature exists to prevent. A name that is taken
// gets a numbered suffix rather than an error, because the alternative is
// telling someone whose power just came back that their export "already exists".
func WriteSession(dir string, sess session.Session, msgs []message.Message, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", dir, err)
	}

	path := freePath(dir, SuggestedName(sess.Title, now))
	body := Render(sess, msgs, now)

	// 0o600: a transcript holds whatever was discussed — file contents, command
	// output, sometimes credentials that appeared on screen. It is not
	// world-readable by default, and on a shared family machine that matters.
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// freePath returns name inside dir, or name-2, name-3... if it is taken. Bounded:
// after 99 attempts something is wrong with the directory rather than with the
// name, and looping forever on a slow disk helps nobody.
func freePath(dir, name string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i < 100; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
	return path
}

// SuggestedName derives a filesystem-safe filename from a session title.
//
// The timestamp is part of the name because exports ACCUMULATE — they are
// artifacts, not source — which is the project's stamping convention, and it
// keeps successive exports of one session distinct.
func SuggestedName(title string, now time.Time) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == ' ', r == '-', r == '_':
			return '-'
		default:
			return -1
		}
	}, title)

	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "session"
	}
	const maxSlug = 60
	if len([]rune(slug)) > maxSlug {
		slug = strings.Trim(string([]rune(slug)[:maxSlug]), "-")
	}
	return fmt.Sprintf("gorilla-opencode-%s-%s.md", slug, now.Format("20060102-150405"))
}

// Branch is one helper session and its messages, ready to be appended to the
// transcript of the conversation that spawned it.
type Branch struct {
	Session  session.Session
	Messages []message.Message
}

// WriteSessionTree writes a conversation AND every helper session it spawned.
//
// GORILLA (2026-08-18): found by exporting a real research run from /sessions.
// The list said 275 messages; the export contained 14. The other 261 lived in
// seventeen helper sessions, and that is where the lanes' reasoning and tool
// calls are — the material the owner named as the reason the export must exist
// at all:
//
//	"the tool calls and model reasoning are very important because they enable
//	 you to understand how the model ended up either posting or leaking your
//	 private messages on GitHub in the happiest case, or erasing half of your
//	 hard drive with your long lost memories."
//
// An export missing 95% of a run is worse than none, because it looks complete.
func WriteSessionTree(dir string, sess session.Session, msgs []message.Message, branches []Branch, now time.Time) (string, int, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", 0, fmt.Errorf("cannot create %s: %w", dir, err)
	}

	total := len(msgs)
	var b strings.Builder
	b.WriteString(Render(sess, msgs, now))

	if len(branches) > 0 {
		fmt.Fprintf(&b, "\n---\n\n# Helper sessions (%d)\n\n", len(branches))
		b.WriteString("Each helper ran in its own session with its own context. They are the " +
			"bulk of what a research run actually did, and they are reproduced in full below " +
			"— reasoning, every tool call with its input, and every result including the " +
			"failures.\n\n")
		for _, br := range branches {
			total += len(br.Messages)
			title := strings.TrimSpace(br.Session.Title)
			if title == "" {
				title = br.Session.ID
			}
			fmt.Fprintf(&b, "\n---\n\n## Helper: %s\n\n", title)
			fmt.Fprintf(&b, "- **Session ID:** `%s`\n", br.Session.ID)
			fmt.Fprintf(&b, "- **Messages:** %d\n", len(br.Messages))
			// GORILLA FIX (2026-08-19): this wrote the LAST TURN's token counts
			// into a document meant to be kept, labelled as though it were the
			// helper's total. A transcript that misstates what a run cost is
			// worse than one that omits it.
			fmt.Fprintf(&b, "- **Tokens (whole run):** %d in / %d out\n",
				br.Session.CumulativePromptTokens, br.Session.CumulativeCompletionTokens)
			fmt.Fprintf(&b, "- **Context at the end:** %d in / %d out\n\n",
				br.Session.PromptTokens, br.Session.CompletionTokens)
			b.WriteString(renderMessages(br.Messages))
		}
	}

	path := freePath(dir, SuggestedName(sess.Title, now))
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", 0, err
	}
	return path, total, nil
}
