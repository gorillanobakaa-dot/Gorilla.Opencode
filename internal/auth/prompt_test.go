package auth

import (
	"context"
	"testing"
)

// The bug: the OAuth flow wrote its URL with fmt.Println. Under the TUI that is a
// write into a screen Bubble Tea owns — the text is painted over the interface,
// Bubble Tea has no record of it, and no redraw can clear it, so the sign-in URL
// stayed burned across the session. The destination must therefore be injectable.
func TestAuthPromptGoesToTheInjectedReporter(t *testing.T) {
	var got []string
	ctx := WithAuthPrompt(context.Background(), func(url string) {
		got = append(got, url)
	})

	AuthPromptFrom(ctx)("https://accounts.google.com/o/oauth2/auth?x=1")

	if len(got) != 1 || got[0] != "https://accounts.google.com/o/oauth2/auth?x=1" {
		t.Errorf("reporter received %v, want the URL exactly once", got)
	}
}

// A bare terminal (`auth login`, first-run setup) still needs the printed form,
// so the default must never be nil — a login flow that cannot say where to go is
// worse than one that prints in the wrong place.
func TestAuthPromptDefaultsToPrinting(t *testing.T) {
	if fn := AuthPromptFrom(context.Background()); fn == nil {
		t.Fatal("nil reporter: the flow would block with no way for the user to know where to go")
	}
	// A nil override must not defeat the default either.
	ctx := WithAuthPrompt(context.Background(), nil)
	if fn := AuthPromptFrom(ctx); fn == nil {
		t.Fatal("nil reporter after WithAuthPrompt(nil)")
	}
}
