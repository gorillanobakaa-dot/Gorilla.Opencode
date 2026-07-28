package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// The reported bug: three lines of
//
//	WARN ignoring duplicate local endpoint kept=... ignored=... baseURL=...
//
// printed onto the terminal on every launch, and stayed there once the TUI took
// the screen.
//
// The warning itself is correct and wanted — it is the record of a real problem
// in the user's config. The defect was WHERE it went. Load configured the slog
// handler about fifty lines after it called registerLocalEndpoints, so those
// warnings were emitted while slog's built-in default handler was still in
// force, and that handler writes to stderr.
//
// This reproduces the fresh-process condition inside the test binary. A brand
// new process starts with a default handler nobody chose; here we install one
// whose output we can inspect and then assert Load never touches it. Any step
// that logs before configureLogging shows up in the buffer — which in a real
// process means burned onto the user's screen.
func TestLoadLogsNothingBeforeTheLoggerIsConfigured(t *testing.T) {
	// Load short-circuits on the package-global cfg, so it has to be cleared to
	// exercise the real sequence. Restore it, and viper, for the tests that
	// follow — this package's globals leak otherwise.
	prevCfg := cfg
	cfg = nil
	t.Cleanup(func() {
		cfg = prevCfg
		viper.Reset()
	})

	// Two endpoints sharing a baseURL: the exact shape that triggers the
	// warning. The port is dead on purpose, so registration fails too — its
	// error must be swallowed by the logger as well, not printed.
	path := GorillaConfigFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"localEndpoints": []LocalEndpoint{
			{Name: "first", BaseURL: "http://127.0.0.1:1/v1", APIKey: "a"},
			{Name: "second", BaseURL: "http://127.0.0.1:1/v1", APIKey: "b"},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })

	// Stand in for the stderr a fresh process would be logging to.
	var stray bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&stray, nil)))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if stray.Len() > 0 {
		t.Errorf("Load logged %d bytes before configuring the logger; in a real run this is printed to stderr and then painted over by the TUI, unclearable:\n%s",
			stray.Len(), stray.String())
	}
}

// And prove the duplicate warning is still produced — moving it out of the way
// of the screen must not amount to deleting it. It is the only signal the user
// has that two config entries are fighting over one baseURL.
func TestDuplicateEndpointStillWarns(t *testing.T) {
	// Don't assume another test has populated the global cfg — this must pass
	// when run alone with -run too.
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prevEndpoints := cfg.LocalEndpoints
	t.Cleanup(func() { cfg.LocalEndpoints = prevEndpoints })

	cfg.LocalEndpoints = []LocalEndpoint{
		{Name: "keeper", BaseURL: "http://127.0.0.1:1/v1", APIKey: "a"},
		{Name: "loser", BaseURL: "http://127.0.0.1:1/v1", APIKey: "b"},
	}

	var logged bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	registerLocalEndpoints()

	out := logged.String()
	if !strings.Contains(out, "duplicate local endpoint") {
		t.Errorf("the duplicate warning was lost, not redirected:\n%s", out)
	}
	if !strings.Contains(out, "keeper") || !strings.Contains(out, "loser") {
		t.Errorf("the warning must name which entry was kept and which ignored:\n%s", out)
	}
}
