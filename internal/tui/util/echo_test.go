package util

import "testing"

// ReportInfoEcho marks a notice for the transcript as well as the footer; plain
// ReportInfo does not, so ordinary toasts ("copied to clipboard") never clutter
// the scrollback.
func TestReportInfoEchoOnlyEchoesWhenAsked(t *testing.T) {
	echoed := ReportInfoEcho("long important notice")().(InfoMsg)
	if !echoed.Echo {
		t.Error("ReportInfoEcho did not set Echo; the full text will not reach the transcript")
	}
	if echoed.Msg != "long important notice" {
		t.Errorf("message mangled: %q", echoed.Msg)
	}
	plain := ReportInfo("copied to clipboard")().(InfoMsg)
	if plain.Echo {
		t.Error("a plain ReportInfo echoed to the transcript; ordinary toasts must not")
	}
}
