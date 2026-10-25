package models

import (
	"sort"
	"strings"
)

// GORILLA OVERRIDE (2026-10-25): where a model runs is decided by the HOST of
// the endpoint it was listed by, never by the name the user typed for it.
//
// Every OpenAI-compatible endpoint lands under the one "local" provider, and
// every model it listed got the id "local.<id>": LM Studio on 127.0.0.1 and
// NVIDIA's cloud at integrate.api.nvidia.com alike. On 2026-10-25 the coder's
// configured model had been deleted from LM Studio, the fallback reached for
// "a local model", and the one it found was local.nvidia/nemotron-3-ultra-
// 550b-a55b: the whole 13K-token prompt went to NVIDIA under a label that said
// it stayed on the machine.
//
// So an id now says where the model runs: "local." only for an endpoint on a
// loopback address, "remote." for every other host. A config written before
// this keeps working through RenamedEndpointID.

const (
	// LocalIDPrefix starts the id of a model served on this machine.
	LocalIDPrefix = "local."
	// RemoteIDPrefix starts the id of a model served by an OpenAI-compatible
	// endpoint on any other host (NVIDIA NIM, a LAN box, a hosted gateway).
	RemoteIDPrefix = "remote."
)

// EndpointIDPrefix is the id prefix for models listed by baseURL.
func EndpointIDPrefix(baseURL string) string {
	if isLoopback(baseURL) {
		return LocalIDPrefix
	}
	return RemoteIDPrefix
}

// IsOnMachine reports whether id is registered AND routed to an endpoint on a
// loopback address. An unregistered id is not on the machine: nothing is known
// about where it would go.
func IsOnMachine(id ModelID) bool {
	r, ok := localRoute[id]
	return ok && isLoopback(r.BaseURL)
}

// IsRemoteEndpointModel reports whether id is routed to an OpenAI-compatible
// endpoint that is NOT on this machine.
func IsRemoteEndpointModel(id ModelID) bool {
	r, ok := localRoute[id]
	return ok && !isLoopback(r.BaseURL)
}

// RenamedEndpointID maps an id written before remote endpoints got their own
// prefix ("local.nvidia/x", served by integrate.api.nvidia.com) to the id it
// has now ("remote.nvidia/x"). ok is false when id is not such an id, or when
// the old id still resolves as it is.
func RenamedEndpointID(id ModelID) (ModelID, bool) {
	rest, had := strings.CutPrefix(string(id), LocalIDPrefix)
	if !had {
		return "", false
	}
	if _, still := SupportedModels[id]; still {
		return "", false
	}
	renamed := ModelID(RemoteIDPrefix + rest)
	if _, ok := SupportedModels[renamed]; ok {
		return renamed, true
	}
	return "", false
}

// stripEndpointPrefix drops "local." or "remote." from an endpoint model id.
func stripEndpointPrefix(id string) string {
	if rest, ok := strings.CutPrefix(id, LocalIDPrefix); ok {
		return rest
	}
	return strings.TrimPrefix(id, RemoteIDPrefix)
}

// AvailableOnMachine says whether id can be used now without leaving the
// machine, and if not, why not, in words for a person.
//
// "Now" means: the endpoint on loopback listed it at this launch (for LM Studio
// a listed model is loaded or loadable on demand), it is a chat model, and the
// last probe did not find it gone.
func AvailableOnMachine(id ModelID) (bool, string) {
	m, ok := SupportedModels[id]
	if !ok {
		return false, "not listed by any model server running on this machine"
	}
	r, routed := localRoute[id]
	if !routed {
		return false, "not served by an endpoint (a cloud provider)"
	}
	if !isLoopback(r.BaseURL) {
		return false, "served by " + r.Endpoint + " at " + hostOf(r.BaseURL) + ", not on this machine"
	}
	if isNotAChatModel(m.APIModel) {
		return false, "not a chat model"
	}
	if v, seen := ProbeVerdictFor(id); seen && v.Dead() {
		return false, "the last probe found it " + v.Label()
	}
	return true, ""
}

// OnMachineModels lists every model available on this machine now, in the
// order the picker would offer them.
func OnMachineModels() []ModelID {
	var ids []ModelID
	for id := range localRoute {
		if ok, _ := AvailableOnMachine(id); ok {
			ids = append(ids, id)
		}
	}
	return candidateOrder(sortedIDs(ids))
}

// OnMachineEndpointNames names the endpoints on this machine that listed
// models at this launch ("lmstudio", "Ollama"), sorted.
func OnMachineEndpointNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range localRoute {
		if isLoopback(r.BaseURL) && r.Endpoint != "" && !seen[r.Endpoint] {
			seen[r.Endpoint] = true
			out = append(out, r.Endpoint)
		}
	}
	sort.Strings(out)
	return out
}

// LocalStateOf is what the runtime said about a listed model ("loaded",
// "not-loaded"), or "" when the endpoint does not say.
func LocalStateOf(id ModelID) string { return SupportedModels[id].LocalState }

func hostOf(baseURL string) string {
	c := CanonicalEndpointURL(baseURL)
	if _, after, ok := strings.Cut(c, "//"); ok {
		c = after
	}
	host, _, _ := strings.Cut(c, "/")
	return host
}
