package app

// GORILLA (2026-10-10): session-to-session messaging, started for the
// interactive interface, plain mode and editor mode (`gorilla-opencode acp`).
// A one-shot -p run never calls this: it has nobody to show a message to and
// ends before an answer could matter, so it opens no endpoint at all.

import (
	"os"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/peers"
	"github.com/opencode-ai/opencode/internal/version"
)

// StartPeers registers this session and opens its local endpoint, unless the
// "Session messaging" row in /context is off. notify, when not nil, is called
// on its own goroutine for each message or idle notice that arrives; a front
// end can also set it later with peers.Active().SetNotify.
//
// It never fails the program: if the endpoint cannot be opened the session
// simply does not take part, and the log says why. The returned function stops
// it again and is never nil.
func (app *App) StartPeers(notify func(peers.Message)) (stop func()) {
	if !config.LoadoutEnabled(config.PeersComponentID) {
		logging.Info("session messaging is switched off in /context: no endpoint opened, not registered")
		return func() {}
	}
	s, err := peers.Start(peers.Options{
		Dir:     peers.DefaultDir(config.StateBase()),
		Folder:  config.WorkingDirectory,
		Version: version.Version,
		PID:     os.Getpid(),
	})
	if err != nil {
		logging.Warn("session messaging could not start; this session is not visible to others", "error", err)
		return func() {}
	}
	if notify != nil {
		s.SetNotify(notify)
	}
	peers.SetActive(s)
	logging.Info("session messaging started", "name", s.Name(), "endpoint", s.Endpoint())
	return func() {
		if peers.Active() == s {
			peers.SetActive(nil)
		}
		s.Close()
	}
}
