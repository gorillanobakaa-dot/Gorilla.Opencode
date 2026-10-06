package config

// GORILLA (2026-10-06): the research forecast is priced from this machine's
// measured runs once one exists. These pin the three things that make that
// true: no run means the assumed basis and says so; runs mean the MEDIAN per
// session and the money follows it; the two forecast entry points agree.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// isolatedRuns points the run history at an empty folder for one test. The
// package's TestMain already redirects the cache; this gives each test its own
// empty one so a run recorded by one cannot leak into another.
func isolatedRuns(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if !strings.HasPrefix(ResearchRunsPath(), dir) {
		t.Fatalf("cache redirection failed: %s", ResearchRunsPath())
	}
}

var forecastModel = models.Model{
	ID: "test/forecast", Name: "Forecast Test", Provider: models.ProviderLocal,
	CostPer1MIn: 1.00, CostPer1MOut: 4.00,
}

func TestForecastBasisIsAssumedUntilARunIsMeasured(t *testing.T) {
	isolatedRuns(t)

	in, out, runs, measured := ResearchHelperSessionTokens()
	if measured || runs != 0 {
		t.Fatalf("no run on record, yet basis reports measured=%v runs=%d", measured, runs)
	}
	if in != ResearchStepsPerHelper*ResearchHelperBasisTokens() || out != ResearchStepsPerHelper*ResearchOutputPerStep {
		t.Errorf("assumed basis is in=%d out=%d, want %d steps x (%d context, %d out)",
			in, out, ResearchStepsPerHelper, ResearchHelperBasisTokens(), ResearchOutputPerStep)
	}

	// The money is the old per-step arithmetic exactly: nothing may have moved
	// for a machine that has never finished a run.
	perStep := float64(ResearchHelperBasisTokens())/1e6*forecastModel.CostPer1MIn + float64(ResearchOutputPerStep)/1e6*forecastModel.CostPer1MOut
	perHelper, perMinute := helperSessionForecast(forecastModel, 4)
	if want := perStep * ResearchStepsPerHelper; !nearly(perHelper, want) {
		t.Errorf("assumed per-helper money = %.6f, want %.6f", perHelper, want)
	}
	spm, _ := helperStepsPerMinute()
	if want := perStep * 4 * spm; !nearly(perMinute, want) {
		t.Errorf("assumed per-minute money = %.6f, want %.6f", perMinute, want)
	}

	// Incomplete or empty rows are not a measurement.
	RecordResearchRun(0, 100, 100, 1, time.Minute)
	RecordResearchRun(4, 0, 0, 1, time.Minute)
	if _, _, _, measured := ResearchHelperSessionTokens(); measured {
		t.Error("an empty run was taken as a measurement")
	}
}

func TestForecastBasisIsTheMedianOfMeasuredRuns(t *testing.T) {
	isolatedRuns(t)

	// Three runs of different sizes per session: 20,000, 35,093 and 1,000,000.
	// The median is the August run; the storm must not move it.
	RecordResearchRun(4, 72000, 8000, 16, 5*time.Minute)
	RecordResearchRun(8, 248122, 32622, 96, 13*time.Minute)
	RecordResearchRun(2, 1990000, 10000, 8, 30*time.Minute)

	tokens, calls, runs, ok := MeasuredRunSize()
	if !ok || runs != 3 || tokens != 35093 {
		t.Fatalf("MeasuredRunSize = %d tokens over %d runs ok=%v, want 35093 over 3", tokens, runs, ok)
	}
	if calls != 4 { // medians of 4, 12 and 4 calls per session
		t.Errorf("median tool calls per session = %v, want 4", calls)
	}

	in, out, n, measured := ResearchHelperSessionTokens()
	if !measured || n != 3 {
		t.Fatalf("basis measured=%v runs=%d, want measured over 3", measured, n)
	}
	if int64(in+out) != tokens {
		t.Errorf("basis in+out = %d, but the screen prints %d as measured; they must be one number", in+out, tokens)
	}
	// The split follows the history's own input share, so the output side is
	// neither zero nor the whole.
	if out <= 0 || out >= in {
		t.Errorf("basis split in=%d out=%d: output is expected to be the smaller, non-zero part", in, out)
	}

	// The money follows from exactly that split at the model's prices.
	perHelper, perMinute := helperSessionForecast(forecastModel, 2)
	if want := float64(in)/1e6*forecastModel.CostPer1MIn + float64(out)/1e6*forecastModel.CostPer1MOut; !nearly(perHelper, want) {
		t.Errorf("measured per-helper money = %.6f, want %.6f from in=%d out=%d", perHelper, want, in, out)
	}
	spm, _ := helperStepsPerMinute()
	if want := perHelper * 2 * spm / ResearchStepsPerHelper; !nearly(perMinute, want) {
		t.Errorf("measured per-minute money = %.6f, want %.6f", perMinute, want)
	}

	// The quota comparison and the synthesis turn count the same session.
	if ordinary := ResearchOrdinaryQuestionTokens(); ordinary > 0 {
		if got, want := ResearchQuotaMultiple(10), int(float64(10*(in+out))/float64(ordinary)+0.5); got != want && got != want+1 && got != want-1 {
			t.Errorf("quota multiple for 10 helpers = %d, want about %d from the measured basis", got, want)
		}
	}
	coder := LoadoutActiveTokens()
	if coder <= 0 {
		coder = 8000
	}
	if _, synth := ResearchOrchestratorTokens(5); synth != coder+5*out {
		t.Errorf("synthesis input carries %d helper tokens for 5 sessions, want 5 x %d measured out", synth-coder, out)
	}

	// Newest ResearchRunsKept are kept; a corrupt file is no measurement.
	for i := 0; i < ResearchRunsKept+3; i++ {
		RecordResearchRun(4, 4000, 0, 4, time.Minute)
	}
	if tokens, _, runs, _ := MeasuredRunSize(); runs != ResearchRunsKept || tokens != 1000 {
		t.Errorf("history holds %d runs at %d tokens/session, want %d at 1000", runs, tokens, ResearchRunsKept)
	}
	if err := os.WriteFile(ResearchRunsPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, measured := ResearchHelperSessionTokens(); measured {
		t.Error("a corrupt history file produced a measured basis")
	}
	if _, err := os.Stat(filepath.Dir(ResearchRunsPath())); err != nil {
		t.Errorf("history folder missing: %v", err)
	}
}

// ResearchCost and ResearchPaidEquivalent price one engine and must give one
// per-helper figure for the same model, measured or not.
func TestResearchCostAndPaidEquivalentAgreeOnTheBasis(t *testing.T) {
	isolatedRuns(t)
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	const id = models.ModelID("local.test/forecast-helper")
	models.SupportedModels[id] = models.Model{
		ID: id, Name: "Forecast Helper", Provider: models.ProviderLocal,
		CostPer1MIn: 0.10, CostPer1MOut: 0.40, ContextWindow: 128000, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(id, "http://127.0.0.1:1/v1", "k", "test-endpoint")
	t.Cleanup(func() {
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
	})
	for _, a := range []AgentName{AgentTask, AgentResearch} {
		if err := UpdateAgentModel(a, id); err != nil {
			t.Fatalf("pin %s: %v", a, err)
		}
	}

	check := func(label string) float64 {
		t.Helper()
		perHelper, perMinute, _, _, priced := ResearchCost(3)
		if !priced {
			t.Fatalf("%s: helper model not priced", label)
		}
		pm, ph, _, ok := ResearchPaidEquivalent(models.SupportedModels[id], 3)
		if !ok || !nearly(ph, perHelper) || !nearly(pm, perMinute) {
			t.Errorf("%s: ResearchCost says %.6f/helper %.6f/min, ResearchPaidEquivalent says %.6f and %.6f", label, perHelper, perMinute, ph, pm)
		}
		return perHelper
	}
	before := check("assumed")
	RecordResearchRun(8, 248122, 32622, 96, 13*time.Minute)
	after := check("measured")
	in, out, _, _ := ResearchHelperSessionTokens()
	assumed := ResearchStepsPerHelper * (ResearchHelperBasisTokens() + ResearchOutputPerStep)
	ratio := float64(in+out) / float64(assumed)
	if moved := after / before; moved < ratio*0.8 || moved > ratio*1.25 {
		t.Errorf("per-helper money moved %.2fx on a measurement; the bases differ %.2fx", moved, ratio)
	}
}

func nearly(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 1e-9*(1+b)
}
