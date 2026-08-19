package tools

// GORILLA OVERRIDE (2026-08-19): narrowing a page before it becomes tokens.
//
// A tool result is re-sent on every later turn, so a fetched page is a
// recurring bill rather than a one-off. Fetching a documentation page to read
// one table and paying for the navigation, sidebar, cookie banner and footer
// as well is the single easiest saving available, and it costs two schema
// properties.

import (
	"strings"
	"testing"
)

const samplePage = `<html><head><style>body{}</style></head><body>
<nav><a href="/home">Home</a><a href="/about">About</a></nav>
<main>
  <h1>Install</h1>
  <p>Run the installer.</p>
  <table><tr><td>debian</td><td>12</td></tr><tr><td>arch</td><td>rolling</td></tr></table>
  <a class="download" href="https://example.invalid/pkg.deb">Debian package</a>
  <a class="download" href="https://example.invalid/pkg.tar.zst">Arch package</a>
</main>
<footer>Copyright nobody</footer>
</body></html>`

func TestSelectorKeepsOnlyWhatWasAskedFor(t *testing.T) {
	got, n, err := applySelector(samplePage, "table", "")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if n != 1 {
		t.Fatalf("matched %d elements, want 1", n)
	}
	if !strings.Contains(got, "debian") {
		t.Error("the table content was lost")
	}
	for _, chrome := range []string{"Copyright nobody", "About", "Run the installer"} {
		if strings.Contains(got, chrome) {
			t.Errorf("page chrome %q survived narrowing — the saving is the point", chrome)
		}
	}
}

// A selector that matched nothing is a mistake to be told about, not an empty
// document to reason over. Silently returning nothing is how a model concludes
// "the page has no install instructions" from a typo.
func TestAZeroMatchIsReportedAsZero(t *testing.T) {
	got, n, err := applySelector(samplePage, ".does-not-exist", "")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if n != 0 {
		t.Fatalf("matched %d, want 0", n)
	}
	if got != "" {
		t.Errorf("a zero match returned content: %q", got)
	}
}

func TestExtractTextDropsMarkup(t *testing.T) {
	got, _, err := applySelector(samplePage, "main h1", "text")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if got != "Install" {
		t.Errorf("got %q, want %q", got, "Install")
	}
}

func TestExtractAttributeReturnsTheAttributeAlone(t *testing.T) {
	got, n, err := applySelector(samplePage, "a.download", "href")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if n != 2 {
		t.Fatalf("matched %d, want 2", n)
	}
	want := "https://example.invalid/pkg.deb\nhttps://example.invalid/pkg.tar.zst"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A bare list of URLs makes the model guess which one it wanted from the path.
func TestExtractLinksKeepsTheLinkTextToo(t *testing.T) {
	got, _, err := applySelector(samplePage, "main", "links")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if !strings.Contains(got, "Debian package") || !strings.Contains(got, "pkg.deb") {
		t.Errorf("links lost their text or their href:\n%s", got)
	}
}

// "Matched, but the thing you asked for is not on them" is a different failure
// from "matched nothing", and laundering one into the other loses the reason.
func TestAMissingAttributeIsNotReportedAsAMissingElement(t *testing.T) {
	got, n, err := applySelector(samplePage, "a.download", "data-checksum")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if n != 2 {
		t.Fatalf("matched %d, want 2 — the elements ARE there", n)
	}
	if !strings.Contains(got, "matched") || !strings.Contains(got, "data-checksum") {
		t.Errorf("the explanation did not say what was missing: %q", got)
	}
}

// The saving has to be real, or the two schema properties are not worth their
// permanent token cost.
func TestNarrowingActuallySavesBytes(t *testing.T) {
	full, err := convertHTMLToMarkdown(samplePage)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	narrow, _, err := applySelector(samplePage, "table", "text")
	if err != nil {
		t.Fatalf("applySelector: %v", err)
	}
	if len(narrow) >= len(full) {
		t.Fatalf("narrowing produced %d bytes against %d for the whole page — no saving",
			len(narrow), len(full))
	}
	t.Logf("whole page %d bytes, narrowed %d bytes (%.0f%% of the original)",
		len(full), len(narrow), float64(len(narrow))/float64(len(full))*100)
}
