package config

// GORILLA OVERRIDE (2026-10-25): an unusable model is never swapped silently.
//
// What happened: the owner deleted deepseek-r1-0528-qwen3-8b from LM Studio
// while config.json still named it for all four agents. A headless run
// (gorilla-opencode -p) printed
//
//	note: agent "coder" model "local.deepseek/deepseek-r1-0528-qwen3-8b" is
//	unusable (unknown model) — falling back to "local.nvidia/nemotron-3-ultra-550b-a55b"
//
// and sent the whole 13K-token prompt to integrate.api.nvidia.com. The note was
// true and useless: it was printed as the prompt left the machine. Two faults
// made it: every OpenAI-compatible endpoint was called "local" whatever its
// host (fixed in models/onmachine.go), and the fallback chose for the person.
//
// The rules now, each with a test in model_choice_test.go:
//
//  1. An agent whose model is unusable is HELD. It gets a placeholder model so
//     the program can start, and nothing is sent to any model while any agent
//     is held (CheckModelChosen, called by every provider before a request).
//  2. With a screen, the person chooses: the TUI opens the model picker with
//     the reason, listing only models available now.
//  3. Without one (-p, ACP, plain), ChooseForRunWithoutScreen takes the first
//     model of preferredLocalModels that is on this machine and listed now,
//     and says which and why. A remote model only if the owner listed it there
//     by hand. Nothing found: the run is refused, non-zero, nothing sent.
//  4. The choice made for a run without a screen is not written to
//     config.json. The next run decides again, from what is there then.

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// DefaultPreferredLocalModels is the order used when config.json has no
// preferredLocalModels: the owner's models on this laptop, best first
// (benchmark of 2026-10-25: Qwen3.6-35B-A3B passed every harness test on the
// GPU; Gemma 4 26B-A4B runs CPU-only; Gemma 4 12B QAT is the small one). Only
// ids on this machine, so the default can never send anything away.
var DefaultPreferredLocalModels = []string{
	"local.qwen3.6-35b-a3b",
	"local.gemma-4-26b-a4b-it",
	"local.google/gemma-4-12b-qat",
}

// PendingModelChoice is one agent held because its configured model is
// unusable.
type PendingModelChoice struct {
	Agent AgentName
	// Configured is the model config.json named, the one that is unusable.
	Configured models.ModelID
	// Reason is one line for a person: "not in lmstudio any more".
	Reason string
	// Placeholder is what the agent holds so the program can start. Nothing is
	// sent through it.
	Placeholder models.ModelID
}

// ErrModelChoicePending is returned, wrapped, by CheckModelChosen.
var ErrModelChoicePending = errors.New("no model chosen yet")

var (
	pendingMu      sync.RWMutex
	pendingChoices = map[AgentName]PendingModelChoice{}
)

func pendingChoiceFor(name AgentName) (PendingModelChoice, bool) {
	pendingMu.RLock()
	defer pendingMu.RUnlock()
	p, ok := pendingChoices[name]
	return p, ok
}

func clearPendingChoice(name AgentName) {
	pendingMu.Lock()
	delete(pendingChoices, name)
	pendingMu.Unlock()
}

// restorePendingChoice puts back the hold an agent had before a failed change.
func restorePendingChoice(name AgentName, prev PendingModelChoice, had bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	if had {
		pendingChoices[name] = prev
	} else {
		delete(pendingChoices, name)
	}
}

func resetPendingChoices() {
	pendingMu.Lock()
	pendingChoices = map[AgentName]PendingModelChoice{}
	pendingMu.Unlock()
}

// PendingModelChoices lists the held agents, the coder first, then by name.
func PendingModelChoices() []PendingModelChoice {
	pendingMu.RLock()
	out := make([]PendingModelChoice, 0, len(pendingChoices))
	for _, p := range pendingChoices {
		out = append(out, p)
	}
	pendingMu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Agent == AgentCoder) != (out[j].Agent == AgentCoder) {
			return out[i].Agent == AgentCoder
		}
		return out[i].Agent < out[j].Agent
	})
	return out
}

// ModelChoicePending reports whether any agent is held.
func ModelChoicePending() bool {
	pendingMu.RLock()
	defer pendingMu.RUnlock()
	return len(pendingChoices) > 0
}

// ClearPendingModelChoices ends every hold. For the TUI, after the person has
// picked a model and the helper agents have followed it.
func ClearPendingModelChoices() { resetPendingChoices() }

// HoldModelChoiceForTest holds an agent as validateAgent would, for tests in
// other packages, and returns the function that ends the hold. Nothing in the
// program calls it.
func HoldModelChoiceForTest(name AgentName, configured models.ModelID, reason string) (release func()) {
	pendingMu.Lock()
	pendingChoices[name] = PendingModelChoice{Agent: name, Configured: configured, Reason: reason}
	pendingMu.Unlock()
	return func() { clearPendingChoice(name) }
}

// CheckModelChosen is the gate every provider passes before a request: while
// any agent is held, nothing is sent to any model. One gate for all, not one
// per agent, because the helpers (title, summary, task) run on behalf of the
// same conversation, and a placeholder must never answer for anyone.
func CheckModelChosen() error {
	held := PendingModelChoices()
	if len(held) == 0 {
		return nil
	}
	p := held[0]
	return fmt.Errorf("%w: agent %s model %s is unusable (%s); choose a model first (/model), nothing has been sent",
		ErrModelChoicePending, p.Agent, p.Configured, p.Reason)
}

// PendingChoiceSummary is the one line the picker shows above its list:
// `coder model "X" is not in lmstudio any more`.
func PendingChoiceSummary() string {
	held := PendingModelChoices()
	if len(held) == 0 {
		return ""
	}
	p := held[0]
	line := fmt.Sprintf("%s model %q is %s", p.Agent, string(p.Configured), p.Reason)
	if len(held) > 1 {
		line += fmt.Sprintf(" (%d agents)", len(held))
	}
	return line
}

// holdAgentForChoice is what validateAgent does with an unusable model.
func holdAgentForChoice(name AgentName, agent Agent, reason string) error {
	why := unusableReason(agent.Model, reason)
	placeholder := placeholderModel()
	if placeholder == "" {
		if !setDefaultModelForAgent(name) {
			return fmt.Errorf("agent %s: model %s is %s, and no other model is available at all", name, agent.Model, why)
		}
		placeholder = cfg.Agents[name].Model
	} else {
		assignAgentModel(name, placeholder)
	}
	pendingMu.Lock()
	pendingChoices[name] = PendingModelChoice{
		Agent: name, Configured: agent.Model, Reason: why, Placeholder: placeholder,
	}
	pendingMu.Unlock()
	// A run without a screen says it once, in ChooseForRunWithoutScreen,
	// with what was done about it.
	if !runWithoutScreen {
		fmt.Fprintf(os.Stderr, "note: agent %q model %q is unusable (%s); nothing is sent until a model is chosen\n",
			name, agent.Model, why)
	}
	return nil
}

// placeholderModel is a model on this machine to hold an agent with: the
// first available of the preferred list, else the first available at all.
// "" when this machine serves nothing.
func placeholderModel() models.ModelID {
	if id, _, _ := preferredAvailable(false); id != "" {
		return id
	}
	if on := models.OnMachineModels(); len(on) > 0 {
		return on[0]
	}
	return ""
}

// assignAgentModel points an agent at a model, with the same max-tokens and
// reasoning defaults setDefaultModelForAgent gives.
func assignAgentModel(name AgentName, id models.ModelID) {
	maxTokens := int64(5000)
	if name == AgentTitle {
		maxTokens = 80
	}
	reasoning := ""
	if info, ok := models.SupportedModels[id]; ok && info.CanReason {
		reasoning = "medium"
	}
	cfg.Agents[name] = Agent{Model: id, MaxTokens: maxTokens, ReasoningEffort: reasoning}
}

// unusableReason turns validateAgent's terse reason into a line for a person.
func unusableReason(id models.ModelID, reason string) string {
	if reason != "unknown model" {
		return reason
	}
	if !strings.HasPrefix(string(id), models.LocalIDPrefix) {
		return "not a model any provider lists"
	}
	servers := models.OnMachineEndpointNames()
	if len(servers) == 0 {
		return "no model server on this machine answered"
	}
	return "not in " + strings.Join(servers, " or ") + " any more"
}

// preferredList is the owner's list, or the default when config.json has
// none. explicit says the owner wrote it.
func preferredList() (list []string, explicit bool) {
	if cfg != nil && cfg.PreferredLocalModels != nil {
		return cfg.PreferredLocalModels, true
	}
	return DefaultPreferredLocalModels, false
}

// preferredAvailable returns the first model of the preferred list usable now,
// with why it was taken, and why each one before it was not. A remote model is
// taken only when allowRemote is set and the owner wrote the list.
func preferredAvailable(allowRemote bool) (models.ModelID, string, []string) {
	list, explicit := preferredList()
	var tried []string
	for _, raw := range list {
		id := models.ModelID(strings.TrimSpace(raw))
		if renamed, ok := models.RenamedEndpointID(id); ok {
			id = renamed
		}
		ok, why := models.AvailableOnMachine(id)
		if ok {
			state := models.LocalStateOf(id)
			if state == "" {
				state = "listed"
			}
			return id, "on this machine, " + state + " now", tried
		}
		if allowRemote && explicit && remoteUsable(id) {
			return id, "NOT on this machine; you listed it in preferredLocalModels", tried
		}
		tried = append(tried, fmt.Sprintf("%s: %s", raw, why))
	}
	return "", "", tried
}

// remoteUsable reports whether a model off this machine can be reached: an
// endpoint that listed it at this launch, or a cloud provider holding a key.
func remoteUsable(id models.ModelID) bool {
	m, ok := models.SupportedModels[id]
	if !ok {
		return false
	}
	if models.IsRemoteEndpointModel(id) {
		return true
	}
	if m.Provider == models.ProviderLocal {
		return false
	}
	if getProviderAPIKey(m.Provider) != "" {
		return true
	}
	p, ok := cfg.Providers[m.Provider]
	return ok && !p.Disabled && p.APIKey != ""
}

// legacyEndpointID is the id a remote endpoint's model had before remote
// endpoints got "remote." ids (models/onmachine.go): "remote.x" was "local.x".
// "" for any other id. Lists the user wrote (hidden, bookmarks) keep the old id
// and still mean the same model.
func legacyEndpointID(id string) string {
	if rest, ok := strings.CutPrefix(id, models.RemoteIDPrefix); ok {
		return models.LocalIDPrefix + rest
	}
	return ""
}

// runWithoutScreen is set before Load by a run nobody is watching (-p, ACP).
var runWithoutScreen bool

// SetRunWithoutScreen says, before Load, that this run has no screen to ask
// on. Load then lists remote endpoints only when something configured names a
// remote model (remoteEndpointsNeeded).
//
// GORILLA OVERRIDE (2026-10-25): listing is not free of egress. Every launch
// asked integrate.api.nvidia.com for its model list, with the owner's key, even
// when every agent ran on LM Studio. With a screen the picker needs that list;
// a -p run on a local model does not, and the door stays shut.
func SetRunWithoutScreen(v bool) { runWithoutScreen = v }

// remoteEndpointsNeeded reports whether an agent or preferredLocalModels names
// a model of a remote endpoint ("remote." id). A "local." id never needs one:
// if it is not on this machine, it is unusable, and a remote endpoint is not
// asked to make up for it.
func remoteEndpointsNeeded() bool {
	for _, a := range cfg.Agents {
		if strings.HasPrefix(string(a.Model), models.RemoteIDPrefix) {
			return true
		}
	}
	for _, id := range cfg.PreferredLocalModels {
		if strings.HasPrefix(strings.TrimSpace(id), models.RemoteIDPrefix) {
			return true
		}
	}
	return false
}

// ChooseForRunWithoutScreen settles every held agent for a run nobody is
// watching (-p, ACP, plain): the first usable model of preferredLocalModels,
// said on stderr with the reason, or an error that refuses the run. It writes
// nothing to config.json.
func ChooseForRunWithoutScreen() error {
	held := PendingModelChoices()
	if len(held) == 0 {
		return nil
	}
	pick, why, tried := preferredAvailable(true)
	if pick == "" {
		var b strings.Builder
		for _, p := range held {
			fmt.Fprintf(&b, "agent %s: model %s is unusable (%s)\n", p.Agent, p.Configured, p.Reason)
		}
		b.WriteString("No model of preferredLocalModels is available on this machine now:\n")
		for _, t := range tried {
			b.WriteString("  " + t + "\n")
		}
		b.WriteString("Nothing was sent. Load one of them (for LM Studio: lms load <model>), " +
			"or name an available model in config.json (agents.<name>.model or preferredLocalModels).")
		return errors.New(b.String())
	}
	model := models.SupportedModels[pick]
	for _, p := range held {
		assignAgentModel(p.Agent, pick)
		if err := validateAgentMaxTokens(cfg, p.Agent, cfg.Agents[p.Agent], model); err != nil {
			return err
		}
		clearPendingChoice(p.Agent)
		fmt.Fprintf(os.Stderr, "note: agent %q model %q is unusable (%s) - using %q from preferredLocalModels (%s)\n",
			p.Agent, p.Configured, p.Reason, pick, why)
	}
	return nil
}
