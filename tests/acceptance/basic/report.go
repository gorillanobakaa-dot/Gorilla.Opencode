package basic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Cause overrides the automatic guess for one failed job after a person (or
// the tool results) settled it. Key in causes.yaml: "MODEL/JOB".
type Cause struct {
	Cause string `yaml:"cause"`
	Note  string `yaml:"note"`
}

// LoadResults reads every results/*.json (one file per model run).
func LoadResults(dir string) ([]JobResult, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var all []JobResult
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var rs []JobResult
		if err := json.Unmarshal(data, &rs); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		all = append(all, rs...)
	}
	return all, nil
}

// LoadCauses reads causes.yaml; a missing file is no overrides.
func LoadCauses(path string) (map[string]Cause, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]Cause{}
	return m, yaml.Unmarshal(data, &m)
}

// Render makes the report tables: job x model -> PASS/FAIL, then every
// failure with its cause. jobs gives the order.
func Render(jobs []Job, results []JobResult, causes map[string]Cause) string {
	var models []string
	byKey := map[string]JobResult{}
	for _, r := range results {
		k := r.Model + variantSuffix(r.Variant) + "|" + r.OS
		found := false
		for _, m := range models {
			if m == k {
				found = true
			}
		}
		if !found {
			models = append(models, k)
		}
		byKey[k+"|"+r.Job] = r
	}
	label := func(m string) string { return strings.Replace(m, "|", " / ", 1) }
	var b strings.Builder
	b.WriteString("| job |")
	for _, m := range models {
		b.WriteString(" " + label(m) + " |")
	}
	b.WriteString("\n|---|")
	for range models {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	pass := map[string]int{}
	for _, j := range jobs {
		fmt.Fprintf(&b, "| %s |", j.ID)
		for _, m := range models {
			r, ok := byKey[m+"|"+j.ID]
			switch {
			case !ok:
				b.WriteString(" not run |")
			case r.Pass:
				pass[m]++
				fmt.Fprintf(&b, " PASS (%.0f s, %d calls) |", r.Seconds, r.ToolCalls)
			default:
				fmt.Fprintf(&b, " FAIL (%.0f s, %d calls) |", r.Seconds, r.ToolCalls)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("| **passed** |")
	for _, m := range models {
		fmt.Fprintf(&b, " **%d of %d** |", pass[m], len(jobs))
	}
	b.WriteString("\n\n| model / OS | job | cause | why it failed |\n|---|---|---|---|\n")
	count := map[string]int{}
	for _, m := range models {
		for _, j := range jobs {
			r, ok := byKey[m+"|"+j.ID]
			if !ok || r.Pass {
				continue
			}
			cause, note := r.CauseAuto+" (auto)", failedWhy(r)
			if c, ok := causes[strings.SplitN(m, "|", 2)[0]+"/"+j.ID]; ok {
				cause, note = c.Cause, c.Note
			}
			count[strings.TrimSuffix(cause, " (auto)")]++
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", label(m), j.ID, cause, strings.ReplaceAll(note, "|", "/"))
		}
	}
	var sorted []string
	for c := range count {
		sorted = append(sorted, c)
	}
	sort.Slice(sorted, func(i, k int) bool { return count[sorted[i]] > count[sorted[k]] })
	b.WriteString("\n| cause | failures |\n|---|---|\n")
	for _, c := range sorted {
		fmt.Fprintf(&b, "| %s | %d |\n", c, count[c])
	}
	return b.String()
}

func variantSuffix(v string) string {
	if v == "" {
		return ""
	}
	return " [" + v + "]"
}

// failedWhy is the first failed check's reason, or the exit reason.
func failedWhy(r JobResult) string {
	for _, c := range r.Checks {
		if !c.OK {
			return c.Why
		}
	}
	return fmt.Sprintf("exit code %d", r.ExitCode)
}
