package cost

import (
	"math"
	"testing"
)

// Claude Sonnet 5 ($2 in, $10 out per million, cache write 1.25x): about 5.5
// cents a short message (the Air billed 4.85 cents for a 19.4K first message,
// which must not exceed the forecast), about 50 cents a long one, about $120
// an hour of the AI working alone.
func TestForecastMatchesTheFiguresTheOwnerWasShown(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < want*0.05 }
	if got := MessageCost(2, 10, ShortConversationTokens); !near(got, 0.055) {
		t.Errorf("short message = %.4f, want about 0.055", got)
	}
	if got := MessageCost(2, 10, LongConversationTokens); !near(got, 0.505) {
		t.Errorf("long message = %.4f, want about 0.505", got)
	}
	if got := WorkingHourCost(2, 10); !near(got, 121.2) {
		t.Errorf("working hour = %.2f, want about 121.2", got)
	}
}

// The measured first message on the Air: 19.4K tokens on Sonnet 5, billed
// $0.0485. An "up to" figure below a real bill is a false promise.
func TestForecastIsNotBelowTheMeasuredBill(t *testing.T) {
	if got := MessageCost(2, 10, 19_400); got < 0.0485 {
		t.Errorf("forecast %.4f is below the measured 0.0485", got)
	}
}
