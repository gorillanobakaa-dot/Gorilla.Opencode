// Package basic is the acceptance suite of everyday jobs for Gorilla OpenCode
// (plan row P5a, owner 2026-10-31).
//
// WHY. "The gorilla opencode on windows seems to lack a lot of basic
// capabilities": a Qwen model could not see or delete a file, a model searched
// the web for how to run `tail -f`. Before adding tools, measure what fails and
// why. jobs.yaml lists ~35 jobs a person would type; each has a setup (files in
// a fresh scratch folder), the prompt, and a deterministic CHECK on what ended
// up on disk or in the answer. A model never judges a model.
//
// This file is the data model and the loader. setup.go makes the scratch
// folder, check.go runs the checks, report.go renders the table. The model runs
// themselves are in acceptance_test.go behind the build tag `acceptance`:
// they are a measurement, not CI.
package basic

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Job is one everyday task.
type Job struct {
	ID     string `yaml:"id"`
	Title  string `yaml:"title"`
	Prompt string `yaml:"prompt"`
	// Files are text files made before the run (path relative to the scratch
	// folder, content). Dirs are empty folders. Fixtures copy a committed
	// binary from fixtures/ (destination -> fixture file name). Hidden marks
	// files hidden (Windows attribute; on Linux the name starts with a dot).
	Files    map[string]string `yaml:"files"`
	Dirs     []string          `yaml:"dirs"`
	Fixtures map[string]string `yaml:"fixtures"`
	Hidden   []string          `yaml:"hidden"`
	// Lines makes a text file of N lines from a format with %03d (path ->
	// "N:format"), for the tail and count jobs.
	Lines map[string]string `yaml:"lines"`
	// Zips makes zip archives (path -> inside name -> content).
	Zips map[string]map[string]string `yaml:"zips"`
	// Service is a helper the runner keeps running during the job: "http"
	// (serves the scratch folder's www/ on {{URL}}), "port" (the runner listens
	// on {{PORT}}, its pid is {{PID}}) or "growing_log" (appends a line to
	// app.log about every 700 ms).
	Service string `yaml:"service"`
	// Needs names the programs a person would use for this job, so a failure
	// can be told apart: missing from the machine, or present and not used.
	// "toolbox:NAME" is a program Gorilla OpenCode embeds.
	Needs []string `yaml:"needs"`
	// Timeout overrides the default per-job limit.
	Timeout string  `yaml:"timeout"`
	Checks  []Check `yaml:"checks"`
}

// Check is one deterministic test of the result. All checks of a job must pass.
type Check struct {
	// Kind: absent, exists, content, answer, magic, zip_has, dims, same_tree,
	// recycled, size_min.
	Kind string `yaml:"kind"`
	Path string `yaml:"path"`

	// content / answer
	Equals   string   `yaml:"equals"`
	Contains string   `yaml:"contains"`
	Regex    string   `yaml:"regex"`
	All      []string `yaml:"all"`  // answer: every string present (case-insensitive)
	Any      []string `yaml:"any"`  // answer: at least one present
	None     []string `yaml:"none"` // answer: none present
	Min      int      `yaml:"min"`  // answer regex: at least this many DISTINCT matches (default 1); size_min: bytes

	// magic: bytes at Offset, as hex
	Hex    string `yaml:"hex"`
	Offset int    `yaml:"offset"`

	// zip_has: names inside the archive
	Names []string `yaml:"names"`

	// dims: image width and height
	W int `yaml:"w"`
	H int `yaml:"h"`

	// same_tree: Path and Other hold the same files with the same bytes
	Other string `yaml:"other"`
}

// File is the YAML document.
type File struct {
	Jobs []Job `yaml:"jobs"`
}

// LoadJobs reads and validates a jobs file. Unknown fields are an error, so a
// misspelt check never silently passes.
func LoadJobs(path string) ([]Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJobs(data)
}

// ParseJobs is LoadJobs for bytes.
func ParseJobs(data []byte) ([]Job, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("jobs file: %w", err)
	}
	seen := map[string]bool{}
	for i := range f.Jobs {
		j := &f.Jobs[i]
		if j.ID == "" || j.Prompt == "" {
			return nil, fmt.Errorf("job %d: id and prompt are required", i+1)
		}
		if seen[j.ID] {
			return nil, fmt.Errorf("job %s: duplicate id", j.ID)
		}
		seen[j.ID] = true
		if len(j.Checks) == 0 {
			return nil, fmt.Errorf("job %s: no checks (a job without a check proves nothing)", j.ID)
		}
		switch j.Service {
		case "", "http", "port", "growing_log":
		default:
			return nil, fmt.Errorf("job %s: unknown service %q", j.ID, j.Service)
		}
		if j.Timeout != "" {
			if _, err := time.ParseDuration(j.Timeout); err != nil {
				return nil, fmt.Errorf("job %s: timeout: %w", j.ID, err)
			}
		}
		for k, c := range j.Checks {
			if err := c.validate(); err != nil {
				return nil, fmt.Errorf("job %s check %d: %w", j.ID, k+1, err)
			}
		}
	}
	return f.Jobs, nil
}

func (c Check) validate() error {
	need := func(ok bool, what string) error {
		if !ok {
			return fmt.Errorf("%s needs %s", c.Kind, what)
		}
		return nil
	}
	switch c.Kind {
	case "absent", "exists", "recycled":
		return need(c.Path != "", "path")
	case "content":
		if err := need(c.Path != "", "path"); err != nil {
			return err
		}
		return need(c.Equals != "" || c.Contains != "" || c.Regex != "", "equals, contains or regex")
	case "answer":
		return need(c.Contains != "" || c.Regex != "" || len(c.All) > 0 || len(c.Any) > 0 || len(c.None) > 0,
			"contains, regex, all, any or none")
	case "magic":
		return need(c.Path != "" && c.Hex != "", "path and hex")
	case "zip_has":
		return need(c.Path != "" && len(c.Names) > 0, "path and names")
	case "dims":
		return need(c.Path != "" && c.W > 0 && c.H > 0, "path, w and h")
	case "same_tree":
		return need(c.Path != "" && c.Other != "", "path and other")
	case "size_min":
		return need(c.Path != "" && c.Min > 0, "path and min")
	}
	return fmt.Errorf("unknown check kind %q", c.Kind)
}

// Vars are the placeholders a job may use in its prompt, files and checks.
type Vars map[string]string

// Expand replaces {{NAME}} in s.
func (v Vars) Expand(s string) string {
	for k, val := range v {
		s = strings.ReplaceAll(s, "{{"+k+"}}", val)
	}
	return s
}
