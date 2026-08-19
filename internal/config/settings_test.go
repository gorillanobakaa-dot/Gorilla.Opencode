package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Registry integrity. This is the test that earns the registry design: a
// half-added setting fails here rather than shipping a blank or lying row in
// /settings. Every requirement the user stated — plain-language description,
// what it accepts, min/max/default — is checked mechanically.
func TestSettingsRegistryIntegrity(t *testing.T) {
	if len(Settings) == 0 {
		t.Fatal("registry is empty")
	}

	seen := map[string]bool{}
	for i := range Settings {
		s := &Settings[i]
		t.Run(s.ID, func(t *testing.T) {
			if s.ID == "" {
				t.Fatal("empty ID")
			}
			if seen[s.ID] {
				t.Fatalf("duplicate ID %q", s.ID)
			}
			seen[s.ID] = true

			if strings.TrimSpace(s.Name) == "" {
				t.Error("empty Name — the row would render nameless")
			}
			if strings.TrimSpace(s.Layman) == "" {
				t.Error("empty Layman — the whole point is that every setting explains itself")
			}
			if !slices.Contains(GroupOrder, s.Group) {
				t.Errorf("group %q is not in GroupOrder, so the row would never be displayed", s.Group)
			}
			if s.Get == nil {
				t.Error("nil Get — the row could not show its current value")
			}

			switch s.Kind {
			case KindBool:
				if strings.TrimSpace(s.WhenOn) == "" || strings.TrimSpace(s.WhenOff) == "" {
					t.Error("bool setting must describe BOTH states; leaving the user to infer the off case is how a toggle becomes a mystery")
				}
				if _, ok := s.Default.(bool); !ok {
					t.Errorf("bool setting has non-bool default %T", s.Default)
				}
			case KindInt:
				if s.Min >= s.Max && !s.ReadOnly {
					t.Errorf("Min %d >= Max %d", s.Min, s.Max)
				}
				d, ok := s.Default.(int)
				if !ok {
					t.Fatalf("int setting has non-int default %T", s.Default)
				}
				if d < s.Min || d > s.Max {
					t.Errorf("default %d is outside its own range %d-%d", d, s.Min, s.Max)
				}
			case KindLadder:
				if len(s.Presets) == 0 {
					t.Fatal("ladder setting has no presets")
				}
				d, ok := s.Default.(int)
				if !ok {
					t.Fatalf("ladder setting has non-int default %T", s.Default)
				}
				if !slices.Contains(s.Presets, d) {
					t.Errorf("default %d is not one of its own presets %v", d, s.Presets)
				}
			case KindEnum:
				d, ok := s.Default.(string)
				if !ok {
					t.Fatalf("enum setting has non-string default %T", s.Default)
				}
				// tui.theme fills Options at runtime from the theme registry.
				if len(s.Options) > 0 && !slices.Contains(s.Options, d) {
					t.Errorf("default %q is not among its own options %v", d, s.Options)
				}
			case KindStringList:
				if _, ok := s.Default.([]string); !ok {
					t.Errorf("list setting has non-[]string default %T", s.Default)
				}
			}

			if s.ReadOnly {
				if strings.TrimSpace(s.ReadOnlyWhy) == "" {
					t.Error("read-only setting must say WHY, or it looks broken rather than deliberate")
				}
				if s.Set != nil {
					t.Error("read-only setting has a Set function; it would be editable in practice")
				}
			} else if s.Set == nil {
				t.Error("editable setting has nil Set")
			}
		})
	}
}

// Every setting must be reachable in the UI: a group with no display order, or a
// row whose range cannot be rendered, is invisible or blank.
func TestEverySettingRendersARange(t *testing.T) {
	for i := range Settings {
		s := &Settings[i]
		if got := SettingRange(s); strings.TrimSpace(got) == "" {
			t.Errorf("%s: SettingRange is empty — the 'what it accepts' half of the row would be blank", s.ID)
		}
	}
}

// Round-trip: setting a value to its own default must leave it there. Catches a
// Set that writes a different field than Get reads, which would make /settings
// appear to do nothing.
func TestSettingsRoundTripDefaults(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for i := range Settings {
		s := &Settings[i]
		if s.ReadOnly || s.Set == nil || s.Default == nil {
			continue
		}
		// The theme setter validates against the real theme registry, which is
		// not loaded in this package's tests.
		if s.ID == "tui.theme" {
			continue
		}
		t.Run(s.ID, func(t *testing.T) {
			if err := s.Set(s.Default); err != nil {
				t.Fatalf("Set(default) failed: %v", err)
			}
			got := FormatSettingValue(s.Get())
			want := FormatSettingValue(s.Default)
			if got != want {
				t.Errorf("after Set(default), Get = %q, want %q — Set and Get disagree on which field they use", got, want)
			}
		})
	}
}

// Rejection: out-of-range, wrong-type and nonsense input must be refused with a
// message a human can act on, and must not change the stored value.
func TestSettingsRejectBadInput(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	t.Run("int above max", func(t *testing.T) {
		s := SettingByID("agents.coder.maxTokens")
		before := FormatSettingValue(s.Get())
		err := s.Set(999999)
		if err == nil {
			t.Fatal("accepted a value above Max")
		}
		if !strings.Contains(err.Error(), "between") {
			t.Errorf("error %q does not state the accepted range", err)
		}
		if after := FormatSettingValue(s.Get()); after != before {
			t.Errorf("value changed despite the error: %q -> %q", before, after)
		}
	})

	t.Run("int below min", func(t *testing.T) {
		s := SettingByID("agents.coder.maxTokens")
		if err := s.Set(1); err == nil {
			t.Error("accepted a value below Min")
		}
	})

	t.Run("unparseable int", func(t *testing.T) {
		s := SettingByID("agents.coder.maxTokens")
		if err := s.Set("banana"); err == nil {
			t.Error("accepted non-numeric text")
		}
	})

	t.Run("unknown enum", func(t *testing.T) {
		s := SettingByID("agents.coder.reasoningEffort")
		err := s.Set("extreme")
		if err == nil {
			t.Fatal("accepted an unknown enum value")
		}
		if !strings.Contains(err.Error(), "low") {
			t.Errorf("error %q does not list the valid options", err)
		}
	})

	t.Run("non-executable shell", func(t *testing.T) {
		s := SettingByID("shell.path")
		// A directory is never a valid shell.
		if err := s.Set(dir); err == nil {
			t.Error("accepted a directory as the shell — every AI command would then fail")
		}
		if err := s.Set("/definitely/not/here"); err == nil {
			t.Error("accepted a non-existent shell path")
		}
	})

	t.Run("empty contextPaths", func(t *testing.T) {
		s := SettingByID("contextPaths")
		if err := s.Set([]string{}); err == nil {
			t.Error("accepted an empty contextPaths; the AI would get no project instructions at all")
		}
	})

	// GORILLA OVERRIDE (2026-08-19): AGENTS.md was missing from the defaults
	// entirely, so this tool read .cursorrules and three case-variants of its
	// own name but not the file 60,000+ repositories actually use — and
	// silence and success looked identical to the user. Asserted here so it
	// cannot quietly go away again.
	t.Run("AGENTS.md is a default context path, ahead of .cursorrules", func(t *testing.T) {
		agents, cursor := -1, -1
		for i, p := range defaultContextPaths {
			switch p {
			case AgentsFile:
				agents = i
			case ".cursorrules":
				cursor = i
			}
		}
		if agents < 0 {
			t.Fatalf("%s is not in defaultContextPaths: %v", AgentsFile, defaultContextPaths)
		}
		if cursor >= 0 && agents > cursor {
			t.Errorf("%s is ordered after .cursorrules (%d vs %d); the open standard should win",
				AgentsFile, agents, cursor)
		}
	})
}

// A read-only row must not be silently editable through the generic path.
func TestReadOnlySettingsHaveNoSetter(t *testing.T) {
	s := SettingByID("agents.title.maxTokens")
	if s == nil {
		t.Fatal("expected the title max-tokens row to exist as a read-only inventory entry")
	}
	if !s.ReadOnly {
		t.Error("title max-tokens should be read-only: Load() force-overwrites it to 80 on every launch, so an editable control would be a lie")
	}
	if s.Set != nil {
		t.Error("read-only row has a setter")
	}
}

// The inventory pointers must name a real owner command, so /settings can be a
// complete list without becoming a second source of truth.
func TestOwnedElsewherePointsAtRealCommands(t *testing.T) {
	if len(ModelOwnedElsewhere) == 0 {
		t.Fatal("no cross-references registered; /settings would look like it is missing things")
	}
	for _, e := range ModelOwnedElsewhere {
		if !strings.HasPrefix(e.Owner, "/") {
			t.Errorf("owner %q for %q is not a slash command", e.Owner, e.Name)
		}
		if strings.TrimSpace(e.Why) == "" {
			t.Errorf("%q has no explanation of what the owner controls", e.Name)
		}
	}
}

func TestSettingsChangedFromDefaultCounts(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := ResetAllSettings(); err != nil {
		t.Fatalf("ResetAllSettings: %v", err)
	}
	if n := SettingsChangedFromDefault(); n != 0 {
		t.Errorf("after resetting everything, %d settings still differ from default", n)
	}

	s := SettingByID("agents.coder.maxTokens")
	if err := s.Set(8192); err != nil {
		t.Fatal(err)
	}
	if n := SettingsChangedFromDefault(); n != 1 {
		t.Errorf("after one change, count = %d, want 1", n)
	}

	if err := ResetAllSettings(); err != nil {
		t.Fatal(err)
	}
	if n := SettingsChangedFromDefault(); n != 0 {
		t.Errorf("after reset, count = %d, want 0", n)
	}
}

// docs/SETTINGS.md is generated from this registry. A hand-maintained reference
// drifts on the first change, so the checked-in file is compared against a fresh
// render — a new setting cannot land undocumented.
//
// Regenerate with: go run ./cmd/settings-doc > docs/SETTINGS.md
func TestSettingsDocIsCurrent(t *testing.T) {
	// Walk up to the repo root: this test runs in internal/config.
	path := filepath.Join("..", "..", "docs", "SETTINGS.md")
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("docs/SETTINGS.md not readable (%v); nothing to compare", err)
	}

	// The generator lives in cmd/settings-doc. Rather than exec it, assert that
	// every registered setting appears in the doc with its id, range and default
	// — the properties that make the doc useful and the ones that go stale.
	doc := string(onDisk)
	for i := range Settings {
		s := &Settings[i]
		for _, want := range []string{
			"`" + s.ID + "`",
			SettingRange(s),
			TildeHome(FormatSettingValue(s.Default)),
		} {
			if want == "" {
				continue
			}
			if !strings.Contains(doc, want) {
				t.Errorf("docs/SETTINGS.md is stale: setting %q is missing %q.\nRegenerate with: go run ./cmd/settings-doc > docs/SETTINGS.md",
					s.ID, want)
				break
			}
		}
	}

	// And every group that has rows must have a heading.
	for _, g := range GroupOrder {
		has := false
		for i := range Settings {
			if Settings[i].Group == g {
				has = true
				break
			}
		}
		if has && !strings.Contains(doc, "## "+string(g)) {
			t.Errorf("docs/SETTINGS.md has no heading for group %q", g)
		}
	}
}
