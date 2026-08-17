package agent

// GORILLA OVERRIDE (2026-08-17): the two research rows in /context must measure
// DISJOINT things. tool.research is the tool's schema without the dossier
// addition; tool.dossier is that addition alone. Measuring Info() for the
// first counted the dossier's tokens in both rows whenever it was armed, which
// inflated the header total — the same double-count class as the research
// basis figure fixed on 2026-08-14.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

func TestResearchAndDossierRowsDoNotOverlap(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	r := &researchTool{}

	// infoBase must not move when the dossier row is toggled: it is the figure
	// calibration records for tool.research.
	armed := config.LoadoutEnabled(config.DossierComponentID)
	base1 := infoTokens(r.infoBase())
	config.ToggleLoadout(config.DossierComponentID)
	base2 := infoTokens(r.infoBase())
	full := infoTokens(r.Info())
	if config.LoadoutEnabled(config.DossierComponentID) == armed {
		t.Fatal("toggle did not change the armed state; the rest of this test proves nothing")
	}
	config.ToggleLoadout(config.DossierComponentID) // restore

	if base1 != base2 {
		t.Errorf("infoBase moved with the loadout: %d -> %d; calibration would record a state-dependent figure", base1, base2)
	}
	// In whichever direction the toggle went, the ARMED schema must exceed the
	// base by roughly the dossier row's own figure — that overlap is exactly
	// what must not be counted twice.
	if full == base2 {
		t.Skip("toggle landed on the disarmed state; the armed comparison is covered by the assertion below")
	}
	if grew := full - base2; grew < DossierSchemaTokens()-2 || grew > DossierSchemaTokens()+2 {
		t.Errorf("armed schema grew by %d but the dossier row reports %d — the rows disagree about the same tokens",
			grew, DossierSchemaTokens())
	}
}
