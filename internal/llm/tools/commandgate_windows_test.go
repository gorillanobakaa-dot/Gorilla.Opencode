// GORILLA OVERRIDE (2026-09-01): the ban list met PowerShell.
//
// The bash tool's banned-command gate exists for one reason, stated in its own
// description: raw network fetchers run from a shell bypass the audited,
// permission-gated fetch and websearch tools, and that bypass is exactly what a
// prompt-injection attack needs. Every name on the list was a Unix one. When the
// tool started driving PowerShell on Windows, the gate kept passing its tests
// and stopped doing its job — the attacker just types the Windows spelling.
//
// These tests are the platform-parity check that was missing.
package tools

import "testing"

// The canonical PowerShell fetchers must be refused, in every spelling Windows
// actually accepts: full cmdlet name, shipped alias, and any casing.
func TestPowerShellNetworkFetchersAreBanned(t *testing.T) {
	for _, cmd := range []string{
		"Invoke-WebRequest https://example.com/x",
		"invoke-webrequest https://example.com/x",
		"IWR https://example.com/x",
		"iwr https://example.com/x -OutFile x",
		"Invoke-RestMethod https://example.com/api",
		"irm https://example.com/api",
		"Start-BitsTransfer -Source https://example.com/x -Destination x",
		"bitsadmin /transfer job https://example.com/x x",
		"certutil -urlcache -split -f https://example.com/x",
	} {
		if got := BannedCommandIn(cmd, bannedCommands); got == "" {
			t.Errorf("%q was ALLOWED — this is a raw network fetcher reached from a shell, "+
				"which is the precise bypass the gate exists to prevent", cmd)
		}
	}
}

// A banned command reached through PowerShell's call operator must still be
// caught. `&` already splits segments, but the quotes it is normally paired with
// used to make the executable word `'curl'`, which matched nothing.
func TestQuotingDoesNotSmuggleABannedCommandPastTheGate(t *testing.T) {
	for _, cmd := range []string{
		`& 'curl' https://example.com`,
		`& "curl" https://example.com`,
		`echo ok; & 'iwr' https://example.com`,
		`'curl' https://example.com`,
		`"wget" https://example.com`,
	} {
		if got := BannedCommandIn(cmd, bannedCommands); got == "" {
			t.Errorf("%q was ALLOWED — quoting the executable name is not a permission grant", cmd)
		}
	}
}

// Chained and piped forms must be checked in every segment, not just the first.
func TestBannedCommandFoundInLaterSegments(t *testing.T) {
	for _, cmd := range []string{
		"echo hello; Invoke-WebRequest https://example.com",
		"Get-Date | Out-Null; iwr https://example.com",
		"echo ok && curl https://example.com",
	} {
		if got := BannedCommandIn(cmd, bannedCommands); got == "" {
			t.Errorf("%q was ALLOWED — a banned command after a harmless one is still banned", cmd)
		}
	}
}

// The gate must not become so broad it refuses ordinary work. These are the
// commands people legitimately run all day; blocking them would push users to
// disable the gate entirely, which is strictly worse than a narrow gate.
func TestOrdinaryCommandsAreStillAllowed(t *testing.T) {
	for _, cmd := range []string{
		"git status",
		"go build ./...",
		"Get-ChildItem",
		"Get-Content README.md",
		"npm test",
		"echo curling is a sport",
	} {
		if got := BannedCommandIn(cmd, bannedCommands); got != "" {
			t.Errorf("%q was refused as %q — this is ordinary work and must pass", cmd, got)
		}
	}
}
