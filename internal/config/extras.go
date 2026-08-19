package config

import (
	"fmt"
	"strings"
)

// GORILLA OVERRIDE: "extras" are the optional behaviours that make the agent's
// working visible — its reasoning, its tool calls, the times things happened.
//
// They live in their own registry rather than in LoadoutComponents, which looks
// similar but measures a different thing. A loadout entry's Tokens field is what
// that component adds to the PROMPT on every turn, and /context sums those into a
// context-size budget. Extras do not affect prompt size at all: one of them makes
// the model GENERATE more, and the rest only change what is displayed from data
// already paid for. Folding them into the same list would put numbers into that
// budget that do not belong there.
//
// The reason this exists as a user-facing choice: reasoning is not free, and the
// program should say so plainly rather than quietly spending on the user's behalf.

// ExtraCost describes what switching an extra ON actually costs.
type ExtraCost int

const (
	// CostFree changes only what is displayed. The data has already been
	// generated and paid for; hiding it saves nothing whatsoever.
	CostFree ExtraCost = iota
	// CostGeneration makes the model produce additional output. This is the one
	// that spends something.
	CostGeneration
)

// Extra is one switchable behaviour.
//
// GORILLA OVERRIDE on the ID format: NO DOTS. These IDs are keys of the Extras
// map in config.json, and viper reads a dotted key as NESTING — so
// "extra.timestamps.show" was unmarshalled as
// {extra: {timestamps: {show: true}}} and then failed to decode into
// map[string]bool, breaking config.Load for the whole application. The loadout
// registry gets away with dotted IDs only because its state lives in a separate
// loadout.json written directly, never through viper.
type Extra struct {
	ID      string
	Name    string
	What    string // what you get when it is ON
	Cost    ExtraCost
	Default bool
}

// Extras is the registry.
//
// Note which of these actually costs anything: exactly one. It would be easy —
// and wrong — to present all four as "extras that increase your bill". Showing a
// tool call costs nothing, because the call already happened and was already
// billed; switching it off would lose the forensic record for no saving. Being
// straight about that is the point.
var Extras = []Extra{
	{
		ID:   "extras-reasoning-generate",
		Name: "Ask the model to think out loud",
		What: "the model works through the problem step by step and you can read how it reached its answer",
		Cost: CostGeneration,
		// OFF by default. Something that spends more should be chosen, not
		// inherited from a default the user never saw.
		Default: false,
	},
	{
		ID:      "extras-reasoning-show",
		Name:    "Show that thinking on screen",
		What:    "the reasoning appears in the conversation instead of only in an export",
		Cost:    CostFree,
		Default: true,
	},
	{
		ID:      "extras-toolcalls-show",
		Name:    "Show tool calls and their results",
		What:    "you see each command or file operation the agent runs, and what came back",
		Cost:    CostFree,
		Default: true,
	},
	{
		ID:      "extras-timestamps-show",
		Name:    "Show a time on every message",
		What:    "you can tell exactly when something happened and build a timeline",
		Cost:    CostFree,
		Default: true,
	},
}

// ExtraByID looks up a registry entry.
func ExtraByID(id string) (Extra, bool) {
	for _, e := range Extras {
		if e.ID == id {
			return e, true
		}
	}
	return Extra{}, false
}

// ExtraEnabled reports whether an extra is on.
//
// Absent means "use the registry default" — deliberately NOT the loadout
// convention of absent-means-enabled, because one of these defaults to OFF and
// that rule would silently switch on the only setting that spends money.
func ExtraEnabled(id string) bool {
	if cfg != nil {
		if v, ok := cfg.Extras[id]; ok {
			return v
		}
	}
	e, ok := ExtraByID(id)
	return ok && e.Default
}

// SetExtra records a choice and persists it.
func SetExtra(id string, enabled bool) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if _, ok := ExtraByID(id); !ok {
		return fmt.Errorf("unknown extra %q", id)
	}
	if cfg.Extras == nil {
		cfg.Extras = map[string]bool{}
	}
	cfg.Extras[id] = enabled

	return updateCfgFile(func(c *Config) {
		if c.Extras == nil {
			c.Extras = map[string]bool{}
		}
		c.Extras[id] = enabled
	})
}

// ExtrasChoiceMade reports whether the user has been shown the explanation and
// made a decision, so the first-run prompt appears exactly once.
func ExtrasChoiceMade() bool {
	return cfg != nil && cfg.ExtrasChoiceMade
}

// MarkExtrasChoiceMade records that the question has been asked and answered.
func MarkExtrasChoiceMade() error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	cfg.ExtrasChoiceMade = true
	return updateCfgFile(func(c *Config) { c.ExtrasChoiceMade = true })
}

// ── What we tell the user, in one place ──────────────────────────────────────
//
// Every surface that mentions the cost of reasoning uses these strings, so the
// first-run prompt, /context and /settings cannot drift into saying different
// things about the same setting.

// ExtraCostLabel is the short marker shown beside a row.
func ExtraCostLabel(c ExtraCost) string {
	switch c {
	case CostGeneration:
		return "costs extra"
	default:
		return "free"
	}
}

// ExtraCostExplanation is the honest account of what switching this on costs.
//
// It deliberately quotes NO price. We hold no pricing data for any local model —
// every entry in the bundled metadata has a zero out-cost — and the endpoints
// actually in use here (an NVIDIA NIM free tier, a local Ollama) bill no money at
// all. Stating "this will cost you $X" would be false on this machine, and a
// warning that is provably wrong teaches people to ignore the ones that are not.
//
// So it describes the real resource cost instead, which is true everywhere.
func ExtraCostExplanation(c ExtraCost) string {
	if c == CostFree {
		return "Free. This only changes what is shown to you. The work has already " +
			"been done and paid for, so turning it off saves you nothing at all — " +
			"you would just see less."
	}
	return strings.Join([]string{
		"NOT free. Thinking out loud means the model writes a great deal more " +
			"text than the answer alone, often several times as much.",
		"",
		"What that costs depends on where the model runs:",
		"  * a provider that charges per token — real money, more of it",
		"  * a free tier such as NVIDIA NIM — no money, but you use up your " +
			"allowance faster and may start hitting request limits",
		"  * a model on your own machine such as Ollama — no money, but noticeably " +
			"more CPU, more heat and more battery",
		"",
		"In every case, replies take longer.",
		"",
		"No price is shown because none is published for the models configured " +
			"here. A made-up figure would be worse than none.",
	}, "\n")
}

// ExtrasSummary is the one-line state used in status lines and cross-references.
func ExtrasSummary() string {
	on, off := 0, 0
	paidOn := false
	for _, e := range Extras {
		if ExtraEnabled(e.ID) {
			on++
			if e.Cost == CostGeneration {
				paidOn = true
			}
		} else {
			off++
		}
	}
	s := fmt.Sprintf("%d on, %d off", on, off)
	if paidOn {
		s += " — thinking is ON and uses extra tokens"
	}
	return s
}

// init registers one /settings row per extra.
//
// GORILLA OVERRIDE: generated from the registry above rather than written out a
// second time in settings.go. The whole point of this feature is that the cost is
// stated honestly wherever the user meets it, and three hand-maintained copies of
// that wording would drift — one of them would end up saying something we have
// measured to be false. Settings is a package-level var, so appending from init
// is safe: var initialisation completes before any init runs.
func init() {
	for _, e := range Extras {
		e := e // capture per iteration; the closures below outlive the loop

		row := Setting{
			ID:      e.ID,
			Group:   GroupExtras,
			Name:    e.Name,
			Layman:  e.What + ".",
			Kind:    KindBool,
			Default: e.Default,
			Get:     func() any { return ExtraEnabled(e.ID) },
			Set: func(v any) error {
				b, err := asBool(v)
				if err != nil {
					return err
				}
				return SetExtra(e.ID, b)
			},
		}

		// Both directions spelled out, and the cost stated on the row that causes
		// it — not buried in a first-run screen the user saw once.
		switch e.Cost {
		case CostGeneration:
			row.WhenOn = "you can read how the model reached its answer — but it writes " +
				"a lot more than the answer alone, so this uses more of your allowance " +
				"(or more CPU on a local model) and every reply takes longer"
			row.WhenOff = "you only get the answer, and nothing extra is generated or spent"
		default:
			row.WhenOn = "it is shown to you, at no extra cost"
			row.WhenOff = "it is hidden — but this saves you NOTHING, because the work " +
				"has already been done and paid for; you simply see less"
		}
		Settings = append(Settings, row)
	}
}

// ── Which interface to start ────────────────────────────────────────────────

const (
	// InterfaceFull is the normal screen-drawing interface.
	InterfaceFull = "full"
	// InterfacePlain is ordinary terminal output, so the session can be selected
	// and copied with the terminal's own keys.
	InterfacePlain = "plain"
)

// InterfaceMode reports which interface to start.
//
// GORILLA OVERRIDE: persisted rather than flag-only. The desktop entry is
// `Exec=gorilla-opencode launch` with no arguments, so anyone who starts the
// program by clicking its icon can never reach a flag-only mode. An explicit
// --plain still wins for a one-off; this is the standing preference.
func InterfaceMode() string {
	if cfg != nil && cfg.Interface == InterfacePlain {
		return InterfacePlain
	}
	return InterfaceFull
}

// SetInterfaceMode records the preference and persists it.
func SetInterfaceMode(mode string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	switch mode {
	case InterfaceFull, InterfacePlain:
	default:
		return fmt.Errorf("unknown interface %q — use %q or %q", mode, InterfaceFull, InterfacePlain)
	}
	cfg.Interface = mode
	return updateCfgFile(func(c *Config) { c.Interface = mode })
}

func init() {
	Settings = append(Settings, Setting{
		ID:     "interface",
		Group:  GroupExtras,
		Name:   "Which interface to start",
		Layman: "\"full\" draws panels and dialogs on screen. \"plain\" writes ordinary text instead, so you can select and copy the whole conversation with your terminal's own keys — which the full interface cannot do, because it draws on a screen buffer your terminal keeps no history of.",
		Kind:   KindEnum,
		// full stays the default: it is what everyone already has, and plain
		// carries fewer commands.
		Default: InterfaceFull,
		Options: []string{InterfaceFull, InterfacePlain},
		// Choosing an interface cannot take effect mid-session — the renderer is
		// already running — and saying so beats appearing to do nothing.
		Restart: true,
		Get:     func() any { return InterfaceMode() },
		Set: func(v any) error {
			s, err := asString(v)
			if err != nil {
				return err
			}
			return SetInterfaceMode(strings.ToLower(strings.TrimSpace(s)))
		},
	})
}

// ── Alternate screen ────────────────────────────────────────────────────────

// AlternateScreenEnabled reports whether the full interface draws on the
// terminal's alternate screen buffer.
//
// GORILLA OVERRIDE: OFF by default, and this is the single most consequential
// default in the program.
//
// The alternate screen is a scratch buffer with no history: your terminal keeps
// no scrollback for it, so nothing drawn there can be scrolled back to, selected
// or copied. Everything that felt missing followed from that one choice — the
// wheel did nothing useful, Select-All returned an empty selection, and copying
// an hours-old reply was impossible. The workarounds people reach for (drawing a
// scrollbar, shipping a clipboard helper, the OSC 52 escape) are all compensation
// for a buffer we chose to use.
//
// Measured, not assumed (internal/tui/inline/scrollback_test.go):
//
//   - Outside the alternate screen, tea.Println puts a line into the terminal's
//     real output exactly ONCE — that is history the terminal owns.
//   - Inside it, the same call is discarded: 0 of 3 lines reach the output.
//
// So with this OFF, finished messages are printed into your scrollback and only
// the editor and status bar are redrawn in place. The wheel scrolls because the
// TERMINAL is scrolling, and copying works because the text is really there.
//
// Turning it ON restores the previous behaviour: one full-screen frame, panels
// that never scroll away, no flicker while streaming — and no history, no
// selection, no copy. Google made the same call in Gemini CLI, whose
// useAlternateBuffer setting also defaults to false.
func AlternateScreenEnabled() bool { return cfg != nil && cfg.AlternateScreen }

// SetAlternateScreen records the preference and persists it.
func SetAlternateScreen(on bool) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	cfg.AlternateScreen = on
	return updateCfgFile(func(c *Config) { c.AlternateScreen = on })
}

func init() {
	Settings = append(Settings, Setting{
		ID:      "alternateScreen",
		Group:   GroupExtras,
		Name:    "Draw on a separate screen",
		Layman:  "Whether the interface takes over the screen on a scratch buffer your terminal keeps no history of. Leaving this OFF is what lets you scroll back through the whole conversation with the wheel, select it with Ctrl+A and copy it with Ctrl+Shift+C — because the text is really in your terminal, not painted over it.",
		WhenOn:  "panels stay put and streaming never flickers, but the conversation cannot be scrolled back to, selected or copied — the terminal keeps no history of this buffer",
		WhenOff: "finished messages go into your terminal's own scrollback, so the wheel, Select-All and copy all work; only the prompt and status line are redrawn in place",
		Kind:    KindBool,
		Default: false,
		// The buffer is entered once, as the program starts. Switching mid-session
		// would mean tearing down the renderer, and pretending otherwise would look
		// like the setting had done nothing.
		Restart: true,
		Get:     func() any { return AlternateScreenEnabled() },
		Set: func(v any) error {
			b, err := asBool(v)
			if err != nil {
				return err
			}
			return SetAlternateScreen(b)
		},
	})
}

// ── Mouse reporting ─────────────────────────────────────────────────────────

// MouseWheelEnabled reports whether the program asks the terminal for mouse events.
//
// GORILLA OVERRIDE: off by default, and that is the whole point.
//
// Asking for mouse events means the TERMINAL stops handling the mouse itself, so
// click-and-drag no longer selects text — in most terminals you must hold Shift to
// get it back, which nobody discovers by accident. Worse, the mode in use
// (bubbletea offers only 1002 "cell motion" and 1003 "all motion") reports one event
// per cell crossed, so a single drag across a wide terminal fires hundreds. That
// flood is documented in tui.go as having stalled the event loop badly enough that
// bubbletea's input parser fell behind and leaked raw escape codes into the editor —
// a literal "[<32;71;41M" appearing where the user was typing.
//
// The trade, stated plainly: with this OFF you scroll with PageUp and PageDown
// instead of the wheel, and your mouse selects text as it does in any other program.
// With it ON the wheel scrolls the conversation and text selection needs Shift.
func MouseWheelEnabled() bool { return cfg != nil && cfg.MouseWheel }

// RequestMouseEvents reports whether the program should actually ask the terminal
// to report mouse events, as opposed to whether the user asked for the wheel.
//
// They differ, and the difference matters: without the alternate screen the
// TERMINAL already scrolls the conversation with the wheel, because the
// conversation is in its scrollback. Asking for mouse events there would take
// that working scroll away, hand us events for a viewport that no longer exists,
// and break drag-to-select for nothing in return. So the preference is honoured
// only where it can buy anything.
func RequestMouseEvents() bool { return AlternateScreenEnabled() && MouseWheelEnabled() }

// SetMouseWheel records the preference and persists it.
func SetMouseWheel(on bool) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	cfg.MouseWheel = on
	return updateCfgFile(func(c *Config) { c.MouseWheel = on })
}

func init() {
	Settings = append(Settings, Setting{
		ID:      "mouseWheel",
		Group:   GroupExtras,
		Name:    "Mouse wheel scrolling",
		Layman:  "Only does anything when \"Draw on a separate screen\" is ON. With that off, your terminal already scrolls the conversation with the wheel, and asking for mouse events would take that away for nothing. When it does apply, the cost is easy to miss: the terminal hands the mouse to this program, so click-and-drag stops selecting text unless you hold Shift.",
		WhenOn:  "on a separate screen, the wheel scrolls but selecting text needs Shift held, and a long drag can briefly stutter the display; ignored otherwise",
		WhenOff: "your mouse selects text exactly as it does anywhere else; scroll with your terminal's own wheel, or with PageUp and PageDown if you are drawing on a separate screen",
		Kind:    KindBool,
		Default: false,
		Restart: true, // mouse mode is requested once, when the program starts
		Get:     func() any { return MouseWheelEnabled() },
		Set: func(v any) error {
			b, err := asBool(v)
			if err != nil {
				return err
			}
			return SetMouseWheel(b)
		},
	})
}
