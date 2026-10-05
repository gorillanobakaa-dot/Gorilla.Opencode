package models

// GORILLA OVERRIDE (2026-10-05): the registry is never written in place.
//
// SupportedModels and localRoute are plain maps read from everywhere: the
// status bar and the transcript on the UI goroutine, the agent on its own. And
// /update rewrote them from a third goroutine (a tea.Cmd). Go does not forgive
// that: a map read that overlaps a map write ends the process with "fatal
// error: concurrent map read and map write", which cannot be recovered, and
// takes the turn in progress with it. Nobody had hit it only because a refresh
// is short. Found by audit.
//
// Every function that changes either map now works on a private copy and
// publishes it in one step when it is done. A reader holds whichever map was
// current when it looked, and that map is never touched again.
//
// How to use it, at the top of any function that adds or removes entries:
//
//	SupportedModels, localRoute, commit := beginRegistryEdit()
//	defer commit()
//
// The two names deliberately SHADOW the package variables, so the body below
// them reads exactly as it did. Rules:
//
//   - never call another editing function before commit (it would wait for
//     ever on the lock, and would not see this one's changes);
//   - never hold an edit across a network request.

import (
	"maps"
	"sync"
)

var registryEditMu sync.Mutex

func beginRegistryEdit() (map[ModelID]Model, map[ModelID]localRouteInfo, func()) {
	registryEditMu.Lock()
	s, r := maps.Clone(SupportedModels), maps.Clone(localRoute)
	if s == nil {
		s = map[ModelID]Model{}
	}
	if r == nil {
		r = map[ModelID]localRouteInfo{}
	}
	done := false
	return s, r, func() {
		if done {
			return
		}
		done = true
		SupportedModels, localRoute = s, r
		registryEditMu.Unlock()
	}
}

// registryNow returns the published registry. For a function that has shadowed
// the name with its own edit and, after committing, needs the real one.
func registryNow() map[ModelID]Model { return SupportedModels }
