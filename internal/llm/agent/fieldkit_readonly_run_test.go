package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

func TestTheOwnersReadOnlyRunRule(t *testing.T) {
	on := config.MCPServer{Type: config.MCPStdio, Trust: config.MCPTrustLocal, ReadOnlyTools: []string{"run:read-only"}}
	off := config.MCPServer{Type: config.MCPStdio, Trust: config.MCPTrustLocal, ReadOnlyTools: []string{"discover"}}
	untrusted := config.MCPServer{Type: config.MCPStdio, ReadOnlyTools: []string{"run:read-only"}}
	cases := []struct {
		why    string
		server string
		cfg    config.MCPServer
		input  string
		want   bool
	}{
		{"read-only offline tool, rule on", "fieldkit", on, `{"tool":"academic-dashes"}`, true},
		{"rule not written by the owner", "fieldkit", off, `{"tool":"academic-dashes"}`, false},
		{"server not marked local", "fieldkit", untrusted, `{"tool":"academic-dashes"}`, false},
		{"another server", "other", on, `{"tool":"academic-dashes"}`, false},
		{"tool that changes things", "fieldkit", on, `{"tool":"academic-build"}`, false},
		{"tool that goes online", "fieldkit", on, `{"tool":"academic-check"}`, false},
		{"call that declares the network", "fieldkit", on, `{"tool":"academic-dashes","network":true}`, false},
		{"tool not in the catalogue", "fieldkit", on, `{"tool":"made-up-tool"}`, false},
		{"no tool named", "fieldkit", on, `{}`, false},
		{"not JSON", "fieldkit", on, `academic-dashes`, false},
	}
	for _, c := range cases {
		if got := fieldkitReadOnlyRun(c.server, "run", c.cfg, c.input); got != c.want {
			t.Errorf("%s: got %v, want %v", c.why, got, c.want)
		}
	}
	if fieldkitReadOnlyRun("fieldkit", "describe", on, `{"tool":"academic-dashes"}`) {
		t.Errorf("the rule covered a tool other than run")
	}
}
