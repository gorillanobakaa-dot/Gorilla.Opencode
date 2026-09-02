package tui

import "testing"

func TestWantsCommandHelp(t *testing.T) {
	yes := []string{"help", "--help", "-h", "-help", "?", "/?", "  help  ", "HELP", "--Help"}
	for _, in := range yes {
		if !wantsCommandHelp(in) {
			t.Errorf("%q should be read as a request for help", in)
		}
	}

	// The dangerous direction. Each of these is real work, and treating any of
	// them as "help" would silently do nothing while looking like it worked.
	// "helpers" matters most: it is a prefix of "help", so a HasPrefix
	// implementation would swallow it.
	no := []string{
		"",                // bare /review reviews the folder; bare /port inspects
		"helpers",         // a folder could be called this
		"help me",         // a question, not the flag
		"./help",          // a path
		"--helpful",       // not a flag we know; belongs in Unknown
		"forward-port",    // an operation
		"--onto v6.12",    // a real flag
		"inspect",         // an operation
		"/home/user/help", // a path ending in help
		"--diff HEAD",     // a real flag
	}
	for _, in := range no {
		if wantsCommandHelp(in) {
			t.Errorf("%q is work, not a request for help", in)
		}
	}
}

// Bare invocations must keep doing what they documented, or this change is a
// regression dressed as an improvement.
func TestHelpRoutingDoesNotHijackBareCommands(t *testing.T) {
	if wantsCommandHelp("") {
		t.Fatal("bare command must not open help; /review reviews, /port inspects")
	}
	// And the parsers still see their own arguments untouched.
	if op := parsePortArgs("").Op; op != "inspect" {
		t.Fatalf("bare /port = %q, want inspect", op)
	}
	if op := parsePortArgs("forward-port").Op; op != "forward-port" {
		t.Fatalf("/port forward-port = %q", op)
	}
}
