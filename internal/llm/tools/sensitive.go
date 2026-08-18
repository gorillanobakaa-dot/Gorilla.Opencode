// GORILLA OVERRIDE (2026-08-18): refuse to read credential files.
//
// # WHAT THIS IS, AND WHAT IT IS DELIBERATELY NOT
//
// This is a BLOCKLIST, not a boundary. roots.go states the design plainly:
// "There is no sandbox in this codebase — tools accept absolute paths anywhere."
// That is a settled decision and this file does not overturn it. A determined
// path can still be constructed; what this stops is the ordinary, high-value
// case, cheaply, without costing the tool anything it needs.
//
// # THE PROBLEM IT ADDRESSES
//
// view and find have no permission service wired in at all and will read any
// absolute path. find additionally PRINTS MATCHING LINES, so a search across a
// home directory puts the matched line — the key itself — into the transcript.
//
// And in this program, "read" means considerably more than read. Anything read
// goes into the model's context, therefore over the wire to whichever provider
// is configured, and is persisted in the session database. A single
// `view ~/.ssh/id_rsa` discloses a private key to a third party and writes it to
// disk in cleartext. Per directive §7 the question is what a value can do ALONE:
// a private key, a cloud credential and a provider API key each act alone.
//
// # WHY NOT JUST GATE EVERY READ
//
// Because reading files is what a coding agent DOES. Prompting on every read
// would make the tool unusable, and a prompt nobody reads is not a control. The
// asymmetry is the whole design: nothing legitimate needs the agent to read an
// SSH private key, so refusing that costs nothing, while refusing reads
// generally would cost everything.
package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
)

// sensitiveBasenames are refused wherever they are found — these names do not
// have innocent versions.
var sensitiveBasenames = []string{
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
	"credentials", "authorized_keys",
	".netrc", ".pgpass", ".htpasswd",
	"secring.gpg", "trustdb.gpg",
}

// sensitiveDirSegments mark a directory whose contents are credentials. Matched
// as a whole path component, never as a substring, so "my.ssh-notes" is not
// caught by ".ssh".
var sensitiveDirSegments = []string{
	".ssh", ".gnupg", ".aws", ".azure", ".kube",
	".docker", ".password-store",
}

// sensitiveSuffixes catch key material by extension.
var sensitiveSuffixes = []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore"}

// RefuseSensitiveRead returns a non-empty reason if this path should not be read
// into the model's context.
//
// Paths INSIDE a configured root are exempt: a project may legitimately contain
// a test fixture named key.pem, and the user chose to work there. The risk being
// addressed is reaching OUT of the workspace for credentials.
func RefuseSensitiveRead(path string) string {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(config.WorkingDirectory(), abs)
	}
	abs = filepath.Clean(abs)

	// Inside the workspace the user has already chosen this ground.
	if _, ok := config.RootFor(abs); ok {
		return ""
	}

	base := filepath.Base(abs)
	lower := strings.ToLower(base)

	for _, n := range sensitiveBasenames {
		if strings.EqualFold(base, n) {
			return reason(abs, "it is a credential file")
		}
	}
	for _, suf := range sensitiveSuffixes {
		if strings.HasSuffix(lower, suf) {
			return reason(abs, "it looks like key material ("+suf+")")
		}
	}

	// The application's own configuration holds provider API keys.
	if strings.Contains(abs, filepath.Join(".config", "gorilla-opencode")) ||
		strings.Contains(abs, filepath.Join(".config", "opencode")) {
		return reason(abs, "it holds this program's own provider API keys")
	}

	for _, seg := range strings.Split(abs, string(filepath.Separator)) {
		for _, d := range sensitiveDirSegments {
			if strings.EqualFold(seg, d) {
				return reason(abs, "it is inside "+d+", which holds credentials")
			}
		}
	}
	return ""
}

func reason(path, why string) string {
	return fmt.Sprintf(
		"Refusing to read %s: %s, and it is outside this project. Anything read here "+
			"goes into the model's context, over the network to the configured provider, "+
			"and into the session database. If you genuinely need it, copy the part you "+
			"need into the project first.",
		path, why)
}
