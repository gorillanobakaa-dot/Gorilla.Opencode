// Package featurecheck is the "new model / new feature" checklist as a
// deterministic tool: after a catalogue change or a new tool it lists every
// surface that must know it and fails on a surface that is stale. The rules
// are YAML (rules.yaml), so a model reads the checklist instead of the owner
// explaining it each time.
package featurecheck

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed rules.yaml
var rulesYAML []byte

type surface struct {
	Surface string `yaml:"surface"`
	Glob    string `yaml:"glob"`
}

type rules struct {
	IDPattern    string    `yaml:"id_pattern"`
	ModelRefs    []surface `yaml:"model_refs"`
	ToolSurfaces []surface `yaml:"tool_surfaces"`
}

// Finding is one stale or missing mention.
type Finding struct {
	Surface string
	File    string
	Kind    string // "stale-model", "missing-tool", "saved-config"
	Detail  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%-14s %s: %s (%s)", f.Kind, f.File, f.Detail, f.Surface)
}

// Input is what the checker compares the surfaces against.
type Input struct {
	Root      string
	Catalogue map[string]bool   // every model id and api id the catalogue has
	NewTools  []string          // tools that every tool surface must mention
	Saved     map[string]string // saved config slot -> model id
}

// Check runs the checklist. A surface file that matches no glob is not an
// error (the repo may not ship it); a surface that exists and is stale is.
func Check(in Input) ([]Finding, error) {
	var r rules
	if err := yaml.Unmarshal(rulesYAML, &r); err != nil {
		return nil, err
	}
	re, err := regexp.Compile(r.IDPattern)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, s := range r.ModelRefs {
		files, _ := filepath.Glob(filepath.Join(in.Root, s.Glob))
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			seen := map[string]bool{}
			for _, id := range re.FindAllString(string(b), -1) {
				id = strings.TrimRight(id, ".-")
				if seen[id] || !looksLikeModelID(id) || in.Catalogue[id] || inFamily(in.Catalogue, id) {
					continue
				}
				seen[id] = true
				out = append(out, Finding{s.Surface, rel(in.Root, f), "stale-model", id + " is not in the catalogue"})
			}
		}
	}
	for _, t := range in.NewTools {
		for _, s := range r.ToolSurfaces {
			files, _ := filepath.Glob(filepath.Join(in.Root, s.Glob))
			if len(files) == 0 {
				out = append(out, Finding{s.Surface, s.Glob, "missing-tool", t + ": surface file not found"})
			}
			for _, f := range files {
				b, _ := os.ReadFile(f)
				if !strings.Contains(string(b), t) {
					out = append(out, Finding{s.Surface, rel(in.Root, f), "missing-tool", t + " is not mentioned"})
				}
			}
		}
	}
	slots := make([]string, 0, len(in.Saved))
	for k := range in.Saved {
		slots = append(slots, k)
	}
	sort.Strings(slots)
	for _, k := range slots {
		if id := in.Saved[k]; id != "" && !in.Catalogue[id] {
			out = append(out, Finding{"saved config", k, "saved-config",
				id + " no longer exists; pick another with /models (or clear the slot)"})
		}
	}
	return out, nil
}

// looksLikeModelID keeps only strings with a digit or a dot: prose such as
// "claude-code" or "gemini-cli" is not a model id.
func looksLikeModelID(id string) bool {
	return strings.ContainsAny(id, "0123456789")
}

// inFamily accepts an id that is a prefix of a catalogue id ("claude-opus-5-5"
// for "claude-opus-5-5-high"): docs name the family, the backend serves efforts.
func inFamily(cat map[string]bool, id string) bool {
	for c := range cat {
		if strings.HasPrefix(c, id+"-") || strings.HasPrefix(c, "antigravity."+id+"-") {
			return true
		}
	}
	return false
}

func rel(root, f string) string {
	if r, err := filepath.Rel(root, f); err == nil {
		return r
	}
	return f
}
