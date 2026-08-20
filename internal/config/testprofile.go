package config

import "testing"

// UseConnProfileForTest switches profile for one test and restores it afterwards.
//
// GORILLA OVERRIDE (2026-08-20): the profile lives in a package-level variable
// behind a sync.Once, so SetConnProfile mutates state for the REST OF THE TEST
// BINARY, not just the calling test. A test that switched to austere silently
// changed what every later test in the process observed — which is exactly how
// working_label_test.go started failing on an assertion it had always passed.
// Go test order is not a contract; depending on it is a bug waiting for someone
// else to hit.
func UseConnProfileForTest(t *testing.T, id ConnProfileID) {
	t.Helper()
	prev := CurrentConnProfile().ID
	if err := SetConnProfile(id); err != nil {
		t.Fatalf("could not set connection profile %q: %v", id, err)
	}
	t.Cleanup(func() { _ = SetConnProfile(prev) })
}

// ResetLinkSamplesForTest clears the in-memory samples so a test starts from a
// known state. Exported for the provider package's reachability test.
func ResetLinkSamplesForTest() {
	linkMu.Lock()
	linkSamples = nil
	linkLoaded = true // do not read whatever is on disk
	linkMu.Unlock()
}
