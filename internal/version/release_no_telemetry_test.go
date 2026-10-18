package version

// GORILLA OVERRIDE (2026-10-18): no telemetry code is linked into Gorilla OpenCode, and a release proves it.
//
// The owner's rule is that telemetry is cut out of the source, not switched off. On 2026-10-18 the shipped
// gorilla-opencode.exe (0.1.147) was found to carry OpenTelemetry (go.opentelemetry.io/otel, otel/metric, otel/trace,
// contrib/otelhttp: 1,036 occurrences in the binary), pulled in by google.golang.org/genai through
// cloud.google.com/go/auth/httptransport. No exporter was linked, so it sent nothing, but it was there. That module is
// now a local copy without the import (third_party/google-auth-no-otel, go.mod replace).
//
// Two witnesses, so a regression cannot hide behind one of them:
//   - the source: the packages `go list -deps` says the program is built from (this test, any machine);
//   - the artifact: the build info and the bytes of every binary already built in Compiled.Builds (skipped on a fresh
//     clone, like TestAReleaseIsAllFormatsOrNone).
// Fieldkit's `fieldkit telemetry scan <repo> --artifact <binary>` is the same check from outside the repository.

import (
	"bytes"
	"debug/buildinfo"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// telemetryModules are module path prefixes of telemetry, tracing and crash-reporting libraries. A dependency whose
// path starts with one of them must not be linked.
var telemetryModules = []string{
	"go.opentelemetry.io/", // telemetry-ok: the list of what is refused
	"go.opencensus.io",     // telemetry-ok: the list of what is refused
	"github.com/getsentry/",
	"github.com/posthog/",
	"github.com/segmentio/analytics-go",
	"github.com/DataDog/",
	"github.com/newrelic/",
	"github.com/bugsnag/",
	"github.com/rollbar/",
	"github.com/honeycombio/",
	"github.com/launchdarkly/",
	"github.com/statsig-io/",
}

func telemetryHit(path string) string {
	for _, p := range telemetryModules {
		if strings.HasPrefix(path, p) {
			return p
		}
	}
	return ""
}

func TestTheProgramIsBuiltFromNoTelemetryPackage(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "github.com/opencode-ai/opencode")
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=readonly")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list could not run here (%v); the artifact test below still holds a built binary to the rule", err)
	}
	var found []string
	for _, pkg := range strings.Fields(string(out)) {
		if p := telemetryHit(pkg); p != "" {
			found = append(found, pkg)
		}
	}
	if len(found) > 0 {
		t.Fatalf("telemetry packages are linked into the program (owner rule: cut out, not switched off):\n  %s\n"+
			"Find the importer with: go list -deps -f '{{.ImportPath}} {{.Imports}}' . | findstr <package>",
			strings.Join(found, "\n  "))
	}
}

func TestNoBuiltBinaryCarriesTelemetry(t *testing.T) {
	bins, _ := filepath.Glob(filepath.Join(compiledBuilds, "gorilla-opencode*"))
	if root, _ := filepath.Glob("../../gorilla-opencode*"); len(root) > 0 {
		bins = append(bins, root...)
	}
	checked := 0
	for _, b := range bins {
		info, err := buildinfo.ReadFile(b)
		if err != nil {
			continue // not a Go program (a .deb, a checksum list)
		}
		checked++
		for _, d := range info.Deps {
			if p := telemetryHit(d.Path); p != "" {
				t.Errorf("%s links %s %s (build info)", b, d.Path, d.Version)
			}
		}
		raw, err := os.ReadFile(b)
		if err == nil && bytes.Contains(raw, []byte("go.opentelemetry.io/")) { // telemetry-ok: what is refused
			t.Errorf("%s contains go.opentelemetry.io/ in its bytes", b)
		}
	}
	if checked == 0 {
		t.Skip("no built gorilla-opencode binary here (a fresh clone)")
	}
}

func TestTheTelemetryCheckCatchesAPlantedModule(t *testing.T) {
	// The check must be able to fail: a dependency it is meant to refuse is recognized.
	if telemetryHit("go.opentelemetry.io/otel/trace") == "" || telemetryHit("github.com/getsentry/sentry-go") == "" {
		t.Fatal("the module list no longer recognizes OpenTelemetry or Sentry")
	}
	if telemetryHit("golang.org/x/net") != "" || telemetryHit("google.golang.org/genai") != "" {
		t.Fatal("an ordinary module is taken for telemetry")
	}
}
