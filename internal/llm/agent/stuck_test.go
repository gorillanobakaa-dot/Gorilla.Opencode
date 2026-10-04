package agent

// GORILLA OVERRIDE (2026-10-04): a loop is told once, then stopped — and a
// model doing its job is never called stuck.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

func record(s *stuckTracker, tool, input, result string, isErr bool) StuckVerdict {
	return s.Record("sess", tool, input, result, isErr)
}

// The failure recorded in toolinput.go: the same call, the same error, again.
func TestTheSameFailingCallIsWarnedThenStopped(t *testing.T) {
	s := newStuckTracker()
	in := `{"url":"https://example.org/","timeout":"30"}`
	var notes, stops []int
	for i := 1; i <= stuckStopError; i++ {
		// A different error text each time: the comparison must not depend on it.
		v := record(s, "web_fetch", in, fmt.Sprintf("invalid parameters (attempt %d)", i), true)
		if v.Note != "" {
			notes = append(notes, i)
		}
		if v.Stop != "" {
			stops = append(stops, i)
		}
	}
	if fmt.Sprint(notes) != fmt.Sprint([]int{stuckWarnError, stuckStopError}) {
		t.Errorf("the model was told at attempts %v; want exactly at %d (warning) and %d (stop)", notes, stuckWarnError, stuckStopError)
	}
	if fmt.Sprint(stops) != fmt.Sprint([]int{stuckStopError}) {
		t.Errorf("the turn was stopped at attempts %v; want only at %d", stops, stuckStopError)
	}
}

func TestTheSameCallWithTheSameResultIsWarnedThenStopped(t *testing.T) {
	s := newStuckTracker()
	var warned, stopped int
	for i := 1; i <= stuckStopRepeat; i++ {
		v := record(s, "view", `{"file_path":"main.go"}`, "package main\n", false)
		if v.Note != "" && v.Stop == "" {
			warned = i
			if !strings.Contains(v.Note, "REPEATING") || !strings.Contains(v.Note, "view") {
				t.Errorf("the warning does not say what is being repeated: %q", v.Note)
			}
		}
		if v.Stop != "" {
			stopped = i
			if !strings.Contains(v.Stop, "continue") {
				t.Errorf("the stop message does not tell the person how to carry on: %q", v.Stop)
			}
		}
	}
	if warned != stuckWarnRepeat || stopped != stuckStopRepeat {
		t.Errorf("warned at %d and stopped at %d; want %d and %d", warned, stopped, stuckWarnRepeat, stuckStopRepeat)
	}
}

// Key order and spacing are not part of what a call means.
func TestTheSameArgumentsSpelledDifferentlyAreTheSameCall(t *testing.T) {
	s := newStuckTracker()
	record(s, "find", `{"pattern":"x","path":"."}`, "none", false)
	record(s, "find", `{ "path": ".", "pattern": "x" }`, "none", false)
	if v := record(s, "find", `{"path":".","pattern":"x"}`, "none", false); v.Note == "" {
		t.Errorf("three identical calls with reordered keys were not recognised as a repeat")
	}
}

func TestTwoCallsAlternatingAreWarnedThenStopped(t *testing.T) {
	s := newStuckTracker()
	var warned, stopped int
	for i := 1; i <= stuckStopPingPong; i++ {
		var v StuckVerdict
		if i%2 == 1 {
			v = record(s, "view", `{"file_path":"a.go"}`, "A", false)
		} else {
			v = record(s, "find", `{"pattern":"foo"}`, "B", false)
		}
		if v.Note != "" && v.Stop == "" && warned == 0 {
			warned = i
		}
		if v.Stop != "" && stopped == 0 {
			stopped = i
		}
	}
	if warned != stuckWarnPingPong || stopped != stuckStopPingPong {
		t.Errorf("warned at %d and stopped at %d; want %d and %d", warned, stopped, stuckWarnPingPong, stuckStopPingPong)
	}
}

// CAPABILITY GUARD. Each of these is a model working, not a model stuck.
func TestOrdinaryWorkIsNeverCalledStuck(t *testing.T) {
	cases := map[string]func(s *stuckTracker) StuckVerdict{
		"reading a file in pages": func(s *stuckTracker) (v StuckVerdict) {
			for i := 0; i < 12; i++ {
				v = record(s, "view", fmt.Sprintf(`{"file_path":"big.c","offset":%d}`, i*200), fmt.Sprintf("page %d", i), false)
				if v.Note != "" {
					return v
				}
			}
			return v
		},
		"build, edit, build again": func(s *stuckTracker) (v StuckVerdict) {
			for i := 0; i < 8; i++ {
				// The same build command, a DIFFERENT result each time.
				v = record(s, "bash", `{"command":"go build ./..."}`, fmt.Sprintf("%d errors", 8-i), i < 7)
				if v.Stop != "" {
					return v
				}
				if v2 := record(s, "edit", fmt.Sprintf(`{"file_path":"f%d.go"}`, i), "ok", false); v2.Note != "" {
					return v2
				}
			}
			return StuckVerdict{}
		},
		"polling until something changes": func(s *stuckTracker) (v StuckVerdict) {
			for i := 0; i < 4; i++ {
				v = record(s, "bash", `{"command":"git status --short"}`, fmt.Sprintf("%d files", i), false)
				if v.Note != "" {
					return v
				}
			}
			return v
		},
		"the same call twice": func(s *stuckTracker) (v StuckVerdict) {
			record(s, "view", `{"file_path":"a.go"}`, "A", false)
			return record(s, "view", `{"file_path":"a.go"}`, "A", false)
		},
		"two different failures": func(s *stuckTracker) (v StuckVerdict) {
			record(s, "bash", `{"command":"make"}`, "err", true)
			record(s, "bash", `{"command":"make -j2"}`, "err", true)
			return record(s, "bash", `{"command":"make V=1"}`, "err", true)
		},
	}
	for name, run := range cases {
		if v := run(newStuckTracker()); v.Note != "" || v.Stop != "" {
			t.Errorf("REGRESSION: %s was called stuck: %q", name, v.Note+v.Stop)
		}
	}
}

// The person speaking is the boundary: what was repeated before is forgotten.
func TestANewMessageFromThePersonClearsTheCount(t *testing.T) {
	s := newStuckTracker()
	for i := 0; i < stuckStopRepeat-1; i++ {
		record(s, "view", `{"file_path":"a.go"}`, "A", false)
	}
	s.Reset("sess")
	if v := record(s, "view", `{"file_path":"a.go"}`, "A", false); v.Note != "" || v.Stop != "" {
		t.Errorf("the count survived a new message from the person: %q", v.Note+v.Stop)
	}
}

// One session's loop must not stop another's work.
func TestSessionsAreCountedSeparately(t *testing.T) {
	s := newStuckTracker()
	for i := 0; i < stuckStopRepeat; i++ {
		s.Record("looping", "view", `{"file_path":"a.go"}`, "A", false)
	}
	if v := s.Record("working", "view", `{"file_path":"a.go"}`, "A", false); v.Note != "" {
		t.Errorf("a different session inherited the count")
	}
}

// The choke point: a cancelled or refused call is not the model repeating
// itself, and the note must not change what is compared next time.
func TestFinishToolResultsRecordsOnlyCallsThatRan(t *testing.T) {
	stuck.Reset("choke")
	t.Cleanup(func() { stuck.Reset("choke") })

	call := message.ToolCall{Name: "view", Input: `{"file_path":"a.go"}`}
	stop := ""
	var last string
	for i := 0; i < stuckStopRepeat; i++ {
		results := []message.ToolResult{{Content: "package main\n"}}
		stop = finishToolResults("choke", []message.ToolCall{call}, results, []bool{true})
		last = results[0].Content
	}
	if stop == "" || !strings.Contains(last, "STOPPED") {
		t.Fatalf("%d identical calls through the choke point did not stop the turn (a note appended to the result must not make it look different)", stuckStopRepeat)
	}

	stuck.Reset("choke")
	for i := 0; i < stuckStopRepeat+2; i++ {
		results := []message.ToolResult{{Content: "Tool execution canceled by user", IsError: true}}
		if got := finishToolResults("choke", []message.ToolCall{call}, results, []bool{false}); got != "" {
			t.Fatalf("cancelled calls were counted as the model repeating itself")
		}
		if results[0].Content != "Tool execution canceled by user" {
			t.Fatalf("a result for a call that never ran was altered")
		}
	}
}
