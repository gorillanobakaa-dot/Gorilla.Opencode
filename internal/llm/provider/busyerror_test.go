package provider

import (
	"errors"
	"testing"
)

func TestIsServerBusyError(t *testing.T) {
	busy := []string{
		// The exact NVIDIA NIM in-band stream error this all started with:
		`received error while streaming: {"message":"ResourceExhausted: Worker local total request limit reached (46/300)"}`,
		"ResourceExhausted: worker busy",
		"Error: too many requests, please retry",
		"rate limit exceeded",
		"the model is overloaded",
		"503 Service Unavailable",
		"temporarily unavailable, try again later",
	}
	for _, m := range busy {
		if !isServerBusyError(errors.New(m)) {
			t.Errorf("isServerBusyError(%q) = false, want true", m)
		}
	}

	notBusy := []string{
		"",
		"invalid api key",
		"context deadline exceeded",
		"json: cannot unmarshal",
		"model not found",
	}
	for _, m := range notBusy {
		if m == "" {
			if isServerBusyError(nil) {
				t.Error("isServerBusyError(nil) = true, want false")
			}
			continue
		}
		if isServerBusyError(errors.New(m)) {
			t.Errorf("isServerBusyError(%q) = true, want false", m)
		}
	}
}

// A server-busy error must NOT be misclassified as a mere transport blip — they
// get different back-off schedules, and this guards that separation.
func TestServerBusyIsNotTransientTransport(t *testing.T) {
	err := errors.New(`{"message":"ResourceExhausted: Worker local total request limit reached (46/300)"}`)
	if !isServerBusyError(err) {
		t.Fatal("expected server-busy classification")
	}
	if isTransientStreamError(err) {
		t.Fatal("ResourceExhausted should not be classified as a transport error")
	}
}
