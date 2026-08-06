package startup

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// twoFieldRow is a row that asks for two values, as Cloudflare does.
func twoFieldRow() ProviderRow {
	return ProviderRow{
		ID: "cloudflare", NeedsInput: true,
		InputPrompt: "Account ID", InputPrompt2: "API token", Secret2: true,
	}
}

// paste mimics an UNBRACKETED terminal paste: the text arrives as runes, but
// every newline in it arrives as its own Enter key.
//
// THIS IS THE BUG. With a single field, a pasted value containing a newline
// submitted the field the moment the newline landed, silently discarding
// everything after it — reported 2026-08-05 as "could not find a Cloudflare
// account ID or API token in that" for a paste that visibly contained both.
func paste(m *providerModel, s string) {
	for _, part := range strings.SplitAfter(s, "\n") {
		if txt := strings.TrimSuffix(part, "\n"); txt != "" {
			m.updateEntering(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(txt)})
		}
		if strings.HasSuffix(part, "\n") {
			m.updateEntering(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
}

// Two fields, filled one at a time, is the whole point: a newline advances
// rather than destroying what follows.
func TestTwoFieldEntryCollectsBothValues(t *testing.T) {
	m := &providerModel{rows: []ProviderRow{twoFieldRow()}, entering: true}

	paste(m, "ACCOUNT-VALUE\n")
	if m.stage != 1 {
		t.Fatalf("after the first value the form is on stage %d, want 1", m.stage)
	}
	if m.done {
		t.Fatal("the form finished after only the first value")
	}
	paste(m, "TOKEN-VALUE\n")

	if !m.done {
		t.Fatal("the form did not finish after both values")
	}
	if m.choice.Input != "ACCOUNT-VALUE" {
		t.Errorf("first value = %q", m.choice.Input)
	}
	if m.choice.Input2 != "TOKEN-VALUE" {
		t.Errorf("second value = %q", m.choice.Input2)
	}
}

// Typing into the second field must not append to the first.
func TestTheSecondFieldIsSeparate(t *testing.T) {
	m := &providerModel{rows: []ProviderRow{twoFieldRow()}, entering: true}
	paste(m, "AAA\n")
	m.updateEntering(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("BBB")})

	if got := string(m.input); got != "AAA" {
		t.Errorf("first field became %q; the second field is writing into it", got)
	}
	if got := string(m.input2); got != "BBB" {
		t.Errorf("second field = %q, want BBB", got)
	}
}

// Backspace must edit the field in focus, not the finished one.
func TestBackspaceEditsTheActiveField(t *testing.T) {
	m := &providerModel{rows: []ProviderRow{twoFieldRow()}, entering: true}
	paste(m, "AAA\n")
	m.updateEntering(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("BBB")})
	m.updateEntering(tea.KeyMsg{Type: tea.KeyBackspace})

	if string(m.input) != "AAA" {
		t.Errorf("backspace reached back into the first field: %q", string(m.input))
	}
	if got := string(m.input2); got != "BB" {
		t.Errorf("second field = %q, want BB", got)
	}
}

// Esc on the second field goes back a step rather than abandoning everything —
// losing a freshly pasted token to one keystroke is its own small disaster.
func TestEscGoesBackOneStepNotAllTheWayOut(t *testing.T) {
	m := &providerModel{rows: []ProviderRow{twoFieldRow()}, entering: true}
	paste(m, "AAA\n")
	m.updateEntering(tea.KeyMsg{Type: tea.KeyEsc})

	if m.stage != 0 {
		t.Errorf("stage = %d after Esc, want 0", m.stage)
	}
	if !m.entering {
		t.Error("Esc on the second field abandoned entry entirely")
	}
	if string(m.input) != "AAA" {
		t.Errorf("the first value was discarded: %q", string(m.input))
	}

	m.updateEntering(tea.KeyMsg{Type: tea.KeyEsc})
	if m.entering {
		t.Error("a second Esc should leave entry and return to the list")
	}
}

// A single-field row must behave exactly as before — one value, submitted on
// the first Enter.
func TestSingleFieldRowsAreUnchanged(t *testing.T) {
	m := &providerModel{
		rows:     []ProviderRow{{ID: "nvidia-nim", NeedsInput: true, InputPrompt: "key", Secret: true}},
		entering: true,
	}
	paste(m, "THE-KEY\n")

	if !m.done {
		t.Fatal("a single-field row did not submit on the first Enter")
	}
	if m.choice.Input != "THE-KEY" {
		t.Errorf("value = %q", m.choice.Input)
	}
	if m.choice.Input2 != "" {
		t.Errorf("a single-field row produced a second value: %q", m.choice.Input2)
	}
}

// The secret must never be echoed, and the non-secret field must be readable —
// an account id is not a credential, and hiding it makes a typo unspottable.
func TestOnlyTheSecretFieldIsMasked(t *testing.T) {
	m := &providerModel{rows: []ProviderRow{twoFieldRow()}, entering: true}
	if got := m.echo([]rune("ACCOUNT-VALUE"), false, 60); !strings.Contains(got, "ACCOUNT-VALUE") {
		t.Errorf("the non-secret field is hidden: %q", got)
	}
	if got := m.echo([]rune("SECRET-TOKEN"), true, 60); strings.Contains(got, "SECRET-TOKEN") {
		t.Errorf("a secret was echoed to the screen: %q", got)
	}
}
