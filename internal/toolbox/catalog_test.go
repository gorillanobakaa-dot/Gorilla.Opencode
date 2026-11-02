package toolbox

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// Every pinned helper on every platform is described, and every description is pinned somewhere.
func TestCatalogMatchesPins(t *testing.T) {
	cat, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	pinned := map[string]bool{}
	for _, p := range Platforms() {
		hs, err := PinnedFor(pinsText, p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if len(hs) == 0 {
			t.Fatalf("%s: no helpers", p)
		}
		for _, h := range hs {
			pinned[h.Name] = true
		}
	}
	for _, h := range cat {
		if !pinned[h.Name] {
			t.Errorf("%s is described but pinned nowhere", h.Name)
		}
		if h.Purpose == "" || h.Version == "" {
			t.Errorf("%s: purpose and version must both be set", h.Name)
		}
	}
}

func TestPinnedWithoutDescriptionFails(t *testing.T) {
	sum := strings.Repeat("a", 64)
	if _, err := PinnedFor(sum+"  windows-amd64/mystery.exe\n", "windows-amd64"); err == nil {
		t.Fatal("a pinned helper nobody described must be an error")
	}
}

// Each version shown to the user is written in third_party/PINS.md.
func TestVersionsAreInPINS(t *testing.T) {
	raw, err := os.ReadFile("../../third_party/PINS.md")
	if err != nil {
		t.Skip("PINS.md not beside the package:", err)
	}
	pins := string(raw)
	cat, _ := Catalog()
	for _, h := range cat {
		for _, v := range []string{h.Version, h.VersionDarwin} {
			if v != "" && !strings.Contains(pins, v) {
				t.Errorf("%s version %s is not named in third_party/PINS.md", h.Name, v)
			}
		}
	}
}

func TestDescribedOnlyWhatIsEmbedded(t *testing.T) {
	sum := strings.Repeat("b", 64)
	pins, _ := ParsePins(sum + "  windows-amd64/sd.exe\n" + sum + "  windows-amd64/ninja.exe\n")
	fsys := fstest.MapFS{"windows-amd64/sd.exe": {Data: []byte("x")}}
	if got := New(nil, "windows-amd64", pins, nil).Described(); len(got) != 0 {
		t.Fatalf("nothing embedded must describe nothing, got %v", got)
	}
	if got := New(fsys, "windows-amd64", pins, nil).Described(); len(got) != 1 || got[0].Name != "sd" {
		t.Fatalf("want only sd, got %v", got)
	}
}

func TestMacNeeds(t *testing.T) {
	if (Helper{}).MacNeeds() != "10.13" || (Helper{MacMin: "11.0"}).MacNeeds() != "11.0" {
		t.Fatal("MacNeeds wrong")
	}
	h := Helper{Version: "7.0.2", VersionDarwin: "8.0"}
	if h.VersionOn("darwin-amd64") != "8.0" || h.VersionOn("linux-amd64") != "7.0.2" {
		t.Fatal("VersionOn wrong")
	}
}
