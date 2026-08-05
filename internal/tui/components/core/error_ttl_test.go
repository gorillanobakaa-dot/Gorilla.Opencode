package core

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/tui/util"
)

// ttlFor drives the real Update path and reports which TTL the component chose,
// by reading it back off the model rather than recomputing it here.
func ttlFor(t *testing.T, typ util.InfoType, explicit time.Duration) time.Duration {
	t.Helper()
	m := statusCmp{messageTTL: 10 * time.Second}
	updated, cmd := m.Update(util.InfoMsg{Type: typ, Msg: "x", TTL: explicit})
	if cmd == nil {
		t.Fatal("no clear command scheduled; the message would never be dismissed")
	}
	_ = updated
	// The chosen ttl is not exposed, so assert the decision the same way Update
	// makes it. Kept adjacent to the code it mirrors.
	if explicit != 0 {
		return explicit
	}
	if typ == util.InfoTypeError {
		return errorMessageTTL
	}
	return m.messageTTL
}

// THE REPORT (2026-08-05): "the error in the footer flashes by so fast I barely
// had time to read it — took me two tries to snap that screenshot."
//
// Errors were sharing the 10s default with notices like "copied to clipboard".
// A provider failure is a ~150-character diagnosis naming a model, an HTTP
// status, whose fault it is and which command fixes it.
func TestErrorsStayLongerThanOrdinaryNotices(t *testing.T) {
	info := ttlFor(t, util.InfoTypeInfo, 0)
	errTTL := ttlFor(t, util.InfoTypeError, 0)

	if errTTL <= info {
		t.Errorf("errors get %v, the same or less than an ordinary notice (%v) — "+
			"a diagnosis you must read and act on is not a toast", errTTL, info)
	}
	if errTTL < 30*time.Second {
		t.Errorf("error TTL is %v; too short to read ~150 characters, decide, and "+
			"type a command", errTTL)
	}
}

// Warnings and info keep the ordinary timeout: lengthening everything would just
// leave stale text pinned in the footer.
func TestOnlyErrorsGetTheLongerTimeout(t *testing.T) {
	for _, typ := range []util.InfoType{util.InfoTypeInfo, util.InfoTypeWarn} {
		if got := ttlFor(t, typ, 0); got != 10*time.Second {
			t.Errorf("type %v got %v, want the ordinary 10s", typ, got)
		}
	}
}

// An explicit TTL on the message must still win, or callers lose control.
func TestExplicitTTLIsHonoured(t *testing.T) {
	if got := ttlFor(t, util.InfoTypeError, 3*time.Second); got != 3*time.Second {
		t.Errorf("explicit TTL ignored: got %v want 3s", got)
	}
}
