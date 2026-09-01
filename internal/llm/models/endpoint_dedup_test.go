package models

// GORILLA OVERRIDE (2026-09-01): the same server must be recognised as the same
// server however its URL is spelled.
//
// The owner's config held two entries for one LM Studio instance --
// http://127.0.0.1:1234/v1 and http://localhost:1234/v1 -- and the duplicate
// check compared raw strings, so both survived. Every local model then appeared
// TWICE in the picker, once as [lmstudio] and once as [LM Studio], and the
// owner asked, reasonably, "so how can i tell them apart?" The answer was that
// there was nothing to tell apart.
//
// It is not only cosmetic. The second registration gets namespaced to dodge an
// id collision, so an agent's configured model becomes something like
// "local.LM Studio/google/gemma-4-e2b" -- an id whose validity depends on the
// duplicate continuing to exist. Remove the duplicate and the reference dangles.

import "testing"

func TestLoopbackSpellingsAreOneServer(t *testing.T) {
	same := []string{
		"http://127.0.0.1:1234/v1",
		"http://localhost:1234/v1",
		"http://LOCALHOST:1234/v1",
		"http://127.0.0.1:1234/v1/",
		"HTTP://localhost:1234/v1",
		"http://0.0.0.0:1234/v1",
	}
	want := CanonicalEndpointURL(same[0])
	for _, u := range same[1:] {
		if got := CanonicalEndpointURL(u); got != want {
			t.Errorf("%q canonicalised to %q, want %q — it is the same server on the "+
				"same port, and treating it as a second one lists every model twice",
				u, got, want)
		}
	}
}

// Different ports are different servers. Ollama on 11434 and LM Studio on 1234
// are the common case and must not be folded together.
func TestDifferentPortsStayDistinct(t *testing.T) {
	a := CanonicalEndpointURL("http://localhost:1234/v1")
	b := CanonicalEndpointURL("http://localhost:11434/v1")
	if a == b {
		t.Errorf("ports 1234 and 11434 both canonicalised to %q; LM Studio and "+
			"Ollama would be collapsed into one endpoint", a)
	}
}

// A real remote host is not loopback, however it is spelled.
func TestRemoteHostsAreNotFoldedIntoLoopback(t *testing.T) {
	local := CanonicalEndpointURL("http://localhost:1234/v1")
	remote := CanonicalEndpointURL("http://192.168.1.50:1234/v1")
	if local == remote {
		t.Error("a LAN address was folded into loopback; a request meant for " +
			"another machine would be routed to this one")
	}
}

// Anything unparseable comes back untouched. An endpoint we cannot understand is
// not this function's problem, and rewriting it would turn a listing bug into a
// routing bug.
func TestUnparseableInputIsReturnedUnchanged(t *testing.T) {
	for _, bad := range []string{"", "not a url", "::::"} {
		if got := CanonicalEndpointURL(bad); got != bad {
			t.Errorf("CanonicalEndpointURL(%q) = %q; unparseable input must be "+
				"left alone rather than rewritten", bad, got)
		}
	}
}
