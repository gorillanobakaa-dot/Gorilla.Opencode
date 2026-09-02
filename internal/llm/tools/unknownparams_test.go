package tools

// GORILLA OVERRIDE (2026-09-02): a parameter the tool has never heard of must
// not be dropped in silence.
//
// Observed live. Gemma 4 called view with {"file_path": "<a directory>",
// "view": "tree"}. There is no "view" parameter -- view takes file_path, offset
// and limit -- so it was discarded without a word. The only feedback the model
// received was "path is a directory", and a turn later it was still theorising
// about the parameter it had invented: "or the tool defaults to reading a single
// file if no specific file is named".
//
// It had no way to learn otherwise. json.Unmarshal ignores unknown fields, so a
// malformed call and a correct one are indistinguishable from the model's side.

import (
	"reflect"
	"testing"
)

func schema() ToolInfo {
	return ToolInfo{
		Name: "view",
		Parameters: map[string]any{
			"file_path": map[string]any{"type": "string"},
			"offset":    map[string]any{"type": "integer"},
			"limit":     map[string]any{"type": "integer"},
		},
		Required: []string{"file_path"},
	}
}

func TestTheInventedParameterIsReported(t *testing.T) {
	got := UnknownParams(schema(), `{"file_path":"/tmp/x","view":"tree"}`)
	if !reflect.DeepEqual(got, []string{"view"}) {
		t.Errorf("UnknownParams = %v, want [view]. This is the exact call that "+
			"cost a round trip: the model invented a parameter and nothing told it so.", got)
	}
}

func TestAWellFormedCallIsNotNagged(t *testing.T) {
	if got := UnknownParams(schema(), `{"file_path":"/tmp/x","limit":50}`); got != nil {
		t.Errorf("UnknownParams = %v on a correct call; a note on every call would "+
			"be noise and would teach the model nothing", got)
	}
}

// Several unknowns are reported together and in a stable order, so the note
// reads the same way twice and cannot flap between turns.
func TestSeveralUnknownsComeBackSorted(t *testing.T) {
	got := UnknownParams(schema(), `{"file_path":"x","zebra":1,"alpha":2,"view":"tree"}`)
	want := []string{"alpha", "view", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnknownParams = %v, want %v", got, want)
	}
}

// Malformed JSON is the tool's own error to report. Guessing at parameter names
// inside something that does not parse would only add noise to whatever error
// the tool is about to produce anyway.
func TestUnparseableInputIsLeftToTheTool(t *testing.T) {
	for _, bad := range []string{"", "{not json", "[]", "null"} {
		if got := UnknownParams(schema(), bad); got != nil {
			t.Errorf("UnknownParams(%q) = %v, want nil", bad, got)
		}
	}
}

// A tool that declares no schema cannot judge what is foreign to it, and must
// not accuse the model of inventing something.
func TestASchemalessToolAccusesNobody(t *testing.T) {
	if got := UnknownParams(ToolInfo{Name: "x"}, `{"anything":1}`); got != nil {
		t.Errorf("UnknownParams = %v for a tool with no declared parameters", got)
	}
}

// The note tells the model what it SHOULD have sent, so the list has to be
// complete and ordered.
func TestDeclaredParamsListsEverythingSorted(t *testing.T) {
	got := DeclaredParams(schema())
	want := []string{"file_path", "limit", "offset"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeclaredParams = %v, want %v", got, want)
	}
}

// Every real tool must be able to answer the question. A tool whose schema is
// empty silently opts out of this check, which is how the fault would return.
func TestTheViewToolStillDeclaresItsParameters(t *testing.T) {
	info := NewViewTool(nil).Info()
	got := DeclaredParams(info)
	if len(got) == 0 {
		t.Fatal("the view tool declares no parameters, so nothing can tell a model " +
			"that view=\"tree\" is not one of them")
	}
	for _, want := range []string{"file_path"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("view no longer declares %q; its parameters are %v", want, got)
		}
	}
	if u := UnknownParams(info, `{"file_path":"x","view":"tree"}`); len(u) != 1 || u[0] != "view" {
		t.Errorf("the real view tool did not flag view=\"tree\": %v", u)
	}
}
