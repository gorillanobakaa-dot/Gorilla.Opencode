package chat

// GORILLA OVERRIDE (2026-09-02): every printed line must begin at column 0.
//
// bubbletea's renderer leaves the cursor at the END of the last frame line --
// that line is written without a trailing newline -- and repositions for the
// next flush with ansi.CursorUp, which changes the ROW and not the COLUMN. It
// then writes queued Println lines with the carriage return AFTER the text:
//
//	buf.WriteString(line)
//	buf.WriteString("\r\n")
//
// so the first line lands wherever the previous frame ended, wraps, and
// corrupts the row. The renderer emits a "\r" for the frame itself, but only
// when i == 0 && r.lastRender == "", which never covers the queued lines.
//
// The owner caught it in a real session: a stranded footer reading
// "odel Gemma 4 E" -- the leading "m" clipped, the signature of a write that
// began one column too far right.
//
// Printing is irreversible. A line written to the wrong column cannot be
// withdrawn and repainted, so this is enforced at the point of emission.

import (
	"os"
	"strings"
	"testing"
)

func TestEveryPrintedLineStartsAtColumnZero(t *testing.T) {
	for _, body := range []string{
		"model Gemma 4 E | context 10.5K",
		"",
		"  indented tool output",
		"multi\nline\nbody",
	} {
		got, ok := printedBody(printLine(body))
		if !ok {
			t.Fatalf("printLine(%q) did not produce a printable message", body)
		}
		if !strings.HasPrefix(got, "\r") {
			t.Errorf("printLine(%q) produced %q, which does not begin with a carriage "+
				"return. bubbletea writes the first queued line at whatever column "+
				"the previous frame left the cursor at, so without it this line "+
				"lands mid-row and overwrites the frame.", body, got)
		}
	}
}

// The carriage return must be added exactly once. Two would still render
// correctly, but a helper that doubles its own prefix is one refactor away from
// doubling something that does not tolerate it.
func TestTheCarriageReturnIsNotDoubled(t *testing.T) {
	got, ok := printedBody(printLine("x"))
	if !ok {
		t.Fatal("printLine did not produce a printable message")
	}
	if strings.HasPrefix(got, "\r\r") {
		t.Errorf("printLine doubled the carriage return: %q", got)
	}
}

// Nothing in this package may call tea.Println directly: one unguarded call is
// enough to reintroduce the fault, and it would show up as a corrupted row far
// from the line that caused it.
func TestNothingCallsTeaPrintlnDirectly(t *testing.T) {
	src, err := os.ReadFile("printer.go")
	if err != nil {
		t.Skipf("cannot read printer.go: %v", err)
	}
	body := string(src)
	// The helper itself is the one legitimate caller.
	body = strings.Replace(body, `return tea.Println("\r" + text)`, "", 1)
	if strings.Contains(body, "tea.Println(") {
		t.Error("printer.go calls tea.Println directly. Use printLine, which puts " +
			"the cursor at column 0 first; bubbletea does not do it for queued lines.")
	}
}
