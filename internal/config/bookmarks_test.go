package config

import (
	"os"
	"testing"
)

// GORILLA OVERRIDE: the shortlist has to survive the things that broke the
// model list itself — a refreshed catalogue, a renamed model, a retired one.
// Storing ids is what makes that possible; these assert the behaviour that
// depends on it.

func loadForTest(t *testing.T) {
	t.Helper()
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("config load: %v", err)
	}
	cfg.BookmarkedModels = nil
}

func TestToggleBookmarkAddsAndRemoves(t *testing.T) {
	loadForTest(t)

	on, err := ToggleBookmark("openrouter.vendor/model-a")
	if err != nil || !on {
		t.Fatalf("first toggle should add: on=%v err=%v", on, err)
	}
	if !IsBookmarked("openrouter.vendor/model-a") {
		t.Fatal("not reported as bookmarked after adding")
	}

	// Deselection is the half that matters when a model disappoints.
	on, err = ToggleBookmark("openrouter.vendor/model-a")
	if err != nil || on {
		t.Fatalf("second toggle should remove: on=%v err=%v", on, err)
	}
	if IsBookmarked("openrouter.vendor/model-a") {
		t.Fatal("still bookmarked after removal")
	}
}

// Order is the order the user added them — the only order they authored.
func TestBookmarksKeepInsertionOrder(t *testing.T) {
	loadForTest(t)
	for _, id := range []string{"c", "a", "b"} {
		if _, err := ToggleBookmark(id); err != nil {
			t.Fatal(err)
		}
	}
	got := BookmarkedModels()
	want := []string{"c", "a", "b"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("order not preserved: got %v want %v", got, want)
		}
	}
}

// Removing from the middle must not disturb the rest.
func TestRemovingFromTheMiddleKeepsTheRest(t *testing.T) {
	loadForTest(t)
	for _, id := range []string{"a", "b", "c"} {
		if _, err := ToggleBookmark(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ToggleBookmark("b"); err != nil {
		t.Fatal(err)
	}
	got := BookmarkedModels()
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("expected [a c], got %v", got)
	}
}

// The caller returns a mutable slice to the picker every frame; handing out the
// live one would let a render mutate stored state.
func TestBookmarkedModelsReturnsACopy(t *testing.T) {
	loadForTest(t)
	if _, err := ToggleBookmark("a"); err != nil {
		t.Fatal(err)
	}
	got := BookmarkedModels()
	got[0] = "tampered"
	if BookmarkedModels()[0] != "a" {
		t.Fatal("callers can mutate the stored list")
	}
}

func TestToggleBookmarkRejectsEmptyID(t *testing.T) {
	loadForTest(t)
	if _, err := ToggleBookmark(""); err == nil {
		t.Fatal("an empty id must be refused, not stored")
	}
}

// It must reach disk: a shortlist that vanishes on restart is worse than none,
// because the user believes they curated something.
func TestBookmarksPersistToDisk(t *testing.T) {
	loadForTest(t)
	if _, err := ToggleBookmark("openrouter.persisted"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(GorillaConfigFile())
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if !contains(string(data), "openrouter.persisted") {
		t.Fatalf("bookmark absent from %s", GorillaConfigFile())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
