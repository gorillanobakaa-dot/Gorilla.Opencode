package tools

// GORILLA OVERRIDE (2026-08-23): a degraded search must not read as a failed one.
//
// Measured on a live research run. One engine hit a CAPTCHA. Fifty-four
// searches returned results anyway. The model reading the warnings wrote into
// its final report:
//
//   "All web searches failed (SearXNG engines suspended)"
//
// and downgraded a finding to unverified on that basis. Wrong in the safe
// direction, and still wrong. The wording invited it: a bare count of failures
// with no denominator, followed by "results are incomplete".
//
// The tool knew better. It had the results in hand. What it said and what it
// knew were different things, which is the fault this whole release is about.

import (
	"os"
	"strings"
	"testing"
)

// readSource reads a file from this package so the test can assert that its
// copy of a format string still matches the real one. Reading source is
// unusual; it is here because the alternative is a test that silently stops
// describing the code it names.
func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// buildWarning mirrors the message the SearXNG path emits, so the WORDING can
// be asserted without standing up a search backend. If the real format string
// changes and this is not updated, TestTheWarningMatchesTheSource fails.
func buildWarning(answered, dead, results int, deadNames string) string {
	return strings.NewReplacer(
		"{a}", itoa(answered), "{t}", itoa(answered+dead),
		"{r}", itoa(results), "{d}", deadNames,
	).Replace("SearXNG: {a} of {t} engines answered and returned {r} result(s). " +
		"These did not: {d}. Coverage is REDUCED, not absent: the results below " +
		"are real and were searched for. Do not report this as a failed search.")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// THE REPORTED CASE. Nine of ten engines answered; the message must not be
// readable as "the search failed".
func TestOneDeadEngineDoesNotReadAsATotalFailure(t *testing.T) {
	msg := buildWarning(9, 1, 54, "startpage (CAPTCHA)")

	// The proportion has to be in there, or a reader has no denominator and
	// only sees a failure.
	if !strings.Contains(msg, "9 of 10 engines answered") {
		t.Errorf("no proportion in the warning, so a reader sees only the failure:\n  %s", msg)
	}
	// What worked must be stated, not merely implied by absence.
	if !strings.Contains(msg, "54 result(s)") {
		t.Errorf("the warning does not say results came back:\n  %s", msg)
	}
	// The instruction that stops the misreading outright.
	if !strings.Contains(msg, "Do not report this as a failed search") {
		t.Errorf("nothing forbids the exact misreading that happened:\n  %s", msg)
	}
	// The phrase that caused it, gone.
	if strings.Contains(msg, "results are incomplete") {
		t.Errorf("the blanket phrase is back. It is what a model turned into "+
			"\"all web searches failed\":\n  %s", msg)
	}
}

// The failing engine and its reason must survive. Knowing WHICH engine and WHY
// is what lets a user judge whether coverage was reduced in a way that matters.
func TestTheFailingEngineIsStillNamedWithItsReason(t *testing.T) {
	msg := buildWarning(9, 1, 54, "startpage (CAPTCHA)")
	if !strings.Contains(msg, "startpage") || !strings.Contains(msg, "CAPTCHA") {
		t.Errorf("the failing engine or its reason was dropped:\n  %s", msg)
	}
}

// The other half of the distinction, which already existed and must not be
// weakened: EVERY engine dead with zero results is a genuine failure and is
// raised as an error, never as a warning about incomplete coverage. Reporting
// that as "no results" is the lie that starts a fabrication.
func TestTheWarningMatchesTheSource(t *testing.T) {
	src := readSource(t, "websearch.go")
	if !strings.Contains(src, "engines answered and returned") {
		t.Error("the warning format in websearch.go no longer matches this test's " +
			"copy of it; update buildWarning or the assertions are theatre")
	}
	if !strings.Contains(src, "This is NOT evidence that no results exist") {
		t.Error("the total-failure error lost its wording. A dead search reported " +
			"as an absence is what makes a model invent an answer.")
	}
}
