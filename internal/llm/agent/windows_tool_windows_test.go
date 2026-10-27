package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// TestWindowsToolIsRegisteredAndInTheToolTable: on Windows the coder gets
// the read-only windows tool, the YOUR TOOLS table names it, and its loadout
// row carries a measured cost.
func TestWindowsToolIsRegisteredAndInTheToolTable(t *testing.T) {
	loadRoleConfig(t)
	set := CoderAgentTools(nil, nil, nil, nil, nil)
	if !toolNames(set)[tools.WindowsToolName] {
		t.Fatal("the coder has no windows tool on Windows")
	}
	table := tools.ToolTableBlock(set, false)
	var line string
	for _, l := range strings.Split(table, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), tools.WindowsToolName+" ") {
			line = l
		}
	}
	if !strings.Contains(line, "registry") {
		t.Errorf("the tool table has no windows line naming the registry:\n%s", table)
	}
	CalibrateLoadout(nil, nil, nil, nil, nil)
	for _, c := range config.LoadoutComponents {
		if c.ID == config.WindowsComponentID {
			if tk := config.ComponentTokens(c); tk <= 0 {
				t.Errorf("the windows row costs %d tokens on Windows", tk)
			}
			return
		}
	}
	t.Error("no loadout row for the windows tool")
}
