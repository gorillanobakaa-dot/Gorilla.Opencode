// GORILLA OVERRIDE: this file did not exist upstream. It is the settings
// registry behind /settings — every user-tunable knob, self-describing.
//
// Why a registry rather than a hand-built dialog: the requirement is that every
// setting carries a plain-language description of what it does, what it accepts,
// and its min/max/default. None of that existed anywhere — defaults were bare
// viper.SetDefault calls with no ranges, no types and no prose. Putting it in one
// table means the dialog needs no per-setting special casing, the docs can be
// generated from the same source, and a half-added setting fails a test rather
// than shipping a blank row.
//
// The precedent is LoadoutComponents: a flat slice of self-describing records
// with a Tradeoff string. That design is right; it just only covered on/off.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// SettingKind determines the editor widget and the validation applied.
type SettingKind int

const (
	KindBool       SettingKind = iota // on/off
	KindInt                           // integer within [Min, Max], nudged by Step
	KindLadder                        // integer snapped to Presets (rpm, sub-agents)
	KindEnum                          // one of Options
	KindString                        // free text (paths, commands)
	KindStringList                    // ordered list (contextPaths, shell.args)
)

func (k SettingKind) String() string {
	switch k {
	case KindBool:
		return "on/off"
	case KindInt:
		return "number"
	case KindLadder:
		return "preset"
	case KindEnum:
		return "choice"
	case KindString:
		return "text"
	case KindStringList:
		return "list"
	}
	return "?"
}

// SettingGroup buckets rows for display.
type SettingGroup string

const (
	GroupAppearance   SettingGroup = "Appearance"
	GroupConversation SettingGroup = "Conversation"
	GroupNetwork      SettingGroup = "Network and pace"
	GroupFiles        SettingGroup = "Files and shell"
	GroupDiagnostics  SettingGroup = "Diagnostics"
	// GORILLA OVERRIDE: the optional behaviours that show the agent's working.
	// Its rows are generated from the Extras registry in extras.go rather than
	// written out again here, so the cost wording cannot drift between the
	// first-run screen, /context and this dialog.
	GroupExtras SettingGroup = "Show me the working"
)

// GroupOrder is the display order; a map would reshuffle between renders.
var GroupOrder = []SettingGroup{
	GroupConversation, GroupExtras, GroupNetwork, GroupFiles, GroupAppearance, GroupDiagnostics,
}

// Setting is one knob, fully self-describing.
type Setting struct {
	ID     string
	Group  SettingGroup
	Name   string // short label
	Layman string // WHAT IT DOES, no jargon. The core requirement.

	// Bool-only: the effect of each state, so the row explains both directions
	// rather than leaving the user to infer the off case.
	WhenOn  string
	WhenOff string

	Kind     SettingKind
	Default  any
	Min, Max int
	Step     int
	Presets  []int
	Options  []string
	Unit     string

	// ReadOnly rows are shown but not editable. Offering a control that gets
	// silently overwritten is worse than showing none.
	ReadOnly    bool
	ReadOnlyWhy string

	// Restart is true when a change only applies on the next launch. Said on the
	// row and in the confirmation, so nothing silently no-ops.
	Restart bool

	Get func() any
	Set func(any) error
}

// clampInt keeps an int inside a setting's declared range and reports what it did
// in plain language, so the dialog can show the reason rather than just refusing.
func clampInt(s *Setting, v int) (int, error) {
	if v < s.Min || v > s.Max {
		return 0, fmt.Errorf("must be between %d and %d", s.Min, s.Max)
	}
	return v, nil
}

func asInt(v any) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case string:
		var out int
		if _, err := fmt.Sscanf(strings.TrimSpace(n), "%d", &out); err != nil {
			return 0, fmt.Errorf("%q is not a whole number", n)
		}
		return out, nil
	}
	return 0, fmt.Errorf("expected a number, got %T", v)
}

func asBool(v any) (bool, error) {
	switch b := v.(type) {
	case bool:
		return b, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "yes", "on", "1":
			return true, nil
		case "false", "no", "off", "0":
			return false, nil
		}
	}
	return false, fmt.Errorf("expected true or false, got %v", v)
}

func asString(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("expected text, got %T", v)
}

// Shared bounds for the per-agent max-tokens rows. Declared as constants so the
// registry entry and its validator cannot disagree.
const (
	minAgentMaxTokens = 512
	maxAgentMaxTokens = 32768
)

// agentMaxTokensSetting builds the per-agent max-tokens row, since three agents
// share identical shape and only differ in which agent they address.
func agentMaxTokensSetting(agent AgentName, name, layman string) Setting {
	return Setting{
		ID:      "agents." + string(agent) + ".maxTokens",
		Group:   GroupConversation,
		Name:    name,
		Layman:  layman,
		Kind:    KindInt,
		Default: 4096,
		Min:     minAgentMaxTokens,
		Max:     maxAgentMaxTokens,
		Step:    512,
		Unit:    "tokens",
		Get: func() any {
			if cfg == nil {
				return 0
			}
			return int(cfg.Agents[agent].MaxTokens)
		},
		Set: func(v any) error {
			n, err := asInt(v)
			if err != nil {
				return err
			}
			// Bounds are inlined rather than read back via SettingByID: doing
			// that from inside a Settings entry's closure creates an
			// initialization cycle (Settings -> helper -> SettingByID -> Settings).
			if n < minAgentMaxTokens || n > maxAgentMaxTokens {
				return fmt.Errorf("must be between %d and %d", minAgentMaxTokens, maxAgentMaxTokens)
			}
			set := func(c *Config) {
				if c.Agents == nil {
					c.Agents = make(map[AgentName]Agent)
				}
				a := c.Agents[agent]
				a.MaxTokens = int64(n)
				c.Agents[agent] = a
			}
			set(cfg)
			return updateCfgFile(set)
		},
	}
}

// Settings is the registry. Order within a group is display order.
var Settings = []Setting{
	// ─── Conversation ────────────────────────────────────────────────
	{
		ID:      "autoCompact",
		Group:   GroupConversation,
		Name:    "Auto-summarise long chats",
		Layman:  "What happens when a conversation grows too long to fit in the AI's memory.",
		WhenOn:  "the program automatically summarises the older part and carries on",
		WhenOff: "you get an error when the chat is full and must start a new one with /clear yourself",
		Kind:    KindBool,
		Default: true,
		Get:     func() any { return cfg != nil && cfg.AutoCompact },
		Set: func(v any) error {
			b, err := asBool(v)
			if err != nil {
				return err
			}
			set := func(c *Config) { c.AutoCompact = b }
			set(cfg)
			return updateCfgFile(set)
		},
	},
	agentMaxTokensSetting(AgentCoder, "Longest single AI reply",
		"The most the AI may write in one reply. Higher lets it produce bigger files in one go, but each reply can cost more and take longer."),
	agentMaxTokensSetting(AgentSummarizer, "Summariser reply length",
		"How much room the summariser gets when it compresses an old conversation."),
	agentMaxTokensSetting(AgentTask, "Helper AI reply length",
		"How much room a helper sub-agent gets for its answer."),
	{
		ID:          "agents.title.maxTokens",
		Group:       GroupConversation,
		Name:        "Session title length",
		Layman:      "How long a generated session name may be.",
		Kind:        KindInt,
		Default:     80,
		Min:         80,
		Max:         80,
		Unit:        "tokens",
		ReadOnly:    true,
		ReadOnlyWhy: "fixed at 80 — titles must be one short line, and the program overwrites this on every launch",
		Get: func() any {
			if cfg == nil {
				return 0
			}
			return int(cfg.Agents[AgentTitle].MaxTokens)
		},
	},
	{
		ID:      "agents.coder.reasoningEffort",
		Group:   GroupConversation,
		Name:    "Thinking effort",
		Layman:  "How long the AI may think before answering. Only some models support this; on the rest it is ignored. Higher is slower but usually better on hard bugs.",
		Kind:    KindEnum,
		Default: "medium",
		Options: []string{"low", "medium", "high"},
		Get: func() any {
			if cfg == nil {
				return ""
			}
			return cfg.Agents[AgentCoder].ReasoningEffort
		},
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			s = strings.ToLower(strings.TrimSpace(s))
			switch s {
			case "low", "medium", "high":
			default:
				return fmt.Errorf("must be one of: low, medium, high")
			}
			set := func(c *Config) {
				if c.Agents == nil {
					c.Agents = make(map[AgentName]Agent)
				}
				a := c.Agents[AgentCoder]
				a.ReasoningEffort = s
				c.Agents[AgentCoder] = a
			}
			set(cfg)
			return updateCfgFile(set)
		},
	},

	// ─── Network and pace ────────────────────────────────────────────
	{
		ID:    "connection.profile",
		Group: GroupNetwork,
		Name:  "Connection profile",
		Layman: "How patient this program is with your internet, and how much " +
			"data one message is allowed to spend. Pick the row that matches your " +
			"connection. A slower profile waits longer before deciding something " +
			"is broken and uploads less; a faster one gives up quickly, because on " +
			"a good line a long silence means a real fault. This changes waiting " +
			"and spending only - it never changes what the AI can do.",
		Kind:    KindEnum,
		Default: connProfileNameByID(DefaultConnProfile),
		Options: ConnProfileNames(),
		Get:     func() any { return CurrentConnProfile().Name },
		Set: func(v any) error {
			name, ok := v.(string)
			if !ok {
				return fmt.Errorf("expected a profile name")
			}
			id, ok := ConnProfileByName(name)
			if !ok {
				return fmt.Errorf("unknown connection profile %q", name)
			}
			return SetConnProfile(id)
		},
	},
	{
		ID:      "ratelimit.rpm",
		Group:   GroupNetwork,
		Name:    "Requests per minute",
		Layman:  "How fast this program is allowed to talk to the AI provider. Free accounts cut you off if you go too fast. Lower is slower but avoids cut-offs. 0 means no limit at all.",
		Kind:    KindLadder,
		Default: DefaultRPM,
		Presets: RPMPresets,
		Unit:    "requests/min",
		Get:     func() any { return RateLimitRPM() },
		Set: func(v any) error {
			n, err := asInt(v)
			if err != nil {
				return err
			}
			if n < 0 {
				return fmt.Errorf("cannot be negative; use 0 for unlimited")
			}
			SetRateLimitRPM(n)
			return nil
		},
	},
	{
		ID:      "subagents.max",
		Group:   GroupNetwork,
		Name:    "Helper AIs allowed",
		Layman:  "How many helper AIs the main AI may start. Each helper is a whole extra conversation you pay for. 0 switches helpers off completely; -1 means no limit.",
		Kind:    KindLadder,
		Default: DefaultMaxSubAgents,
		Presets: SubAgentPresets,
		Get:     func() any { return MaxSubAgents() },
		Set: func(v any) error {
			n, err := asInt(v)
			if err != nil {
				return err
			}
			if n < SubAgentsUnlimited {
				return fmt.Errorf("use -1 for unlimited, 0 to disable helpers, or a positive cap")
			}
			SetMaxSubAgents(n)
			return nil
		},
	},

	// ─── Files and shell ─────────────────────────────────────────────
	{
		ID:      "shell.path",
		Group:   GroupFiles,
		Name:    "Shell used for commands",
		Layman:  "Which shell runs the commands the AI asks for. Change this only if your shell lives somewhere unusual — a wrong value breaks every command the AI tries to run.",
		Kind:    KindString,
		Default: "/bin/bash",
		Get: func() any {
			if cfg == nil {
				return ""
			}
			return cfg.Shell.Path
		},
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			s = strings.TrimSpace(s)
			if s == "" {
				return fmt.Errorf("cannot be empty")
			}
			// The one genuinely destructive setting: a wrong value breaks every
			// bash call, and the user's next instinct would be to blame the AI.
			info, statErr := os.Stat(s)
			if statErr != nil {
				return fmt.Errorf("%s does not exist", s)
			}
			if info.IsDir() {
				return fmt.Errorf("%s is a directory, not a shell", s)
			}
			if info.Mode().Perm()&0o111 == 0 {
				return fmt.Errorf("%s is not executable — the AI would not be able to run any commands", s)
			}
			set := func(c *Config) { c.Shell.Path = s }
			set(cfg)
			return updateCfgFile(set)
		},
	},
	{
		ID:      "shell.args",
		Group:   GroupFiles,
		Name:    "Shell flags",
		Layman:  "Extra flags for that shell. -l makes it a login shell, so your PATH and aliases are loaded.",
		Kind:    KindStringList,
		Default: []string{"-l"},
		Get: func() any {
			if cfg == nil {
				return []string{}
			}
			return cfg.Shell.Args
		},
		Set: func(v any) error {
			list, err := asStringList(v)
			if err != nil {
				return err
			}
			set := func(c *Config) { c.Shell.Args = list }
			set(cfg)
			return updateCfgFile(set)
		},
	},
	{
		ID:      "contextPaths",
		Group:   GroupFiles,
		Name:    "Project instruction files",
		Layman:  "Filenames the program looks for in each of your folders and feeds to the AI as project instructions. Removing entries means less is sent every turn, but the AI then knows less about your conventions.",
		Kind:    KindStringList,
		Default: defaultContextPaths,
		Get: func() any {
			if cfg == nil {
				return []string{}
			}
			return cfg.ContextPaths
		},
		Set: func(v any) error {
			list, err := asStringList(v)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				return fmt.Errorf("cannot be empty; the AI would get no project instructions at all")
			}
			set := func(c *Config) { c.ContextPaths = list }
			set(cfg)
			return updateCfgFile(set)
		},
	},
	{
		ID:     "askWorkspaceOnStartup",
		Group:  GroupFiles,
		Name:   "Ask which folder to work in at startup",
		Layman: "Whether the program asks you to pick a working folder each time it starts.",
		WhenOn: "you pick the folder on launch, so clicking the desktop icon does not scope the AI to your whole home folder",
		// Stored inverted (SkipWorkspacePrompt) because omitempty drops a false
		// and an "ask" bool could then never be saved as off. The row reads the
		// way the user thinks about it; only the storage is negative.
		WhenOff: "it starts silently in the folder you last used, and you change it with /cd",
		Kind:    KindBool,
		Default: true,
		Get:     func() any { return cfg == nil || !cfg.SkipWorkspacePrompt },
		Set: func(v any) error {
			b, err := asBool(v)
			if err != nil {
				return err
			}
			return SetSkipWorkspacePrompt(!b)
		},
	},
	{
		ID:      "data.directory",
		Group:   GroupFiles,
		Name:    "Program data folder",
		Layman:  "Where this program keeps its database and logs. One folder for the whole machine, not one per project.",
		Kind:    KindString,
		Default: DataBase(),
		Restart: true,
		Get: func() any {
			if cfg == nil {
				return ""
			}
			return cfg.Data.Directory
		},
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			s = strings.TrimSpace(s)
			if s == "" {
				return fmt.Errorf("cannot be empty")
			}
			set := func(c *Config) { c.Data.Directory = s }
			set(cfg)
			return updateCfgFile(set)
		},
	},

	// ─── Appearance ──────────────────────────────────────────────────
	{
		ID:      "tui.theme",
		Group:   GroupAppearance,
		Name:    "Colour scheme",
		Layman:  "How the program looks. Cosmetic only — changes nothing about what the AI does or costs.",
		Kind:    KindEnum,
		Default: "opencode",
		Options: nil, // filled at runtime from the theme registry
		Get: func() any {
			if cfg == nil {
				return ""
			}
			return cfg.TUI.Theme
		},
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			return UpdateTheme(strings.TrimSpace(s))
		},
	},

	// ─── Diagnostics ─────────────────────────────────────────────────
	{
		ID:      "debug",
		Group:   GroupDiagnostics,
		Name:    "Debug logging",
		Layman:  "Extra logging to help diagnose problems with this program itself.",
		WhenOn:  "writes a detailed log; it can get large",
		WhenOff: "normal logging only",
		Kind:    KindBool,
		Default: false,
		Restart: true,
		Get:     func() any { return cfg != nil && cfg.Debug },
		Set: func(v any) error {
			b, err := asBool(v)
			if err != nil {
				return err
			}
			set := func(c *Config) { c.Debug = b }
			set(cfg)
			return updateCfgFile(set)
		},
	},
	{
		ID:      "debugLSP",
		Group:   GroupDiagnostics,
		Name:    "Language-server logging",
		Layman:  "Whether the raw chatter from language servers is shown live in your terminal.",
		WhenOn:  "very noisy, and it will interleave with the interface — only for chasing a language-server bug",
		WhenOff: "that chatter goes to the log file instead of over the top of the interface",
		Kind:    KindBool,
		Default: false,
		Get:     func() any { return cfg != nil && cfg.DebugLSP },
		Set: func(v any) error {
			b, err := asBool(v)
			if err != nil {
				return err
			}
			set := func(c *Config) { c.DebugLSP = b }
			set(cfg)
			return updateCfgFile(set)
		},
	},
}

func asStringList(v any) ([]string, error) {
	switch l := v.(type) {
	case []string:
		return l, nil
	case string:
		// Comma-separated, which is how the dialog's single-line editor gives it.
		var out []string
		for _, part := range strings.Split(l, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("expected a list, got %T", v)
}

// SettingByID finds a registered setting, or nil.
func SettingByID(id string) *Setting {
	for i := range Settings {
		if Settings[i].ID == id {
			return &Settings[i]
		}
	}
	return nil
}

// SetThemeOptions fills the theme row's Options from the theme registry. The
// theme package imports config, so config cannot import it back — the list is
// pushed in at startup, the same inversion used for prompt sections.
func SetThemeOptions(names []string) {
	if s := SettingByID("tui.theme"); s != nil {
		s.Options = names
	}
}

// SettingsChangedFromDefault counts rows that differ from their shipped value.
// /reset uses this to show what there is to undo before touching anything.
func SettingsChangedFromDefault() int {
	n := 0
	for i := range Settings {
		s := &Settings[i]
		if s.ReadOnly || s.Get == nil {
			continue
		}
		if !sameSettingValue(s.Get(), s.Default) {
			n++
		}
	}
	return n
}

// ResetAllSettings restores every editable setting to its shipped default.
func ResetAllSettings() error {
	var firstErr error
	for i := range Settings {
		s := &Settings[i]
		if s.ReadOnly || s.Set == nil || s.Default == nil {
			continue
		}
		if sameSettingValue(s.Get(), s.Default) {
			continue
		}
		if err := s.Set(s.Default); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func sameSettingValue(a, b any) bool {
	return FormatSettingValue(a) == FormatSettingValue(b)
}

// FormatSettingValue renders a value for display. Exported so the dialog and the
// generated docs agree on how a value looks.
// TildeHome rewrites the current user's home directory to "~".
//
// GORILLA OVERRIDE: v0.1.85 published `/home/gorilla/.local/share/gorilla-opencode`
// as the documented default for data.directory. That value is computed at
// package-init from whoever is running, so the generated docs bake in the build
// machine's username — a leak of the build environment, and wrong for every
// reader of the shipped file.
//
// It lives here, next to FormatSettingValue, so the doc generator and the
// staleness test share ONE owner for the rule. When they each had their own
// idea of how a path is displayed, the test asserted the raw path and the
// generator wrote the tilde form, and the doc could never satisfy both.
func TildeHome(s string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return s
	}
	return strings.ReplaceAll(s, home, "~")
}

func FormatSettingValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case bool:
		if t {
			return "ON"
		}
		return "OFF"
	case []string:
		return strings.Join(t, ", ")
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case string:
		return t
	}
	return fmt.Sprintf("%v", v)
}

// SettingRange renders the accepted values for display: the min/max, the preset
// ladder, or the enum options. This is the "what it accepts" half of the
// requirement and belongs on screen, not in a help page.
func SettingRange(s *Setting) string {
	switch s.Kind {
	case KindBool:
		return "on or off"
	case KindInt:
		return fmt.Sprintf("%d-%d", s.Min, s.Max)
	case KindLadder:
		parts := make([]string, 0, len(s.Presets))
		for _, p := range s.Presets {
			switch {
			case s.ID == "subagents.max" && p == SubAgentsUnlimited:
				parts = append(parts, "-1=unlimited")
			case s.ID == "subagents.max" && p == SubAgentsNuclear:
				parts = append(parts, "0=off")
			case s.ID == "ratelimit.rpm" && p == 0:
				parts = append(parts, "0=unlimited")
			default:
				parts = append(parts, fmt.Sprintf("%d", p))
			}
		}
		return strings.Join(parts, " ")
	case KindEnum:
		if len(s.Options) == 0 {
			return "(no options registered)"
		}
		return strings.Join(s.Options, " / ")
	case KindString:
		return "text"
	case KindStringList:
		return "comma-separated list"
	}
	return ""
}

// ModelOwnedElsewhere reports settings deliberately NOT in this registry, with
// the command that does own them. Shown as read-only pointers so /settings is a
// complete inventory without becoming a second source of truth.
var ModelOwnedElsewhere = []struct {
	Name, Owner, Why string
}{
	{"AI model", "/model", "which model each agent uses"},
	{"Provider keys and endpoints", "/connect", "API keys, local endpoints, Google login"},
	{"Tools and prompt sections", "/context", "what rides every turn, with token costs"},
	{"System prompt text", "/prompts", "edit or reset the prompts themselves"},
	{"Workspace roots", "/add-dir", "which directories are part of the workspace"},
}

// CurrentModelSummary is a read-only line for the /settings inventory.
func CurrentModelSummary() string {
	if cfg == nil {
		return "(config not loaded)"
	}
	a, ok := cfg.Agents[AgentCoder]
	if !ok || a.Model == "" {
		return "(no model selected)"
	}
	if m, found := models.SupportedModels[a.Model]; found && m.Name != "" {
		return fmt.Sprintf("%s (%s)", m.Name, a.Model)
	}
	return string(a.Model)
}
