package startup

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A fake key in the shape of a real one. Built from pieces so that no scanner
// reads this file as holding a credential.
var fakeNIMKey = "nvapi-" + strings.Repeat("Ab3_", 16)

func nimRow() ProviderRow {
	return ProviderRow{
		ID: "nvidia-nim", Name: "NVIDIA NIM", NeedsInput: true, Secret: true,
		InputPrompt: "Paste your NVIDIA NIM key (nvapi-...).",
		Check: func(v string) string {
			if !strings.HasPrefix(v, "nvapi-") || len(v) < 40 {
				return "not an NVIDIA key, NOT saved"
			}
			return ""
		},
	}
}

func openField(t *testing.T) *providerModel {
	t.Helper()
	m := NewProviderModel([]ProviderRow{nimRow()}, true)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.entering {
		t.Fatal("Enter on an unconfigured row did not open the key field")
	}
	return m
}

func withClipboard(t *testing.T, text string, err error) {
	t.Helper()
	old := readClipboard
	readClipboard = func() (string, error) { return text, err }
	t.Cleanup(func() { readClipboard = old })
}

// THE FAULT: Ctrl+V reached this field as a keypress and was dropped, so a key
// could not be pasted on the one screen that exists for pasting keys. The
// owner's NVIDIA key was never saved because it never arrived.
func TestCtrlVPastesTheClipboardIntoTheKeyField(t *testing.T) {
	// Copied from a web page: a line break rides along, as it usually does.
	withClipboard(t, "  "+fakeNIMKey+"\r\n", nil)
	m := openField(t)

	m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if got := string(m.input); got != "  "+fakeNIMKey {
		t.Fatalf("after Ctrl+V the field holds %d characters, want the %d of the key (plus the leading spaces)",
			len(m.input), len(fakeNIMKey))
	}
	// The key must never be drawn; its length must be, so the user can see it landed.
	view := m.View()
	if strings.Contains(view, fakeNIMKey) {
		t.Error("the pasted key is visible on screen")
	}
	if !strings.Contains(view, "chars)") || !strings.Contains(view, "Ctrl+V to paste") {
		t.Errorf("the field does not show its length or say how to paste:\n%s", view)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.done {
		t.Fatalf("Enter on a good key did not finish; hint: %q", m.hint)
	}
	if m.choice.ID != "nvidia-nim" || m.choice.Input != fakeNIMKey {
		t.Errorf("the choice carries %d characters, want the key exactly, trimmed", len(m.choice.Input))
	}
}

// A terminal that performs the paste itself sends the text as keystrokes. That
// route worked before and must go on working, one character at a time or all
// at once, with whatever wrapping the terminal adds taken off.
func TestATerminalPasteStillArrives(t *testing.T) {
	m := openField(t)
	for _, r := range "\x1b[200~" + fakeNIMKey[:10] {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(fakeNIMKey[10:] + "\n\x1b[201~")})
	// One rune at a time the marker cannot be recognised as a whole; what must
	// hold is that no control character is ever stored.
	for _, r := range m.input {
		if r < 0x20 {
			t.Fatalf("a control character (%#x) was stored inside the key", r)
		}
	}
	if !strings.HasSuffix(string(m.input), fakeNIMKey) {
		t.Errorf("the key did not arrive whole: %d characters held", len(m.input))
	}
}

func TestCleanPastedDropsWhatAKeyNeverContains(t *testing.T) {
	for in, want := range map[string]string{
		"\x1b[200~abc\x1b[201~":           "abc",
		"abc\r\n":                         "abc",
		string(rune(0xfeff)) + "abc\tdef": "abcdef",
		"[200~abc[201~":                   "abc",
		"a b":                             "a b", // a space is the caller's to trim, not ours to judge
	} {
		if got := string(cleanPasted(in)); got != want {
			t.Errorf("cleanPasted(%q) = %q, want %q", in, got, want)
		}
	}
}

// What the owner's screenshot shows: one character in the field. Pressing
// Enter on that used to SAVE it. NVIDIA lists models without checking the key,
// so setup then reported success and the first message failed.
func TestAValueThatIsNotAKeyIsRefusedAndNothingIsChosen(t *testing.T) {
	m := openField(t)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.done || m.choice.ID != "" {
		t.Fatal("a one-character value was accepted as an NVIDIA key")
	}
	if !strings.Contains(m.hint, "NOT saved") {
		t.Errorf("the refusal does not say the value was not saved: %q", m.hint)
	}
	if len(m.input) != 0 {
		t.Error("the refused value was left in the field, so the next paste would be glued onto it")
	}
	if !m.entering {
		t.Error("the refusal threw the user out of the field")
	}
	if !strings.Contains(m.View(), "NOT saved") {
		t.Error("the refusal is not on screen")
	}
}

func TestAnEmptyOrUnreadableClipboardSaysSo(t *testing.T) {
	for name, c := range map[string]struct {
		text string
		err  error
	}{"empty": {"", nil}, "blank": {" \r\n", nil}, "unreadable": {"", errors.New("no clipboard")}} {
		withClipboard(t, c.text, c.err)
		m := openField(t)
		m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
		if len(m.input) != 0 || !strings.Contains(m.hint, "clipboard") {
			t.Errorf("%s clipboard: input=%d hint=%q", name, len(m.input), m.hint)
		}
	}
}

// Ctrl+V in the LIST (not in a field) must do nothing: it must not open a
// field, pick a provider or read the clipboard.
func TestCtrlVOutsideAFieldReadsNothing(t *testing.T) {
	read := false
	old := readClipboard
	readClipboard = func() (string, error) { read = true; return "x", nil }
	defer func() { readClipboard = old }()

	m := NewProviderModel([]ProviderRow{nimRow()}, true)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	if read || m.entering || m.done {
		t.Errorf("Ctrl+V on the list: read=%v entering=%v done=%v", read, m.entering, m.done)
	}
}
