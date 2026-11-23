package dialog

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// effortStop is one selectable reasoning level in the picker's Effort slider.
type effortStop struct {
	Level       string
	Description string
}

// Effort slider colors: adaptive so both dark and light terminals stay legible
// while matching Agy CLI's progressive color scale (low=green, medium=amber,
// high=red, xhigh=purple/magenta, max/ultra=bright magenta/pink, none/minimal=cyan).
var (
	effortColorCyan   = lipgloss.AdaptiveColor{Dark: "#38BDF8", Light: "#0284C7"}
	effortColorLow    = lipgloss.AdaptiveColor{Dark: "#22C55E", Light: "#15803D"}
	effortColorMedium = lipgloss.AdaptiveColor{Dark: "#EAB308", Light: "#B45309"}
	effortColorHigh   = lipgloss.AdaptiveColor{Dark: "#EF4444", Light: "#DC2626"}
	effortColorXHigh  = lipgloss.AdaptiveColor{Dark: "#C026D3", Light: "#9333EA"}
	effortColorMax    = lipgloss.AdaptiveColor{Dark: "#E879F9", Light: "#A21CAF"}
	effortColorUltra  = lipgloss.AdaptiveColor{Dark: "#F472B6", Light: "#BE185D"}
	sliderArrowActive = lipgloss.AdaptiveColor{Dark: "#60A5FA", Light: "#2563EB"}
)

func effortColor(level string) lipgloss.AdaptiveColor {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "none", "minimal":
		return effortColorCyan
	case "low":
		return effortColorLow
	case "medium":
		return effortColorMedium
	case "high":
		return effortColorHigh
	case "xhigh":
		return effortColorXHigh
	case "max":
		return effortColorMax
	case "ultra":
		return effortColorUltra
	default:
		return theme.CurrentTheme().Accent()
	}
}

// defaultEffortDescription supplies Agy CLI's live effort explanations when a
// provider (notably Antigravity) does not ship per-level descriptions.
func defaultEffortDescription(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "none", "minimal":
		return "Direct answers with minimal or no extra thinking – fastest"
	case "low":
		return "Faster responses, lighter reasoning – great for simpler tasks"
	case "medium":
		return "Balanced speed and reasoning quality for most tasks"
	case "high":
		return "Deeper reasoning for complex problems – slower but stronger"
	case "xhigh":
		return "Extra-deep reasoning for demanding problems – slower than high"
	case "max", "ultra":
		return "Maximum reasoning depth for the hardest problems – slowest"
	default:
		return ""
	}
}

// collapseFamilies turns the flat provider list into one row per model family.
// The row stands for the variant of the chosen effort (m.effort), so Enter
// sends exactly the backend id for that effort. Families with no effort choice
// stay a single plain row. current is the saved model: its effort is the
// starting point for its family.
func (m *modelDialogCmp) collapseFamilies(current models.ModelID) {
	groups := models.GroupFamilies(m.models)
	cleanFamilyNames(groups)
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

// cleanFamilyNames removes redundant bracketed raw IDs that finishAntigravity
// added when multiple backend IDs shared a label (e.g. "Gemini 4 Hydrogen
// [gemini-4-hydrogen-high]") while preserving readable route tags ("(Tiered)",
// "(2.5 Flash)", etc.) so distinct routes stay distinguishable.
func cleanFamilyNames(groups []models.FamilyGroup) {
	bases := make([]string, len(groups))
	familyIDs := make([]string, len(groups))
	candidates := make([]string, len(groups))

	for i, g := range groups {
		famID := g.Key
		if slash := strings.Index(famID, "/"); slash >= 0 {
			famID = famID[slash+1:]
		}
		familyIDs[i] = famID

		base, hadBracket := stripBracketRawID(g.Name)
		bases[i] = base
		if !hadBracket {
			candidates[i] = g.Name
			continue
		}
		if tag := familyRouteTag(base, famID); tag != "" {
			candidates[i] = base + " (" + tag + ")"
		} else {
			candidates[i] = base
		}
	}

	counts := map[string]int{}
	for _, c := range candidates {
		counts[c]++
	}
	for i := range groups {
		name := candidates[i]
		if counts[name] > 1 && familyIDs[i] != "" {
			name = bases[i] + " (" + familyIDs[i] + ")"
		}
		groups[i].Name = name
	}
}

func stripBracketRawID(name string) (base string, hadBracket bool) {
	if i := strings.LastIndex(name, " ["); i >= 0 && strings.HasSuffix(name, "]") {
		return strings.TrimSpace(name[:i]), true
	}
	return name, false
}

func familyRouteTag(baseName, familyID string) string {
	idTokens := splitIDTokens(familyID)
	if len(idTokens) == 0 {
		return ""
	}
	baseTokens := splitNameTokens(baseName)

	// Exact match (including dotted vs hyphenated versions like "5.5" vs "5-5"):
	// this is the primary route, so it needs no disambiguating suffix.
	normID := strings.ReplaceAll(strings.Join(idTokens, "-"), ".", "-")
	normBase := strings.ReplaceAll(strings.Join(baseTokens, "-"), ".", "-")
	if normID == normBase {
		return ""
	}

	common := 0
	for common < len(idTokens) && common < len(baseTokens) && idTokens[common] == baseTokens[common] {
		common++
	}
	rem := idTokens[common:]
	if len(rem) == 0 {
		rem = idTokens
	}
	words := make([]string, 0, len(rem))
	for _, tok := range rem {
		words = append(words, titleRouteToken(tok))
	}
	return strings.Join(words, " ")
}

func splitIDTokens(id string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(id)), func(r rune) bool {
		return r == '-' || r == '_' || r == '/' || unicode.IsSpace(r)
	})
}

func splitNameTokens(name string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(name)), func(r rune) bool {
		return r == '-' || r == '_' || r == '(' || r == ')' || unicode.IsSpace(r)
	})
}

func titleRouteToken(tok string) string {
	if tok == "" {
		return ""
	}
	r := []rune(tok)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// cycleEffort moves the highlighted family one effort level up or down and
// swaps the row to the variant for that level.
func (m *modelDialogCmp) cycleEffort(delta int) {
	if m.selectedIdx >= len(m.models) {
		return
	}
	model := m.models[m.selectedIdx]
	// GORILLA FIX (2026-11-21): support ReasoningEfforts on ANY provider (not
	// only ChatGPT), so free-tier Google Code Assist accounts get the exact
	// same interactive Thinking Effort bar as paid accounts.
	if model.Provider == models.ProviderChatGPT || len(model.EffortOptions()) > 0 {
		efforts := model.EffortOptions()
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

// effortStops returns the selectable effort levels and active index for row i,
// or nil when the model offers no effort choice.
func (m *modelDialogCmp) effortStops(i int) ([]effortStop, int) {
	if i < 0 || i >= len(m.models) {
		return nil, -1
	}
	if m.models[i].Provider == models.ProviderChatGPT || len(m.models[i].EffortOptions()) > 0 {
		model := m.models[i]
		if len(model.EffortOptions()) == 0 {
			return nil, -1
		}
		selected := m.chatGPTEffort(model)
		stops := make([]effortStop, 0, len(model.EffortOptions()))
		active := -1
		for j, opt := range model.EffortOptions() {
			stops = append(stops, effortStop{Level: opt.Effort, Description: opt.Description})
			if opt.Effort == selected {
				active = j
			}
		}
		return stops, active
	}
	if m.families == nil || i >= len(m.families) {
		return nil, -1
	}
	efforts := m.families[i].Efforts()
	// GORILLA FIX (2026-11-21): keep single-effort variants (common on free
	// Antigravity tiers) visible in the Effort bar instead of hiding the bar
	// completely and making the UI look outdated. Models with 0 efforts still
	// return nil.
	if len(efforts) == 0 {
		return nil, -1
	}
	_, _, cur := models.FamilyOf(m.models[i])
	stops := make([]effortStop, 0, len(efforts))
	active := -1
	for j, e := range efforts {
		stops = append(stops, effortStop{Level: e})
		if e == cur {
			active = j
		}
	}
	return stops, active
}

// effortStrip renders a plain-text effort summary ("Effort  low  [HIGH]  max")
// for narrow fallbacks or non-graphical callers, or "" when the model offers no
// effort choice.
func (m *modelDialogCmp) effortStrip(i int) string {
	stops, active := m.effortStops(i)
	if len(stops) == 0 {
		return ""
	}
	selected := ""
	if active >= 0 && active < len(stops) {
		selected = stops[active].Level
	}
	parts := []string{"Effort"}
	for _, s := range stops {
		parts = append(parts, effortLabel(s.Level, selected))
	}
	if selected == "" && i < len(m.models) && m.models[i].Provider == models.ProviderChatGPT {
		parts = append(parts, "[PROVIDER DEFAULT]")
	}
	return strings.Join(parts, "  ")
}

func effortLabel(effort, selected string) string {
	if effort == selected {
		return "[" + strings.ToUpper(effort) + "]"
	}
	return effort
}

// effortSliderLines renders the Agy CLI visual Effort slider for row i:
//   - Row 1: Effort  ◀   ●―――――――――●―――――――――◎―――――――――○   ▶
//   - Row 2:            low     medium      high      xhigh
//   - Row 3:        <live italic explanation>
//
// Falls back cleanly to a compact single-row colored bar on narrow terminals so
// no line ever exceeds width.
func (m *modelDialogCmp) effortSliderLines(i int, width int) []string {
	stops, active := m.effortStops(i)
	if len(stops) == 0 {
		return nil
	}
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	activeLevel := ""
	if active >= 0 && active < len(stops) {
		activeLevel = stops[active].Level
	}
	color := effortColor(activeLevel)
	desc := m.effortDescription(i)

	if len(stops) < 2 {
		var b strings.Builder
		b.WriteString(base.Foreground(t.Text()).Bold(true).Render("Effort"))
		b.WriteString(base.Render("  "))
		b.WriteString(base.Foreground(color).Bold(true).Render("◎ " + stops[0].Level))
		lines := []string{pickerLine(b.String(), width)}
		if desc != "" {
			lines = append(lines, base.Foreground(t.TextMuted()).Italic(true).Render(pickerLine("  "+desc, width)))
		}
		return lines
	}

	// Minimum center-to-center distance so adjacent stop labels never touch.
	minStep := 4
	for j := 0; j < len(stops)-1; j++ {
		need := (len(stops[j].Level)-1)/2 + len(stops[j+1].Level)/2 + 2
		if need > minStep {
			minStep = need
		}
	}

	const preferredStep = 10
	type sliderSpec struct {
		preArrow  string
		postArrow string
		preRight  string
		dotStart  int
		suffixW   int
		step      int
		ok        bool
	}
	fitSpec := func(preArrow, postArrow, preRight string) sliderSpec {
		dotStart := len("Effort") + len(preArrow) + 1 + len(postArrow)
		suffixW := len(preRight) + 1
		maxStep := preferredStep
		if maxStep < minStep {
			maxStep = minStep
		}
		for step := maxStep; step >= minStep; step-- {
			row1W := dotStart + (len(stops)-1)*step + 1 + suffixW
			lastLabelRight := dotStart + (len(stops)-1)*step + (len(stops[len(stops)-1].Level)-1)/2 + 1
			firstLabelLeft := dotStart - len(stops[0].Level)/2
			if row1W <= width && lastLabelRight <= width && firstLabelLeft >= 0 {
				return sliderSpec{
					preArrow:  preArrow,
					postArrow: postArrow,
					preRight:  preRight,
					dotStart:  dotStart,
					suffixW:   suffixW,
					step:      step,
					ok:        true,
				}
			}
		}
		return sliderSpec{}
	}

	// Prefer generous Agy CLI margins, then compact margins, then single-line fallback.
	spec := fitSpec("  ", "   ", "   ")
	if !spec.ok {
		spec = fitSpec(" ", " ", " ")
	}
	if !spec.ok {
		return m.effortCompactLines(stops, active, color, desc, width)
	}

	activeStyle := base.Foreground(color).Bold(true)
	filledNodeStyle := base.Foreground(color)
	mutedStyle := base.Foreground(t.TextMuted())
	arrowStyle := func(enabled bool) lipgloss.Style {
		if enabled {
			return base.Foreground(sliderArrowActive).Bold(true)
		}
		return mutedStyle
	}

	// Row 1: label, left arrow, colored track & circular stops, right arrow.
	var r1 strings.Builder
	r1.WriteString(base.Foreground(t.Text()).Bold(true).Render("Effort"))
	r1.WriteString(base.Render(spec.preArrow))
	r1.WriteString(arrowStyle(active > 0).Render("◀"))
	r1.WriteString(base.Render(spec.postArrow))

	segStr := strings.Repeat("―", spec.step-1)
	for j := range stops {
		switch {
		case active >= 0 && j < active:
			r1.WriteString(filledNodeStyle.Render("●"))
		case j == active:
			r1.WriteString(activeStyle.Render("◎"))
		default:
			r1.WriteString(mutedStyle.Render("○"))
		}
		if j < len(stops)-1 {
			if active >= 0 && j < active {
				r1.WriteString(activeStyle.Render(segStr))
			} else {
				r1.WriteString(mutedStyle.Render(segStr))
			}
		}
	}
	r1.WriteString(base.Render(spec.preRight))
	r1.WriteString(arrowStyle(active >= 0 && active < len(stops)-1).Render("▶"))

	// Row 2: stop labels centered directly beneath each circular node.
	var r2 strings.Builder
	col := 0
	for j, s := range stops {
		center := spec.dotStart + j*spec.step
		start := center - len(s.Level)/2
		if j == 0 && start < 0 {
			start = 0
		}
		if j > 0 && start <= col {
			start = col + 1
		}
		if gap := start - col; gap > 0 {
			r2.WriteString(base.Render(strings.Repeat(" ", gap)))
		}
		if j == active {
			r2.WriteString(activeStyle.Render(s.Level))
		} else {
			r2.WriteString(mutedStyle.Render(s.Level))
		}
		col = start + len(s.Level)
	}

	lines := []string{r1.String(), r2.String()}
	if desc != "" {
		indent := ""
		if wantIndent := min(8, spec.dotStart); wantIndent+len([]rune(desc)) <= width {
			indent = strings.Repeat(" ", wantIndent)
		}
		lines = append(lines, base.Foreground(t.TextMuted()).Italic(true).Render(pickerLine(indent+desc, width)))
	}
	return lines
}

func (m *modelDialogCmp) effortCompactLines(stops []effortStop, active int, color lipgloss.AdaptiveColor, desc string, width int) []string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	activeStyle := base.Foreground(color).Bold(true)
	mutedStyle := base.Foreground(t.TextMuted())

	var b strings.Builder
	b.WriteString(base.Foreground(t.Text()).Bold(true).Render("Effort"))
	for j, s := range stops {
		b.WriteString(base.Render(" "))
		if j == active {
			b.WriteString(activeStyle.Render("[" + strings.ToUpper(s.Level) + "]"))
		} else {
			b.WriteString(mutedStyle.Render(s.Level))
		}
	}
	lines := []string{pickerLine(b.String(), width)}
	if desc != "" {
		lines = append(lines, base.Foreground(t.TextMuted()).Italic(true).Render(pickerLine(desc, width)))
	}
	return lines
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
	stops, active := m.effortStops(i)
	if active < 0 || active >= len(stops) {
		return ""
	}
	if d := strings.TrimSpace(stops[active].Description); d != "" {
		return d
	}
	return defaultEffortDescription(stops[active].Level)
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
