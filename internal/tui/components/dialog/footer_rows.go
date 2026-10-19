package dialog

// GORILLA FIX (2026-10-19): the rows the app draws UNDER a dialog are not a
// constant. Below about 120 columns they are three (a blank, the divider, the
// prompt); wider, two information lines are added ("model ... | in ... |
// context ..." and "tokens ..."). /models and /settings had their height
// budgets tuned against the narrow three, so on the owner's MacBook Air at
// 138x37 both were two rows taller than the window and lost their top border
// and title (measured with the tui-sizes tool (Fieldkit)).
//
// The app measures the real number on every resize and sets it here; a dialog
// whose budget assumed the narrow footer subtracts FooterExtraRows().

// rowsUnderOverlay is what the app last measured: the rows drawn under a
// dialog, not counting the status bar (the dialog's height already excludes it).
var rowsUnderOverlay int

// narrowFooterRows is the footer every older budget constant was tuned against.
const narrowFooterRows = 3

// SetRowsUnderOverlay is called by the app after each resize.
func SetRowsUnderOverlay(n int) { rowsUnderOverlay = n }

// FooterExtraRows is how many more rows than the narrow footer are drawn under
// a dialog right now. Zero when nothing has been measured yet.
func FooterExtraRows() int {
	if rowsUnderOverlay <= narrowFooterRows {
		return 0
	}
	return rowsUnderOverlay - narrowFooterRows
}
