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
	"runtime"
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
	// GORILLA OVERRIDE (2026-10-04): four names this list lacked, found by
	// comparing it with alibaba/open-code-review's default_secret_patterns.json.
	// _netrc is what Windows calls .netrc; .npmrc and .pypirc hold registry
	// upload tokens; .dockercfg is the old location of registry passwords.
	"_netrc", ".npmrc", ".pypirc", ".dockercfg",
}

// isDotEnv reports whether a basename is an environment file holding real
// values. The three template spellings carry placeholders by convention and are
// what a person is told to copy FROM, so they stay readable. Same rule as
// upstream's isSecretEnvPath.
func isDotEnv(lowerBase string) bool {
	switch lowerBase {
	case ".env.example", ".env.sample", ".env.template":
		return false
	}
	return lowerBase == ".env" || strings.HasPrefix(lowerBase, ".env.")
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
	lexical := resolveForGuard(path)
	// GORILLA FIX (2026-10-05): judge where the path really LEADS. The guard
	// compared text, and on Windows three different texts open the same file:
	// a short 8.3 name (AWS~1\CREDEN~1), a name with trailing dots or spaces
	// (.ssh.\id_rsa.), and a junction inside the project that points at
	// ~\.ssh, which also counted as "inside the workspace" and was exempt.
	abs := realForGuard(lexical)

	// Inside the workspace the user has already chosen this ground. Both the
	// path as written AND where it leads must be inside: a link in the project
	// that leads out of it is not the project.
	if _, ok := config.RootFor(lexical); ok {
		if _, okReal := config.RootFor(abs); okReal || strings.EqualFold(abs, lexical) {
			return ""
		}
	}

	base := filepath.Base(abs)
	lower := strings.ToLower(base)

	for _, n := range sensitiveBasenames {
		if strings.EqualFold(base, n) {
			return reason(abs, "it is a credential file")
		}
	}
	if isDotEnv(lower) {
		return reason(abs, "it is an environment file, which is where API keys and passwords are kept")
	}
	for _, suf := range sensitiveSuffixes {
		if strings.HasSuffix(lower, suf) {
			return reason(abs, "it looks like key material ("+suf+")")
		}
	}

	// The application's own configuration holds provider API keys.
	// Case-insensitively (Windows opens .CONFIG as .config), and wherever the
	// configuration really is: with XDG_CONFIG_HOME set it is not under .config.
	lowAbs := strings.ToLower(abs)
	if strings.Contains(lowAbs, strings.ToLower(filepath.Join(".config", "gorilla-opencode"))) ||
		strings.Contains(lowAbs, strings.ToLower(filepath.Join(".config", "opencode"))) {
		return reason(abs, "it holds this program's own provider API keys")
	}
	if base := strings.ToLower(filepath.Clean(config.ConfigBase())); len(base) > 3 &&
		(lowAbs == base || strings.HasPrefix(lowAbs, base+string(filepath.Separator))) {
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

// resolveForGuard turns a requested path into the absolute path this guard
// should judge.
//
// GORILLA OVERRIDE (2026-09-01): a rooted path must not be mistaken for a
// relative one. This was a real hole in the credential guard on Windows.
//
// filepath.IsAbs("/home/user/.ssh/id_rsa") is FALSE on Windows, because Windows
// calls that "rooted but volume-relative", not absolute. The guard therefore
// treated it as a path relative to the PROJECT, joined it to the working
// directory, and got C:\project\home\user\.ssh\id_rsa — which is inside the
// workspace, and the workspace is explicitly exempt on the grounds that the user
// has already chosen that ground. So the check returned "allowed" for
// id_rsa, .aws/credentials, .netrc, secring.gpg and .kube/config alike: all nine
// paths the test asserts, waved through, with the refusal never reached.
//
// The fix is to resolve a rooted path against the working directory's VOLUME
// rather than against the working directory. C:\home\user\.ssh\id_rsa is then
// outside the workspace and is judged on its merits, which is what the caller
// meant and what the guard is for.
func resolveForGuard(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if runtime.GOOS == "windows" && (strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`)) {
		if vol := filepath.VolumeName(config.WorkingDirectory()); vol != "" {
			return filepath.Clean(vol + filepath.FromSlash(path))
		}
		return filepath.Clean(filepath.FromSlash(path))
	}
	return filepath.Clean(filepath.Join(config.WorkingDirectory(), path))
}

func reason(path, why string) string {
	return fmt.Sprintf(
		"Refusing to read %s: %s, and it is outside this project. Anything read here "+
			"goes into the model's context, over the network to the configured provider, "+
			"and into the session database. If you genuinely need it, copy the part you "+
			"need into the project first.",
		path, why)
}

// realForGuard resolves a path to the file it actually names: links and
// junctions followed, short names expanded, and (on Windows) the trailing dots
// and spaces that Win32 ignores removed. A path that does not exist yet is
// resolved as far as its parent.
func realForGuard(abs string) string {
	if runtime.GOOS == "windows" {
		parts := strings.Split(abs, `\`)
		for i := 1; i < len(parts); i++ {
			if t := strings.TrimRight(parts[i], ". "); t != "" {
				parts[i] = t
			}
		}
		abs = strings.Join(parts, `\`)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(dir, filepath.Base(abs))
	}
	return abs
}

// refuseSensitiveGlob refuses a search whose PATTERN reaches into a credential
// folder from outside the workspace.
//
// GORILLA FIX (2026-10-05): find checked only the folder it was pointed at.
// find(path="C:\\Users", glob=".aws/**", query="secret") passed, because
// C:\Users is not sensitive, and printed lines out of .aws\credentials.
func refuseSensitiveGlob(glob, searchRoot string) string {
	if strings.TrimSpace(glob) == "" {
		return ""
	}
	if _, inside := config.RootFor(realForGuard(resolveForGuard(searchRoot))); inside {
		return ""
	}
	for _, seg := range strings.FieldsFunc(strings.TrimPrefix(glob, "!"), func(r rune) bool { return r == '/' || r == '\\' }) {
		name := strings.ToLower(strings.Trim(seg, "*?"))
		if name == "" {
			continue
		}
		hit := isDotEnv(name)
		for _, d := range sensitiveDirSegments {
			hit = hit || name == d
		}
		for _, b := range sensitiveBasenames {
			hit = hit || name == strings.ToLower(b)
		}
		for _, suf := range sensitiveSuffixes {
			hit = hit || strings.HasSuffix(name, suf)
		}
		if hit {
			return fmt.Sprintf("Refusing this search: the pattern %q reaches for %s, which is where credentials are kept, "+
				"and the search starts outside the project. Search inside the project, or name a different pattern.", glob, seg)
		}
	}
	return ""
}
