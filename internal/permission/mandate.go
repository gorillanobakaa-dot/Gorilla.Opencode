package permission

// GORILLA OVERRIDE (2026-11-22, Fieldkit plan row F0.7, OpenCode half): one
// pre-approval mandate per job.
//
// WHY. The Fieldkit audit found OpenCode asking for the same approvals again and
// again during a job. The owner approves a job ONCE, at its start; mid-job a
// missing approval takes its fallback (a refusal handed back to the model),
// never a wait for a person.
//
// WHAT. GORILLA_MANDATE_FILE names a JSON file: {"allow":["bash","edit",...]}.
// While it is set and readable:
//   - a request for a tool on the list, with no carve-out (mustAskAnyway:
//     irreversible, hard stop, path outside the roots), is approved;
//   - every other request that would have raised a dialog is refused at once.
//     No dialog is published and nothing waits.
// Irreversible and hard-stop requests are never approved by the mandate.
//
// FAIL CLOSED. A named file that is missing, unreadable or not JSON gives an
// empty allow list: nothing is approved and nothing prompts. The setting comes
// from the process environment, which the agent's bash tool cannot change.

import (
	"encoding/json"
	"os"
	"strings"
)

type mandate struct {
	allow map[string]bool
}

// loadMandate returns nil when no mandate is in force.
func loadMandate() *mandate {
	path := strings.TrimSpace(os.Getenv("GORILLA_MANDATE_FILE"))
	if path == "" {
		return nil
	}
	m := &mandate{allow: map[string]bool{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	var f struct {
		Allow []string `json:"allow"`
	}
	if json.Unmarshal(b, &f) != nil {
		return m
	}
	for _, t := range f.Allow {
		m.allow[strings.TrimSpace(t)] = true
	}
	return m
}

func (m *mandate) covers(tool string) bool {
	return m != nil && (m.allow[tool] || m.allow["*"])
}
