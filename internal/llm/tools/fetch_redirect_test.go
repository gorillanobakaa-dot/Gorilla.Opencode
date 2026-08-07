package tools

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// GORILLA OVERRIDE: these tests exist because TestBlockedFetchTarget passed
// throughout the period the tool was vulnerable. That test checks the literal
// URL string, which was never the broken layer. The holes were:
//
//   - http.Client follows redirects by default, so a permitted URL could 302
//     into 169.254.169.254 and the string check never saw the second URL.
//   - the check ran on the hostname, never on the address DNS resolved to, so
//     a public name pointing at a private IP was dialled happily.
//
// Both are now enforced at the connection, so the tests below assert on
// connection behaviour rather than on string parsing. Revert either the
// CheckRedirect hook or the dialer Control in fetch.go and the matching test
// here fails - that is the non-vacuous check the original lacked.

// permissiveDial replaces only the dialer, so a test can reach a loopback
// httptest server while leaving CheckRedirect under test.
func permissiveDial(c *http.Client) *http.Client {
	tr := c.Transport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	c.Transport = tr
	return c
}

func TestRefusesRedirectIntoLinkLocal(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	client := permissiveDial(newSafeClient(10 * time.Second))
	resp, err := client.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("SSRF hole: a redirect to cloud metadata was followed")
	}
	if !strings.Contains(err.Error(), "refusing to follow redirect") {
		t.Fatalf("blocked, but not by the redirect guard: %v", err)
	}
}

func TestRefusesRedirectIntoPrivateLAN(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://192.168.1.1/router", http.StatusMovedPermanently)
	}))
	defer srv.Close()

	client := permissiveDial(newSafeClient(10 * time.Second))
	if resp, err := client.Get(srv.URL); err == nil {
		resp.Body.Close()
		t.Fatal("SSRF hole: a redirect into the private LAN was followed")
	}
}

// The dialer sees the address the kernel is about to connect to, so it catches
// a hostname that resolves somewhere private - which no string check can.
// httptest listens on loopback, so the safe client must refuse its own test
// server. That is the assertion.
func TestDialerRefusesResolvedLoopback(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("should never be read"))
	}))
	defer srv.Close()

	client := newSafeClient(10 * time.Second) // real dialer, with Control
	resp, err := client.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("dialer connected to a loopback address; Control is not wired up")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("connection refused, but not by the IP guard: %v", err)
	}
}

func TestBlockedIPCoversMappedIPv6(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"127.0.0.1":          true,
		"::1":                true,
		"::ffff:127.0.0.1":   true, // IPv4-mapped loopback
		"::ffff:169.254.1.1": true, // IPv4-mapped link-local
		"169.254.169.254":    true,
		"10.1.2.3":           true,
		"172.16.0.1":         true,
		"192.168.0.1":        true,
		"0.0.0.0":            true,
		"93.184.216.34":      false,
		"8.8.8.8":            false,
	}
	for s, wantBlocked := range cases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("bad test input %q", s)
		}
		got := blockedIP(ip) != ""
		if got != wantBlocked {
			t.Errorf("blockedIP(%s) = %v, want blocked=%v", s, got, wantBlocked)
		}
	}
}

func TestRewriteGitHubBlobToRaw(t *testing.T) {
	t.Parallel()
	in := "https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.71/README.md"
	want := "https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.71/README.md"
	got, note := rewriteToSource(in)
	if got != want {
		t.Errorf("rewriteToSource(%s)\n got %s\nwant %s", in, got, want)
	}
	if note == "" {
		t.Error("rewrite happened but was not reported to the model")
	}
	// A non-blob GitHub URL must be left alone.
	if got, _ := rewriteToSource("https://github.com/o/r/issues/1"); got != "https://github.com/o/r/issues/1" {
		t.Errorf("rewrote a non-blob URL: %s", got)
	}
}

func TestMarkdownSiblingOnlyGuessesExtensionlessPaths(t *testing.T) {
	t.Parallel()
	if got := markdownSibling("https://example.com/docs/install"); got != "https://example.com/docs/install.md" {
		t.Errorf("want .md sibling, got %q", got)
	}
	// Already has an extension: guessing would be nonsense.
	if got := markdownSibling("https://example.com/a/style.css"); got != "" {
		t.Errorf("guessed a sibling for an extensioned path: %q", got)
	}
	// Query strings usually mean a generated page.
	if got := markdownSibling("https://example.com/search?q=x"); got != "" {
		t.Errorf("guessed a sibling for a query URL: %q", got)
	}
}

func TestBinaryKindNamesPDFRatherThanReturningMojibake(t *testing.T) {
	t.Parallel()
	if got := binaryKind([]byte("%PDF-1.7\n%\xe2\xe3")); got != "PDF" {
		t.Errorf("PDF not detected, got %q", got)
	}
	if got := binaryKind([]byte("# A markdown heading")); got != "" {
		t.Errorf("text misreported as binary: %q", got)
	}
}

func TestLooksLikeHTMLHandlesXHTMLAndSniffs(t *testing.T) {
	t.Parallel()
	if !looksLikeHTML("application/xhtml+xml", nil) {
		t.Error("xhtml not recognised as HTML (the old Contains check missed it)")
	}
	if looksLikeHTML("text/markdown; charset=utf-8", []byte("# hi")) {
		t.Error("markdown misclassified as HTML")
	}
	if !looksLikeHTML("application/octet-stream", []byte("<!DOCTYPE html><html>")) {
		t.Error("sniffing failed for a generic content type")
	}
}
