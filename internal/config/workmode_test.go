package config

import "testing"

func TestParseWorkMode(t *testing.T) {
	for in, want := range map[string]int{"economy": 0, "SOLO": 0, "0": 0, "time-sensitive": 6, "timed": 6, "6": 6, "fast": 1000, " 1000 ": 1000} {
		m := ParseWorkMode(in)
		if m == nil || m.Max != want {
			t.Errorf("ParseWorkMode(%q) = %v, want max %d", in, m, want)
		}
	}
	if ParseWorkMode("turbo") != nil {
		t.Error("unknown mode must be nil")
	}
}

func TestDefaultIsEconomy(t *testing.T) {
	if DefaultMaxSubAgents != WorkModes[0].Max {
		t.Fatalf("default leash %d is not Economy %d", DefaultMaxSubAgents, WorkModes[0].Max)
	}
}
