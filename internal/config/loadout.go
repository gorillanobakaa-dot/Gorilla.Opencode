// GORILLA OVERRIDE: this file did not exist upstream. It implements the
// "context loadout" — a Slackware-style, total-control view of everything
// this agent sends to the model on EVERY turn, with the approximate token
// cost of each piece and the ability to switch any of it off.
//
// Philosophy (see PHILOSOPHY.md): radical transparency and total user
// control. You can see exactly what you are paying for just to say "yo",
// and you can strip it to the bone. Past a point that lobotomises the
// agent — that is your call, and a one-key reset brings the defaults back.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// LoadoutComponent is one switchable piece of per-turn context.
type LoadoutComponent struct {
	ID       string // stable key, also used by the tool/prompt gates
	Name     string // human label
	Tradeoff string // what you lose if you turn it OFF
	Tokens   int    // approximate tokens this adds to EVERY turn
	Default  bool   // shipped-on by default
	Critical bool   // turning this off significantly lobotomises the agent
}

// LoadoutComponents is the registry. Token figures are approximate
// (measured from description/schema sizes, ~4 chars/token) and exist to
// inform a decision, not to bill anyone.
var LoadoutComponents = []LoadoutComponent{
	{"tool.bash", "Bash tool", "agent can't run shell commands (build, test, git, run anything)", 1200, true, true},
	{"tool.edit", "Edit tool", "agent can't modify files in place", 1300, true, true},
	{"tool.write", "Write tool", "agent can't create or overwrite files", 350, true, true},
	{"tool.view", "View tool", "agent can't read file contents", 500, true, true},
	// GORILLA OVERRIDE: tool.ls, tool.grep and tool.glob (450+600+400 as
	// estimated here, ~1,485 measured) are replaced by one find tool. Three
	// descriptions repeating WHEN TO USE / HOW TO USE / LIMITATIONS / TIPS is
	// three times the boilerplate for one job, and grep returned paths only —
	// so the agent paid a second turn and a whole-file view to actually read
	// anything. find is deliberately NOT as small as it could be: smaller
	// models were failing to search big trees at all, so its description
	// teaches the narrowing arguments (type, path, glob) with worked examples.
	// It still costs roughly a third of the three it replaces. The old
	// components are gone from this registry, not from the tree.
	{"tool.find", "Find tool (search + list + glob)", "agent can't search code, find files, or list directories — it is blind to the tree", 520, true, true},
	{"tool.patch", "Patch tool", "agent loses multi-hunk patch edits (edit/write still work)", 900, true, false},
	{"tool.fetch", "Fetch a web page", "agent can't open a link you give it", 300, true, false},
	{"tool.websearch", "Find sources + web search", "agent can't look anything up — only what you paste in", 300, true, false},
	{"tool.diagnostics", "Diagnostics tool", "agent can't read LSP errors/warnings", 400, true, false},
	{"tool.agent", "Sub-agent tool", "agent can't spawn read-only search sub-agents", 200, true, false},
	// GORILLA OVERRIDE: multi-role research (4-10 helpers in fixed lanes).
	// Default ON but it is the first thing to drop on a metered link: helpers
	// run sequentially, so a run is several LLM sessions and real money. Its
	// own schema is ~450 tokens a turn; the run itself costs far more.
	//
	// This 450 is deliberately a GUESS and must stay one. CalibrateLoadout
	// measures the real figure at startup (622 when measured 2026-08-14) and
	// overwrites it; TestCalibrationCoversEveryComponentWithNoLSPClients proves
	// calibration ran by asserting the displayed value DIFFERS from the number
	// written here. Set this to the measured value and that test can no longer
	// tell a calibrated figure from an uncalibrated one.
	// GORILLA FIX (2026-08-17): the name went back to what it always was. It was
	// briefly renamed "OSINT research (multi-agent)" when /osint was added, which
	// left BOTH research rows saying OSINT and no way to tell the everyday tool
	// from the expensive one — "i don't know which is the old one". The command
	// name is now in the row, because that is the thing a user can act on.
	{"tool.research", "Research helpers — /research", "agent can't send helpers to investigate a question — it works alone, which is how two days went into the wrong fix. A RUN is 4-10 full model sessions", 450, true, false},
	// GORILLA OVERRIDE: the serious dossier ships OFF and is armed here by hand.
	// That is the whole design: a run is 4-10 full model sessions plus a gap
	// round, so nobody meets it by accident — /osint refuses until this row is
	// on, and every run still starts with the burn-rate warning screen.
	{DossierComponentID, DossierRowName, "the /osint all-source assessment refuses to run. ARMING it costs the tokens on the left; RUNNING it costs 4-10 full model sessions plus a gap round — the most expensive thing here, and the warning screen prices it before each run", 120, false, false},
	// GORILLA OVERRIDE: default OFF. sparse is the kernel's own semantic checker
	// (__user/__kernel pointers, endianness, lock imbalance) — invaluable on
	// kernel work, meaningless everywhere else, so its schema should not ride
	// every request for a user who never touches the kernel. Turn it on when
	// starting kernel patches.
	{"tool.sparse", "Sparse checker (kernel)", "agent can't check kernel semantics (__user pointers, endianness, lock imbalance) — needs a build to catch them instead", 180, false, false},
	// GORILLA OVERRIDE (2026-08-18): the code-review toolkit — ~30 static
	// analysers embedded in the binary. ON by default: a review capability
	// nobody is told about is not a capability, and this is the one feature no
	// other coding agent ships. Its description is deliberately long because it
	// has to say what static analysis CANNOT find; a short description here
	// would produce reviews that claim to be complete and are half a review.
	// 475 is MEASURED (toolTokens), not estimated — the first hand-written
	// guess here was 320, out by 48%, and calibrate_test.go caught it.
	{"tool.review", "Code review — /review", "agent loses the 30 static-analysis and security tools; it can still read your code, but nothing mechanically checks for memory errors, injection, leaked secrets or unchecked errors", 475, true, false},
	// GORILLA OVERRIDE: env estimate was 150 when the block was a recursive
	// 1000-file tree dump (real cost often 10k–30k). After the shallow
	// project_summary refactor it really is ~100–200 tokens; calibrate
	// still overwrites this at startup with a measured value.
	{"prompt.env", "Environment info", "agent won't be told your cwd, OS, top-level files, or short git status", 150, true, false},
	{"prompt.lsp", "LSP info", "agent won't be told which language servers are active", 100, true, false},
	// GORILLA OVERRIDE (2026-08-19): five hints that REPLACE WRONG INSTINCTS
	// rather than add capability — adb backup has been dead since Android 12
	// and fails silently, yt-dlp downloads the whole video unless told not to,
	// carving cannot recover source code. Each one is a wrong first move that
	// costs the user a wasted round trip or, worse, a promise the agent cannot
	// keep.
	//
	// Default OFF, and that is the point. They are worth real money to someone
	// doing Android or media or forensics work and worth nothing to everyone
	// else, and a prompt line is a RECURRING bill — it rides every turn
	// forever. Shipping them on by default would tax every user for a
	// capability most of them will never use.
	{"prompt.localtools", "Local tool hints (android, media, forensics)", "agent keeps the wrong first instincts about local tools: it will reach for adb backup (dead since Android 12, fails silently), let yt-dlp pull whole videos to get subtitles, and may imply carving can recover source code", 120, false, false},
}

// RegisterLoadoutComponents appends dynamically discovered switchable
// components — prompt sections, language servers — to the registry.
//
// Idempotent by ID so calling it twice (config reload, a second Load in a test
// binary) cannot duplicate a row. Sorted by the caller; append order is kept so
// related rows stay together in the /context menu.
//
// GORILLA OVERRIDE: the registry was a fixed var. Prompt sections are only
// knowable after parsing the ACTIVE prompt, and config cannot import prompt
// (prompt imports config), so registration is pushed in from the other side.
func RegisterLoadoutComponents(extra []LoadoutComponent) {
	if len(extra) == 0 {
		return
	}
	existing := make(map[string]bool, len(LoadoutComponents))
	for _, c := range LoadoutComponents {
		existing[c.ID] = true
	}
	for _, c := range extra {
		if c.ID == "" || existing[c.ID] {
			continue
		}
		LoadoutComponents = append(LoadoutComponents, c)
		existing[c.ID] = true
	}
}

// DossierComponentID gates the /osint professional dossier; DossierRowName is
// its label in /context (referenced by the /osint refusal message, so the two
// can never drift apart).
const (
	DossierComponentID = "tool.dossier"
	// GORILLA FIX (2026-08-17): "EXPENSIVE" left the NAME. It sat beside the
	// per-turn token column and read as a claim about that number — which is
	// the smallest on the screen, because arming this only adds a paragraph to
	// one tool's description. What is expensive is RUNNING it. The row says
	// which command it is; the cost lives in RunCostRow and in the row's text,
	// in the unit it belongs to.
	//
	// RENAMED 2026-08-17 from "dossier" to all-source assessment, at the
	// owner's direction and because it is the accurate term: the tool fuses
	// several source types and grades them, which is all-source assessment,
	// not OSINT collection. The name follows the UK Professional Development
	// Framework for All-Source Intelligence Assessment (PHIA); the row is
	// abbreviated to fit its column, and AllSourceProductName is the full one.
	DossierRowName = "OSINT All-Source — /osint"
)

// RunCostRow reports whether a component's real cost is paid when it is USED
// rather than per turn.
//
// GORILLA OVERRIDE (2026-08-17): /context has one number column and it means
// "tokens added to every message". For almost every row that is the whole
// story. For the two research rows it is the smaller half of the story: a run
// is several complete model sessions. Displaying only the per-turn figure made
// the expensive row look like the cheap one — reported from a real screen,
// where the dossier showed ~163 beside "EXPENSIVE" while the everyday research
// row showed ~1,007. Both figures were right and the screen was misleading.
func RunCostRow(id string) bool {
	return id == "tool.research" || id == DossierComponentID
}

// AllSourceProductName is what the product is called in full, on the capability
// page and in the written assessment. "All-source" is the accurate description:
// the tool fuses scholarly, official, corporate, humanitarian and news sources
// and grades them, which is assessment rather than collection.
const AllSourceProductName = "OSINT All-Source Intelligence Analysis"

// DossierDir is where /osint writes its finished dossiers.
//
// GORILLA OVERRIDE: deliberately OUTSIDE the working folder, always. The
// working folder is often a git repository; a dossier answering someone's
// private question must never be swept into a commit and pushed. And nothing
// here is hardcoded to any particular machine: the path is derived from the
// USER'S OWN home directory at runtime — their Documents folder when they have
// one, their home when they don't, the app's config directory as the last
// resort when even home is unknowable.
func DossierDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(ConfigBase(), "dossiers")
	}
	if docs := filepath.Join(home, "Documents"); dirExists(docs) {
		return filepath.Join(docs, "Gorilla-OSINT-Dossiers")
	}
	return filepath.Join(home, "Gorilla-OSINT-Dossiers")
}

// SessionExportDir is where /sessions writes a transcript.
//
// Derived from the user's home, never hardcoded to this developer's machine —
// the same rule DossierDir follows, and for the same reason: this has to work
// on every computer it is installed on. Never the working folder: that is often
// a git repository, and a transcript holds whatever was discussed.
func SessionExportDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(ConfigBase(), "session-exports")
	}
	if docs := filepath.Join(home, "Documents"); dirExists(docs) {
		return filepath.Join(docs, "Gorilla-Session-Exports")
	}
	return filepath.Join(home, "Gorilla-Session-Exports")
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// LSPComponentID is the loadout id for one configured language server.
// Keeping the prefix in one function means the gate in internal/app/lsp.go and
// the registration below cannot drift apart.
func LSPComponentID(name string) string { return "lsp." + name }

// RegisterLSPComponents adds one switchable loadout row per configured language
// server, so clangd / gopls / rust-analyzer can be turned off individually
// instead of only in bulk via the single "LSP info" row.
//
// GORILLA OVERRIDE: idempotent by id, so calling it twice (or after a config
// reload) does not duplicate rows. The default follows the config's own
// Disabled flag — a server the user disabled in config.json starts life as an
// off row rather than silently re-enabling itself.
//
// What turning a row OFF actually saves: the LSP process never starts (real
// memory and CPU — clangd on Firefox is 500MB-2GB), the per-edit diagnostics
// for that language stop riding tool responses, and the language stops being
// named in the prompt's LSP block. It is NOT primarily a token saving; the
// honest win is process weight and per-edit diagnostic volume.
func RegisterLSPComponents(lspNames map[string]bool) {
	existing := make(map[string]bool, len(LoadoutComponents))
	for _, c := range LoadoutComponents {
		existing[c.ID] = true
	}

	names := make([]string, 0, len(lspNames))
	for n := range lspNames {
		names = append(names, n)
	}
	sort.Strings(names) // stable row order in the /context menu

	for _, name := range names {
		id := LSPComponentID(name)
		if existing[id] {
			continue
		}
		LoadoutComponents = append(LoadoutComponents, LoadoutComponent{
			ID:   id,
			Name: "LSP: " + name,
			Tradeoff: "agent gets no live compiler/linter feedback for " + name +
				" files, and that language server does not run at all",
			// Measured by agent.CalibrateLoadout at startup where possible.
			// The per-turn prompt cost of one named server is small (a word in
			// the LSP block); the real cost is the process and its per-edit
			// diagnostics, which no per-turn number can represent.
			Tokens:   10,
			Default:  !lspNames[name], // map value is "disabled"
			Critical: false,
		})
	}
}

// lowBandwidthOff lists components switched OFF by ApplyLowBandwidthLoadout.
// Critical tools (bash/edit/write/view/find) stay on; optional network, LSP and
// extra edit surfaces drop, along with the LSP prompt blurb.
//
// GORILLA OVERRIDE (2026-08-23): tool.review added, four days late.
//
// This list was last touched on 2026-08-14. tool.review landed on 2026-08-18,
// default ON and costing 759 measured tokens, and nobody came back here. So the
// preset built for someone on a satellite link was still shipping a 30-analyser
// static-review schema on every turn, when its own docstring says it exists to
// drop what is "not required for core edit/build loops". A code review is not a
// core edit/build loop.
//
// That is the same shape as the Arch package vanishing for four releases: a new
// thing lands, an existing list is not updated, and nothing fails. The preset
// still worked, still saved tokens, still reported a smaller number. It was just
// quietly leaving 759 of them on the table, and only a direct count found it.
//
// The default stays ON, and that is a separate, deliberate decision recorded at
// the tool.review row: a review capability nobody is told about is not a
// capability. Being on by default and being dropped on a metered link are not in
// conflict.
//
// Now guarded by TestEveryOptionalComponentHasALowBandwidthDecision, which
// fails if a default-ON, non-critical component is in neither this map nor
// lowBandwidthKeep. A new tool must be DECIDED about, not merely forgotten.
var lowBandwidthOff = map[string]bool{
	"tool.patch":       true,
	"tool.fetch":       true,
	"tool.websearch":   true,
	"tool.diagnostics": true,
	"tool.agent":       true,
	"tool.research":    true,
	"tool.review":      true,
	"prompt.lsp":       true,
}

// lowBandwidthKeep is the other half of the decision: default-ON, non-critical
// components deliberately KEPT on a metered link, each with the reason.
//
// It exists so that "not in lowBandwidthOff" stops being ambiguous. Before this,
// an entry missing from that map could mean "we decided to keep it" or "nobody
// looked", and those two are indistinguishable from the outside. That ambiguity
// is exactly what let tool.review sit unconsidered for four days. A component
// must now appear in one map or the other.
var lowBandwidthKeep = map[string]string{
	"prompt.env": "cheap since the shallow project_summary change (~130 measured " +
		"tokens) and MORE useful on a remote link, not less: knowing the cwd, OS and " +
		"git state prevents a wasted round trip, which is the expensive thing here.",
}

const loadoutFileName = "loadout.json"

// GORILLA OVERRIDE: token figures start as estimates but are replaced at
// startup with REAL measured values (agent.CalibrateLoadout serialises
// each tool's actual schema and the actual system prompt). This is why
// the menu total matches what the model really receives, and why turning
// a tool off drops the number by its true cost.
var (
	basePromptTokens = 3000 // measured system prompt; always on, not switchable
	tokenOverride    = map[string]int{}
	tokenOverrideMu  sync.RWMutex
)

// SetBasePromptTokens records the measured base system-prompt token count.
func SetBasePromptTokens(n int) {
	tokenOverrideMu.Lock()
	basePromptTokens = n
	tokenOverrideMu.Unlock()
}

// SetLoadoutTokens records a component's measured token cost.
func SetLoadoutTokens(id string, n int) {
	tokenOverrideMu.Lock()
	tokenOverride[id] = n
	tokenOverrideMu.Unlock()
}

// ComponentTokens returns the measured cost if known, else the estimate.
func ComponentTokens(c LoadoutComponent) int {
	tokenOverrideMu.RLock()
	defer tokenOverrideMu.RUnlock()
	if v, ok := tokenOverride[c.ID]; ok {
		return v
	}
	return c.Tokens
}

var (
	loadoutOnce  sync.Once
	loadoutState map[string]bool // id -> enabled (only entries that differ or are set)
	loadoutMu    sync.RWMutex
)

// GORILLA OVERRIDE: Gorilla-specific config lives under a "gorilla-opencode"
// directory, matching the desktop launch key file — NOT under "opencode",
// which is a different app's (SST opencode) config directory. Using appName
// ("opencode") polluted that other app's dir and split our own config.
const gorillaConfigDir = "gorilla-opencode"

// GORILLA OVERRIDE: loadoutConfigBase() lived here with a body byte-identical to
// gorillaConfigBase() in config.go. Both are now ConfigBase() in store.go, the
// single owner of this directory.

func loadoutPath() string {
	return filepath.Join(ConfigBase(), loadoutFileName)
}

func initLoadout() {
	loadoutOnce.Do(func() {
		loadoutState = map[string]bool{}
		// start from defaults
		for _, c := range LoadoutComponents {
			loadoutState[c.ID] = c.Default
		}
		// overlay persisted overrides
		if data, err := os.ReadFile(loadoutPath()); err == nil {
			var saved map[string]bool
			if json.Unmarshal(data, &saved) == nil {
				for k, v := range saved {
					loadoutState[k] = v
				}
			}
		}
	})
}

// LoadoutEnabled reports whether a component is currently on. Unknown ids
// default to enabled so a new component is never silently dropped.
func LoadoutEnabled(id string) bool {
	initLoadout()
	loadoutMu.RLock()
	defer loadoutMu.RUnlock()
	v, ok := loadoutState[id]
	if !ok {
		return true
	}
	return v
}

// ToggleLoadout flips a component and persists.
//
// GORILLA OVERRIDE: reads the CURRENT value the way LoadoutEnabled does, where
// an absent key means enabled.
//
// The old `loadoutState[id] = !loadoutState[id]` took the zero value for a
// missing key — false — and flipped it to true. But LoadoutEnabled reports a
// missing key as ENABLED, so for any component with no saved entry the first
// press set it from "on" to on: a toggle that visibly did nothing. That is every
// component registered after the state was first loaded, which includes the
// language servers, since those rows come from the config at Load time.
func ToggleLoadout(id string) {
	initLoadout()
	loadoutMu.Lock()
	cur, ok := loadoutState[id]
	if !ok {
		cur = true
	}
	loadoutState[id] = !cur
	loadoutMu.Unlock()
	saveLoadout()
}

// SetAllLSPs switches every registered language server on or off at once.
// Returns how many rows changed.
//
// GORILLA OVERRIDE: with nine servers configured, turning them all off meant
// nine separate toggles, and the granular control is only pleasant once there is
// a bulk switch beside it. Nothing else is touched — the prompt blocks and tool
// rows keep their settings, because "no language servers" is a common working
// mode and "no tools" is not.
func SetAllLSPs(enabled bool) int {
	initLoadout()
	loadoutMu.Lock()
	changed := 0
	for _, c := range LoadoutComponents {
		if !strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		// An ABSENT key means enabled — the same rule LoadoutEnabled applies, so
		// that a component added after the state was loaded is never silently
		// dropped. Comparing the raw map value instead reads absent as false and
		// skips exactly those rows: a newly configured language server would be
		// left running by "switch them all off".
		cur, ok := loadoutState[c.ID]
		if !ok {
			cur = true
		}
		if cur != enabled {
			loadoutState[c.ID] = enabled
			changed++
		}
	}
	loadoutMu.Unlock()
	if changed > 0 {
		saveLoadout()
	}
	return changed
}

// LSPLoadoutCounts reports how many language-server rows are on and off, so the
// UI can label a bulk switch with what it will actually do.
func LSPLoadoutCounts() (on, off int) {
	initLoadout()
	loadoutMu.RLock()
	defer loadoutMu.RUnlock()
	for _, c := range LoadoutComponents {
		if !strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		if v, ok := loadoutState[c.ID]; !ok || v {
			on++
		} else {
			off++
		}
	}
	return on, off
}

// ResetLoadout restores every component to its shipped default.
func ResetLoadout() {
	initLoadout()
	loadoutMu.Lock()
	for _, c := range LoadoutComponents {
		loadoutState[c.ID] = c.Default
	}
	loadoutMu.Unlock()
	saveLoadout()
}

// ApplyLowBandwidthLoadout turns off optional tools/blocks that are not
// required for core edit/build loops. Intended for metered, satellite, or
// high-latency links. Persists like any other loadout change. Returns the
// new active token estimate (after current calibration overrides).
func ApplyLowBandwidthLoadout() int {
	initLoadout()
	loadoutMu.Lock()
	for _, c := range LoadoutComponents {
		if lowBandwidthOff[c.ID] {
			loadoutState[c.ID] = false
		} else {
			// Keep shipped defaults for everything else (including critical tools).
			loadoutState[c.ID] = c.Default
		}
	}
	loadoutMu.Unlock()
	saveLoadout()
	return LoadoutActiveTokens()
}

func saveLoadout() {
	loadoutMu.RLock()
	data, _ := json.MarshalIndent(loadoutState, "", " ")
	loadoutMu.RUnlock()
	_ = writeSecretFile(loadoutPath(), data)
}

// LoadoutActiveTokens is the approximate per-turn overhead of everything
// currently switched on, including the always-present base system prompt.
func LoadoutActiveTokens() int {
	tokenOverrideMu.RLock()
	total := basePromptTokens
	tokenOverrideMu.RUnlock()
	for _, c := range LoadoutComponents {
		if LoadoutEnabled(c.ID) {
			total += ComponentTokens(c)
		}
	}
	return total
}

// LoadoutBaseTokens is the fixed, non-switchable overhead (base prompt).
func LoadoutBaseTokens() int {
	tokenOverrideMu.RLock()
	defer tokenOverrideMu.RUnlock()
	return basePromptTokens
}

// LoadoutCost is the money side of the token counter: what the fixed
// per-turn context (LoadoutActiveTokens) actually costs, priced at the
// active coder model's INPUT rate. Same formula the agent uses to bill a
// real turn (CostPer1MIn/1e6 * inputTokens), so the numbers line up.
//
//   - dollars:   cost of one turn's fixed overhead in USD.
//   - per1MIn:   the model's input price per 1M tokens (0 = free/flat/OAuth).
//   - modelName: human name of the active model (for the label).
//   - priced:    false when we have no model or no price table entry, so the
//     UI can say "unpriced" instead of a misleading $0.00.
//
// On a free or flat-rate tier (per1MIn == 0) dollars is genuinely 0 — that
// is the real bill, not missing data; priced stays true.
func LoadoutCost() (dollars, per1MIn float64, modelName string, priced bool) {
	tokens := LoadoutActiveTokens()
	if cfg == nil {
		return 0, 0, "", false
	}
	agent, ok := cfg.Agents[AgentCoder]
	if !ok {
		return 0, 0, "", false
	}
	m, ok := models.SupportedModels[agent.Model]
	if !ok {
		// Unknown/custom model: tokens are real but we can't price them.
		return 0, 0, string(agent.Model), false
	}
	name := m.Name
	if name == "" {
		name = string(m.ID)
	}
	return float64(tokens) / 1e6 * m.CostPer1MIn, m.CostPer1MIn, name, true
}

// ResearchCost prices research helpers, and reports the BURN RATE.
//
// GORILLA OVERRIDE, third revision. The wording history matters because each
// version was rejected for a reason worth keeping:
//
//	v1  "20 model sessions. Not one answer — 20."   -> meaningless
//	v2  "20 separate conversations with the AI"     -> "tells a non-technical
//	                                                   user sweet fuck all"
//	v3  "23% of a $2 day"                           -> "disingenuous and a bit
//	                                                   evasive". Also wrong in
//	    its assumption: not everyone in these places is dirt poor, some have a
//	    parent paying for a subscription. Inventing a poverty line for the
//	    reader is patronising AND imprecise.
//
// What the author asked for instead, and he is right: COST PER MINUTE. "The
// biggest slap in the face, the coldest bucket of water" — so nobody can say
// they were not warned, and so the support mail does not read "sir, your tool
// ruined me".
//
// Returns:
//   - perHelper: estimated cost of one helper, USD
//   - perMinute: estimated PEAK burn while the run is going, USD/minute
//   - per1MIn:   model input price (0 = free tier / OAuth / flat)
//   - modelName: the model HELPERS run on — often NOT the chat model
//   - priced:    false when there is no price entry, so the UI says so rather
//     than showing a confident and wrong $0.00
//
// The estimate's ASSUMPTIONS, named so they can be quoted on screen and
// argued with. Everything else in the arithmetic is measured or published.
//
// NONE of these three is measured. They were invented, and on 2026-08-14 the
// author audited the dialog and correctly found that the per-minute figure
// rests entirely on ResearchSecondsPerStep — a number with no evidence behind
// it. They are surfaced in the UI rather than buried so the reader can judge
// the forecast instead of trusting it.
//
// TO REPLACE THEM PROPERLY: record each helper's real duration and token usage
// when a run finishes, and average over past runs. Until that exists these stay
// labelled as assumptions.
const (
	ResearchStepsPerHelper = 3
	ResearchOutputPerStep  = 700
	ResearchSecondsPerStep = 15.0
)

func ResearchCost(inFlight int) (perHelper, perMinute, per1MIn float64, modelName string, priced bool) {
	if cfg == nil {
		return 0, 0, 0, "", false
	}
	name := AgentResearch
	if _, ok := cfg.Agents[AgentResearch]; !ok {
		name = AgentTask
	}
	agent, ok := cfg.Agents[name]
	if !ok {
		return 0, 0, 0, "", false
	}
	m, ok := models.SupportedModels[agent.Model]
	if !ok {
		return 0, 0, 0, string(agent.Model), false
	}
	label := m.Name
	if label == "" {
		label = string(m.ID)
	}

	// Per-STEP shape. The input floor is measured for this install
	// (LoadoutActiveTokens + base prompt); the rest is the estimate, and the UI
	// prints the assumptions next to the number.
	// One source of truth for the basis, and NOT LoadoutActiveTokens() +
	// LoadoutBaseTokens() — that double-counted the base prompt. See the note on
	// ResearchBasisTokens.
	base := ResearchBasisTokens()
	costPerStep := float64(base)/1e6*m.CostPer1MIn + float64(ResearchOutputPerStep)/1e6*m.CostPer1MOut
	perHelper = costPerStep * ResearchStepsPerHelper

	// Peak burn: every in-flight helper completing a step every secondsPerStep.
	if inFlight < 1 {
		inFlight = 1
	}
	perMinute = costPerStep * float64(inFlight) * (60.0 / ResearchSecondsPerStep)

	return perHelper, perMinute, m.CostPer1MIn, label, true
}

// ResearchHelperModel reports which model helpers run on, and whether that is
// on a DIFFERENT PROVIDER from the chat model.
//
// GORILLA OVERRIDE: this used to flag any difference in model NAME, which fired
// for two near-identical Gemini free-tier models and read as confusing trivia —
// "helpers run on 3.6, not 3.7, which you are chatting with". So what? The user
// could not act on it and it did not mean anything.
//
// A different PROVIDER is the case that actually matters: different billing,
// different quota, possibly a local server. That is the shape of the real bug
// this was written for, where research helpers were silently pointed at a local
// Llama while the user was signed in to Antigravity. Name-only differences
// within one provider are noise and are no longer reported.
func ResearchHelperModel() (helper, chat string, differentProvider bool) {
	if cfg == nil {
		return "", "", false
	}
	lookup := func(a AgentName) (models.Model, bool) {
		ag, ok := cfg.Agents[a]
		if !ok {
			return models.Model{}, false
		}
		m, ok := models.SupportedModels[ag.Model]
		return m, ok
	}
	hm, hok := lookup(AgentResearch)
	if !hok {
		hm, hok = lookup(AgentTask)
	}
	cm, cok := lookup(AgentCoder)
	if !hok || !cok {
		return "", "", false
	}
	name := func(m models.Model) string {
		if m.Name != "" {
			return m.Name
		}
		return string(m.ID)
	}
	return name(hm), name(cm), hm.Provider != cm.Provider
}

// ResearchModelChoice reports the two models research could run on: the one it
// WILL use, and the one the user is actually chatting with.
//
// GORILLA FIX 2026-08-14: the screen was blind to the chat model unless the two
// were on different PROVIDERS. That gate was chosen to suppress noise about
// billing — same provider, same bill, nothing to warn about — and it was the
// wrong axis entirely.
//
// Reported the same day: the user switched to Claude Opus 4.6 (Thinking) and the
// research screen carried on pricing Gemini 2.0 Flash without a word, because
// both are Antigravity. Nobody selects Opus for billing reasons. They select it
// because the work is hard, and the whole point of research is to do the hard
// part — so silently running it on Flash caps the answer at Flash while the
// status bar says Opus.
//
// So the comparison is now on the MODEL, and the thing reported is CAPABILITY,
// not just cost. Provider difference is still called out separately, because a
// different bill is a different harm from a weaker answer.
func ResearchModelChoice() (helper, chat models.Model, ok bool) {
	if cfg == nil {
		return models.Model{}, models.Model{}, false
	}
	lookup := func(a AgentName) (models.Model, bool) {
		ag, found := cfg.Agents[a]
		if !found {
			return models.Model{}, false
		}
		m, found := models.SupportedModels[ag.Model]
		return m, found
	}
	h, hok := lookup(AgentResearch)
	if !hok {
		h, hok = lookup(AgentTask)
	}
	c, cok := lookup(AgentCoder)
	if !hok || !cok {
		return models.Model{}, models.Model{}, false
	}
	return h, c, true
}

// ModelLabel is a model's human name, falling back to its id.
func ModelLabel(m models.Model) string {
	if m.Name != "" {
		return m.Name
	}
	return string(m.ID)
}

// UseChatModelForResearch pins the research agent to the coder's model, so
// helpers run on whatever the user actually selected.
//
// This also CREATES the "research" agent entry, which most configs lack — the
// reason researchAgentName() has to fall back to AgentTask, and the reason the
// old dialog could tell the user to "set a research agent" while offering no way
// to do it.
func UseChatModelForResearch() error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	coder, ok := cfg.Agents[AgentCoder]
	if !ok {
		return fmt.Errorf("no coder agent configured")
	}
	return UpdateAgentModel(AgentResearch, coder.Model)
}

// ResearchHelperModelInfo returns the actual model helpers will run on.
func ResearchHelperModelInfo() (models.Model, bool) {
	if cfg == nil {
		return models.Model{}, false
	}
	name := AgentResearch
	if _, ok := cfg.Agents[AgentResearch]; !ok {
		name = AgentTask
	}
	ag, ok := cfg.Agents[name]
	if !ok {
		return models.Model{}, false
	}
	m, ok := models.SupportedModels[ag.Model]
	return m, ok
}

// ResearchPaidEquivalent prices the run for the MODEL THE USER IS ACTUALLY ON,
// at its paid-API rate, when their own tier is flat or free.
//
// GORILLA OVERRIDE: this replaces a cheapest/most-expensive RANGE across the
// whole catalogue, which was rejected — correctly — as "generic crap displaying
// not so relevant information". It offered BGE-M3 (an embedding model) as the
// cheap end and o1 pro as the dear end, neither of which the user had selected
// or would ever run. Someone on Muse Glimmer wants to know what Muse Glimmer
// costs, not trivia about two models they have never heard of.
//
// Method: find a METERED entry for the same model family — the same model sold
// through a paid API rather than a free tier — and price the identical token
// volume against it. If no sibling exists, say so. Never substitute an
// unrelated model.
func ResearchPaidEquivalent(helperModel models.Model, inFlight int) (perMin, perHelper float64, viaName string, ok bool) {
	if inFlight < 1 {
		inFlight = 1
	}
	base := ResearchBasisTokens()

	price := func(m models.Model) (float64, float64) {
		perStep := float64(base)/1e6*m.CostPer1MIn + float64(ResearchOutputPerStep)/1e6*m.CostPer1MOut
		return perStep * float64(inFlight) * (60.0 / ResearchSecondsPerStep), perStep * ResearchStepsPerHelper
	}

	// Already metered: the real rate IS the answer, no equivalent needed.
	if helperModel.CostPer1MIn > 0 {
		pm, ph := price(helperModel)
		return pm, ph, "", true
	}

	want := familyTokens(string(helperModel.ID) + " " + helperModel.Name)
	var best models.Model
	bestScore := 0
	var bestID models.ModelID
	for id, m := range models.SupportedModels {
		if m.CostPer1MIn <= 0 || m.DefaultMaxTokens <= 0 {
			continue
		}
		score := 0
		for tok := range familyTokens(string(id) + " " + m.Name) {
			if want[tok] {
				score++
			}
		}
		// Deterministic: on a tie, the lexicographically smaller id wins, so
		// this cannot flicker between renders.
		if score > bestScore || (score == bestScore && score > 0 && id < bestID) {
			best, bestScore, bestID = m, score, id
		}
	}
	// Two shared family tokens is the floor, AND at least one must be a real
	// name rather than a qualifier. "gemini"+"flash" is a family; "mini"+"code"
	// is two coincidences stacked.
	if bestScore < 2 || !sharesASubstantiveToken(want, familyTokens(string(bestID)+" "+best.Name)) {
		return 0, 0, "", false
	}
	name := best.Name
	if name == "" {
		name = string(best.ID)
	}
	pm, ph := price(best)
	return pm, ph, name, true
}

// familyTokens reduces a model id/name to comparable words: "gemini-3.6-flash"
// and "Gemini 3.5 Flash" share {gemini, flash}. Version numbers are kept as
// their own tokens so 3.6 does not silently match 2.0.
func familyTokens(s string) map[string]bool {
	out := map[string]bool{}
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() >= 2 {
			out[strings.ToLower(cur.String())] = true
		}
		cur.Reset()
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	// Words that appear across half the catalogue carry no family information.
	// Matching on these produced nonsense: "Cohere North Mini Code" was priced
	// via "AionLabs Aion-3.0-Mini" because both contain "mini".
	for _, noise := range []string{
		"free", "medium", "high", "low", "tiered", "thinking", "preview", "latest",
		"antigravity", "mini", "small", "large", "nano", "micro", "turbo", "lite",
		"base", "chat", "instruct", "code", "coder", "plus", "max", "ultra", "pro",
		"exp", "experimental", "beta", "alpha", "vision", "text", "openrouter",
	} {
		delete(out, noise)
	}
	// Bare numbers are version noise on their own — "3" matches half the
	// catalogue. They only count attached to a name, which the tokeniser
	// already keeps ("gemini25" stays whole where the source has no separator).
	for tok := range out {
		allDigits := true
		for _, r := range tok {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			delete(out, tok)
		}
	}
	return out
}

// ResearchBasisTokens reports the measured per-step input size, so the UI can
// show where the arithmetic starts instead of asserting a total.
// GORILLA FIX 2026-08-14: THE BASE PROMPT WAS COUNTED TWICE.
//
// LoadoutActiveTokens() ALREADY opens with `total := basePromptTokens` (see
// :395). Adding LoadoutBaseTokens() on top added the same figure a second time,
// so the research basis carried a phantom copy of the system prompt — 3,000
// tokens at shipped defaults, 28% over — and it fed straight into costPerStep.
// EVERY dollar figure on the /research screen was inflated by it, and the
// "MEASURED: N tokens of context per step (this machine)" line disagreed with
// what /context prints for the same quantity.
//
// Found by an independent audit on 2026-08-14 after three failed attempts to
// fix this screen by hand. "Measured" has to mean measured; a figure labelled
// as such and then quietly doubled is worse than an honest estimate.
func ResearchBasisTokens() int {
	base := LoadoutActiveTokens()
	if base <= 0 {
		base = 8000
	}
	return base
}

// ResearchQuotaMultiple is how many ORDINARY questions this run is worth in
// tokens. Derived, not the helper count — see the note in research_agent_test.go.
func ResearchQuotaMultiple(helpers int) int {
	if helpers < 1 {
		return 0
	}
	return helpers * ResearchStepsPerHelper
}

// sharesASubstantiveToken requires a shared word long enough to identify a
// model family, rather than two generic qualifiers coinciding.
func sharesASubstantiveToken(a, b map[string]bool) bool {
	for tok := range b {
		if len(tok) >= 5 && a[tok] {
			return true
		}
	}
	return false
}

// AgentModel reports an agent's currently configured model id, or "" if that
// agent is not configured. A small accessor, but it means callers (and tests
// asserting that a revert really landed) read the same place the app does
// rather than reaching into cfg.
func AgentModel(name AgentName) models.ModelID {
	if cfg == nil {
		return ""
	}
	return cfg.Agents[name].Model
}

// TurnUploadBudgetBytes is how much one turn may push up the link before it
// stops trying, including retries.
//
// GORILLA OVERRIDE (2026-08-18): measured, not chosen. A dropping link produced
// 14 attempts and 1.01 MB of upload for a single unanswered question, because
// the application's retry loop and Go's http.Transport each counted separately.
// See internal/llm/provider/uploadbudget.go.
//
// 4 MB is deliberately generous: a large conversation is ~100 KB per attempt,
// so this still allows dozens of legitimate retries and a long agent turn with
// many tool calls. It is a backstop against a storm, not a diet. What it stops
// is the unbounded case — and on a metered satellite plan, unbounded is the
// only number that actually hurts.
//
// GORILLA_OPENCODE_TURN_UPLOAD_MB overrides it; 0 disables the budget.
func TurnUploadBudgetBytes() int64 {
	if v := os.Getenv("GORILLA_OPENCODE_TURN_UPLOAD_MB"); v != "" {
		if mb, err := strconv.ParseFloat(v, 64); err == nil && mb >= 0 {
			return int64(mb * 1024 * 1024)
		}
	}
	return int64(CurrentConnProfile().UploadMB * 1024 * 1024)
}

// NonInteractiveDeadline bounds a headless run (-p).
//
// GORILLA OVERRIDE (2026-08-18): the interactive path deliberately has no
// timeout — a slow model on a slow link must not be killed mid-answer, and the
// user can press ESC. Headless has no user. Measured against a link that went
// silent without closing: it waited indefinitely, with no error and no output,
// which for a script or a cron job means hanging forever.
//
// 30 minutes: long enough for a large model on a bad link doing a multi-step
// agent turn, short enough that an unattended run fails and reports rather than
// wedging. GORILLA_OPENCODE_HEADLESS_TIMEOUT accepts a Go duration ("45m");
// "0" disables the deadline.
func NonInteractiveDeadline() time.Duration {
	if v := os.Getenv("GORILLA_OPENCODE_HEADLESS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return 30 * time.Minute
}

// FirstByteTimeout bounds how long to wait for a server's response HEADERS
// after the request body has finished uploading.
//
// GORILLA OVERRIDE (2026-08-18): measured against a live provider, not chosen
// defensively. httpclient.go deliberately set no ResponseHeaderTimeout, on the
// stated grounds that "first byte can be legitimately slow on a big model + slow
// link". That reasoning conflated two different things, and the second half of
// it is simply not how Go counts:
//
//   - Go's ResponseHeaderTimeout starts AFTER the request body is fully written.
//     A slow uplink therefore never counts against it. Uploading 100 KB at
//     2 KB/s spends 50 seconds of wall clock and zero seconds of this budget.
//   - What remains is genuine server-side thinking time before the first token,
//     which is real for reasoning models but is not unbounded.
//
// What made this urgent: NVIDIA NIM advertises models in /v1/models that it then
// black-holes. On 2026-08-18, of eight models drawn from its own catalogue, four
// returned an honest 404 in under 0.2s, ONE served normally with a first byte at
// 0.36s — and two, including the configured default, accepted the connection and
// returned nothing at all, forever. A bare curl hung identically, so this is the
// provider, not the client. But the client's response was to sit on it silently.
//
// 120 seconds is ~330x the measured first byte of the model that worked, which
// leaves enormous room for a slow reasoning model on a bad link, while still
// turning "hangs until the user gives up" into a stated failure.
//
// GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT accepts a Go duration ("5m"); "0" restores
// the old unbounded behaviour.
func FirstByteTimeout() time.Duration {
	if v := os.Getenv("GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return CurrentConnProfile().FirstByte
}

// StreamStallTimeout bounds a gap BETWEEN chunks once an answer has started.
//
// GORILLA OVERRIDE (2026-08-18): FirstByteTimeout only covers the wait for
// headers. A link that dies after the answer has begun leaves the socket open
// and the stream simply stops — headers arrived, so no header timeout can fire,
// and the read blocks forever. That is the exact shape of the "black hole" the
// satellite proxy reproduces, and it is what a satellite dropout actually looks
// like mid-answer.
//
// This is a STALL timer, not a wall clock: it resets on every chunk. A stream
// that is making progress is never killed, however slowly it crawls. Only a
// stream making NO progress for the whole window is.
//
// 90 seconds. Deliberately longer than the gap any healthy stream shows, since
// the cost of a false positive is a destroyed answer the user already paid for.
//
// GORILLA_OPENCODE_STREAM_STALL_TIMEOUT overrides it; "0" disables it.
func StreamStallTimeout() time.Duration {
	if v := os.Getenv("GORILLA_OPENCODE_STREAM_STALL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return CurrentConnProfile().StreamStall
}

// BrowserUserAgent is the User-Agent the web-fetch tool sends when reading a
// page.
//
// GORILLA OVERRIDE (2026-08-18): measured, then decided. The tool used to send
// an honest bot token, "gorilla-opencode/1.0 (+github...)". Measured that day, that
// token alone was the block: https://www.google.com/search returned 302 to the
// bot token and 200 to a Firefox token, from the same client, same second;
// news sites behaved the same way. lynx did not help — Reuters 401'd it too —
// so the lever is the User-Agent, not a browser subprocess.
//
// Reading a public page a human could read, while identified as the browser a
// human would use, is standard practice for every research tool and scraper on
// the web. The tool is not evading authentication or a paywall; it is declining
// to wear a "bot" badge that some sites reject reflexively. That was the owner's
// explicit call.
//
// This does NOT touch two other User-Agents on purpose:
//   - the provider auth handshakes (Antigravity, CodeAssist), which MUST
//     identify the real client or the OAuth flow breaks; and
//   - the JSON-API contact string in websearch.go, because some fair-access
//     APIs REQUIRE a specific identity — SEC EDGAR 403s anything that is not an
//     email-form contact (measured 2026-08-17). A browser token would break it.
//
// A default UA ages into a fingerprint of its own; refresh it periodically, or
// override per-machine with GORILLA_OPENCODE_USER_AGENT. Setting that variable
// to "honest" restores the identifying bot token.
func BrowserUserAgent() string {
	if v := os.Getenv("GORILLA_OPENCODE_USER_AGENT"); v != "" {
		if strings.EqualFold(v, "honest") {
			return honestUserAgent
		}
		return v
	}
	return defaultBrowserUserAgent
}

const (
	// A current, common desktop Firefox on Linux — authentic for this project's
	// own audience, which runs a Linux distribution with a Firefox fork.
	defaultBrowserUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
	// The original identifying token, restored by GORILLA_OPENCODE_USER_AGENT=honest.
	honestUserAgent = "gorilla-opencode/1.0 (+https://github.com/gorillanobakaa-dot/Gorilla.Opencode)"
)
