package tools

import "testing"

// The exact call that looped on 2026-08-20, copied from the transcript.
func TestTheWebFetchCallThatLoopedNowDecodes(t *testing.T) {
	const wire = `{"url":"https://www.debian.org/","format":"markdown","timeout":"30"}`
	var p FetchParams
	if err := UnmarshalToolInput(wire, &p); err != nil {
		t.Fatalf("still refused: %v", err)
	}
	if p.Timeout != 30 {
		t.Errorf("timeout = %d, want 30", p.Timeout)
	}
	if p.URL != "https://www.debian.org/" {
		t.Errorf("url mangled: %q", p.URL)
	}
	if p.Format != "markdown" {
		t.Errorf("format mangled: %q", p.Format)
	}
}

func TestQuotedBooleansAreAccepted(t *testing.T) {
	var p FetchParams
	if err := UnmarshalToolInput(`{"url":"u","summarise":"true"}`, &p); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !p.Summarise {
		t.Error("summarise did not become true")
	}
}

// The relaxation must be TYPE-DIRECTED. A string field handed "true" or "30"
// keeps its text, or a search for the word "true" would silently become a
// search for a boolean.
func TestStringFieldsAreNeverCoerced(t *testing.T) {
	var p FindParams
	if err := UnmarshalToolInput(`{"query":"true","path":"30"}`, &p); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if p.Query != "true" {
		t.Errorf("query became %q - a string field was coerced", p.Query)
	}
	if p.Path != "30" {
		t.Errorf("path became %q - a string field was coerced", p.Path)
	}
}

// Nonsense must stay an error the model can read and act on. Accepting
// "30 seconds" as 30 would be inventing intent.
func TestUnparseableValuesStillFail(t *testing.T) {
	var p FetchParams
	err := UnmarshalToolInput(`{"url":"u","timeout":"30 seconds"}`, &p)
	if err == nil {
		t.Fatal("accepted \"30 seconds\" as a number")
	}
	if !contains(err.Error(), "timeout") {
		t.Errorf("error %q should name the field the model got wrong", err)
	}
}

// Well-formed input must not be reshaped on its way through.
func TestCorrectInputIsUntouched(t *testing.T) {
	var p FetchParams
	if err := UnmarshalToolInput(`{"url":"u","timeout":45,"summarise":true}`, &p); err != nil {
		t.Fatalf("refused valid input: %v", err)
	}
	if p.Timeout != 45 || !p.Summarise {
		t.Errorf("valid input altered: %+v", p)
	}
}

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
