// GORILLA OVERRIDE: this file did not exist upstream. It is the user-editable
// prompt layer: a factory default compiled into the binary, an optional override
// on disk, and a reset that restores the factory copy.
//
// Why the factory copy is the embedded one and not a file: it is the thing no
// user edit can corrupt. "Reset to default" is only trustworthy if the default
// lives somewhere the user cannot have already broken. Deleting the override
// file is therefore a complete, guaranteed reset.
package prompt

import (
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
)

// PromptID identifies an editable prompt.
type PromptID string

const (
	PromptCoder      PromptID = "coder"
	PromptSummarizer PromptID = "summarizer"
	PromptTask       PromptID = "task"
	PromptTitle      PromptID = "title"
)

// AllPromptIDs is the set the /prompts dialog and /reset iterate.
var AllPromptIDs = []PromptID{PromptCoder, PromptSummarizer, PromptTask, PromptTitle}

// PromptDisplayName is what the user sees, with a note on when it runs — a
// prompt you cannot place is a prompt you edit blind.
var PromptDisplayName = map[PromptID]string{
	PromptCoder:      "coder — every chat turn",
	PromptSummarizer: "summarizer — /clear compaction and session summaries",
	PromptTask:       "task — helper sub-agents",
	PromptTitle:      "title — naming new sessions",
}

// Factory returns the shipped default: the //go:embed copy, compiled into the
// binary. No user edit can reach it.
func Factory(id PromptID) string {
	switch id {
	case PromptCoder:
		return normaliseNewlines(baseModernCoderPrompt)
	case PromptSummarizer:
		return normaliseNewlines(baseSummarizerPrompt)
	case PromptTask:
		return normaliseNewlines(baseTaskPrompt)
	case PromptTitle:
		return normaliseNewlines(baseTitlePrompt)
	}
	return ""
}

// normaliseNewlines strips carriage returns and trims, so a prompt is the same
// bytes whichever platform the binary was built on.
//
// GORILLA OVERRIDE (2026-09-01): the prompt files are //go:embed'd, so whatever
// line endings the working tree happens to have at BUILD time are compiled into
// the binary. Git checks text files out with CRLF on Windows, so a Windows build
// embedded CR LF where a Linux build embedded LF — the same source producing two
// different prompts.
//
// That is not merely cosmetic. The section splitter rejoins bodies with a bare
// blank line, so on a CRLF checkout the rejoined prompt lost the carriage return
// from every section boundary: 24 bytes on the coder prompt. The round-trip
// guard — whose entire job is "splitting and rejoining must not lose a single
// byte" — then failed while pointing at code that was correct.
//
// It matters for prompt caching too. A cache is a prefix match, so a prompt that
// differs only in line endings between two builds shares no cache with itself.
//
// Normalised here rather than in ParseSections because this is the one place the
// embedded bytes enter the program.
func normaliseNewlines(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

// overrideCache avoids re-reading four files on every prompt render. Prompts are
// rendered on every turn; the files change only when the user edits one.
var (
	overrideMu    sync.RWMutex
	overrideCache = map[PromptID]string{}
	overrideRead  = map[PromptID]bool{}
)

func overrideFileName(id PromptID) string { return string(id) + ".txt" }

// Text returns the prompt actually in use: the user's override if one exists and
// is non-blank, otherwise the factory default.
//
// The blank guard is deliberate. An empty system prompt does not error — it
// silently produces a much worse agent, which is the hardest failure mode to
// diagnose. Falling back with a warning beats shipping nothing.
func Text(id PromptID) string {
	overrideMu.RLock()
	if overrideRead[id] {
		v := overrideCache[id]
		overrideMu.RUnlock()
		if v != "" {
			return v
		}
		return Factory(id)
	}
	overrideMu.RUnlock()

	overrideMu.Lock()
	defer overrideMu.Unlock()
	if overrideRead[id] { // another goroutine won the race
		if v := overrideCache[id]; v != "" {
			return v
		}
		return Factory(id)
	}

	raw, ok := config.ReadPromptOverride(overrideFileName(id))
	overrideRead[id] = true
	if !ok {
		overrideCache[id] = ""
		return Factory(id)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		logging.Warn("prompt override is empty, using the shipped default instead",
			"prompt", id, "file", overrideFileName(id))
		overrideCache[id] = ""
		return Factory(id)
	}
	overrideCache[id] = trimmed
	return trimmed
}

// IsOverridden reports whether a user copy exists on disk. The /prompts dialog
// shows this as EDITED so a tampered prompt is never invisible — the next person
// to look must be able to see at a glance that it was changed.
func IsOverridden(id PromptID) bool {
	Text(id) // populate the cache
	overrideMu.RLock()
	defer overrideMu.RUnlock()
	return overrideCache[id] != ""
}

// SaveOverride writes the user's version and drops the cache so the next render
// picks it up. Refuses blank content rather than storing a file that Text would
// silently ignore.
func SaveOverride(id PromptID, text string) error {
	if strings.TrimSpace(text) == "" {
		return errBlankPrompt
	}
	if err := config.WritePromptOverride(overrideFileName(id), text); err != nil {
		return err
	}
	invalidateOverride(id)
	return nil
}

// ResetPrompt deletes the override, restoring the factory default.
func ResetPrompt(id PromptID) error {
	if err := config.RemovePromptOverride(overrideFileName(id)); err != nil {
		return err
	}
	invalidateOverride(id)
	return nil
}

// ResetAllPrompts restores every prompt to its shipped default.
func ResetAllPrompts() error {
	var firstErr error
	for _, id := range AllPromptIDs {
		if err := ResetPrompt(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func invalidateOverride(id PromptID) {
	overrideMu.Lock()
	delete(overrideCache, id)
	delete(overrideRead, id)
	overrideMu.Unlock()
	// Section toggles are derived from the ACTIVE prompt text, so a changed
	// prompt may have changed its section set.
	invalidateSections()
}

// OverridePath is where a given prompt's override lives, for the dialog to show
// and for $EDITOR to open.
func OverridePath(id PromptID) string {
	return config.PromptOverridePath(overrideFileName(id))
}

type blankPromptError struct{}

func (blankPromptError) Error() string {
	return "refusing to save an empty prompt: a blank system prompt does not error, " +
		"it silently produces a much worse agent"
}

var errBlankPrompt = blankPromptError{}
