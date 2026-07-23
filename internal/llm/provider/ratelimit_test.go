package provider

import (
	"context"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA OVERRIDE: guards the proactive pace-setter. Unlimited must not wait;
// a finite cap must space calls; a cancelled ctx must return promptly.
func TestPaceRequest(t *testing.T) {
	// unlimited -> no measurable wait across several calls
	config.SetRateLimitRPM(0)
	rlNextAllowed = time.Time{}
	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := paceRequest(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("unlimited should not pace, took %v", d)
	}

	// finite cap spaces calls: 120 rpm => 0.5s apart; 3 calls ~= 1.0s
	config.SetRateLimitRPM(120)
	rlNextAllowed = time.Time{}
	start = time.Now()
	for i := 0; i < 3; i++ {
		_ = paceRequest(context.Background())
	}
	if d := time.Since(start); d < 800*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("120rpm×3 spacing off: got %v, want ~1.0s", d)
	}

	// paced-out + cancelled ctx returns an error quickly
	config.SetRateLimitRPM(2) // 30s spacing
	rlNextAllowed = time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := paceRequest(ctx); err == nil {
		t.Error("expected cancellation error while paced out")
	}

	config.SetRateLimitRPM(config.DefaultRPM)
	rlNextAllowed = time.Time{}
}

func TestRPMLadderStepping(t *testing.T) {
	config.SetRateLimitRPM(25)
	if got := config.StepRateLimitRPM(+1); got != 30 { // faster
		t.Errorf("up from 25 = %d, want 30", got)
	}
	config.SetRateLimitRPM(3)
	if got := config.StepRateLimitRPM(-1); got != 2 { // slower
		t.Errorf("down from 3 = %d, want 2", got)
	}
	if got := config.StepRateLimitRPM(-1); got != 2 { // clamp at floor
		t.Errorf("down from 2 should clamp, got %d", got)
	}
	config.SetRateLimitRPM(config.DefaultRPM)
}
