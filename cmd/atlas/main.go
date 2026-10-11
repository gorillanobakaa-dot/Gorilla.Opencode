// Command atlas renders the research helpers' source atlas
// (internal/llm/agent/source-atlas.txt) from the source registry
// (docs/source-registry.json) and the atlas curation
// (internal/llm/agent/source-atlas.yaml).
//
//	go run ./cmd/atlas > internal/llm/agent/source-atlas.txt
//
// GORILLA OVERRIDE (2026-10-11): the atlas was typed by hand beside a
// 985-entry registry that no code read, and the two had drifted: two grades
// and one address disagreed. The curation decides which sources a helper is
// shown, in what order, with what advice; the registry alone decides each
// source's grade, how it is reached, and where.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type source struct {
	Name        string `json:"name"`
	Access      string `json:"access"`
	Category    string `json:"category"`
	Free        *bool  `json:"free"`
	Reliability string `json:"reliability"`
	URL         string `json:"url"`
	Note        string `json:"note"`
}

type registry struct {
	Sources []source `json:"sources"`
}

type entry struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"` // the registry's name, when the atlas shows another
	Advice string `yaml:"advice"`
}

type section struct {
	Title   string  `yaml:"title"`
	Entries []entry `yaml:"entries"`
}

type curation struct {
	Header   string    `yaml:"header"`
	Sections []section `yaml:"sections"`
}

// reach is how the atlas names each access class.
var reach = map[string]string{
	"api_keyless": "API", "api_key": "API-key", "searxng_enabled": "SearXNG", "searxng_available": "SearXNG*",
	"fetch_pattern": "web", "bulk_download": "bulk", "paid_only": "paid", "unknown": "?",
}

var reliabilities = map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true, "F": true}

// checkRegistry lists what is wrong with the registry as data. notes are
// worth knowing and break nothing.
func checkRegistry(r registry) (problems, notes []string) {
	seen := map[string]bool{}
	for i, s := range r.Sources {
		where := fmt.Sprintf("source %d (%q)", i+1, s.Name)
		if strings.TrimSpace(s.Name) == "" {
			problems = append(problems, where+": no name")
		}
		key := strings.ToLower(strings.TrimSpace(s.Name))
		if seen[key] {
			problems = append(problems, where+": the name is used twice")
		}
		seen[key] = true
		if _, ok := reach[s.Access]; !ok {
			problems = append(problems, fmt.Sprintf("%s: access %q is not one of the known classes", where, s.Access))
		}
		if !reliabilities[s.Reliability] {
			problems = append(problems, fmt.Sprintf("%s: reliability %q is not A-F", where, s.Reliability))
		}
		if s.Free == nil {
			problems = append(problems, where+": free is not set")
		}
		if strings.TrimSpace(s.Category) == "" {
			problems = append(problems, where+": no category")
		}
		switch {
		case s.URL == "":
			notes = append(notes, where+": no URL")
		case !strings.HasPrefix(s.URL, "http://") && !strings.HasPrefix(s.URL, "https://"):
			problems = append(problems, fmt.Sprintf("%s: URL %q is not http(s)", where, s.URL))
		}
	}
	return problems, notes
}

// render builds the atlas text. Every entry must name a registry source that
// has an address, or rendering fails: the atlas never shows a source the
// registry does not hold.
func render(r registry, c curation) (string, error) {
	byName := map[string]source{}
	for _, s := range r.Sources {
		byName[strings.ToLower(s.Name)] = s
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(c.Header, "\n") + "\n")
	for _, sec := range c.Sections {
		fmt.Fprintf(&b, "\n%s:\n", sec.Title)
		for _, e := range sec.Entries {
			key := e.Source
			if key == "" {
				key = e.Name
			}
			s, ok := byName[strings.ToLower(key)]
			if !ok {
				return "", fmt.Errorf("atlas entry %q: no source %q in the registry", e.Name, key)
			}
			if s.URL == "" {
				return "", fmt.Errorf("atlas entry %q: the registry has no URL for %q", e.Name, s.Name)
			}
			addr := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(s.URL, "https://"), "http://"), "/")
			fmt.Fprintf(&b, "  %s [%s %s] %s", e.Name, s.Reliability, reach[s.Access], addr)
			if adv := strings.TrimSpace(e.Advice); adv != "" {
				lines := strings.Split(adv, "\n")
				fmt.Fprintf(&b, " — %s", lines[0])
				for _, l := range lines[1:] {
					fmt.Fprintf(&b, "\n    %s", l)
				}
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// repoRoot is the repository this file lives in, so the command works from
// any directory inside it.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func load(root string) (registry, curation, error) {
	var r registry
	var c curation
	raw, err := os.ReadFile(filepath.Join(root, "docs", "source-registry.json"))
	if err != nil {
		return r, c, err
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, c, fmt.Errorf("docs/source-registry.json: %w", err)
	}
	raw, err = os.ReadFile(filepath.Join(root, "internal", "llm", "agent", "source-atlas.yaml"))
	if err != nil {
		return r, c, err
	}
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return r, c, fmt.Errorf("source-atlas.yaml: %w", err)
	}
	return r, c, nil
}

func main() {
	r, c, err := load(repoRoot())
	if err != nil {
		fmt.Fprintln(os.Stderr, "atlas:", err)
		os.Exit(1)
	}
	if problems, _ := checkRegistry(r); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "registry:", p)
		}
		os.Exit(3)
	}
	out, err := render(r, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "atlas:", err)
		os.Exit(3)
	}
	fmt.Print(out)
}
