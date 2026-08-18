package tui

// GORILLA OVERRIDE (2026-08-18): the flag must be recognised BEFORE anything is
// spent.
//
// This is the whole bug, in one assertion. `/osint --recover` was documented in
// two places — the findings file the salvage path writes, and the research
// tool's own report — before the command existed. Typing it handed the literal
// string "--recover" to a ten-helper supervised dossier as the subject under
// investigation. The model refused to fabricate a dossier about a flag, which
// was the correct call and is not a substitute for the flag working.

import "testing"

func TestRecoverFlagIsRecognisedInTheFormsPeopleType(t *testing.T) {
	for _, arg := range []string{"--recover", "-recover", "recover", "  --recover  ", "--RECOVER", "--resume"} {
		if !isRecoverFlag(arg) {
			t.Errorf("%q is not recognised as the recovery flag — it would be researched as a question, at full cost", arg)
		}
	}
	// A real question must never be swallowed by the flag check. Losing a run
	// to a false positive is the mirror of the bug this fixes.
	for _, arg := range []string{
		"", "who is Kelexine?",
		"how do I recover a deleted file",
		"recover the kernel build state",
	} {
		if isRecoverFlag(arg) {
			t.Errorf("%q was mistaken for the recovery flag; it is a question and would never be researched", arg)
		}
	}
}
