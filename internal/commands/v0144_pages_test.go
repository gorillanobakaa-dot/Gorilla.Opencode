package commands

// GORILLA (2026-10-10): editor mode, helper roles and hooks shipped in v0.1.144
// with no way to find them from inside the program. Their three commands must
// stay in /help, under the groups a reader would look in. That every
// registered command is also handled is TestEveryDocumentedCommandIsDispatched.

import "testing"

func TestTheV0144PagesAreInHelp(t *testing.T) {
	for name, group := range map[string]Group{
		"editor":  GroupReference,
		"acp":     GroupReference,
		"helpers": GroupHelpers,
		"roles":   GroupHelpers,
		"hooks":   GroupTuning,
	} {
		c := ByName(name)
		if c == nil {
			t.Errorf("/%s is not in the reference, so /help does not show it", name)
			continue
		}
		if c.Group != group {
			t.Errorf("/%s is under %q, want %q", name, c.Group, group)
		}
	}
}
