package toolbox

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// GORILLA (2026-11-01, W6): the user can SEE what is built in. The pins say which
// bytes may run; helpers.json says, in plain words, what each helper is for and
// which version it is. A test ties the two together (every pinned helper has a
// line here, every line here is pinned, every version is named in
// third_party/PINS.md), so the screens and the release page below are generated
// from the pins and cannot drift from them.

//go:embed helpers.json
var helpersJSON []byte

// Helper is one built-in helper as a person reads about it.
type Helper struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	VersionDarwin string `json:"version_darwin,omitempty"`
	Purpose       string `json:"purpose"`
	// MacMin is the oldest macOS the macOS file runs on when that is newer than
	// the 10.13 the others need.
	MacMin string `json:"mac_min,omitempty"`
}

// MacNeeds is the macOS version the helper needs on macOS: "10.13" unless it says more.
func (h Helper) MacNeeds() string {
	if h.MacMin != "" {
		return h.MacMin
	}
	return "10.13"
}

// VersionOn is the version shipped for the platform ("windows-amd64", ...).
func (h Helper) VersionOn(platform string) string {
	if strings.HasPrefix(platform, "darwin-") && h.VersionDarwin != "" {
		return h.VersionDarwin
	}
	return h.Version
}

// Catalog is every helper described in helpers.json, in file order.
func Catalog() ([]Helper, error) {
	var hs []Helper
	if err := json.Unmarshal(helpersJSON, &hs); err != nil {
		return nil, fmt.Errorf("helpers.json: %w", err)
	}
	return hs, nil
}

// PinnedFor lists the helpers pinned for the platform in pinsText (the text of
// SHA256SUMS), described, in catalog order. A pinned helper with no catalog line
// is an error: it would be carried but invisible.
func PinnedFor(pinsText, platform string) ([]Helper, error) {
	pins, err := ParsePins(pinsText)
	if err != nil {
		return nil, err
	}
	cat, err := Catalog()
	if err != nil {
		return nil, err
	}
	byName := map[string]Helper{}
	for _, h := range cat {
		byName[h.Name] = h
	}
	have := map[string]bool{}
	for key := range pins {
		plat, file, ok := strings.Cut(key, "/")
		if !ok || plat != platform {
			continue
		}
		name := strings.TrimSuffix(file, ".exe")
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("%s is pinned for %s but has no line in helpers.json", name, platform)
		}
		have[name] = true
	}
	var out []Helper
	for _, h := range cat {
		if have[h.Name] {
			out = append(out, h)
		}
	}
	return out, nil
}

// Pinned is PinnedFor over the pins compiled into this binary.
func Pinned(platform string) []Helper {
	hs, _ := PinnedFor(pinsText, platform)
	return hs
}

// Platforms lists the platforms that have at least one pin, sorted.
func Platforms() []string {
	pins, _ := ParsePins(pinsText)
	seen := map[string]bool{}
	for key := range pins {
		if p, _, ok := strings.Cut(key, "/"); ok {
			seen[p] = true
		}
	}
	var out []string
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Described lists the helpers this Box embeds, described, in catalog order. A
// plain `go build` embeds none: the answer is empty.
func (b *Box) Described() []Helper {
	if b == nil {
		return nil
	}
	have := map[string]bool{}
	for _, n := range b.Names() {
		have[n] = true
	}
	var out []Helper
	for _, h := range Pinned(b.platform) {
		if have[h.Name] {
			out = append(out, h)
		}
	}
	return out
}

// BuiltIn is Default().Described(): what this running binary carries.
func BuiltIn() []Helper {
	b, _ := Default()
	return b.Described()
}
