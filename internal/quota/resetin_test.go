package quota

import (
	"testing"
	"time"
)

func TestResetInFormat(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{4*time.Hour + 58*time.Minute + 30*time.Second, "resets in 4h 59m"}, // rounds up
		{5 * time.Hour, "resets in 5h"},
		{30 * time.Minute, "resets in 30m"},
		{59*time.Minute + 1*time.Second, "resets in 1h"},
		{3*24*time.Hour + 15*time.Hour, "resets in 3d 15h"},
		{3*24*time.Hour + 14*time.Hour + 20*time.Minute, "resets in 3d 15h"},
		{23 * 24 * time.Hour, "resets in 23d"},
	} {
		if got := ResetIn(tc.d); got != tc.want {
			t.Errorf("ResetIn(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
