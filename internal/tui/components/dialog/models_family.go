package dialog

import (
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// collapseFamilies turns the flat provider list into one row per model family.
// The row stands for the variant of the chosen effort (m.effort), so Enter
// sends exactly the backend id for that effort. Families with no effort choice
// stay a single plain row. current is the saved model: its effort is the
// starting point for its family.
func (m *modelDialogCmp) collapseFamilies(current models.ModelID) {
	groups := models.GroupFamilies(m.models)
	if m.effort == nil {
		m.effort = map[string]string{}
	}
	rows := make([]models.Model, 0, len(groups))
	for _, g := range groups {
		for _, v := range g.Variants {
			if v.ID == current && m.effort[g.Key] == "" {
				if _, _, e := models.FamilyOf(v); e != "" {
					m.effort[g.Key] = e
				}
			}
		}
		rows = append(rows, familyRow(g, m.effort[g.Key]))
	}
	m.families = groups
	m.models = rows
}

// cycleEffort moves the highlighted family one effort level up or down and
// swaps the row to the variant for that level.
func (m *modelDialogCmp) cycleEffort(delta int) {
	if m.selectedIdx >= len(m.models) {
		return
	}
	model := m.models[m.selectedIdx]
	if model.Provider == models.ProviderChatGPT {
		efforts := model.ReasoningEfforts
		i := -1
		if delta < 0 {
			i = len(efforts)
		}
		selected := m.chatGPTEffort(model)
		for j, effort := range efforts {
			if effort.Effort == selected {
				i = j
				break
			}
		}
		if i += delta; i >= 0 && i < len(efforts) {
			if m.effort == nil {
				m.effort = make(map[string]string)
			}
			key, _, _ := models.FamilyOf(model)
			m.effort[key] = efforts[i].Effort
		}
		return
	}
	if m.families == nil || m.selectedIdx >= len(m.families) {
		return
	}
	g := m.families[m.selectedIdx]
	efforts := g.Efforts()
	if len(efforts) < 2 {
		return
	}
	_, _, cur := models.FamilyOf(m.models[m.selectedIdx])
	i := 0
	for j, e := range efforts {
		if e == cur {
			i = j
		}
	}
	i += delta
	if i < 0 || i >= len(efforts) {
		return
	}
	m.effort[g.Key] = efforts[i]
	m.models[m.selectedIdx] = familyRow(g, efforts[i])
}

// effortStrip renders "low  [HIGH]  max" for a family row, or "" when the
// model offers no effort choice.
func (m *modelDialogCmp) effortStrip(i int) string {
	if i < len(m.models) && m.models[i].Provider == models.ProviderChatGPT {
		model := m.models[i]
		selected := m.chatGPTEffort(model)
		parts := []string{"Reasoning"}
		for _, option := range model.ReasoningEfforts {
			parts = append(parts, effortLabel(option.Effort, selected))
		}
		if selected == "" {
			parts = append(parts, "[PROVIDER DEFAULT]")
		}
		return strings.Join(parts, "  ")
	}
	if m.families == nil || i >= len(m.families) {
		return ""
	}
	efforts := m.families[i].Efforts()
	if len(efforts) < 2 {
		return ""
	}
	_, _, cur := models.FamilyOf(m.models[i])
	parts := make([]string, 0, len(efforts))
	for _, e := range efforts {
		parts = append(parts, effortLabel(e, cur))
	}
	return "Reasoning  " + strings.Join(parts, "  ")
}

func effortLabel(effort, selected string) string {
	if effort == selected {
		return "[" + strings.ToUpper(effort) + "]"
	}
	return effort
}

func (m *modelDialogCmp) chatGPTEffort(model models.Model) string {
	key, _, _ := models.FamilyOf(model)
	saved := m.effort[key]
	if saved == "" {
		if cfg := config.Get(); cfg != nil {
			if coder := cfg.Agents[config.AgentCoder]; coder.Model == model.ID {
				saved = coder.ReasoningEffort
			}
		}
	}
	return model.ResolveReasoningEffort(saved)
}

func (m *modelDialogCmp) effortDescription(i int) string {
	if i >= len(m.models) || m.models[i].Provider != models.ProviderChatGPT {
		return ""
	}
	model := m.models[i]
	selected := m.chatGPTEffort(model)
	for _, option := range model.ReasoningEfforts {
		if option.Effort == selected {
			return option.Description
		}
	}
	return ""
}

// familyRow is the row for a family at an effort: the variant's model with the
// family's display name (no "(High)", no provider suffix); the ID stays the
// backend id for that effort.
func familyRow(g models.FamilyGroup, effort string) models.Model {
	v := g.Pick(effort)
	if g.Name != "" {
		v.Name = g.Name
	}
	return v
}

// noticeFor is the backend's own notice for row i (a removal date, a plan
// limit). A family shows the notice of any of its variants: the backend tags
// one effort id, and the owner must see it whichever effort is highlighted.
func (m *modelDialogCmp) noticeFor(i int) string {
	if i >= len(m.models) {
		return ""
	}
	if i < len(m.families) {
		for _, v := range m.families[i].Variants {
			if v.Notice != "" {
				return v.Notice
			}
		}
	}
	return m.models[i].Notice
}
