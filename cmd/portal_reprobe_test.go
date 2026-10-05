// GORILLA FIX (2026-10-05): the provider menu must not re-ask a connection's
// models at every launch.
//
// Measured on v0.1.140, opened in the owner's home folder: ready 1.3 s after Esc
// at the menu, 11.0 s after Enter on the NVIDIA row. The ten seconds were three
// test questions to NVIDIA, repeated at every start to learn what the last
// start had already learned.
package cmd

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// reprobeFixture registers one model on the NVIDIA connection, saves the
// connection with a key, and counts how often the models are asked.
func reprobeFixture(t *testing.T) (id models.ModelID, asked *int) {
	t.Helper()
	loadCfg(t)
	id = models.ModelID("local.reprobe/model-120b")
	models.SupportedModels[id] = models.Model{ID: id, Name: "Reprobe Model", Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096}
	models.RegisterLocalRouteForTestNamed(id, nimBaseURL, "nvapi-stored", nimEndpointName)
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{Name: nimEndpointName, BaseURL: nimBaseURL, APIKey: "nvapi-stored"}); err != nil {
		t.Fatal(err)
	}
	origReg, origProbe := registerLocalEndpoint, probeEndpoint
	registerLocalEndpoint = func(string, string, string) (int, models.ModelID) { return 1, id }
	n := 0
	probeEndpoint = func(name, _ string) models.EndpointProbeReport {
		n++
		return models.EndpointProbeReport{Endpoint: name, Asked: 1, Working: []models.ModelID{id}, Failed: map[models.ModelID]models.ProbeVerdict{}}
	}
	t.Cleanup(func() {
		registerLocalEndpoint, probeEndpoint = origReg, origProbe
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
		models.ClearProbeVerdictForTest(id)
		_ = config.RemoveLocalEndpoint(nimEndpointName)
	})
	return id, &n
}

func TestARecentCheckIsReusedWhenTheSameConnectionIsChosenAgain(t *testing.T) {
	id, asked := reprobeFixture(t)
	models.SetProbeVerdictForTest(id, models.ProbeWorks, time.Now().Add(-2*time.Hour))

	// Enter on the row already set up: no key typed.
	if err := applyLocalEndpoint(nimEndpointName, nimBaseURL, ""); err != nil {
		t.Fatal(err)
	}
	if *asked != 0 {
		t.Fatalf("the models were asked %d time(s) again, two hours after they answered", *asked)
	}
	if got := config.Get().Agents[config.AgentCoder].Model; got != id {
		t.Errorf("the coder is on %q, want the model that was seen to answer", got)
	}
}

func TestTheModelsAreAskedWhenItMatters(t *testing.T) {
	for name, c := range map[string]struct {
		key     string
		verdict string
		age     time.Duration
		none    bool
	}{
		"a new key was typed":              {key: "nvapi-brand-new", verdict: models.ProbeWorks, age: time.Minute},
		"the last check is over a day old": {verdict: models.ProbeWorks, age: 25 * time.Hour},
		"the last check found it broken":   {verdict: models.ProbeError, age: time.Minute},
		"it was never checked":             {none: true},
	} {
		id, asked := reprobeFixture(t)
		if !c.none {
			models.SetProbeVerdictForTest(id, c.verdict, time.Now().Add(-c.age))
		}
		if err := applyLocalEndpoint(nimEndpointName, nimBaseURL, c.key); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if *asked != 1 {
			t.Errorf("%s: the models were asked %d time(s), want 1", name, *asked)
		}
	}
}

func TestEndpointProvenWithinReadsTheFreshestGoodAnswer(t *testing.T) {
	id, _ := reprobeFixture(t)
	if _, ok := models.EndpointProvenWithin(nimEndpointName, 24*time.Hour); ok {
		t.Fatal("a connection with no recorded answer counts as proven")
	}
	models.SetProbeVerdictForTest(id, models.ProbeGone, time.Now())
	if _, ok := models.EndpointProvenWithin(nimEndpointName, 24*time.Hour); ok {
		t.Fatal("a retired model counts as proof the connection works")
	}
	models.SetProbeVerdictForTest(id, models.ProbeWorks, time.Now().Add(-3*time.Hour))
	age, ok := models.EndpointProvenWithin(nimEndpointName, 24*time.Hour)
	if !ok || age < 2*time.Hour || age > 4*time.Hour {
		t.Fatalf("age=%s ok=%v, want about three hours", age, ok)
	}
	if _, ok := models.EndpointProvenWithin(nimEndpointName, time.Hour); ok {
		t.Fatal("a three-hour-old answer counts as fresh within one hour")
	}
	if _, ok := models.EndpointProvenWithin("some other connection", 24*time.Hour); ok {
		t.Fatal("another connection inherited this one's proof")
	}
}
