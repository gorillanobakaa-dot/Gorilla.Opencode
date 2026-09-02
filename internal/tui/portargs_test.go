package tui

import "strings"

import "testing"

// The defect these guard against has already happened twice in this codebase:
// `/review --deep` was read as "review a folder called --deep", and `/osint
// --recover` the same day. A flag swallowed as content is silent — the command
// runs, on the wrong thing.
func TestParsePortArgs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want portRequest
	}{
		{
			name: "bare is inspect, because inspect changes nothing",
			in:   "",
			want: portRequest{Op: "inspect"},
		},
		{
			name: "operation word",
			in:   "forward-port",
			want: portRequest{Op: "forward-port"},
		},
		{
			name: "short operation aliases",
			in:   "back",
			want: portRequest{Op: "backport"},
		},
		{
			name: "separated flag values",
			in:   "forward-port --onto v6.12 --series ../patches",
			want: portRequest{Op: "forward-port", Onto: "v6.12", Series: "../patches"},
		},
		{
			name: "inline flag values",
			in:   "backport --onto=v5.15 --patch=fix.patch",
			want: portRequest{Op: "backport", Onto: "v5.15", Patch: "fix.patch"},
		},
		{
			name: "build command survives as one value",
			in:   "rebase --onto main --build make",
			want: portRequest{Op: "rebase", Onto: "main", Build: "make"},
		},
		{
			name: "a folder is a folder, not an operation",
			in:   "forward-port /src/linux",
			want: portRequest{Op: "forward-port", Tree: "/src/linux"},
		},
		{
			name: "unknown flag is reported, never treated as a path",
			in:   "forward-port --deep",
			want: portRequest{Op: "forward-port", Unknown: []string{"--deep"}},
		},
		{
			name: "operation given as a flag",
			in:   "--rebase --onto main",
			want: portRequest{Op: "rebase", Onto: "main"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePortArgs(tc.in)
			if got.Op != tc.want.Op {
				t.Errorf("Op = %q, want %q", got.Op, tc.want.Op)
			}
			if got.Onto != tc.want.Onto {
				t.Errorf("Onto = %q, want %q", got.Onto, tc.want.Onto)
			}
			if got.Series != tc.want.Series {
				t.Errorf("Series = %q, want %q", got.Series, tc.want.Series)
			}
			if got.Patch != tc.want.Patch {
				t.Errorf("Patch = %q, want %q", got.Patch, tc.want.Patch)
			}
			if got.Build != tc.want.Build {
				t.Errorf("Build = %q, want %q", got.Build, tc.want.Build)
			}
			if got.Tree != tc.want.Tree {
				t.Errorf("Tree = %q, want %q", got.Tree, tc.want.Tree)
			}
			if len(got.Unknown) != len(tc.want.Unknown) {
				t.Errorf("Unknown = %v, want %v", got.Unknown, tc.want.Unknown)
			}
		})
	}
}

// A destructive operation must never be the default. Someone typing /port to
// find out what it is should get a read-only answer, not a rebase.
func TestBarePortIsNotDestructive(t *testing.T) {
	if op := parsePortArgs("").Op; op != "inspect" {
		t.Fatalf("bare /port runs %q; it must be inspect", op)
	}
	if op := parsePortArgs("   ").Op; op != "inspect" {
		t.Fatalf("whitespace-only /port runs %q; it must be inspect", op)
	}
}

// The prompt has one job beyond naming the operation: it must stop the model
// reporting an unverified port as finished.
func TestPortPromptDemandsHonestyAboutVerification(t *testing.T) {
	p := portPrompt(parsePortArgs("forward-port --onto v6.12"))
	if !strings.Contains(p, "nothing will be compiled") {
		t.Error("a port with no --build must say nothing was compiled")
	}
	if !strings.Contains(p, "applied-with-fuzz") {
		t.Error("the prompt must name the fuzz outcome; it is the one that can be silently wrong")
	}

	withBuild := portPrompt(parsePortArgs("forward-port --onto v6.12 --build make"))
	if strings.Contains(withBuild, "nothing will be compiled") {
		t.Error("a build command was given, so the unverified warning should not appear")
	}

	// inspect changes nothing, so the unverified warning is noise there.
	if strings.Contains(portPrompt(parsePortArgs("inspect")), "nothing will be compiled") {
		t.Error("inspect does not need the unverified warning")
	}
}
