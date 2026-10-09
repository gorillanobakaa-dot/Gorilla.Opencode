package config

// GORILLA OVERRIDE (2026-10-09): user-configured lifecycle hooks.
//
// A hook is a LOCAL command the user wrote into their own config.json, run by
// the agent loop at three points: before a tool call (and able to refuse it),
// after a tool call, and at the end of a turn. Pattern taken from Kimi Code's
// lifecycle hooks.
//
// # WHO CAN ACT THROUGH IT
//
// Only the person who can write config.json. Nothing here fetches, syncs or
// defaults a hook: no hook ships configured, and an empty "hooks" array is the
// same as no key at all. It is a door the user opened on purpose, so it must
// also be VISIBLE: every configured hook is written to the log at start-up,
// one line each, and a hook that refuses a tool call says so in the result the
// model and the receipt both see.
//
// # FAIL CLOSED
//
// A malformed hook stops the program at load with a message naming it by its
// index and command. A gate that silently did not load would be worse than no
// gate, because the user believes it is there.

import (
	"fmt"
	"strings"

	"github.com/opencode-ai/opencode/internal/logging"
)

// Hook events.
const (
	HookBeforeTool = "before_tool"
	HookAfterTool  = "after_tool"
	HookTurnEnd    = "turn_end"
)

// Hook timeout bounds, in seconds.
const (
	HookDefaultTimeoutSeconds = 30
	HookMaxTimeoutSeconds     = 600
)

// Hook is one user-configured lifecycle hook.
//
// The mapstructure tags are load-bearing: viper decodes with THEM, not the json
// tags, and "timeout_seconds" would never match TimeoutSeconds without one.
type Hook struct {
	Event string `json:"event" mapstructure:"event"`
	// Tools limits a tool hook to these tool names. Empty means every tool.
	// Ignored for turn_end.
	Tools   []string `json:"tools,omitempty" mapstructure:"tools"`
	Command string   `json:"command" mapstructure:"command"`
	// TimeoutSeconds: 0 means the default (30); at most 600.
	TimeoutSeconds int `json:"timeout_seconds,omitempty" mapstructure:"timeout_seconds"`
}

// Timeout returns the effective timeout in seconds.
func (h Hook) Timeout() int {
	if h.TimeoutSeconds <= 0 {
		return HookDefaultTimeoutSeconds
	}
	return h.TimeoutSeconds
}

// AppliesToTool reports whether a tool hook covers the named tool.
func (h Hook) AppliesToTool(name string) bool {
	if len(h.Tools) == 0 {
		return true
	}
	for _, t := range h.Tools {
		if t == name {
			return true
		}
	}
	return false
}

// Describe is the one-line form used in the start-up log.
func (h Hook) Describe() string {
	tools := "every tool"
	switch {
	case h.Event == HookTurnEnd:
		tools = "-"
	case len(h.Tools) > 0:
		tools = strings.Join(h.Tools, ",")
	}
	return fmt.Sprintf("event=%s tools=%s timeout=%ds command=%s", h.Event, tools, h.Timeout(), h.Command)
}

// validateHooks rejects any malformed hook, naming it by index and command.
func validateHooks(hooks []Hook) error {
	for i, h := range hooks {
		name := fmt.Sprintf("hooks[%d] (command %q)", i, h.Command)
		switch h.Event {
		case HookBeforeTool, HookAfterTool, HookTurnEnd:
		default:
			return fmt.Errorf("%s: unknown event %q; use %q, %q or %q",
				name, h.Event, HookBeforeTool, HookAfterTool, HookTurnEnd)
		}
		if strings.TrimSpace(h.Command) == "" {
			return fmt.Errorf("%s: the command is empty", name)
		}
		if h.TimeoutSeconds < 0 || h.TimeoutSeconds > HookMaxTimeoutSeconds {
			return fmt.Errorf("%s: timeout_seconds %d is out of range; use 1 to %d (0 or absent means %d)",
				name, h.TimeoutSeconds, HookMaxTimeoutSeconds, HookDefaultTimeoutSeconds)
		}
		for _, t := range h.Tools {
			if strings.TrimSpace(t) == "" {
				return fmt.Errorf("%s: an entry in tools is empty", name)
			}
		}
	}
	return nil
}

// logHooks writes one line per configured hook, so an opened door is never
// invisible.
func logHooks(hooks []Hook) {
	for i, h := range hooks {
		logging.Info(fmt.Sprintf("lifecycle hook %d configured: %s", i, h.Describe()))
	}
}

// Hooks returns the configured lifecycle hooks; nil when none or not loaded.
func Hooks() []Hook {
	if cfg == nil {
		return nil
	}
	return cfg.Hooks
}
