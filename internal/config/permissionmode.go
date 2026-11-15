package config

// GORILLA OVERRIDE (2026-10-11): the standing permission mode, persisted.
//
// Until now the only way to stop being asked was /yolo, which armed ONE
// conversation and was deliberately never written to disk, so every new
// conversation started in "ask" and the owner had to re-arm it each time. A
// 58-minute session in "ask" raised 31 shell prompts; the owner's verdict was
// that it "defeated the purpose of having an AI". Other coding agents carry a
// standing mode for this (Claude Code: default / acceptEdits / plan / auto),
// switched with Shift+Tab; this is the same set.
//
//	ask           every change asks, once per thing ("allow for session" and
//	              "allow family" make that once rather than every time).
//	accept-edits  file edits inside the workspace roots are approved; shell
//	              commands, the web and MCP servers still ask.
//	plan          read-only: anything that would change a file or run a
//	              command is refused at once, with no dialog, and the model
//	              is told to describe the plan instead. Web reads still ask.
//	auto          what /yolo did for one conversation, for every
//	              conversation — with the same carve-outs /yolo always had
//	              (permission.mustAskAnyway): an irreversible command, sudo
//	              or an install for the whole machine, a path outside every
//	              workspace root. (Until 2026-10-23 also a request leaving the
//	              machine after the turn read untrusted content; removed.)
//
// The mode is shown in the status bar on every frame while it is not "ask",
// for the same reason /yolo was. Who else can act through this: nobody new —
// a local setting in the user's own config file, read at launch and on
// change, never fetched from anywhere.

import (
	"fmt"
	"strings"
)

const (
	PermissionAsk         = "ask"
	PermissionAcceptEdits = "accept-edits"
	PermissionPlan        = "plan"
	PermissionAuto        = "auto"
)

// PermissionModes is the Shift+Tab cycle order.
var PermissionModes = []string{PermissionAsk, PermissionAcceptEdits, PermissionPlan, PermissionAuto}

// PermissionsConfig is the persisted block.
type PermissionsConfig struct {
	Mode string `json:"mode,omitempty"`
	// Web is "allow" (default) or "ask": whether web_search and public GET
	// web_fetch run without a dialog. See WebPermission.
	Web string `json:"web,omitempty"`
}

const (
	WebAllow = "allow"
	WebAsk   = "ask"
)

// WebPermission is the one decision for the web tools: "allow" (the default)
// lets web_search and a public GET web_fetch run without asking, in every
// mode except plan. "ask" restores a dialog per search and per site.
// Dangerous shapes (non-http(s), private addresses, long query data) ask
// regardless; see tools.webNeedsAsk.
func WebPermission() string {
	if cfg == nil || cfg.Permissions.Web != WebAsk {
		return WebAllow
	}
	return WebAsk
}

// SetWebPermission records and persists the web decision.
func SetWebPermission(v string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if v != WebAllow && v != WebAsk {
		return fmt.Errorf("permissions.web must be allow or ask, not %q", v)
	}
	cfg.Permissions.Web = v
	return updateCfgFile(func(c *Config) { c.Permissions.Web = v })
}

// PermissionMode reports the standing mode; "ask" until something is set.
func PermissionMode() string {
	if cfg == nil {
		return PermissionAsk
	}
	switch cfg.Permissions.Mode {
	case PermissionAcceptEdits, PermissionPlan, PermissionAuto:
		return cfg.Permissions.Mode
	}
	return PermissionAsk
}

// NormalisePermissionMode accepts the spellings people type and returns the
// canonical one, or "" for anything unknown.
func NormalisePermissionMode(s string) string {
	switch strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-") {
	case PermissionAsk, "default", "manual", "normal":
		return PermissionAsk
	case PermissionAcceptEdits, "accept_edits", "acceptedits", "edits", "accept":
		return PermissionAcceptEdits
	case PermissionPlan, "planning", "read-only", "readonly":
		return PermissionPlan
	case PermissionAuto, "yolo", "autopilot":
		return PermissionAuto
	}
	return ""
}

// SetPermissionMode records the mode and persists it.
func SetPermissionMode(mode string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	canon := NormalisePermissionMode(mode)
	if canon == "" {
		return fmt.Errorf("unknown permission mode %q — use one of %s", mode, strings.Join(PermissionModes, ", "))
	}
	cfg.Permissions.Mode = canon
	return updateCfgFile(func(c *Config) { c.Permissions.Mode = canon })
}

// NextPermissionMode is the mode after cur in the Shift+Tab cycle.
func NextPermissionMode(cur string) string {
	for i, m := range PermissionModes {
		if m == cur {
			return PermissionModes[(i+1)%len(PermissionModes)]
		}
	}
	return PermissionModes[0]
}

// PermissionModeLayman is the one-sentence explanation of a mode, used by the
// status line, /mode and the settings row.
func PermissionModeLayman(mode string) string {
	switch mode {
	case PermissionAcceptEdits:
		return "file edits inside your project are approved; shell commands, the web and MCP servers still ask"
	case PermissionPlan:
		return "read-only: nothing is changed or run; the AI describes what it would do"
	case PermissionAuto:
		return "approved without asking, except an irreversible command, sudo or an install for the whole computer, or a path outside your workspace"
	}
	return "every change asks once; answer \"allow for session\" or \"allow family\" to stop repeats"
}

func init() {
	Settings = append(Settings, Setting{
		ID:    "permissions.mode",
		Group: GroupConversation,
		Name:  "Permission mode",
		Layman: "How much the AI may do before asking you. ask: every change asks. accept-edits: file edits in your project are approved, commands and the web still ask. " +
			"plan: read-only, the AI describes instead of doing. auto: approved without asking, except an irreversible command, sudo or an install for the whole computer, or a path outside your workspace. " +
			"Shift+Tab cycles through them; /mode sets one by name. Shown in the status bar whenever it is not \"ask\".",
		Kind:    KindEnum,
		Default: PermissionAsk,
		Options: PermissionModes,
		Get:     func() any { return PermissionMode() },
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			return SetPermissionMode(s)
		},
	})
}

func init() {
	Settings = append(Settings, Setting{
		ID:    "permissions.web",
		Group: GroupConversation,
		Name:  "Web access without asking",
		Layman: "allow (default): searching the web and reading a public web page never asks. ask: every search and every new site asks. " +
			"Either way it still asks for a non-web address, a page on your own network or this computer, a very long query string, or anything that uploads. " +
			"Plan mode always asks for web reads.",
		Kind:    KindEnum,
		Default: WebAllow,
		Options: []string{WebAllow, WebAsk},
		Get:     func() any { return WebPermission() },
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			return SetWebPermission(s)
		},
	})
}
