package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHideAndRestoreRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ResetHiddenModelsForTest()

	if IsModelHidden("meta/llama-3.3-70b-instruct") {
		t.Fatal("nothing should be hidden to start with")
	}
	if err := HideModels("a/one", "a/two", "b/three"); err != nil {
		t.Fatal(err)
	}
	if HiddenCount() != 3 {
		t.Errorf("HiddenCount()=%d, want 3", HiddenCount())
	}
	if !IsModelHidden("a/two") {
		t.Error("a/two should be hidden")
	}
	if err := UnhideModels("a/two"); err != nil {
		t.Fatal(err)
	}
	if IsModelHidden("a/two") {
		t.Error("a/two should be restored")
	}
	if HiddenCount() != 2 {
		t.Errorf("after restore HiddenCount()=%d, want 2", HiddenCount())
	}
}

// Hiding must never be a one-way door: a user who hides the wrong thing needs a
// way back that is not hand-editing JSON.
func TestUnhideAllIsAnEscapeHatch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ResetHiddenModelsForTest()
	if err := HideModels("x/1", "x/2", "x/3"); err != nil {
		t.Fatal(err)
	}
	if err := UnhideAll(); err != nil {
		t.Fatal(err)
	}
	if HiddenCount() != 0 {
		t.Errorf("UnhideAll left %d hidden", HiddenCount())
	}
}

// The list has to survive a restart, or a purge is undone by the next launch —
// which is the entire reason it is a file and not a runtime deletion.
func TestHiddenListSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	ResetHiddenModelsForTest()
	if err := HideModels("keep/me/hidden"); err != nil {
		t.Fatal(err)
	}
	// Simulate a fresh process: forget everything in memory, reload from disk.
	hiddenMu.Lock()
	hiddenLoaded = false
	hiddenSet = nil
	hiddenMu.Unlock()

	if !IsModelHidden("keep/me/hidden") {
		t.Error("the hidden list did not survive a reload")
	}
}

// A corrupt file must fail in the SAFE direction: show the models, not hide
// everything. Hiding everything on a parse error would look like data loss.
func TestCorruptHiddenFileShowsModelsRatherThanHidingThem(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(dir+"/gorilla-opencode", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gorilla-opencode", hiddenModelsFileName),
		[]byte("{not json at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	hiddenMu.Lock()
	hiddenLoaded = false
	hiddenSet = nil
	hiddenMu.Unlock()

	if HiddenCount() != 0 {
		t.Errorf("a corrupt file hid %d models; it must hide none", HiddenCount())
	}
	if IsModelHidden("anything") {
		t.Error("a corrupt file must not hide models")
	}
}
