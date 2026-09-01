// GORILLA OVERRIDE (2026-09-01): a leaf package holding one list, to break an
// import cycle.
//
// Plain mode has its own small command dispatcher, separate from the TUI command
// registry in internal/commands. TestEverySlashCommandNamedInProseActuallyExists
// checks every "/word" in the source against that registry — so /exit, /extras,
// /show and /hide, which plain mode's own help text promises and its own switch
// handles, were reported as commands that do not exist.
//
// The obvious fix was for that test to import internal/plain. It cannot:
//
//	plain -> tui/components/dialog -> commands
//
// and the test lives in commands. So the list lives here instead, in a package
// that imports nothing and can therefore be imported by both.
//
// The list is not trusted on its own: TestCommandNamesMatchesTheSwitch in
// internal/plain reads the case labels straight out of the dispatcher and fails
// if the two drift in either direction. A hand-maintained allowlist that nothing
// verifies is how a picker row got added without being added to its test earlier
// the same day.
package plaincmd

// Names is every slash command plain mode handles, including aliases.
var Names = []string{
	"exit", "quit", "q",
	"help", "?", "commands",
	"export",
	"clear", "new",
	"extras", "context",
	"show", "hide",
	"model",
}
