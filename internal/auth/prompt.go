// GORILLA OVERRIDE: this file did not exist upstream. It decides WHERE the
// sign-in URL is shown.
//
// The OAuth flow used to fmt.Println the URL. That is correct for a bare
// terminal and wrong under the TUI, where stdout belongs to Bubble Tea: the text
// is painted over the interface, Bubble Tea has no record of having drawn it, and
// no redraw can erase it. The URL stayed on screen for the rest of the session,
// overlapping the editor and the status bar.
//
// So the destination is passed in through the context. Callers that own a screen
// supply their own reporter; everything else gets the printing default.
package auth

import (
	"context"
	"fmt"
)

// AuthPromptFunc receives the URL the user has to visit.
type AuthPromptFunc func(url string)

type authPromptKeyType struct{}

var authPromptKey authPromptKeyType

// WithAuthPrompt returns a context whose login flow reports its URL to fn
// instead of printing it.
func WithAuthPrompt(ctx context.Context, fn AuthPromptFunc) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, authPromptKey, fn)
}

// AuthPromptFrom returns the reporter for this context, or the printing default.
// Never returns nil: a login flow that cannot tell the user where to go is
// useless, so falling back to stdout is better than falling back to silence.
func AuthPromptFrom(ctx context.Context) AuthPromptFunc {
	if fn, ok := ctx.Value(authPromptKey).(AuthPromptFunc); ok && fn != nil {
		return fn
	}
	return PrintAuthPrompt
}

// PrintAuthPrompt writes the URL to stdout. Correct for a bare terminal
// (`auth login`, first-run setup); never use it while the TUI is running.
func PrintAuthPrompt(url string) {
	fmt.Println("Opening your browser to sign in with Google.")
	fmt.Println("If it does not open, paste this URL into your browser:")
	fmt.Println()
	fmt.Println("  " + url)
	fmt.Println()
}
