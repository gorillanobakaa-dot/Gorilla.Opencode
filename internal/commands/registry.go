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
		Name:    "research",
		Group:   GroupHelpers,
		Args:    "<question>",
		Summary: "Send helper agents to investigate, each on one angle.",
		Detail: "The everyday investigation tool: four to ten helpers, each given ONE " +
			"angle, collecting with the same intelligence-cycle discipline as /osint " +
			"but in a single pass. A verifier attacks their conclusions. Each helper " +
			"is a full model session, so the dialog shows the cost before anything " +
			"starts. Worth it when being wrong is expensive; waste when a single " +
			"search would answer. For the full professional dossier — rounds, graded " +
			"sources, the works — see /osint.",
	},
	{
		Name:    "osint",
		Aliases: []string{"dossier"},
		Group:   GroupHelpers,
		Args:    "<question>",
		Summary: "The serious one. Professional dossier. Burns real money.",
		Detail: "A professional intelligence assessment, not a chat answer: plans your " +
			"question into sub-questions, collects from hundreds of free primary " +
			"sources (scholarly APIs, SEC filings, World Bank, humanitarian data, " +
			"global news), grades every claim on two axes like a real intelligence " +
			"shop, hunts its own gaps, and tells you plainly what it could NOT " +
			"establish. OFF by default — arm it in /context. Every run starts with a " +
			"warning showing the burn rate in money, because 4-10 helpers is 4-10 " +
			"full model sessions. Type /osint alone for the full explanation page.\n\n" +
			"/osint --recover writes up a run that collected its findings but never " +
			"produced the dossier — the usual outcome when a connection drops or the " +
			"model runs out of room at the very last step. It costs nothing to look: " +
			"the findings are already on disk and in the local store, and it lists " +
			"every past run so you can pick one. The write-up happens in a fresh " +
			"conversation carrying only those findings, which is exactly why it " +
			"succeeds where the original run ran out of room. Nothing is collected " +
			"again and no helpers are sent out.",
	},
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
		Name:    "review",
		Aliases: []string{"audit", "codereview"},
		Group:   GroupHelpers,
		Args:    "[folder or file]",
		Summary: "Run 30 real analysers over your code and report honestly.",
		Detail: "A professional static-analysis and security review, built in. Point " +
			"it at a folder, a file, or your changes and it runs around thirty real " +
			"analysers — the ones that find memory errors, injection, leaked " +
			"secrets, unchecked errors — picking whichever suit the languages " +
			"actually present. C, C++, Go, Python, JavaScript, TypeScript, Rust, " +
			"shell and more.\n\n" +
			"With no arguments it reviews your current folder. Add a path for " +
			"somewhere else. The tools live inside the program; nothing is " +
			"downloaded when you run it.\n\n" +
			"**It tells you what did NOT run.** That is the part that matters. Those " +
			"thirty analysers have to be installed on your machine, and if they are " +
			"missing they simply find nothing — which looks exactly like a clean " +
			"report. So the answer always starts with which tools ran, which are " +
			"missing, and which failed; and if none of them are installed it refuses " +
			"to run at all rather than hand you a reassuring blank.\n\n" +
			"It also flags every line that two or more DIFFERENT tools complained " +
			"about independently. Those are the ones worth reading first.\n\n" +
			"What it cannot do: find wrong logic, a broken assumption, or an error " +
			"quietly ignored. No static tool can. This is half a review and it says " +
			"so — the AI still has to read the code, and should tell you it did.",
	},
	{
		Name:    "resume",
		Aliases: []string{"continue", "handoff"},
		Group:   GroupSession,
		Summary: "Pick up work that stopped, or work another model started.",
		Detail: "For when the job is not finished: the power went, the connection " +
			"dropped, the model ran out of room, or you want a different model to " +
			"take over. It opens the same list as /sessions — press Ctrl+R on the " +
			"one you want.\n\n" +
			"This is NOT the same as reopening the conversation. Reopening loads " +
			"every message back in, which is right for a short chat and wrong for " +
			"a long job — and a long job is exactly what gets interrupted. Putting " +
			"a thousand messages back into a small model is what stopped the work " +
			"in the first place.\n\n" +
			"Instead it writes a short brief and starts a FRESH conversation with " +
			"just that: everything you asked for, word for word, in order; which " +
			"files were changed and which commands were run; what went wrong; and " +
			"where it stopped. The brief is built by the program itself, so it " +
			"costs nothing and cannot fail the way the original did.\n\n" +
			"It also says plainly what it does NOT know — whether any of the work " +
			"was correct, and whether it was finished. That matters most when you " +
			"hand it to a different model, which has no way to tell a finished job " +
			"from an abandoned one and would otherwise assume the best.",
	},
	{
		Name:    "sessions",
		Aliases: []string{"history"},
		Group:   GroupSession,
		Summary: "Every past conversation: search, reopen, save, erase.",
		Detail: "The one to reach for when a session ended without you — the power " +
			"went, the connection dropped, the machine was closed. It lists every " +
			"conversation you have ever had, newest first, with the date, how many " +
			"messages, and how much space it is taking up.\n\n" +
			"Type to search. It looks inside the messages as well as the titles, " +
			"because titles are generated and are often useless weeks later — you " +
			"remember the error you were chasing, not what the summary called that " +
			"day.\n\n" +
			"Enter reopens a conversation exactly where it stopped. Ctrl+E saves it " +
			"to a file — the whole thing: every message with its time, the model's " +
			"reasoning, and every tool it ran with the result that came back, " +
			"including the failures. That is what lets you work out afterwards how " +
			"something ended up published, or deleted.\n\n" +
			"Ctrl+D erases one for good, along with the helper sessions it spawned, " +
			"and returns the space to your disk — really returns it, and tells you " +
			"how much came back. Deleting alone frees nothing on this kind of " +
			"database; the file only shrinks when it is rebuilt, which is why this " +
			"reports the actual before-and-after. Ctrl+S sorts by size, so the " +
			"conversations worth deleting are the ones at the top.",
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
			"what happened and when.\n\n" +
			"This one saves the conversation you are in, and lets you name the " +
			"file. To save a conversation you have LEFT — after a power cut, or " +
			"from last week — use /sessions instead, which can also reach the " +
			"helper sessions a research run spawned.",
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
		Aliases: []string{"connections"},
		Group:   GroupModels,
		Summary: "Add or manage your AI accounts and keys.",
		Detail: "Where you paste an API key, add a local server such as Ollama or " +
			"NVIDIA, or turn a connection off without deleting it. Adding a " +
			"connection makes its models appear in /model. The list shows the " +
			"servers you have added as well as the ones on offer; press d to " +
			"remove one of yours for good, or space to just switch it off.",
	},
	{
		Name:    "providers",
		Aliases: []string{"provider", "switch"},
		Group:   GroupModels,
		Summary: "Switch to a different AI provider.",
		Detail: "Reopens the same picker you saw when the app started, with the " +
			"free options marked. Use it when the provider you chose does not " +
			"work — a key refused, a model not included in your plan — instead " +
			"of quitting and starting again. Esc leaves everything as it is.",
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
	{
		Name:    "usage",
		Group:   GroupModels,
		Summary: "Show your quota and balances — how many bananas are left.",
		Detail: "Shows what you have left to spend, in plain words. If you signed in " +
			"with the Antigravity free tier: how much of your weekly allowance " +
			"remains — Gemini has a separate pool from Claude and GPT-OSS — and " +
			"when each resets. If you have a DeepSeek or OpenRouter key: your " +
			"remaining balance there too. A one-line summary also appears on its " +
			"own at the start of each session.",
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
		Name:    "compact",
		Aliases: []string{"summarize", "summarise"},
		Group:   GroupSession,
		Summary: "Squeeze the conversation down so it keeps working.",
		Detail: "Every message you send carries the whole conversation with it, and " +
			"each model has a limit on how much it can hold. Approach that limit " +
			"and answers get worse, then stop. This writes a summary of everything " +
			"so far and continues from that instead, so the thread survives while " +
			"the bulk goes.\n\n" +
			"Use it when a long session starts to drift, or before starting a big " +
			"job in an old conversation. It costs one model call to write the " +
			"summary. Models with small windows need this often — the status bar " +
			"shows how full you are. It also runs by itself at 95% full if you " +
			"leave that setting on in /settings.",
	},
	{
		Name:    "init",
		Group:   GroupWhere,
		Summary: "Write the project notes file the AI reads first.",
		Detail: "Looks through the project and writes a short file describing how to " +
			"build, test and work in it, in the house style of this codebase. Every " +
			"future conversation in this folder reads that file before anything " +
			"else, so the AI starts knowing your conventions instead of guessing " +
			"at them. Run it once per project, and again after big changes.",
	},
	{
		Name:    "yolo",
		Aliases: []string{"auto", "autopilot", "goal"},
		Group:   GroupHelpers,
		Summary: "Approve everything for this conversation. No more prompts.",
		Detail: "Normally the program stops and asks before it edits a file, runs a " +
			"command, or reaches the internet. This turns that off for the " +
			"conversation you are in: every tool call is approved automatically, " +
			"including every research helper — which is the point, because a " +
			"ten-helper run otherwise asks you the same question ten times.\n\n" +
			"What you are handing over: file edits, shell commands and web access, " +
			"unattended. Use it when you have told the agent to get on with a job " +
			"and you do not want to babysit it. Do not use it in a folder you " +
			"cannot afford to have changed.\n\n" +
			"It lasts only as long as this conversation and is never written to " +
			"disk, so it cannot silently follow you into tomorrow. Type /yolo again " +
			"to turn it off. /tasks still stops helpers at any time.",
	},
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
