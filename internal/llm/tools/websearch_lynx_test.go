package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// mojeekDump is a trimmed but VERBATIM `lynx -dump` capture from
// https://www.mojeek.com/search?q=tcp+bbr+congestion on 2026-08-09 — the
// engine's own chrome at the top, three real results, and lynx's References
// list. Written from observed output rather than invented, because a fixture I
// made up would only prove the parser agrees with my idea of the format.
const mojeekDump = `   Mojeek

   tcp bbr congestion__[3]✕ (BUTTON) [4]Mojeek User Survey
     * [ ] Search
          + [5]Web
          + [6]Images
     * [ ] Company
          + [9]About

     * [40]https://cloud.google.com › ... › ... ›
       tcp-bbr-congestion-control-com...
       [41]TCP BBR congestion control comes to GCP – your Internet just
       We're excited to announce that Google Cloud Platform  (GCP) now
       features a cutting-edge new congestion control algorithm, TCP BBR,
       which achieves ...
     * [42]https://toonk.io › tcp-bbr-exploring-tcp-congestion-control ›
       index.html
       [43]TCP BBR - Exploring TCP congestion control
       Bottleneck Bandwidth and Round-trip propagation time (BBR) is a TCP
       congestion control algorithm developed at Google in 2016.
       [44]See more results from toonk.io »
     * [45]https://lwn.net › Articles › 701165
       [46]BBR congestion control [LWN.net]
       BBR isn't a traffic control algorithm (like CoDel is), it's a TCP
       congestion control algorithm (like CUBIC is).

References

   Visible links:
   3. https://www.mojeek.com/search?q=tcp+bbr+congestion
   4. https://blocksurvey.io/mojeek-stub-survey-short-2024-28uiW3
   5. https://www.mojeek.com/
   6. https://www.mojeek.com/images
   9. https://www.mojeek.com/about.html
   40. https://cloud.google.com/blog/products/gcp/tcp-bbr-congestion-control-comes-to-gcp
   41. https://cloud.google.com/blog/products/gcp/tcp-bbr-congestion-control-comes-to-gcp
   42. https://toonk.io/tcp-bbr-exploring-tcp-congestion-control/index.html
   43. https://toonk.io/tcp-bbr-exploring-tcp-congestion-control/index.html
   45. https://lwn.net/Articles/701165/
   46. https://lwn.net/Articles/701165/
`

func TestParseLynxDumpExtractsResultsNotChrome(t *testing.T) {
	hits := parseLynxDump(mojeekDump, "mojeek.com", 10)

	if len(hits) != 3 {
		t.Fatalf("want 3 results, got %d: %+v", len(hits), hits)
	}

	// The engine's own nav must not become results.
	for _, h := range hits {
		if strings.Contains(h.URL, "mojeek.com") {
			t.Errorf("mojeek's own chrome leaked into results: %s", h.URL)
		}
	}

	// The address and the title point at the same URL; the TITLE must win.
	if hits[0].Title != "TCP BBR congestion control comes to GCP – your Internet just" {
		t.Errorf("title should beat the displayed address, got %q", hits[0].Title)
	}
	if hits[0].URL != "https://cloud.google.com/blog/products/gcp/tcp-bbr-congestion-control-comes-to-gcp" {
		t.Errorf("wrong URL: %s", hits[0].URL)
	}
	if !strings.Contains(hits[0].Abstract, "cutting-edge new congestion control") {
		t.Errorf("snippet not captured, got %q", hits[0].Abstract)
	}
	// Same target appearing twice (address link + title link) is ONE result.
	if hits[1].URL == hits[0].URL {
		t.Error("duplicate URLs were not merged")
	}
	if hits[2].Title != "BBR congestion control [LWN.net]" {
		t.Errorf("third title wrong: %q", hits[2].Title)
	}
}

// Regression for the worst bug this backend produced: a real title attached to
// the WRONG url.
//
// Verbatim from a Brave capture, 2026-08-09. lynx wrapped "More on reddit.com"
// across two lines, so the link's label read as the unusable "More on" — the
// first parser then searched forward for a title and ran past the block's
// furniture into the NEXT result's heading. The output looked entirely
// legitimate: a real Reddit URL, a real title, about different things.
//
// The fix is structural, not a bigger blocklist: a marker WITH text has its
// label, and looking elsewhere is never allowed.
func TestParseLynxDumpNeverBorrowsATitleFromAnotherResult(t *testing.T) {
	const braveDiscussions = `   Windows 11 24H2 and BBR2
   (BUTTON)
   It breaks 'localhost' (the loopback interface) TCP traffic leading to
   slow or unresponsive connections within the same machine. [15]More on
   reddit.com
   🌐 r/Windows11
   3

   SMB Performance fixed by switching TCP congestion algo to BBR
   (BUTTON)
   Some other discussion entirely. [16]More on
   reddit.com

References

   Visible links:
   15. https://www.reddit.com/r/Windows11/comments/1h07kj5/windows_11_24h2_and_bbr2/
   16. https://www.reddit.com/r/homelab/comments/17iwe0v/smb_performance/
`
	for _, h := range parseLynxDump(braveDiscussions, "brave.com", 10) {
		if strings.Contains(h.URL, "1h07kj5") && strings.Contains(h.Title, "SMB Performance") {
			t.Fatalf("title borrowed from the next result: %q -> %s", h.Title, h.URL)
		}
		if isChromeLink(h.Title) {
			t.Errorf(`"More on" furniture became a result: %q -> %s`, h.Title, h.URL)
		}
	}
}

// A favicon glyph followed by a bare domain is not a title, though it has a
// space and is long enough in BYTES to look like one.
func TestParseLynxDumpRejectsFaviconAndBareDomain(t *testing.T) {
	if plausibleTitle("🌐 youtube.com") {
		t.Error(`"🌐 youtube.com" must not pass as a title`)
	}
	if plausibleTitle("atoonk.medium.com ›") {
		t.Error("a trailing breadcrumb must not pass as a title")
	}
	if !plausibleTitle("BBR: Congestion-Based Congestion Control (paper review)") {
		t.Error("a real title was rejected")
	}
}

func TestParseLynxDumpRespectsMax(t *testing.T) {
	if got := len(parseLynxDump(mojeekDump, "mojeek.com", 2)); got != 2 {
		t.Errorf("max=2 should cap results, got %d", got)
	}
}

// The load-bearing case, and the reason byte counts and exit codes were both
// rejected as success signals. On 2026-08-08 a rate-limited DuckDuckGo returned
// 12,075 bytes at exit 0, and a Chrome-spoofed request returned a 1,122-byte
// CAPTCHA at exit 0. Both are "successful" by every measure except the only one
// that matters: did any result come out.
func TestParseLynxDumpTreatsCaptchaAsNoResults(t *testing.T) {
	const captcha = `   #DuckDuckGo (Lite)

   Unfortunately, bots use DuckDuckGo too.
   Please complete the following challenge to confirm this search was made
   by a human.
   Select all squares containing a duck:
   [1]Images not loading?

References

   Visible links:
   1. https://duckduckgo.com/lite/
`
	if hits := parseLynxDump(captcha, "duckduckgo.com", 10); len(hits) != 0 {
		t.Fatalf("a CAPTCHA page must yield zero results, got %d: %+v", len(hits), hits)
	}
}

func TestParseLynxDumpHandlesGarbage(t *testing.T) {
	for name, in := range map[string]string{
		"empty":         "",
		"no references": "   Update your browser\n   Your browser isn't supported any more.",
		"refs only":     "\nReferences\n\n   1. https://example.com/\n",
		"truncated":     "   [1]Something\n\nReferences\n   1. notaurl\n",
	} {
		t.Run(name, func(t *testing.T) {
			if hits := parseLynxDump(in, "example.com", 10); len(hits) != 0 {
				t.Errorf("want 0 results from %s, got %d", name, len(hits))
			}
		})
	}
}

// Google serves text browsers "Your browser isn't supported any more" with no
// search box at all — 208 bytes, measured 2026-08-08. It must read as failure.
func TestParseLynxDumpGoogleRefusal(t *testing.T) {
	const refusal = `   Update your browser
   Your browser isn't supported any more. To continue your search, upgrade
   to a recent version. [1]Learn more

References

   1. https://support.google.com/websearch/answer/16515119
`
	// The one link is off-domain, so a naive "any external URL" check would
	// call this a result. It is not one: nothing here answers the query.
	hits := parseLynxDump(refusal, "google.com", 10)
	for _, h := range hits {
		if strings.Contains(strings.ToLower(h.Title), "learn more") {
			t.Errorf("a support link in a refusal page is not a search result: %+v", h)
		}
	}
}

func TestSearchLynxReportsMissingBinary(t *testing.T) {
	if lynxPath() != "" {
		t.Skip("lynx is installed; this covers the machine where it is not")
	}
	_, err := newTestTool().searchLynx(context.Background(), "anything", 5)
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("missing lynx must be reported plainly, got: %v", err)
	}
}

// Live check. Opt-in — it queries real search engines.
//
//	GORILLA_LIVE_SEARCH=1 go test ./internal/llm/tools/ -run LiveLynx -v
func TestLiveLynxSearch(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("set GORILLA_LIVE_SEARCH=1 to run (queries real search engines)")
	}
	if lynxPath() == "" {
		t.Skip("lynx not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx, warnings := withSearchWarnings(ctx)

	hits, err := newTestTool().searchLynx(ctx, "tcp bbr congestion control", 8)
	if err != nil {
		t.Fatalf("live lynx search failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits for a query that certainly has some")
	}
	// "Title is non-empty" is NOT enough. The first live run of this backend
	// passed that check while every title read "More on reddit.com", and a later
	// one attached a real title to the wrong URL. Assert the properties that
	// distinguish a result from furniture.
	for i, h := range hits {
		if !strings.HasPrefix(h.URL, "http") {
			t.Errorf("hit %d has no usable URL: %+v", i, h)
		}
		if !plausibleTitle(h.Title) {
			t.Errorf("hit %d has a junk title (chrome, breadcrumb or bare domain): %q -> %s",
				i, h.Title, h.URL)
		}
		if isChromeLink(h.Title) {
			t.Errorf("hit %d is engine furniture, not a result: %q", i, h.Title)
		}
	}
	t.Logf("%d hits via %s, %d warning(s): %v", len(hits), hits[0].Backend, len(*warnings), *warnings)
	for i, h := range hits {
		if i < 4 {
			t.Logf("  %s — %s", h.Title, h.URL)
		}
	}
}
