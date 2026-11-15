package page

import (
	"strings"
	"testing"
)

func TestEscNoticeAsksContinueOrChange(t *testing.T) {
	for _, toolOnly := range []bool{true, false} {
		n := escStoppedNotice(toolOnly)
		if !strings.Contains(n, "Continue or change?") || !strings.Contains(n, "continue") {
			t.Errorf("toolOnly=%v: notice does not ask: %q", toolOnly, n)
		}
	}
}
