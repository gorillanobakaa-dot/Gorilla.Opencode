package agent

// GORILLA OVERRIDE (2026-08-23): count the tool-schema cost, do not infer it.
//
// docs/WHAT-A-CURIOUS-USER-COSTS.md attributed ~4,994 tokens per turn to tool
// schemas, honestly labelled "[inference] ... by subtraction, not by direct
// count". It was arrived at by measuring a 10K baseline, subtracting the system
// prompt and the injected CLAUDE.md, and calling the remainder schemas.
//
// Counted directly on 2026-08-23, with default settings: the default-ON tool
// schemas are 8,462 tokens, not ~4,994. The inference was 41% low, and the
// figure was about to be used to decide which tools ship switched off.
//
// The subtraction was not careless. It is that a baseline measured on ONE
// machine carries that machine's loadout, and the owner's has bash, edit,
// review, diagnostics and the sub-agent tool switched off. Subtracting from
// somebody's configured total tells you about their configuration, not about
// the default a new user actually pays.
//
// Every per-tool number was already being measured (calibrate.go marshals the
// real schema and divides by 4), so the direct count needed no new machinery,
// only the decision to add the rows up. That is the lesson worth keeping: the
// measurement existed and an inference was published beside it.
//
// The /4 is a byte-count approximation, not a tokeniser, and the loadout screen
// says so. The BYTES are exact; only the conversion is approximate, and it is
// applied to every row equally, so comparisons between rows are sound.

import (
	"sort"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// Measured 2026-08-23 with default settings, `go test` on a clean config.
// These are here to make a silent drift loud, not because the exact values are
// sacred: a new tool or a reworded description SHOULD move them, and then this
// test tells you by how much instead of letting it pass unremarked.
const (
	measuredDefaultToolSchemaTokens = 8462
	measuredBasePromptTokens        = 1791
	// The band is generous on purpose. It is a tripwire for "somebody added a
	// tool with a 900-token description and nobody noticed", not a lock.
	schemaDriftAllowance = 500
)

// schemaCosts sums the calibrated per-component costs by category.
func schemaCosts(t *testing.T) (toolsOn, toolsAll, promptOn int, rows []struct {
	ID   string
	Tok  int
	On   bool
	Name string
}) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// nil deps: every tool's schema is static, so it can be measured without a
	// database, an LSP client or a permission service. calibrate_test.go relies
	// on the same property.
	CalibrateLoadout(nil, nil, nil, nil, nil)

	for _, c := range config.LoadoutComponents {
		// Per-server LSP rows come from the user's config and have no schema.
		if strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		tk := config.ComponentTokens(c)
		on := config.LoadoutEnabled(c.ID)
		rows = append(rows, struct {
			ID   string
			Tok  int
			On   bool
			Name string
		}{c.ID, tk, on, c.Name})

		switch {
		case strings.HasPrefix(c.ID, "tool."):
			toolsAll += tk
			if on {
				toolsOn += tk
			}
		case strings.HasPrefix(c.ID, "prompt.") && on:
			promptOn += tk
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Tok > rows[j].Tok })
	return toolsOn, toolsAll, promptOn, rows
}

// TestDefaultToolSchemaCostIsCounted is the measurement, kept runnable. It
// prints the full breakdown so the number in the docs can always be regenerated
// rather than trusted:
//
//	go test ./internal/llm/agent/ -run TestDefaultToolSchemaCost -v
func TestDefaultToolSchemaCostIsCounted(t *testing.T) {
	toolsOn, toolsAll, promptOn, rows := schemaCosts(t)
	base := config.LoadoutBaseTokens()

	t.Logf("%-24s %-4s %7s  %s", "ID", "ON", "TOKENS", "NAME")
	for _, r := range rows {
		flag := "off"
		if r.On {
			flag = "ON"
		}
		t.Logf("%-24s %-4s %7d  %s", r.ID, flag, r.Tok, r.Name)
	}
	t.Logf("")
	t.Logf("base system prompt          %7d", base)
	t.Logf("tool schemas, default ON    %7d", toolsOn)
	t.Logf("tool schemas, every row     %7d", toolsAll)
	t.Logf("prompt blocks, default ON   %7d", promptOn)
	t.Logf("per-turn total (no CLAUDE.md) %5d", config.LoadoutActiveTokens()+base)

	if d := toolsOn - measuredDefaultToolSchemaTokens; d > schemaDriftAllowance || d < -schemaDriftAllowance {
		t.Errorf("default tool schemas are %d tokens, recorded as %d (drift %+d).\n"+
			"  Every turn pays this, so a jump here is a recurring bill on someone\n"+
			"  else's metered connection. If the change is intended, update the\n"+
			"  constant AND docs/WHAT-A-CURIOUS-USER-COSTS.md, which quotes it.",
			toolsOn, measuredDefaultToolSchemaTokens, d)
	}
	if d := base - measuredBasePromptTokens; d > 200 || d < -200 {
		t.Errorf("base prompt is %d tokens, recorded as %d (drift %+d)", base, measuredBasePromptTokens, d)
	}
}

// The headline finding, pinned as a relationship rather than a number: tool
// schemas dominate the per-turn cost. If this ever stops being true the
// progressive-disclosure work is aimed at the wrong thing and should be
// re-argued, not quietly continued.
func TestToolSchemasAreTheLargestPerTurnLineItem(t *testing.T) {
	toolsOn, _, promptOn, _ := schemaCosts(t)
	base := config.LoadoutBaseTokens()

	if toolsOn <= base {
		t.Errorf("tool schemas (%d) no longer exceed the base prompt (%d); the case for "+
			"progressive disclosure rests on them being the dominant line item", toolsOn, base)
	}
	total := toolsOn + base + promptOn
	if share := float64(toolsOn) / float64(total); share < 0.5 {
		t.Errorf("tool schemas are %.0f%% of the per-turn load (%d of %d); "+
			"recorded as the majority", share*100, toolsOn, total)
	}
}

// No single tool may quietly become enormous. tool.find is the largest at 1,322
// and it earns it: it replaced glob, grep and ls, which together cost ~1,485.
// Anything materially past that is a regression worth arguing about rather than
// absorbing.
func TestNoSingleToolSchemaRunsAway(t *testing.T) {
	_, _, _, rows := schemaCosts(t)
	const cap = 1600
	for _, r := range rows {
		if !strings.HasPrefix(r.ID, "tool.") || !r.On {
			continue
		}
		if r.Tok > cap {
			t.Errorf("%s costs %d tokens on EVERY turn, over the %d cap.\n"+
				"  find is the largest at 1,322 and justifies it by replacing three tools\n"+
				"  that cost ~1,485 between them. A new row above this needs the same\n"+
				"  argument, in a comment, or a shorter description.", r.ID, r.Tok, cap)
		}
	}
}
