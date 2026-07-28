package tui

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

// The overlay set is discovered from the struct rather than listed by hand,
// because a forgotten nineteenth dialog would draw into a short inline footer,
// overflow it and corrupt the terminal — with no visible connection to the list
// someone did not update. This test pins the discovery down.
func TestEveryDialogFlagIsDiscovered(t *testing.T) {
	found := overlayFlagNames()
	if len(found) == 0 {
		t.Fatal("no dialog flags discovered at all; every dialog would try to draw " +
			"inside the inline footer")
	}

	// Cross-check against the struct independently of the production helper, so a
	// bug in the discovery cannot hide itself.
	tp := reflect.TypeFor[appModel]()
	var want []string
	for i := range tp.NumField() {
		f := tp.Field(i)
		if f.Type.Kind() == reflect.Bool && strings.HasPrefix(f.Name, "show") {
			want = append(want, f.Name)
		}
	}
	if len(found) != len(want) {
		t.Errorf("discovered %d flags, the struct has %d bool show* fields\n  found: %v\n   want: %v",
			len(found), len(want), found, want)
	}

	// A canary: these three are known dialogs from different eras of the file. If
	// discovery silently stopped working, this names it.
	for _, must := range []string{"showQuit", "showLoadoutDialog", "showSettingsDialog"} {
		if !contains(found, must) {
			t.Errorf("%s was not discovered; discovery found %v", must, found)
		}
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// Each discovered flag, on its own, must be enough to report an overlay. A flag
// that is discovered but not consulted is the same bug as one never discovered.
func TestEachFlagAloneOpensAnOverlay(t *testing.T) {
	if (appModel{}).anyOverlayOpen() {
		t.Fatal("a zero appModel reports an overlay, so the per-flag checks below " +
			"cannot distinguish anything")
	}

	tp := reflect.TypeFor[appModel]()
	for _, i := range overlayFields() {
		a := appModel{}
		v := reflect.ValueOf(&a).Elem()
		// The flags are unexported, and reflect refuses to assign through an
		// unexported field even inside its own package. Re-deriving an addressable
		// Value at the same address is the standard way round it. Worth the ugliness:
		// the alternative is naming all eighteen flags here, which is the very list
		// this test exists to make unnecessary.
		f := v.Field(i)
		reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().SetBool(true)
		if !a.anyOverlayOpen() {
			t.Errorf("%s set alone does not report an overlay; that dialog would open "+
				"without entering the alternate screen and would overflow the footer",
				tp.Field(i).Name)
		}
	}
}

// The two overlays that are not booleans paint over the full view in View() just
// as a dialog does, so they need the same treatment.
func TestNonBooleanOverlaysAlsoCount(t *testing.T) {
	withURL := appModel{loginURL: "https://example.invalid/device"}
	if !withURL.anyOverlayOpen() {
		t.Error("a pending sign-in URL does not count as an overlay, but it is drawn " +
			"over the whole view")
	}
	compacting := appModel{isCompacting: true}
	if !compacting.anyOverlayOpen() {
		t.Error("the summarising notice does not count as an overlay, but it is drawn " +
			"over the whole view")
	}
}
