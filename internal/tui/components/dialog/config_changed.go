package dialog

// GORILLA OVERRIDE: this file did not exist upstream.
//
// ConfigChangedMsg is the single hot-apply signal every settings, workspace-root
// and prompt change emits. The handler in tui.go rebuilds the coder agent's tool
// set and re-renders the system prompt, and reports back whether that had to be
// deferred because a turn was in flight.
//
// There is deliberately ONE message rather than one per dialog: the reload path
// (app.ReloadCoderTools) and the honesty rule about deferred changes are the
// same for all of them, and a second mechanism would be a second place for that
// rule to be forgotten. LoadoutChangedMsg predates this and is kept because
// /context also reports a token total, but it goes through the same reload.
type ConfigChangedMsg struct {
	// Info is shown to the user verbatim, e.g. `Longest AI reply: 4096 tokens`.
	// A deferred note is appended by the handler when applicable.
	Info string

	// InvalidateCtx must be set by anything that changes WHICH files feed the
	// system prompt — adding or removing a workspace root, changing the working
	// directory, editing contextPaths. Without it the project-context cache holds
	// and the change never reaches the model, which is the failure mode
	// prompt.InvalidateContextCache exists to prevent.
	InvalidateCtx bool
}
