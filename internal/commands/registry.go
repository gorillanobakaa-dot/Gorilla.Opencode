// GORILLA OVERRIDE: this package did not exist upstream. It is the single
// source of truth for what every slash command does, in plain language.
//
// Why it exists, in the user's words: "at this stage we have made so many
// modifications, we have introduced so many features that the normal user will
// get confused. I MYSELF knowing what's in here am getting confused."
//
// The rule that keeps this honest is the drift test in registry_test.go: it reads
// the dispatch switch in internal/tui/tui.go and fails if a command can be typed
// but is not documented here, or is documented here but cannot be typed. A
// reference nobody maintains is worse than none, because it is believed.
//
// Descriptions are written for someone who has never read the code:
// what it does, and what it costs or changes. No jargon, no internal names.
package commands

import (
	"sort"
	"strings"
)

// Group buckets commands so the reference reads as a map rather than a list.
type Group string

const (
	GroupSession   Group = "Your conversation"
	GroupWhere     Group = "Which files the AI can see"
	GroupModels    Group = "Models and accounts"
	GroupTuning    Group = "Cost, speed and behaviour"
	GroupHelpers   Group = "Background helpers"
	GroupReference Group = "Help"
)

// GroupOrder is display order. Most-reached-for first.
var GroupOrder = []Group{
	GroupSession, GroupWhere, GroupModels, GroupTuning, GroupHelpers, GroupReference,
}

// Command is one slash command, described for a human.
type Command struct {
	// Name is what the user types, without the slash.
	Name string
	// Aliases are other spellings that reach the same place.
	Aliases []string
	Group   Group
	// Summary is a single short line — what it does. Shown in the list.
	Summary string
	// Detail is the "why would I use this" paragraph, and any cost or
	// consequence worth knowing before pressing it.
	Detail string
	// Args describes what may follow the command, or "" if it takes none.
	Args string
}

// All is the registry. Order within a group is display order.
var All = []Command{
	// ─── Your conversation ───────────────────────────────────────────
	{
		Name:    "clear",
		Aliases: []string{"new"},
		Group:   GroupSession,
		Summary: "Start a fresh conversation.",
		Detail: "The AI forgets everything said so far. Use this when you move to a " +
			"different task — a long conversation costs more on every message, " +
			"because the whole history is sent each time.",
	},
	{
		Name:    "plain",
		Aliases: []string{"copy", "copyable"},
		Group:   GroupSession,
		Summary: "Switch to the interface you can select and copy.",
		Detail: "This interface draws on a screen your terminal keeps no history of, " +
			"which is why Ctrl+A selects nothing here. Plain mode writes ordinary " +
			"text instead, so you can select, copy and search the whole conversation " +
			"with your terminal's own keys. It has fewer commands. This takes effect " +
			"next time you start the program \u2014 the current screen is already " +
			"running. Switch back in /settings, or right-click the desktop icon for " +
			"a one-off.",
	},
	{
		Name:    "export",
		Group:   GroupSession,
		Summary: "Save this conversation to a file.",
		Detail: "Asks you which folder and what to call it, then writes the whole " +
			"session out as text: every message with its date and time, how far " +
			"into the session it happened, which model answered, the model's " +
			"reasoning, and every tool it ran with the result that came back — " +
			"including the ones that failed. Use it when you need to know exactly " +
			"what happened and when.",
	},

	// ─── Which files the AI can see ──────────────────────────────────
	{
		Name:    "cd",
		Group:   GroupWhere,
		Args:    "[folder]",
		Summary: "Switch to working in one folder.",
		Detail: "This is the important one. The AI searches and reads inside your " +
			"working folder, so pointing it at one project instead of your whole " +
			"home folder is the difference between a handful of files and a " +
			"million. Fewer files means faster answers and far less of your quota " +
			"spent. Typing it with no folder opens a chooser.",
	},
	{
		Name:    "add-dir",
		Aliases: []string{"adddir", "dirs", "roots"},
		Group:   GroupWhere,
		Summary: "Work in more than one folder at once.",
		Detail: "Adds a second (or third) folder alongside your main one — useful " +
			"when a change spans two projects. Each folder you add is more for " +
			"the AI to search, so add only what you need. To move to a single " +
			"folder instead of adding one, use /cd.",
	},

	// ─── Models and accounts ─────────────────────────────────────────
	{
		Name:    "model",
		Aliases: []string{"models"},
		Group:   GroupModels,
		Summary: "Choose which AI answers you.",
		Detail: "Bigger models are better at hard problems and cost more; small " +
			"ones are cheap and fast. Models running on your own machine cost " +
			"nothing to use. Each is listed with the connection it comes from.",
	},
	{
		Name:    "connect",
		Aliases: []string{"connections", "providers", "provider", "switch"},
		Group:   GroupModels,
		Summary: "Add or manage your AI accounts and keys.",
		Detail: "Where you paste an API key, add a local server such as Ollama or " +
			"NVIDIA, or turn a connection off without deleting it. Adding a " +
			"connection makes its models appear in /model. The list shows the " +
			"servers you have added as well as the ones on offer; press d to " +
			"remove one of yours for good, or space to just switch it off.",
	},
	{
		Name:    "login",
		Group:   GroupModels,
		Summary: "Sign in with your Google account.",
		Detail: "Opens your browser. Lets you use Google's models through your " +
			"account instead of pasting an API key.",
	},
	{
		Name:    "logout",
		Group:   GroupModels,
		Summary: "Sign out of your Google account.",
		Detail:  "Removes the stored sign-in. Any API keys you typed are untouched.",
	},

	// ─── Cost, speed and behaviour ───────────────────────────────────
	{
		Name:    "context",
		Aliases: []string{"loadout", "tokens"},
		Group:   GroupTuning,
		Summary: "Turn features off to spend less.",
		Detail: "Everything the AI can do is described to it on every single " +
			"message, and you pay for that description each time. Switching off " +
			"what you are not using makes every message cheaper. This is also " +
			"where you turn language servers off — press L for all of them at " +
			"once, or pick them one by one.",
	},
	{
		Name:    "settings",
		Aliases: []string{"config", "prefs"},
		Group:   GroupTuning,
		Summary: "Every option, what it accepts, and its default.",
		Detail: "One list of every setting with a plain-language description, the " +
			"range it accepts and what it shipped as, so you can always get back " +
			"to a known state.",
	},
	{
		Name:    "prompts",
		Aliases: []string{"prompt"},
		Group:   GroupTuning,
		Summary: "Read or change the AI's standing instructions.",
		Detail: "The instructions the AI is given before it sees your message — " +
			"how careful to be, how to report what it did. You can switch " +
			"sections off or rewrite them. Advanced: changing these changes how " +
			"the AI behaves everywhere.",
	},
	{
		Name:    "reset",
		Aliases: []string{"defaults"},
		Group:   GroupTuning,
		Summary: "Put things back the way they shipped.",
		Detail: "Undoes your changes, in whichever area you pick — settings, " +
			"instructions, or feature switches. Use this when something is " +
			"behaving oddly and you no longer remember what you changed.",
	},

	// ─── Background helpers ──────────────────────────────────────────
	{
		Name:    "tasks",
		Aliases: []string{"task", "agents", "kill"},
		Group:   GroupHelpers,
		Summary: "See and stop background helpers.",
		Detail: "The AI can start helpers to work on parts of a job. Each one costs " +
			"quota of its own, so this is where you check what is running and " +
			"stop anything you do not want.",
	},

	// ─── Help ────────────────────────────────────────────────────────
	{
		Name:    "help",
		Aliases: []string{"commands", "?"},
		Group:   GroupReference,
		Summary: "This list.",
		Detail:  "Every command, what it does, and what it costs or changes.",
	},
}

// ByName returns the command reached by a name or alias, or nil.
func ByName(name string) *Command {
	name = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(name)), "/")
	for i := range All {
		if All[i].Name == name {
			return &All[i]
		}
		for _, a := range All[i].Aliases {
			if a == name {
				return &All[i]
			}
		}
	}
	return nil
}

// InGroup returns the commands in a group, in registry order.
func InGroup(g Group) []*Command {
	var out []*Command
	for i := range All {
		if All[i].Group == g {
			out = append(out, &All[i])
		}
	}
	return out
}

// Names returns every name and alias, sorted. Used for suggestions.
func Names() []string {
	var out []string
	for _, c := range All {
		out = append(out, c.Name)
		out = append(out, c.Aliases...)
	}
	sort.Strings(out)
	return out
}

// Suggest returns up to n command names close to what was typed, so an unknown
// command can point somewhere useful instead of listing everything.
func Suggest(typed string, n int) []string {
	typed = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(typed), "/"))
	if typed == "" {
		return nil
	}
	var pre, sub []string
	for _, name := range Names() {
		switch {
		case strings.HasPrefix(name, typed):
			pre = append(pre, name)
		case strings.Contains(name, typed) || strings.Contains(typed, name):
			sub = append(sub, name)
		}
	}
	out := append(pre, sub...)

	// A typo is neither a prefix nor a substring — "modl" for "model" shares no
	// run of characters long enough to match — so fall back to edit distance.
	// Without this the near-miss case, which is the common one, suggested nothing.
	if len(out) == 0 {
		limit := 2
		if len(typed) <= 4 {
			limit = 1 // on a short word, 2 edits reaches almost anything
		}
		for _, name := range Names() {
			if editDistance(typed, name) <= limit {
				out = append(out, name)
			}
		}
	}

	if len(out) > n {
		out = out[:n]
	}
	return out
}

// editDistance is Levenshtein, iterative with two rows. Command names are a
// handful of characters, so the simple form is the right one.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
