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
		// GORILLA FIX (2026-10-05): audit findings S8 and O1. This did not say that
		// every run writes its findings to disk, or where; it promised a verifier
		// that only runs from five helpers up while the dialog opens at four; and
		// it sold /osint as "rounds", which no code ran. The counts and the folder
		// name below are held to the code by TestResearchHelpStatesWhatTheCodeDoes.
		Detail: "The everyday investigation tool, for questions about code and this " +
			"machine: 4 to 10 helpers, each working ONE fixed lane (what already " +
			"exists here, prior art, the primary sources, what the target actually " +
			"demands; from 5 helpers up a verifier attacks the others' conclusions). " +
			"Each helper is a full model session, so the dialog shows the cost before " +
			"anything starts. Worth it when being wrong is expensive; waste when a " +
			"single search would answer.\n\n" +
			"Every run saves each helper's report to a file in your " +
			"Gorilla-OSINT-Dossiers folder (in Documents, or in your home folder if " +
			"there is no Documents), outside the working folder and readable by your " +
			"account only, before the answer is written. If the answer never gets " +
			"written, /osint --recover writes it up from that file.\n\n" +
			"/research help shows this page and starts nothing. If the research " +
			"helpers are switched off in /context the command says so instead of " +
			"opening the cost dialog. For questions about the world rather than this " +
			"machine — graded sources, the official record against the reporting — " +
			"see /osint.",
	},
	{
		Name:    "osint",
		Aliases: []string{"dossier"},
		Group:   GroupHelpers,
		Args:    "<question>",
		Summary: "The serious one. Professional dossier. Burns real money.",
		// GORILLA FIX (2026-10-05): audit findings O1, O2, O4 and O6. This said the
		// command "plans your question into sub-questions" (there is no planning
		// code), "collects from hundreds of free primary sources" (helpers are
		// handed a source atlas of tens), and "hunts its own gaps" (the model may
		// ask for one follow-up; nothing ran one). It also said nothing about the
		// question leaving the machine. It now describes what runs.
		Detail: "A professional intelligence assessment, not a chat answer. It works " +
			"your question in fixed lanes, one helper per lane: the official record, " +
			"the scholarly literature and data, the reporting, and the strongest case " +
			"against, with more lanes as you add helpers. No lane searches your own " +
			"machine. Helpers start from a built-in atlas of free sources (scholarly " +
			"indexes, official statistics, filings, humanitarian data, news) and the " +
			"open web, are told to grade every claim on two axes like a real " +
			"intelligence shop, and the dossier states plainly what could NOT be " +
			"established. The model may ask for ONE follow-up to close a gap; the " +
			"program refuses a second. OFF by default — arm it in /context; it also " +
			"needs the research helpers switched on there. Every run starts with a " +
			"warning showing the burn rate in money, because 4-10 helpers is 4-10 " +
			"full model sessions. Your question and the search terms go to your model " +
			"provider and to the search services; the dossier file stays on this " +
			"machine. Type /osint alone, or /osint help, for the full explanation page.\n\n" +
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
		Name:    "arsenal",
		Aliases: []string{"tools"},
		Group:   GroupTuning,
		Summary: "What this agent can do here, what it could do, and the cost.",
		// GORILLA FIX (2026-10-05), from the /arsenal audit (A3, A7, A8). Four
		// sentences here were not true. "About thirty tools" was a typed count
		// (the screen counts them itself, so the help no longer does).
		// "Transcribing audio" named a capability the list does not have: ffmpeg
		// pulls the audio OUT for transcription, nothing in it transcribes.
		// "Costs are measured ... by your own package manager" is false on
		// Windows, where Scoop reports no size. And "it prints the exact command
		// into the conversation" described the ? key only, which was printed
		// nowhere; the command is shown on the page's own install plan.
		Detail: "A map of capabilities, not a chat answer. It checks THIS machine for the tools " +
			"behind each one — reading PDFs and scanned pages, taking audio and video apart, " +
			"opening firmware and disk images, inspecting Android apps, looking up who owns a " +
			"domain — and shows what is already here, what is not, and the exact command to " +
			"get the rest. The screen counts them itself. Tools that cannot exist on your " +
			"operating system are left off the list rather than shown as missing.\n\n" +
			"It exists because a capability was found sitting unused: a model reported that it " +
			"could not read a screenshot while the tool that reads screenshots was already " +
			"installed on the same machine. Nobody stumbles onto binwalk or sleuthkit or " +
			"ssdeep unaided, and you cannot ask for a thing you do not know exists. The " +
			"barrier was never bandwidth — it was the map.\n\n" +
			"Slackware style: take a whole group, walk it item by item, or pick single tools. " +
			"Every entry says in plain words what it is FOR, what the agent gains, and — the " +
			"part most documentation leaves out — what will disappoint you about it. On Linux " +
			"(apt or pacman) the cost is measured against your own machine by your own package " +
			"manager, allowing for everything already installed, and is shown in minutes as " +
			"well as megabytes. On Windows (Scoop) the size cannot be known before installing, " +
			"and the screen says so instead of showing a figure.\n\n" +
			"It NEVER installs anything and never asks for your password. It shows the exact " +
			"command on its own install plan and you decide whether to run it. Nothing is sent " +
			"to the model unless you press ? there, which hands your selection to the model to " +
			"talk over, costs one model turn, and tells it not to run anything. Everything " +
			"listed is free and needs no account.",
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
		Aliases: []string{"copyable"},
		Group:   GroupSession,
		Summary: "Switch to the interface you can select and copy.",
		Detail: "This interface draws on a screen your terminal keeps no history of, " +
			"which is one reason the terminal's own Select-All cannot reach it. " +
			"Plain mode writes ordinary " +
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
		Args:    "[--quick|--security|--full] [--diff REF] [folder]",
		// GORILLA FIX (2026-10-05): this text made five claims the code did not
		// keep. "30 analysers" was typed and wrong. "Nothing is downloaded" was
		// false: semgrep, cargo audit and the Go tools all reach the network.
		// "--quick skips the security stages" was false: they ran. It never said
		// Python 3 is needed, and never said where the logs go — which was
		// inside the folder being reviewed. Each is now either true or gone;
		// see internal/llm/tools/review.go and its tests.
		//
		// GORILLA FIX (2026-10-06): --security and --full were the same run.
		// Both sent the toolkit's --deep flag; "reports only security findings"
		// was a filter on the summary, not a different review. --security is
		// now its own mode (security categories only, deep forced), and the
		// two lines below say exactly what each one runs.
		Summary: "Run real analysers over your code and report honestly.",
		Detail: "A professional static-analysis and security review, built in. Point " +
			"it at a folder, a file, or your changes and it runs the real " +
			"analysers installed on your machine — the ones that find memory " +
			"errors, injection, leaked secrets, unchecked errors — picking " +
			"whichever suit the languages actually present. C, C++, Go, Python, " +
			"JavaScript, TypeScript, Rust, shell and more.\n\n" +
			"With no arguments it reviews your current folder. Add a path for " +
			"somewhere else; put it in quotes if it has spaces.\n\n" +
			"**What it needs.** Python 3, and the analysers themselves. The part " +
			"that drives them and reads their output lives inside this program; the " +
			"analysers do not, and it tells you which are missing and how to get " +
			"them.\n\n" +
			"**What leaves your machine.** Most analysers read your files and " +
			"nothing else. A few fetch something when they run: semgrep downloads " +
			"its rule packs (its usage reporting is switched off), cargo audit " +
			"downloads a list of known vulnerabilities, and the Go and Rust tools " +
			"download any dependency of your project that is not already on disk. " +
			"Before anything runs you are shown exactly which of these are " +
			"installed and apply to your code, and asked.\n\n" +
			"**Where the results go.** Every tool's full output is kept in this " +
			"program's own cache folder, and the answer gives you the path. " +
			"Nothing is written into the folder being reviewed.\n\n" +
			"**How deep:**\n" +
			"  /review                    the normal pass — fast checks and static analysis, " +
			"and it escalates to the deep security tools ON ITS OWN for any file that looks " +
			"security-shaped. This is usually the one you want.\n" +
			"  /review --quick            linters and formatters only. No static analysis, " +
			"no security tools, no search for leaked secrets — and the answer names every " +
			"analyser it left out.\n" +
			"  /review --security         only the security analysers — the search for leaked " +
			"secrets, the security tools and the static analysers — with the deep pass forced " +
			"over every file. No linters, no formatters; the answer names the ones it left out, " +
			"and lists only security findings.\n" +
			"  /review --full             every analyser of every kind, with the deep pass forced " +
			"over every file. The slowest, and the only one that leaves nothing out.\n" +
			"  /review --diff HEAD        only what you changed. Add a ref for something " +
			"else: --diff origin/main. If nothing has changed it says so; that is not a " +
			"clean report.\n\n" +
			"These combine, and the order does not matter: /review --security --diff HEAD " +
			"internal/auth\n\n" +
			"**It tells you what did NOT run.** That is the part that matters. The " +
			"analysers have to be installed on your machine, and if they are " +
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
		Name:    "port",
		Aliases: []string{"patch", "backport", "forwardport"},
		Group:   GroupHelpers,
		Args:    "[operation] [--onto REF] [--series DIR] [--build CMD]",
		Summary: "Move patches to another version of the code.",
		Detail: "For work written against one version of a tree that has to live on " +
			"another. The kernel and Firefox loop: old tree, existing patches, new " +
			"upstream version, rebase, conflicts, resolve, build, test.\n\n" +
			"**What it does:**\n" +
			"  /port inspect                  read the tree and patches, change NOTHING. Start here.\n" +
			"  /port forward-port --onto REF  carry patches onto a NEWER base\n" +
			"  /port backport --onto REF      carry them onto an OLDER base\n" +
			"  /port rebase --onto REF        move this branch's own commits\n" +
			"  /port refresh --patch FILE     regenerate a patch so it applies again\n" +
			"  /port series --series DIR      a whole numbered series, in order\n\n" +
			"**Why the answer is longer than yes or no.** A patch can land four " +
			"different ways and they are not equivalent. Applied clean means it " +
			"matched exactly. Applied three-way means the context had moved and git " +
			"merged it using real history. Applied WITH FUZZ means the hunk was " +
			"relocated by searching for the lines around it — and if those lines " +
			"appear twice in the file, the change can land in the wrong place and " +
			"still report success. Already-present means upstream has it and the " +
			"patch should be dropped, not forced in twice.\n\n" +
			"So it tells you which of those happened for every patch, and shows you " +
			"the diff for anything fuzzed.\n\n" +
			"**Add --build to actually verify.** Without a build command nothing is " +
			"compiled: the patches applying is not evidence the result works, and it " +
			"will say so rather than let you think otherwise.\n" +
			"  /port forward-port --onto v6.12 --build \"make -j8\"\n\n" +
			"Anything that changes your tree asks first. /port inspect never does, " +
			"because it only reads.\n\n" +
			"Type /port help to read this again at any time.",
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
		Name:    "copy",
		Aliases: []string{"clip", "selectall"},
		Group:   GroupSession,
		Summary: "Copy the whole conversation to the clipboard.",
		Detail: "Puts the entire session on your clipboard, ready to paste: every " +
			"message with its date and time, which model answered, the model's " +
			"reasoning, and every tool it ran with the result. Ctrl+A does the " +
			"same thing.\n\n" +
			"This exists because Select-All is not something this program can " +
			"leave to the terminal on Windows. On Linux your terminal window is " +
			"a separate program and answers Ctrl+Shift+A itself, before this one " +
			"sees the key. On Windows the keystroke is handed straight to this " +
			"program, so the terminal cannot answer it and nothing happens. " +
			"Copying from here works the same way on both.\n\n" +
			"It also copies MORE than selecting would: the stored conversation, " +
			"not the glyphs on screen, so anything already scrolled out of the " +
			"terminal is still included.\n\n" +
			"To copy only PART of it, drag over that part with the mouse and " +
			"press Enter.\n\n" +
			"Use /export to write the same text to a file instead. On Linux the " +
			"clipboard needs xclip or xsel installed; if it is missing, /export " +
			"still works.",
	},
	// GORILLA (2026-10-10): session-to-session messaging (internal/peers).
	{
		Name:    "peers",
		Group:   GroupSession,
		Args:    "[name NEWNAME]",
		Summary: "See the other sessions on this computer; name this one.",
		Detail: "When Gorilla OpenCode runs in more than one window, each window is a session. " +
			"This page lists the other sessions you started on this computer, with their folder and " +
			"whether their AI is busy, and shows this session's own name. /peers name NEWNAME renames " +
			"this one. A session is named after its folder until you rename it.\n\n" +
			"Nothing goes over the internet: only your own account on this computer can reach a " +
			"session. Other sessions learn this one's name, folder, busy or idle, and version, never " +
			"the conversation. Switch it off in /context (the Session messaging row); from the next " +
			"start this session is then listed nowhere and cannot be reached.",
	},
	{
		Name:    "message",
		Group:   GroupSession,
		Args:    "NAME text",
		Summary: "Send a short message to another session.",
		Detail: "Sends your text to the session called NAME (see /peers for the names) and says " +
			"whether it arrived. It appears in that window at once, and its AI reads it at the start " +
			"of its next answer, marked as text from another program rather than its own person's " +
			"instruction.\n\n" +
			"A message cannot answer a permission question, switch on /yolo, change a setting or make " +
			"the other AI start working; only the person at that window can. At most 16 KB, and at most " +
			"20 a minute from one session. The AI can send messages too, with your permission.",
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
		Name:    "purge",
		Aliases: []string{"purgemodels", "purge-models"},
		Group:   GroupModels,
		Summary: "Empty the downloaded model lists and start clean.",
		Detail: "Providers like OpenRouter offer hundreds of models, and the list " +
			"gets long enough to be useless. This clears the downloaded lists so " +
			"the picker only shows the models that come with the app. Your own " +
			"bookmarked list is NOT touched, and neither is anything you hid. " +
			"Run /update afterwards to fetch the lists again.",
	},
	{
		Name:    "update",
		Aliases: []string{"updatemodels", "update-models", "refresh"},
		Group:   GroupModels,
		Summary: "Download fresh model lists from your providers.",
		Detail: "Asks each provider what it serves today and rebuilds the list, so " +
			"retired models stop appearing and new ones show up. Models you hid " +
			"stay hidden - refreshing does not undo that. Pair it with /purge for " +
			"a clean slate: purge, then update.",
	},
	{
		Name:    "connection",
		Aliases: []string{"conn", "link"},
		Group:   GroupModels,
		Summary: "Tell the app how fast your internet is.",
		Detail: "Picks how patient the app is with your connection and how much " +
			"data one message may spend. Shows a speed measured from traffic it " +
			"already sent - it never downloads anything just to test your line. " +
			"Pick the row that matches your connection; a slower one waits longer " +
			"and uploads less. It changes waiting and data only, never what the AI " +
			"can do.",
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
	// GORILLA (2026-10-10): lifecycle hooks (v0.1.144) were documented only in
	// the README. This page is live: it lists the hooks actually loaded.
	{
		Name:    "hooks",
		Group:   GroupTuning,
		Summary: "Your own checks before and after the AI's actions.",
		Detail: "A hook is a command you write in your settings file. It runs before " +
			"the AI uses a tool (and can stop it), after a tool has run, or when an " +
			"answer is finished. The page lists the hooks loaded now, or says there " +
			"are none; shows where the settings file is on this computer; gives an " +
			"example ready to copy for your system; and states the rules. None ships " +
			"with the program. Opening the page changes nothing.",
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
	// GORILLA (2026-10-10): helper roles (v0.1.144) were reachable only by
	// knowing to ask for them. This page says how.
	{
		Name:    "helpers",
		Aliases: []string{"roles"},
		Group:   GroupHelpers,
		Summary: "The three kinds of helper and how to ask for each.",
		Detail: "The AI can send out helpers: short side conversations that each do " +
			"one job. There are three kinds. explore only looks; plan writes a " +
			"step-by-step plan; coder changes files and runs commands, and still " +
			"asks you before every change. You ask in ordinary words, such as " +
			"\"use a plan helper to work out how to ...\". The page explains each " +
			"kind, where to watch them (/tasks) and how many are allowed (/context). " +
			"Opening it changes nothing.",
	},

	// ─── Help ────────────────────────────────────────────────────────
	{
		Name:    "help",
		Aliases: []string{"commands", "?"},
		Group:   GroupReference,
		Summary: "This list.",
		Detail:  "Every command, what it does, and what it costs or changes.",
	},
	// GORILLA (2026-10-10): editor mode (`gorilla-opencode acp`, v0.1.144) was
	// listed only in --help, which nobody inside the program sees.
	{
		Name:    "editor",
		Aliases: []string{"acp"},
		Group:   GroupReference,
		Summary: "Use this program inside Zed or a JetBrains editor.",
		Detail: "Explains editor mode: Gorilla OpenCode working inside a code editor " +
			"that speaks the Agent Client Protocol, such as Zed or a JetBrains " +
			"editor, instead of in this window. The page has the exact lines to put " +
			"in Zed's settings, says what the editor shows (each action, each " +
			"permission question, the record of what ran) and what does not work " +
			"there yet. Choose a provider here first; the editor uses it. Opening " +
			"the page changes nothing.",
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
