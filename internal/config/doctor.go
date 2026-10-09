package config

// GORILLA OVERRIDE (2026-10-05): the model doctor.
//
// On that day the owner found, by looking at his own screen, that /update had
// "worked" and left him with: two retired models at the top of a list, four
// rows nobody could tell apart, a ranking printed upside down, a coder agent
// pointed at a model its provider no longer listed, a key field that had never
// accepted his key, and a default NVIDIA model that sorted first only because
// its name begins with "01". Each was then found with a one-off script.
//
// This file is those scripts, kept. Every check is a rule over what is in
// memory and on disk: no network, no model, no judgement, the same answer every
// time. /update runs it after refreshing, /model and /provider run it before
// they show anything, and `gorilla-opencode models doctor` prints it.
//
// A check either FIXES (when there is exactly one right repair and it loses
// nothing) or REPORTS, in a sentence a person can act on. It never guesses.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// Finding is one thing the doctor saw.
type Finding struct {
	// Fixed is true when the doctor repaired it; false means the user must act.
	Fixed bool
	Text  string
}

func (f Finding) String() string {
	if f.Fixed {
		return "fixed: " + f.Text
	}
	return "PROBLEM: " + f.Text
}

// keyPrefixes is what each vendor's key starts with. Used only to say "this
// looks like another vendor's key", never to reject one: vendors change these.
//
// Several per vendor where a vendor has several. Google issued "AIza..." keys
// for years and now issues "AQ...." ones: the owner's working key, proven by a
// real request on 2026-10-05, starts with AQ. and the first version of this
// table would have called it somebody else's.
var keyPrefixes = map[models.ModelProvider][]string{
	models.ProviderAnthropic:  {"sk-ant-"},
	models.ProviderOpenRouter: {"sk-or-"},
	models.ProviderGROQ:       {"gsk_"},
	models.ProviderCerebras:   {"csk-"},
	models.ProviderXAI:        {"xai-"},
	models.ProviderGemini:     {"AIza", "AQ."},
}

func startsWithAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return len(prefixes) == 0
}

// agentSetter moves an agent to a model. A variable so the TUI can route the
// coder through the live agent (which must rebuild its provider) and tests can
// observe the calls.
type AgentSetter func(name AgentName, to models.ModelID) error

// RunModelDoctor runs every check. With set == nil nothing is changed and
// everything is reported. cacheDir is CacheBase().
func RunModelDoctor(cacheDir string, set AgentSetter) []Finding {
	if cfg == nil {
		return nil
	}
	var out []Finding
	out = append(out, doctorAgents(set)...)
	out = append(out, doctorKeys()...)
	out = append(out, doctorEndpoints(set != nil)...)
	out = append(out, doctorNames()...)
	out = append(out, doctorRanks()...)
	out = append(out, doctorStaleness(cacheDir)...)
	return out
}

// 1. Every agent must be on a model that is registered and not known dead.
func doctorAgents(set AgentSetter) []Finding {
	var out []Finding
	// GORILLA OVERRIDE (2026-10-09): a CONFIGURED sub-coder or planner on a
	// dead model fails every spawn of that role, so it is checked like the
	// others. A derived one is only a copy of its parent, which is checked
	// already; moving it through the setter would write a default to config.json.
	for _, name := range []AgentName{AgentCoder, AgentSummarizer, AgentTask, AgentTitle, AgentSubCoder, AgentPlan} {
		if isDerivedRoleAgent(name) {
			continue
		}
		cur := cfg.Agents[name].Model
		// Moved at load, in memory only (validateAgent). Say it, and with a
		// setter write it down so it is not silently redone at every launch.
		if mv, moved := legacyMoves[name]; moved && cur == mv[1] {
			toName := models.SupportedModels[mv[1]].Name
			if set == nil {
				out = append(out, Finding{Text: fmt.Sprintf("the %s agent is saved as %s, which is no longer offered. It is being run on %s instead; this is not yet written to config.json.", name, mv[0], toName)})
			} else if err := set(name, mv[1]); err == nil {
				delete(legacyMoves, name)
				out = append(out, Finding{Fixed: true, Text: fmt.Sprintf("the %s agent was saved as %s, which is no longer offered. Now saved as %s.", name, mv[0], toName)})
			}
			continue
		}
		if cur == "" {
			continue
		}
		why, to := "", models.ModelID("")
		if _, ok := models.SupportedModels[cur]; !ok {
			why = "is no longer offered by its provider"
			to = models.LegacyModelIDs[cur]
		} else if v, seen := models.ProbeVerdictFor(cur); seen && v.Dead() {
			why = v.Label()
			to = models.PreferredOnSameEndpoint(cur)
		}
		if why == "" {
			continue
		}
		if _, ok := models.SupportedModels[to]; !ok || to == cur {
			out = append(out, Finding{Text: fmt.Sprintf("the %s agent is set to %s, which %s, and there is no obvious successor. Choose a model with /model.", name, cur, why)})
			continue
		}
		if set == nil {
			out = append(out, Finding{Text: fmt.Sprintf("the %s agent is set to %s, which %s. Successor: %s.", name, cur, why, models.SupportedModels[to].Name)})
			continue
		}
		if err := set(name, to); err != nil {
			out = append(out, Finding{Text: fmt.Sprintf("the %s agent is set to %s, which %s, and could not be moved to %s: %v", name, cur, why, to, err)})
			continue
		}
		out = append(out, Finding{Fixed: true, Text: fmt.Sprintf("the %s agent was on %s, which %s. Moved to %s.", name, cur, why, models.SupportedModels[to].Name)})
	}
	return out
}

// 2. A saved key must be one unbroken run of printable characters. A key with a
// control character or a space inside it is a failed paste, and it fails every
// request with an error that points nowhere near the cause.
func doctorKeys() []Finding {
	var out []Finding
	check := func(where, key string, prefixes ...string) {
		if key == "" || key == "oauth-login" {
			return
		}
		switch {
		case strings.TrimSpace(key) == "" || hasControl(key):
			out = append(out, Finding{Text: fmt.Sprintf("the saved key for %s is %d character(s) and contains invisible control characters, so it is not a key: it is what a failed paste leaves behind. Enter it again with /provider (press r on the row).", where, len([]rune(key)))})
		case strings.ContainsAny(strings.TrimSpace(key), " \t"):
			out = append(out, Finding{Text: fmt.Sprintf("the saved key for %s has a space inside it. A key is one unbroken word. Enter it again with /provider.", where)})
		case len(strings.TrimSpace(key)) < 20:
			out = append(out, Finding{Text: fmt.Sprintf("the saved key for %s is only %d characters, too short to be a key. Enter it again with /provider.", where, len(strings.TrimSpace(key)))})
		case !startsWithAny(strings.TrimSpace(key), prefixes):
			out = append(out, Finding{Text: fmt.Sprintf("the saved key for %s does not start with %s, which is how that provider's keys start. It may be another provider's key.", where, strings.Join(prefixes, " or "))})
		}
	}
	gone := make([]string, 0, len(discardedKeys))
	for where := range discardedKeys {
		gone = append(gone, where)
	}
	sort.Strings(gone)
	for _, where := range gone {
		out = append(out, Finding{Text: fmt.Sprintf("the key saved for %s is %d invisible control character(s), not a key: it is what a failed paste leaves behind, and it is being ignored. Enter the real key with /provider (press r on the row), or remove the entry.", where, discardedKeys[where])})
	}
	provs := make([]string, 0, len(cfg.Providers))
	for p := range cfg.Providers {
		provs = append(provs, string(p))
	}
	sort.Strings(provs)
	for _, p := range provs {
		pr := cfg.Providers[models.ModelProvider(p)]
		if pr.Disabled && pr.APIKey == "" {
			continue
		}
		check(p, pr.APIKey, keyPrefixes[models.ModelProvider(p)]...)
	}
	for _, e := range cfg.LocalEndpoints {
		if e.Disabled || e.APIKey == "" {
			continue
		}
		var prefix []string
		if strings.Contains(e.BaseURL, "integrate.api.nvidia.com") {
			prefix = []string{"nvapi-"}
		}
		if isLoopbackURL(e.BaseURL) {
			// A local runtime takes any placeholder as a key; only a broken
			// paste is worth reporting there.
			if hasControl(e.APIKey) {
				check("the endpoint "+e.Name, e.APIKey)
			}
			continue
		}
		check("the endpoint "+e.Name, e.APIKey, prefix...)
	}
	return out
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func isLoopbackURL(u string) bool {
	c := models.CanonicalEndpointURL(u)
	return strings.Contains(c, "//localhost") || strings.Contains(c, "//127.") || strings.Contains(c, "//[::1]")
}

// 3. Two endpoints that point at the same server show every model twice and
// take each other's routes.
//
// With fix, the spare entries are removed and one is kept: the one whose name
// the registered models are routed through, else the one holding a key, else
// the first. Nothing is lost by it: both entries named the same server, and the
// kept one is the one already in use. Without fix it only reports.
func doctorEndpoints(fix bool) []Finding {
	var out []Finding
	keyed := map[string]bool{}
	for _, e := range cfg.LocalEndpoints {
		keyed[e.Name] = e.APIKey != ""
	}
	byURL := map[string][]string{}
	var order []string
	for _, e := range cfg.LocalEndpoints {
		if e.Disabled || e.BaseURL == "" {
			continue
		}
		c := models.CanonicalEndpointURL(e.BaseURL)
		if len(byURL[c]) == 0 {
			order = append(order, c)
		}
		byURL[c] = append(byURL[c], e.Name)
	}
	for _, c := range order {
		names := byURL[c]
		if len(names) < 2 {
			continue
		}
		if !fix {
			out = append(out, Finding{Text: fmt.Sprintf("%d saved connections point at the same server (%s): %s. Keep one and remove the rest in /connect.", len(names), c, strings.Join(names, ", "))})
			continue
		}
		keep := ""
		for _, n := range names {
			if models.EndpointHasModels(n) {
				keep = n
				break
			}
		}
		if keep == "" {
			for _, n := range names {
				if keyed[n] {
					keep = n
					break
				}
			}
		}
		if keep == "" {
			keep = names[0]
		}
		for _, n := range names {
			if n == keep {
				continue
			}
			if err := RemoveLocalEndpoint(n); err != nil {
				out = append(out, Finding{Text: fmt.Sprintf("the connection %s is a second entry for the same server as %s and could not be removed: %v", n, keep, err)})
				continue
			}
			out = append(out, Finding{Fixed: true, Text: fmt.Sprintf("removed the connection %s: it was a second entry for the same server as %s (%s), which is kept.", n, keep, c)})
		}
	}
	return out
}

// 4. No two rows of one provider's list may read the same.
func doctorNames() []Finding {
	type key struct {
		prov models.ModelProvider
		ep   string
		name string
	}
	seen := map[key][]string{}
	for id, m := range models.SupportedModels {
		k := key{m.Provider, models.LocalEndpointFor(id), m.Name}
		seen[k] = append(seen[k], string(id))
	}
	var lines []string
	for k, ids := range seen {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		where := string(k.prov)
		if k.ep != "" {
			where = k.ep
		}
		lines = append(lines, fmt.Sprintf("%s lists %d different models under the one name %q (%s), so they cannot be told apart in /model.", where, len(ids), k.name, strings.Join(ids, ", ")))
	}
	sort.Strings(lines)
	out := make([]Finding, 0, len(lines))
	for _, l := range lines {
		out = append(out, Finding{Text: l})
	}
	return out
}

// 5. Where a sign-in provider ranks its models, the best is rank 1 and no two
// share a rank. The picker prints "1 = best" and sorts ascending.
func doctorRanks() []Finding {
	var out []Finding
	for _, p := range []models.ModelProvider{models.ProviderChatGPT, models.ProviderAntigravity} {
		ranks := map[int]int{}
		lowest := 0
		for _, m := range models.SupportedModels {
			if m.Provider != p || m.Rank <= 0 {
				continue
			}
			ranks[m.Rank]++
			if lowest == 0 || m.Rank < lowest {
				lowest = m.Rank
			}
		}
		if len(ranks) == 0 {
			continue
		}
		if lowest != 1 {
			out = append(out, Finding{Text: fmt.Sprintf("the %s list starts at rank %d, not 1, so /model shows it in the wrong order. Run /update.", p, lowest)})
		}
		for r, n := range ranks {
			if n > 1 {
				out = append(out, Finding{Text: fmt.Sprintf("%d %s models share rank %d, so their order in /model is arbitrary. Run /update.", n, p, r)})
				break
			}
		}
	}
	return out
}

// staleAfter is how old a fetched list may be before the doctor says so.
const staleAfter = 7 * 24 * time.Hour

// 6. Lists that have not been fetched lately, and typed rankings that no longer
// describe what a provider serves.
func doctorStaleness(cacheDir string) []Finding {
	var out []Finding
	old := func(label string, age time.Duration, ok bool, signedIn bool) {
		if signedIn && ok && age > staleAfter {
			out = append(out, Finding{Text: fmt.Sprintf("the %s model list was last fetched %d days ago. Run /update.", label, int(age.Hours()/24))})
		}
	}
	_, ag := cfg.Providers[models.ProviderAntigravity]
	age, ok := models.AntigravityCatalogueAge(cacheDir)
	old("Antigravity", age, ok, ag)
	_, cg := cfg.Providers[models.ProviderChatGPT]
	age, ok = models.ChatGPTCatalogueAge(cacheDir)
	old("ChatGPT", age, ok, cg)
	if ProviderAPIKey(models.ProviderGemini) != "" {
		if age, ok = models.GeminiCatalogueAge(cacheDir); !ok {
			out = append(out, Finding{Text: "the Gemini model list has never been fetched with your key, so it is the list typed into this program. Run /update."})
		} else {
			old("Gemini", age, ok, true)
		}
	}

	// A remote connection whose models have never been asked anything: its
	// default is a guess until /update has run the checks.
	for _, name := range models.ProbedEndpoints() {
		if !models.EndpointWasProbed(name) {
			out = append(out, Finding{Text: fmt.Sprintf("no model on %s has been checked yet, so its default model is a guess. Run /update.", name)})
		}
	}
	return out
}

// DoctorSummary renders findings for a status line: fixes first, then problems.
func DoctorSummary(fs []Finding) []string {
	var fixed, problems []string
	for _, f := range fs {
		if f.Fixed {
			fixed = append(fixed, f.String())
		} else {
			problems = append(problems, f.String())
		}
	}
	return append(fixed, problems...)
}
