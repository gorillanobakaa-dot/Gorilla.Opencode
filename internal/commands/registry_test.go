package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// dispatchedNames extracts every name the TUI actually accepts, by reading the
// slash-command switch in internal/tui/tui.go.
//
// Reading the source is deliberate. The alternative — a hand-kept list in the
// test — drifts exactly as fast as the reference it is supposed to protect, and
// the failure being guarded against IS drift: someone adds a command and never
// documents it, or renames one and leaves a reference entry pointing nowhere.
// This is the only mechanical link between the two.
func dispatchedNames(t *testing.T) []string {
	t.Helper()

	path := filepath.Join("..", "tui", "tui.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read %s: %v", path, err)
	}
	src := string(data)

	start := strings.Index(src, "case chat.SlashCommandMsg:")
	if start < 0 {
		t.Fatalf("could not find the slash-command dispatch in %s — if it moved, this test must be pointed at it, not deleted", path)
	}
	// The switch ends at its default arm.
	rest := src[start:]
	if end := strings.Index(rest, "\t\tdefault:"); end > 0 {
		rest = rest[:end]
	}

	caseRe := regexp.MustCompile(`case ((?:"[a-z0-9?-]+"(?:,\s*)?)+):`)
	strRe := regexp.MustCompile(`"([a-z0-9?-]+)"`)

	var names []string
	for _, m := range caseRe.FindAllStringSubmatch(rest, -1) {
		for _, s := range strRe.FindAllStringSubmatch(m[1], -1) {
			names = append(names, s[1])
		}
	}
	if len(names) == 0 {
		t.Fatal("extracted no command names; the parser no longer matches the dispatch code")
	}
	return names
}

// Every command a user can type must be documented, or the reference is a lie by
// omission — which is worse than no reference, because it is believed.
func TestEveryDispatchedCommandIsDocumented(t *testing.T) {
	for _, name := range dispatchedNames(t) {
		if ByName(name) == nil {
			t.Errorf("/%s can be typed but is not in the reference; add it to All in registry.go", name)
		}
	}
}

// And the reverse: a documented command that does nothing sends the user in
// circles.
func TestEveryDocumentedCommandIsDispatched(t *testing.T) {
	dispatched := dispatchedNames(t)
	for _, c := range All {
		for _, name := range append([]string{c.Name}, c.Aliases...) {
			if !slices.Contains(dispatched, name) {
				t.Errorf("/%s is documented but not handled by the TUI — typing it does nothing", name)
			}
		}
	}
}

// The whole point is plain language. A blank or jargon-shaped entry defeats it.
func TestEveryCommandExplainsItself(t *testing.T) {
	seen := map[string]string{}

	for i := range All {
		c := &All[i]
		t.Run(c.Name, func(t *testing.T) {
			if strings.TrimSpace(c.Name) == "" {
				t.Fatal("empty name")
			}
			if prev, dup := seen[c.Name]; dup {
				t.Fatalf("name %q already used by %q", c.Name, prev)
			}
			seen[c.Name] = c.Name
			for _, a := range c.Aliases {
				if prev, dup := seen[a]; dup {
					t.Errorf("alias %q collides with %q", a, prev)
				}
				seen[a] = c.Name
			}

			if strings.TrimSpace(c.Summary) == "" {
				t.Error("empty Summary — the row would render nameless")
			}
			if !strings.HasSuffix(strings.TrimSpace(c.Summary), ".") {
				t.Errorf("Summary should be a sentence: %q", c.Summary)
			}
			if n := len(c.Summary); n > 60 {
				t.Errorf("Summary is %d chars; it has to fit one line: %q", n, c.Summary)
			}
			if strings.TrimSpace(c.Detail) == "" {
				t.Error("empty Detail — no explanation of why anyone would use this")
			}
			if !slices.Contains(GroupOrder, c.Group) {
				t.Errorf("group %q is not in GroupOrder, so the command would never be displayed", c.Group)
			}

			// Internal vocabulary that means nothing to someone who has not read
			// the code. "loadout" and "root" are the words this codebase uses for
			// the feature switches and the working folders; neither belongs in
			// user-facing text.
			for _, jargon := range []string{"loadout", "workspace root", "pubsub", "tea.", "LSP client", "provider entry"} {
				if strings.Contains(strings.ToLower(c.Summary+" "+c.Detail), strings.ToLower(jargon)) {
					t.Errorf("uses internal jargon %q: the reference exists because the user said they were getting confused", jargon)
				}
			}
		})
	}
}

// Every group with commands must be displayable, and no group may be empty.
func TestGroupsAreConsistent(t *testing.T) {
	for _, g := range GroupOrder {
		if len(InGroup(g)) == 0 {
			t.Errorf("group %q has no commands but is in GroupOrder — it would render as an empty heading", g)
		}
	}
}

func TestByName(t *testing.T) {
	if c := ByName("models"); c == nil || c.Name != "model" {
		t.Errorf("alias lookup failed: %v", c)
	}
	if c := ByName("/CD"); c == nil || c.Name != "cd" {
		t.Errorf("lookup should tolerate a leading slash and case: %v", c)
	}
	if c := ByName("  settings "); c == nil {
		t.Error("lookup should tolerate surrounding space")
	}
	if c := ByName("definitely-not-a-command"); c != nil {
		t.Errorf("unknown name returned %v", c)
	}
}

// An unknown command should point somewhere useful. The old message listed a
// dozen commands regardless of what was typed.
func TestSuggest(t *testing.T) {
	if got := Suggest("modl", 3); len(got) == 0 {
		t.Error("no suggestion for a near-miss of 'model'")
	}
	if got := Suggest("mod", 3); !slices.Contains(got, "model") {
		t.Errorf("Suggest(\"mod\") = %v, want it to include model", got)
	}
	if got := Suggest("dir", 3); len(got) == 0 {
		t.Errorf("Suggest(\"dir\") = %v, want the dir-ish commands", got)
	}
	if got := Suggest("", 3); got != nil {
		t.Errorf("empty input should suggest nothing, got %v", got)
	}
	if got := Suggest("s", 10); len(got) > 10 {
		t.Errorf("Suggest ignored its limit: %d results", len(got))
	}
}
