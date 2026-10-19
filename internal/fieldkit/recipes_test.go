package fieldkit

import (
	"strings"
	"testing"
)

// 2026-10-20: the catalogue (schema 2) carries Fieldkit's published recipes, so a
// small model with no harness of its own is told how something was done before,
// not only which tool exists.
func TestARecipeQuestionOffersTheRecipeAsARecipe(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Schema < 2 || len(c.Recipes) == 0 {
		t.Fatalf("the embedded catalogue has schema %d and %d recipes; regenerate it with `fieldkit cards export`",
			c.Schema, len(c.Recipes))
	}
	hits := Match(c, "build a Go program for an old Mac on High Sierra 10.13", 5)
	if len(hits) == 0 || hits[0].ID != "go-binary-for-macos-10-13" || !hits[0].Recipe {
		t.Fatalf("hits %v, want the go-binary-for-macos-10-13 recipe first", ids(hits))
	}
	h := Hint(c, "build a Go program for an old Mac on High Sierra 10.13", 5)
	if !strings.Contains(h, "- go-binary-for-macos-10-13 [RECIPE: read it with fieldkit_describe, never run it] ") {
		t.Errorf("the hint does not mark the recipe as one:\n%s", h)
	}
	if !strings.Contains(h, "never run it.]") {
		t.Errorf("the hint's header does not say what a recipe is:\n%s", h)
	}
}

func TestAToolLeadsARecipeOfTheSameFit(t *testing.T) {
	c := Catalogue{Tools: []Tool{{ID: "hdmi-status", Title: "Is the TV connected over HDMI", Trust: "carded",
		Safety: "read-only"}},
		// the recipe fits a little BETTER (a tag), so only the recipe factor keeps
		// the tool first; the trust tie-break alone would not
		Recipes: []Recipe{{ID: "hdmi-notes", Title: "Is the TV connected over HDMI", Tags: []string{"tv"}}}}
	hits := Match(c, "is the tv connected over hdmi", 5)
	if len(hits) != 2 || hits[0].ID != "hdmi-status" || hits[0].Recipe || !hits[1].Recipe {
		t.Fatalf("hits %v, want the tool first and the recipe after it", ids(hits))
	}
}

// The owner's own recipes (Fieldkit local/recipes.yaml) never ship inside the
// published program: the export takes the public file only.
func TestNoPrivateRecipeIsEmbedded(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range c.Recipes {
		for _, private := range []string{"antigravity-tier-probe", "release-screenshots-of-a-tui",
			"powershell-code-written-through-a-shell", "organize-loose-scripts-into-tool-folders"} {
			if r.ID == private {
				t.Errorf("the owner's own recipe %q is inside the published catalogue", r.ID)
			}
		}
	}
}

func ids(ts []Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
