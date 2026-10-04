package tools

// GORILLA OVERRIDE (2026-10-04): the two repairs taken in idea from
// alibaba/open-code-review's comment_args_repair.go, and the checks that stop
// them inventing data. Every case here is a call a model really forms.

import (
	"reflect"
	"testing"
)

type repairWrite struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type repairItem struct {
	Content string `json:"content"`
	Path    string `json:"path"`
}

type repairBatch struct {
	Items   []repairItem `json:"items"`
	Pattern string       `json:"pattern"`
	Timeout int          `json:"timeout"`
}

// The failure this exists for: a file written with a real newline inside the
// JSON string. Strict JSON refuses it and the content is lost.
func TestARealNewlineInsideAStringIsRecovered(t *testing.T) {
	raw := "{\"file_path\":\"a.txt\",\"content\":\"line one" + "\n" + "line two" + "\t" + "end\"}"
	var p repairWrite
	if err := UnmarshalToolInput(raw, &p); err != nil {
		t.Fatalf("a bare newline inside a string still fails the call: %v", err)
	}
	if want := "line one\nline two\tend"; p.Content != want {
		t.Errorf("content was altered by the repair: got %q, want %q", p.Content, want)
	}
}

// Both breakages in one call: an unescaped Windows path and a bare newline.
func TestAWindowsPathAndANewlineTogetherAreRecovered(t *testing.T) {
	raw := "{\"file_path\":\"C:\\Users\\someone\\a.txt\",\"content\":\"x" + "\n" + "y\"}"
	var p repairWrite
	if err := UnmarshalToolInput(raw, &p); err != nil {
		t.Fatalf("path and newline together still fail: %v", err)
	}
	if p.FilePath != `C:\Users\someone\a.txt` || p.Content != "x\ny" {
		t.Errorf("recovered the wrong values: %+v", p)
	}
}

// A newline BETWEEN tokens is legal JSON whitespace and must stay untouched.
func TestWhitespaceOutsideStringsIsNotEscaped(t *testing.T) {
	if _, changed := escapeBareControls([]byte("{\n\t\"a\": \"b\"\n}")); changed {
		t.Errorf("legal whitespace between tokens was rewritten")
	}
}

func TestAnArraySentAsAStringIsUnwrapped(t *testing.T) {
	var p SparseParams
	if err := UnmarshalToolInput(`{"file_path":"f.c","includes":"[\"inc\",\"arch/x86\"]"}`, &p); err != nil {
		t.Fatalf("a list serialised into a string still fails: %v", err)
	}
	if !reflect.DeepEqual(p.Includes, []string{"inc", "arch/x86"}) {
		t.Errorf("unwrapped to the wrong list: %#v", p.Includes)
	}
}

func TestOneBareStringForAListBecomesAListOfOne(t *testing.T) {
	var p SparseParams
	if err := UnmarshalToolInput(`{"file_path":"f.c","defines":"DEBUG=1"}`, &p); err != nil {
		t.Fatalf("a single string where a list is wanted still fails: %v", err)
	}
	if !reflect.DeepEqual(p.Defines, []string{"DEBUG=1"}) {
		t.Errorf("got %#v, want one element", p.Defines)
	}
}

// One level of escaping dropped: prose quotes inside a serialised list.
func TestProseQuotesInsideASerialisedListAreRepaired(t *testing.T) {
	raw := `{"items":"[{\"content\":\"rename \"tmp\" to something clearer\",\"path\":\"a.go\"}]"}`
	var p repairBatch
	if err := UnmarshalToolInput(raw, &p); err != nil {
		t.Fatalf("a recoverable batch was refused: %v", err)
	}
	if len(p.Items) != 1 || p.Items[0].Content != `rename "tmp" to something clearer` || p.Items[0].Path != "a.go" {
		t.Errorf("repaired to the wrong values: %+v", p.Items)
	}
}

// The repair's one possible mistake is ending a string early. A value cut at a
// quote that happens to precede a comma must be refused, not delivered short.
func TestAMisjudgedTerminatorIsRefusedNotDeliveredShort(t *testing.T) {
	raw := `{"items":"[{\"content\":\"he said \"stop\", then left\",\"path\":\"a.go\"}]"}`
	var p repairBatch
	if err := UnmarshalToolInput(raw, &p); err == nil {
		t.Fatalf("an ambiguous batch was accepted; a truncated value may have been delivered: %+v", p.Items)
	}
}

// The case the acceptance checks exist for, end to end. The prose here contains
// `"x","bogus":"y"`, so the repair ends the string after x and re-reads the rest
// as structure. The result is VALID JSON and decodes without complaint — into a
// content cut short at `say "x`. Only the acceptance checks stand between that
// and the tool. (Found by mutation: with the checks removed, every other test
// in this file still passed.)
func TestAValidLookingButWrongRepairIsRefused(t *testing.T) {
	raw := `{"items":"[{\"content\":\"say \"x\",\"bogus\":\"y\",\"path\":\"a.go\"}]"}`
	var p repairBatch
	if err := UnmarshalToolInput(raw, &p); err == nil {
		t.Fatalf("a repair that cut the value short was delivered: %+v", p.Items)
	}
}

// Prose re-read as structure must not smuggle in a field the schema lacks.
func TestARepairThatInventsAFieldIsRefused(t *testing.T) {
	if repairedContainerAcceptable(`[{"content":"a","bogus":"b"}]`, reflect.TypeOf([]repairItem{})) {
		t.Errorf("a field the destination does not define was accepted")
	}
	if repairedContainerAcceptable(`[{"content":"an \" odd one","path":"p"}]`, reflect.TypeOf([]repairItem{})) {
		t.Errorf("a value with an unpaired quote was accepted")
	}
	if !repairedContainerAcceptable(`[{"content":"a \"b\" c","path":"p"}]`, reflect.TypeOf([]repairItem{})) {
		t.Errorf("a sound repair was refused")
	}
}

// A string headed for a STRING field is never reshaped, even when it looks like
// a list and another field in the same call does need coercing.
func TestAStringFieldThatLooksLikeAListStaysAString(t *testing.T) {
	var p repairBatch
	if err := UnmarshalToolInput(`{"pattern":"[a-z]","timeout":"30"}`, &p); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if p.Pattern != "[a-z]" || p.Timeout != 30 {
		t.Errorf("got %+v", p)
	}
}

// When nothing can be repaired the model must see the error for what IT sent.
func TestAnUnrepairableCallKeepsTheOriginalError(t *testing.T) {
	var p repairBatch
	err := UnmarshalToolInput(`{"items":"[{\"content\":`, &p)
	if err == nil {
		t.Fatalf("garbage was accepted")
	}
	var q repairBatch
	if err := UnmarshalToolInput(`{"timeout":"soon"}`, &q); err == nil {
		t.Fatalf("a non-number was accepted as a number")
	}
}
