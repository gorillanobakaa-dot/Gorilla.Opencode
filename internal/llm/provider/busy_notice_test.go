// Version: 1.0.0 · updated 26-08-21-15-50
package provider

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA FIX (2026-08-21): the back-off line must name WHO is busy.
//
// It used to say only "Provider busy (rate-limit/5xx)". The owner read that
// under a footer claiming Antigravity and concluded Google was throttling a
// months-old account — while the requests were really going to NVIDIA NIM,
// because of the stale-provider bug fixed the same day. A message that cannot be
// checked invites a wrong explanation, and I supplied one.
func TestBusyNoticeNamesTheProviderAndModel(t *testing.T) {
	o := &openaiClient{providerOptions: providerClientOptions{
		model: models.Model{
			ID: "groq.openai/gpt-oss-120b", APIModel: "openai/gpt-oss-120b",
			Provider: models.ProviderGROQ,
		},
	}}
	got := o.busyNotice(2, 4800)
	for _, want := range []string{"groq", "openai/gpt-oss-120b", "2/", "4.8s"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %q", want, got)
		}
	}
}

// A local endpoint is somebody's own machine or key. The name THEY gave it is
// the only identifier that means anything — "local busy" would tell the user
// nothing about which of their endpoints is struggling.
func TestBusyNoticeUsesTheUsersOwnEndpointName(t *testing.T) {
	const id models.ModelID = "local.meta/llama-3.3-70b-instruct"
	models.RegisterLocalRouteForTestNamed(id, "https://integrate.api.nvidia.com/v1", "k", "Gorilla.FREE.NVIDIA.NIM")
	t.Cleanup(func() { models.ClearLocalRouteForTest(id) })

	o := &openaiClient{providerOptions: providerClientOptions{
		model: models.Model{ID: id, APIModel: "meta/llama-3.3-70b-instruct", Provider: models.ProviderLocal},
	}}
	got := o.busyNotice(1, 1200)
	if !strings.Contains(got, "Gorilla.FREE.NVIDIA.NIM") {
		t.Errorf("does not name the endpoint the user configured: %q", got)
	}
	if strings.Contains(got, "local busy") {
		t.Errorf("says %q, which identifies nothing", got)
	}
}
