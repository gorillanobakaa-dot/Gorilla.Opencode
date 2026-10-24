package agent

import (
	"strings"
	"testing"
)

// The words the person reads: the share, the model, the window, where to
// change it, and the cuts when there are any.
func TestTheCrowdedNoticeSaysWhatAndWhere(t *testing.T) {
	got := crowdedText(12592, 20224, "Gemma 4", 0.62, " Biggest you could switch off: Review ~839 (~839 back).")
	for _, want := range []string{"Gemma 4", "62%", "/context", "Review", "first answer"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice lacks %q:\n%s", want, got)
		}
	}
}

func TestTheCrowdedNoticeIsSaidOncePerSession(t *testing.T) {
	crowdWarned.Delete("s-once")
	if _, told := crowdWarned.LoadOrStore("s-once", struct{}{}); told {
		t.Fatal("a fresh session counted as already told")
	}
	if _, told := crowdWarned.LoadOrStore("s-once", struct{}{}); !told {
		t.Error("the second ask was not remembered")
	}
}
