package basic

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedJobsFileLoads(t *testing.T) {
	jobs, err := LoadJobs("jobs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) < 30 {
		t.Fatalf("want about 30 jobs, got %d", len(jobs))
	}
	for _, j := range jobs {
		for _, name := range j.Fixtures {
			if _, err := os.Stat(filepath.Join("fixtures", name)); err != nil {
				t.Errorf("job %s: %v", j.ID, err)
			}
		}
	}
}

func TestParseRejectsBadJobs(t *testing.T) {
	for name, doc := range map[string]string{
		"unknown field": "jobs:\n- {id: a, prompt: p, checkz: []}\n",
		"no checks":     "jobs:\n- {id: a, prompt: p}\n",
		"unknown kind":  "jobs:\n- {id: a, prompt: p, checks: [{kind: telepathy}]}\n",
		"missing field": "jobs:\n- {id: a, prompt: p, checks: [{kind: content, path: x}]}\n",
		"duplicate id":  "jobs:\n- {id: a, prompt: p, checks: [{kind: exists, path: x}]}\n- {id: a, prompt: p, checks: [{kind: exists, path: x}]}\n",
		"bad service":   "jobs:\n- {id: a, prompt: p, service: ftp, checks: [{kind: exists, path: x}]}\n",
	} {
		if _, err := ParseJobs([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func prepared(t *testing.T, doc string) (Job, *Prepared) {
	t.Helper()
	jobs, err := ParseJobs([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(jobs[0], t.TempDir(), "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Stop)
	return jobs[0], p
}

func TestSetupMakesFilesAndChecksPassAndFail(t *testing.T) {
	j, p := prepared(t, `
jobs:
- id: del
  prompt: delete junk
  files: {junk.tmp: "x", keep.txt: "keep"}
  lines: {log.txt: "5:l%02d"}
  checks:
  - {kind: absent, path: junk.tmp}
  - {kind: content, path: keep.txt, equals: keep}
  - {kind: content, path: log.txt, regex: '(?m)l05$'}
`)
	// Nothing deleted yet: the absent check must FAIL, the others pass.
	rs := RunChecks(j, p.Dir, p.Vars, "")
	if AllPassed(rs) || rs[0].OK || !rs[1].OK || !rs[2].OK {
		t.Fatalf("before the delete: %+v", rs)
	}
	os.Remove(filepath.Join(p.Dir, "junk.tmp"))
	if rs = RunChecks(j, p.Dir, p.Vars, ""); !AllPassed(rs) {
		t.Fatalf("after the delete: %+v", rs)
	}
}

func TestAnswerChecks(t *testing.T) {
	j, p := prepared(t, `
jobs:
- id: ans
  prompt: p
  checks:
  - {kind: answer, all: [Alpha, beta], none: [gamma]}
  - {kind: answer, regex: 'tick-[0-9]+', min: 2}
`)
	if rs := RunChecks(j, p.Dir, p.Vars, "ALPHA and Beta; tick-1 tick-2 tick-2"); !AllPassed(rs) {
		t.Fatalf("good answer failed: %+v", rs)
	}
	for name, ans := range map[string]string{
		"missing":        "alpha only tick-1 tick-2",
		"forbidden":      "alpha beta gamma tick-1 tick-2",
		"too few unique": "alpha beta tick-1 tick-1 tick-1",
	} {
		if AllPassed(RunChecks(j, p.Dir, p.Vars, ans)) {
			t.Errorf("%s: passed", name)
		}
	}
}

func TestZipFixtureAndSameTree(t *testing.T) {
	j, p := prepared(t, `
jobs:
- id: z
  prompt: p
  zips: {b.zip: {"in/a.txt": "A"}}
  files: {src/a.txt: "A", dst/a.txt: "A"}
  checks:
  - {kind: zip_has, path: b.zip, names: [a.txt]}
  - {kind: same_tree, path: src, other: dst}
`)
	if rs := RunChecks(j, p.Dir, p.Vars, ""); !AllPassed(rs) {
		t.Fatalf("%+v", rs)
	}
	os.WriteFile(filepath.Join(p.Dir, "dst", "a.txt"), []byte("B"), 0o644)
	if rs := RunChecks(j, p.Dir, p.Vars, ""); rs[1].OK {
		t.Fatalf("differing copy passed: %+v", rs)
	}
}

func TestServicesAndPlaceholders(t *testing.T) {
	_, p := prepared(t, `
jobs:
- id: h
  prompt: p
  service: http
  files: {"@www/x.txt": "served"}
  checks: [{kind: exists, path: x}]
`)
	if !strings.HasPrefix(p.Vars["URL"], "http://127.0.0.1:") || p.Vars["PORT"] == "" {
		t.Fatalf("vars: %v", p.Vars)
	}
	if _, err := os.Stat(filepath.Join(p.Www, "x.txt")); err != nil {
		t.Fatal("served file is not in the www folder: ", err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "x.txt")); err == nil {
		t.Fatal("served file leaked into the job folder")
	}
	if got := p.Vars.Expand("go {{URL}}"); strings.Contains(got, "{{") {
		t.Fatal(got)
	}
}

func TestParseOutputAndCause(t *testing.T) {
	out := []byte(`{"response":"done","receipt":{"tool_calls":2,"calls":[{"tool":"bash","detail":"7z a x.7z docs","outcome":"exit code 1","times":1}]}}`)
	ans, calls, n := parseOutput(out)
	if ans != "done" || n != 2 || len(calls) != 1 || calls[0].Detail != "7z a x.7z docs" {
		t.Fatalf("%q %v %d", ans, calls, n)
	}
	r := JobResult{Calls: calls, ToolCalls: 1, NeedsPresent: map[string]bool{"7z": false}}
	if got := guessCause(r); got != CauseToolMissing {
		t.Errorf("missing tool: %s", got)
	}
	r.NeedsPresent = map[string]bool{"7z": true}
	if got := guessCause(r); got != CauseModelError {
		t.Errorf("present and used, still failed: %s", got)
	}
	r.Calls = []Call{{Tool: "bash", Detail: "dir", Outcome: "ok"}}
	if got := guessCause(r); got != CauseNotKnown {
		t.Errorf("present, never used: %s", got)
	}
	r.Calls = []Call{{Tool: "bash", Detail: "del x", Outcome: "refused, not run"}}
	if got := guessCause(r); got != CauseRefused {
		t.Errorf("refused: %s", got)
	}
	if _, _, n := parseOutput([]byte("not json")); n != 0 {
		t.Error("plain text counted as calls")
	}
}

func TestRenderTable(t *testing.T) {
	jobs := []Job{{ID: "a"}, {ID: "b"}}
	rs := []JobResult{
		{Job: "a", Model: "m", OS: "windows", Pass: true, Seconds: 3},
		{Job: "b", Model: "m", OS: "windows", CauseAuto: CauseToolMissing, Checks: []Result{{Kind: "exists", Why: "x.7z does not exist"}}},
	}
	out := Render(jobs, rs, map[string]Cause{"m/b": {Cause: CauseNotKnown, Note: "never tried"}})
	for _, want := range []string{"PASS", "FAIL", "**1 of 2**", CauseNotKnown, "never tried"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tool missing") {
		t.Error("the override did not replace the automatic cause")
	}
}

func TestZipHasMissingName(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "z.zip"))
	zw := zip.NewWriter(f)
	zw.Create("docs/a.txt")
	zw.Close()
	f.Close()
	if ok, why := runCheck(Check{Kind: "zip_has", Path: "z.zip", Names: []string{"b.txt"}}, dir, Vars{}, ""); ok {
		t.Fatal("a missing name passed: " + why)
	}
	if ok, why := runCheck(Check{Kind: "zip_has", Path: "z.zip", Names: []string{"a.txt"}}, dir, Vars{}, ""); !ok {
		t.Fatal("a name inside a folder failed: " + why)
	}
}
