package agent

// GORILLA OVERRIDE (2026-10-04): notice when the model is going round in
// circles, say so, and stop before the bill does.
//
// # THE FAILURE
//
// toolinput.go records it twice: a model sends a call, reads the error, and
// "forms the SAME call again, and the pair loop until the user gives up. Four
// identical failures in forty-four seconds, each one paid for." Each of those
// was fixed at its cause. Nothing watched for the loop itself, so the next
// cause — whatever it is — loops the same way until a person notices.
//
// # WHAT IS WATCHED
//
// Three patterns, all over the calls made since the person last spoke:
//
//	repeat     the same call returning the same result, again and again
//	error      the same call failing, again and again (the result may differ)
//	ping-pong  two calls alternating, A B A B A B, each with its same result
//
// Each has two thresholds. At the first the model is TOLD, in the tool result,
// that it is repeating itself and what to do instead — most loops end there,
// because the model was not aware of them. At the second the turn is STOPPED
// with a plain sentence for the person, who can say "continue" if the repeats
// were intended.
//
// # WHERE IT CAME FROM
//
// The three patterns and the stop are OpenHands' stuck detector
// (OpenHands/software-agent-sdk, conversation/stuck_detector.py, MIT), which
// stops at 4 repeats, 3 errors, or 6 alternating steps. The warn-first step is
// DeepSeek Harness's repeat-tool reminder (deepseek-ai/deepseek-harness,
// packages/guard/repeat-tool-reminder, MIT), which reminds at 3, 5 and 8 and
// never stops. This warns once and then stops, because a reminder that can be
// ignored forever is paid for forever. goose's RepetitionInspector
// (aaif-goose/goose, tool_monitor.rs, Apache-2.0) is the same idea with one
// pattern. The code here is ours.
//
// # WHAT IT DELIBERATELY DOES NOT DO
//
// It does not compare "similar" calls. Only exact repeats count: same tool,
// same arguments once key order and spacing are normalised. A model reading
// the next 200 lines of a file, or re-running a build after an edit, is doing
// its job and produces a different call or a different result each time.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

const (
	stuckWarnRepeat   = 3  // identical call + identical result
	stuckStopRepeat   = 5  //
	stuckWarnError    = 3  // identical call, failing each time
	stuckStopError    = 5  //
	stuckWarnPingPong = 6  // A B A B A B  (three full rounds)
	stuckStopPingPong = 10 // five full rounds

	// stuckWindow is how many calls are remembered. It only needs to hold the
	// longest pattern.
	stuckWindow = 16
)

type stuckCall struct {
	tool    string
	key     string // tool + normalised arguments
	result  string // hash of the result
	isError bool
}

// StuckVerdict is what the tracker says about the call just recorded.
type StuckVerdict struct {
	// Note, when not empty, is appended to the tool result so the model reads
	// it. It is a warning at the first threshold and an explanation at the stop.
	Note string
	// Stop, when not empty, is the sentence for the person; the turn ends.
	Stop string
}

type stuckTracker struct {
	mu       sync.Mutex
	sessions map[string][]stuckCall
}

func newStuckTracker() *stuckTracker {
	return &stuckTracker{sessions: map[string][]stuckCall{}}
}

// Reset forgets a session's history. Called when the person speaks: they have
// seen what happened and are asking for the next thing.
func (s *stuckTracker) Reset(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// Record notes one finished tool call and reports whether it completes a loop.
func (s *stuckTracker) Record(sessionID, tool, input, result string, isError bool) StuckVerdict {
	call := stuckCall{
		tool:    tool,
		key:     tool + "\x00" + normaliseArguments(input),
		result:  hashResult(result),
		isError: isError,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	h := append(s.sessions[sessionID], call)
	if len(h) > stuckWindow {
		h = h[len(h)-stuckWindow:]
	}
	s.sessions[sessionID] = h

	if n := trailingErrors(h); n >= stuckWarnError {
		return verdictFor(n, stuckWarnError, stuckStopError, fmt.Sprintf(
			"the %s call has now failed %d times in a row with the same arguments", tool, n),
			"Sending it again will fail again. Read the error above and change the call: "+
				"different arguments, a different tool, or tell the person what is blocking you.")
	}
	if n := trailingRepeats(h); n >= stuckWarnRepeat {
		return verdictFor(n, stuckWarnRepeat, stuckStopRepeat, fmt.Sprintf(
			"the %s call has now been made %d times in a row with the same arguments and the same result", tool, n),
			"Calling it again will return the same thing. Use the result you already have, "+
				"or do something different, or finish and report.")
	}
	if n := trailingPingPong(h); n >= stuckWarnPingPong {
		other := h[len(h)-2].tool
		return verdictFor(n, stuckWarnPingPong, stuckStopPingPong, fmt.Sprintf(
			"%s and %s have been called in turn %d times, each returning the same result every time", other, tool, n/2),
			"Going back and forth is not producing anything new. Use what you already have, "+
				"or try a different approach, or finish and report.")
	}
	return StuckVerdict{}
}

// verdictFor turns a count into a warning (exactly at warn), a stop (at or
// past stop), or nothing in between — one warning, not one per repeat.
func verdictFor(n, warn, stop int, what, advice string) StuckVerdict {
	switch {
	case n >= stop:
		return StuckVerdict{
			Note: "\n\nSTOPPED: " + what + ". This turn is being ended so it does not continue to repeat.",
			Stop: "Stopped because the model was going round in circles: " + what +
				". Nothing was lost — everything up to here is recorded. " +
				"If the repeats were intended, say \"continue\"; otherwise tell it what to try instead.",
		}
	case n == warn:
		return StuckVerdict{Note: "\n\nYOU ARE REPEATING YOURSELF: " + what + ". " + advice}
	}
	return StuckVerdict{}
}

// trailingRepeats counts how many of the most recent calls are the same call
// with the same result.
func trailingRepeats(h []stuckCall) int {
	if len(h) == 0 {
		return 0
	}
	last := h[len(h)-1]
	n := 0
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].key != last.key || h[i].result != last.result {
			break
		}
		n++
	}
	return n
}

// trailingErrors counts how many of the most recent calls are the same call,
// failing. The error text may differ between attempts (a timestamp, a
// temporary file name), so the result is not compared.
func trailingErrors(h []stuckCall) int {
	if len(h) == 0 || !h[len(h)-1].isError {
		return 0
	}
	last := h[len(h)-1]
	n := 0
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].key != last.key || !h[i].isError {
			break
		}
		n++
	}
	return n
}

// trailingPingPong counts the length of an A B A B ... run at the end of the
// history, where A and B are different calls and each returns its own same
// result every time. Returns 0 unless the run is at least four long.
func trailingPingPong(h []stuckCall) int {
	if len(h) < 4 {
		return 0
	}
	a, b := h[len(h)-1], h[len(h)-2]
	if a.key == b.key {
		return 0
	}
	n := 0
	for i := len(h) - 1; i >= 0; i-- {
		want := a
		if (len(h)-1-i)%2 == 1 {
			want = b
		}
		if h[i].key != want.key || h[i].result != want.result {
			break
		}
		n++
	}
	if n < 4 {
		return 0
	}
	return n
}

// normaliseArguments makes two spellings of the same arguments compare equal:
// key order and whitespace are not part of what a call means.
func normaliseArguments(input string) string {
	var v any
	dec := json.NewDecoder(strings.NewReader(input))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return strings.TrimSpace(input) // not JSON: compare it as written
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // encoding/json sorts map keys
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return strings.TrimSpace(input)
	}
	return strings.TrimSpace(buf.String())
}

func hashResult(result string) string {
	sum := sha256.Sum256([]byte(result))
	return hex.EncodeToString(sum[:8])
}
