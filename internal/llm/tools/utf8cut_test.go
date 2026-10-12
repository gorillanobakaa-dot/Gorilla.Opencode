package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCutAtRuneNeverSplitsACharacter(t *testing.T) {
	s := "Grüße, façade — 東京 🦍 end"
	for n := -1; n <= len(s)+1; n++ {
		got := cutAtRune(s, n)
		if !utf8.ValidString(got) || !strings.HasPrefix(s, got) || len(got) > max(n, 0) {
			t.Fatalf("cutAtRune(%d) = %q", n, got)
		}
	}
}

// The truncation that guards every untrusted answer is the one most likely to
// cut foreign text.
func TestUntrustedTruncationKeepsValidUTF8(t *testing.T) {
	body := strings.Repeat("ß", MaxToolResponseBytes)
	resp := NewUntrustedTextResponse("web page", "https://example.org", "", body)
	if !utf8.ValidString(resp.Content) {
		t.Fatal("a truncated untrusted answer is not valid UTF-8")
	}
}
