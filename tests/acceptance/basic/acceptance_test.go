//go:build acceptance

package basic

// The model runs. Not CI: this spends model time and, on Windows, puts a few
// scratch files in the Recycle Bin (job delete_to_bin).
//
//	fieldkit guard -- go test -tags acceptance ./tests/acceptance/basic -run TestAcceptance -timeout 3h -v
//
// Environment:
//
//	ACC_MODELS   comma-separated model ids (default chatgpt.gpt-5.6-luna)
//	ACC_JOBS     comma-separated job ids (default all)
//	ACC_ROOT     scratch root (default a folder under the OS temp dir)
//	ACC_EXE      use this binary instead of building one
//	ACC_HELPERS  folder of the pinned toolbox helpers (default: the installed copy's cache)
//	ACC_PATH     "clean" (default) or "host"; see Config.PathMode
//	ACC_TIMEOUT  per-job limit (default 8m)
//
// Results: results/<model>-<os>.json, one file per model, rewritten after every
// job so an interrupted run keeps what it measured.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAcceptance(t *testing.T) {
	jobs, err := LoadJobs("jobs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if want := os.Getenv("ACC_JOBS"); want != "" {
		keep := map[string]bool{}
		for _, id := range strings.Split(want, ",") {
			keep[strings.TrimSpace(id)] = true
		}
		var sel []Job
		for _, j := range jobs {
			if keep[j.ID] {
				sel = append(sel, j)
			}
		}
		jobs = sel
	}
	models := strings.Split(envOr("ACC_MODELS", "chatgpt.gpt-5.6-luna"), ",")
	root := os.Getenv("ACC_ROOT")
	if root == "" {
		root = filepath.Join(os.TempDir(), "gorilla-acceptance-basic")
	}
	root, _ = filepath.Abs(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures, _ := filepath.Abs("fixtures")
	resultsDir, _ := filepath.Abs("results")
	os.MkdirAll(resultsDir, 0o755)

	cfg := Config{Root: root, Fixtures: fixtures, PathMode: envOr("ACC_PATH", "clean"), Timeout: 8 * time.Minute}
	if d, err := time.ParseDuration(os.Getenv("ACC_TIMEOUT")); err == nil {
		cfg.Timeout = d
	}
	cfg.Exe = os.Getenv("ACC_EXE")
	if cfg.Exe == "" {
		repo, _ := filepath.Abs(filepath.Join("..", "..", ".."))
		cfg.Exe = filepath.Join(root, "gorilla-opencode-acceptance.exe")
		t.Logf("building %s", cfg.Exe)
		cfg.Embedded, err = BuildExe(repo, cfg.Exe, os.Getenv("ACC_HELPERS"))
		if err != nil {
			t.Fatal(err)
		}
	} else {
		cfg.Embedded = os.Getenv("ACC_EMBEDDED") == "1"
	}

	for _, model := range models {
		cfg.Model = strings.TrimSpace(model)
		var results []JobResult
		for _, j := range jobs {
			r, err := RunJob(cfg, j)
			if err != nil {
				t.Errorf("%s/%s: setup: %v", cfg.Model, j.ID, err)
				continue
			}
			results = append(results, r)
			verdict := "PASS"
			if !r.Pass {
				verdict = "FAIL " + r.CauseAuto
			}
			t.Logf("%-18s %-34s %5.0fs %2d calls  %s", j.ID, cfg.Model, r.Seconds, r.ToolCalls, verdict)
			data, _ := json.MarshalIndent(results, "", "  ")
			name := strings.NewReplacer("/", "_", ":", "_").Replace(cfg.Model) + "-" + runtime.GOOS + ".json"
			os.WriteFile(filepath.Join(resultsDir, name), data, 0o644)
		}
	}
}

// TestRenderReport prints the tables from results/ (plus results/causes.yaml)
// to results/tables.md.
func TestRenderReport(t *testing.T) {
	jobs, err := LoadJobs("jobs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rs, err := LoadResults("results")
	if err != nil {
		t.Fatal(err)
	}
	causes, err := LoadCauses(filepath.Join("results", "causes.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("results", "tables.md"), []byte(Render(jobs, rs, causes)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
