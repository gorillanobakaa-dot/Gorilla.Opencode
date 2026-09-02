// GORILLA (2026-09-02): a command must not ask for a tool the agent has not got.
//
// Found the only way it could be: a user ran /review, and the model replied
// "I do not have access to a tool named review". It was telling the truth.
// tool.review was switched off in the loadout, so the tool was never handed to
// it — but /review dispatched a prompt saying "use the review tool" regardless,
// and the failure surfaced as the model apparently being broken.
//
// The user had done nothing wrong. The low-bandwidth trim in /context switches
// off seven components at once, including review, research, web search and
// fetch. It writes that to loadout.json and nothing ever puts it back: the
// connection profile can return to unconstrained while the tools stay off. The
// only evidence left is a model that seems to have forgotten how to work.
//
// /osint already guarded itself this way. /review and /port did not, and /port
// was written this session with the same hole, which is how a pattern that
// lives at one call site out of three eventually gets missed.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// toolRowName is the label of a loadout row, for telling someone what to look
// for in /context. Falls back to the raw id, which is still findable.
func toolRowName(id string) string {
	for _, c := range config.LoadoutComponents {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

// disabledToolWarning returns a message when a command's tool is switched off,
// or "" when it is armed and the command may proceed.
//
// It names the row rather than only the id, because the row is what is on
// screen in /context, and it says how to turn it on. "Feature unavailable" with
// no route back is the same dead end as the silent version.
func disabledToolWarning(componentID, command string) string {
	if config.LoadoutEnabled(componentID) {
		return ""
	}
	return fmt.Sprintf(
		"%s cannot run: \"%s\" is switched OFF, so the tool is not given to the model. "+
			"Turn it on with /context, move to \"%s\" and press space.\n\n"+
			"If you did not turn this off yourself, the low-bandwidth trim in /context "+
			"does it — it switches off several tools at once and does not put them back "+
			"when your connection improves.",
		command, toolRowName(componentID), toolRowName(componentID))
}

// requireTool is the guard a command calls before dispatching work that needs a
// tool. It returns nil when the tool is available.
func (a appModel) requireTool(componentID, command string) tea.Cmd {
	if msg := disabledToolWarning(componentID, command); msg != "" {
		return util.ReportWarn(msg)
	}
	return nil
}
