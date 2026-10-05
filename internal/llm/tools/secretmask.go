package tools

// GORILLA OVERRIDE (2026-10-04): a credential printed by a tool does not go to
// the provider.
//
// # THE GAP
//
// sensitive.go refuses to READ a credential file. That closes the front door
// and leaves the windows open: `env`, `printenv`, `git config --list`, a build
// script echoing its settings, a failing test dumping its environment — each
// prints a live key as ordinary output. Tool output goes into the model's
// context, so over the wire to whichever provider is configured, and into the
// session database in cleartext.
//
// # WHAT THIS DOES
//
// Every tool result passes through MaskSecrets before it is stored or sent.
// Two kinds of thing are replaced with a labelled placeholder:
//
//  1. VALUES THIS MACHINE HOLDS. The provider API keys in this program's own
//     configuration, and the value of any environment variable whose NAME says
//     it is a secret. These are matched exactly, wherever they appear, in the
//     output of every tool.
//
//  2. SHAPES THAT ARE ONLY EVER SECRETS. A PEM private-key block, and tokens
//     whose prefix is assigned by the issuer (ghp_, sk-ant-, AKIA...). These
//     are matched in COMMAND output only.
//
// # WHY (2) IS NOT APPLIED TO FILE READS
//
// Because a file the model reads, it may write back. If `view` showed a test
// fixture's fake token as [REDACTED], the model's next edit would either fail
// to match the real text or write the placeholder into the file. Masking a
// known live value is safe everywhere — nothing legitimate needs it in context.
// Masking a shape in a source file trades a leak nobody has for a corruption
// somebody will.
//
// # WHERE THE IDEA CAME FROM
//
// OpenHands' secret registry (OpenHands/software-agent-sdk,
// openhands-sdk/openhands/sdk/conversation/secret_registry.py, MIT), which
// masks registered secret values in command output. Part (1) is that. Part (2)
// and the file-read exemption are ours.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// minSecretLen keeps short values out of the known-value list. A variable
// called API_KEY set to "test" or "1" must not turn every "test" in every
// output into a placeholder.
const minSecretLen = 12

// secretNameRe decides, from an environment variable's NAME, that its value is
// a secret. Anchored on word boundaries made of underscores so that
// MONKEY and KEYBOARD_LAYOUT do not qualify.
var secretNameRe = regexp.MustCompile(`(?i)(?:^|_)(?:KEY|APIKEY|API_KEY|TOKEN|SECRET|PASSWORD|PASSWD|PWD|CREDENTIAL|CREDENTIALS|AUTH)(?:_|$)`)

// notSecretNames are variables that match secretNameRe and hold no secret.
var notSecretNames = map[string]bool{
	"PWD": true, "OLDPWD": true, "SSH_AUTH_SOCK": true, "GPG_AGENT_INFO": true,
}

type knownSecret struct {
	value string
	label string
}

var (
	knownSecretsMu   sync.Mutex
	knownSecretsList []knownSecret
	knownSecretsDone bool
)

// knownSecrets gathers the values this machine holds, once. Longest first, so
// a key that contains another is replaced whole.
func knownSecrets() []knownSecret {
	knownSecretsMu.Lock()
	defer knownSecretsMu.Unlock()
	// GORILLA FIX (2026-10-05): the list was built once per process, so a key
	// entered with /connect or /provider afterwards was never masked. It is
	// rebuilt when it is more than a few seconds old; building it reads the
	// configuration in memory and three small files.
	if knownSecretsDone && time.Since(knownSecretsAt) < 10*time.Second {
		return knownSecretsList
	}
	knownSecretsDone = true
	knownSecretsAt = time.Now()
	knownSecretsList = nil

	seen := map[string]bool{}
	add := func(value, label string) {
		value = strings.TrimSpace(value)
		if len(value) < minSecretLen || seen[value] {
			return
		}
		// A path or a phrase is not a credential, whatever the variable is
		// called (SSH_KEY_PATH=/home/u/.ssh/id_rsa). Judged by how the value
		// STARTS, not by whether it contains a slash: an AWS secret key is
		// base64 and has slashes in the middle.
		if strings.Contains(value, " ") || looksLikePath(value) {
			return
		}
		seen[value] = true
		knownSecretsList = append(knownSecretsList, knownSecret{value: value, label: label})
	}

	if cfg := safeConfig(); cfg != nil {
		for name, p := range cfg.Providers {
			add(p.APIKey, "provider key for "+string(name))
		}
		// GORILLA FIX (2026-10-05): the keys of saved connections (NVIDIA NIM,
		// Cloudflare ...) were not on the list at all, though they are the
		// keys most people here actually hold.
		for _, e := range cfg.LocalEndpoints {
			add(e.APIKey, "key for the connection "+e.Name)
		}
	}
	// ... nor were the sign-in tokens, which are as good as a password until
	// they expire. Read from the files the sign-ins write; no parsing of who
	// wrote them, just the token fields.
	for _, f := range []string{"chatgpt-oauth.json", "antigravity-oauth.json", "gemini-oauth.json", "oauth_creds.json"} {
		blob, err := os.ReadFile(filepath.Join(config.ConfigBase(), f))
		if err != nil {
			continue
		}
		var fields map[string]any
		if json.Unmarshal(blob, &fields) != nil {
			continue
		}
		for _, k := range []string{"access_token", "refresh_token", "id_token"} {
			if v, ok := fields[k].(string); ok {
				add(v, "sign-in token ("+strings.TrimSuffix(f, ".json")+")")
			}
		}
	}
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || notSecretNames[strings.ToUpper(name)] || !secretNameRe.MatchString(name) {
			continue
		}
		add(value, "value of $"+name)
	}

	sort.SliceStable(knownSecretsList, func(i, j int) bool {
		return len(knownSecretsList[i].value) > len(knownSecretsList[j].value)
	})
	return knownSecretsList
}

// safeConfig returns the loaded configuration, or nil if none is loaded yet.
// config.Get panics in that case, and masking must never be what brings a tool
// call down.
func safeConfig() (cfg *config.Config) {
	defer func() {
		if recover() != nil {
			cfg = nil
		}
	}()
	return config.Get()
}

// resetKnownSecretsForTest forgets the gathered values so a test can set its
// own environment.
func resetKnownSecretsForTest() {
	knownSecretsMu.Lock()
	knownSecretsDone = false
	knownSecretsList = nil
	knownSecretsMu.Unlock()
}

type secretShape struct {
	label string
	re    *regexp.Regexp
}

// secretShapes are formats whose prefix is assigned by the issuer, so a match
// is a credential and not a coincidence. Deliberately short: a generic
// "long random string" rule would mask commit hashes and checksums, which a
// coding agent reads all day.
var secretShapes = []secretShape{
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)},
	{"GitHub token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b`)},
	{"GitHub token", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{40,}\b`)},
	{"Anthropic key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{24,}`)},
	{"OpenAI key", regexp.MustCompile(`\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{32,}`)},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"AWS access key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"Groq key", regexp.MustCompile(`\bgsk_[A-Za-z0-9]{40,}\b`)},
	{"Hugging Face token", regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`)},
	{"Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{20,}`)},
	{"GitLab token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`)},
	{"npm token", regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36}\b`)},
}

// toolsWhoseOutputIsNotAFile are the tools whose result is produced by running
// something, not by reading a file the model may write back. Shape masking
// applies to these only. See the header.
var toolsWhoseOutputIsNotAFile = map[string]bool{
	BashToolName: true,
}

// MaskSecrets replaces credentials in a tool result and reports how many
// replacements it made. toolName decides whether the shape rules apply.
func MaskSecrets(toolName, content string) (string, int) {
	if content == "" {
		return content, 0
	}
	n := 0
	for _, s := range knownSecrets() {
		if c := strings.Count(content, s.value); c > 0 {
			content = strings.ReplaceAll(content, s.value, "[REDACTED: "+s.label+"]")
			n += c
		}
	}
	if toolsWhoseOutputIsNotAFile[toolName] {
		for _, shape := range secretShapes {
			content = shape.re.ReplaceAllStringFunc(content, func(string) string {
				n++
				return "[REDACTED: " + shape.label + "]"
			})
		}
	}
	return content, n
}

// MaskNotice is the sentence appended to a result that was masked, so the
// model knows the placeholder is this program's doing and not what the command
// printed, and does not go looking for the value another way.
func MaskNotice(n int) string {
	if n == 0 {
		return ""
	}
	// GORILLA FIX (2026-10-04): written to be true WHOEVER reads it.
	//
	// Measured with a real model (gemma-4-e2b), three wordings:
	//
	//  1. "...before you saw it. Do not try to recover it; you do not need it."
	//     Copied word for word into the answer, so the PERSON was told they had
	//     not seen their own key and should not try to recover it.
	//  2. "[Note to the assistant ... do not repeat it ...]". Obeyed on one run,
	//     copied out in full on the next. An instruction a small model follows
	//     half the time is not a control.
	//  3. This one: no "you", no instruction, nothing that is false or odd if
	//     the model reads it out. Whether it is repeated stops mattering.
	//
	// The same principle as a truncation notice: say what happened, in words
	// that stay correct when they are passed on.
	verb := "was"
	if n > 1 {
		verb = "were"
	}
	return "\n\n[Gorilla OpenCode: " + pluralSecrets(n) + " in this output " + verb +
		" hidden and shown as [REDACTED: ...]. The real value stays on this computer and was not sent to the AI.]"
}

func pluralSecrets(n int) string {
	if n == 1 {
		return "1 credential"
	}
	return strconv.Itoa(n) + " credentials"
}

// looksLikePath reports whether a value begins the way a filesystem path does.
func looksLikePath(v string) bool {
	if strings.HasPrefix(v, "/") || strings.HasPrefix(v, "~") || strings.HasPrefix(v, ".") || strings.HasPrefix(v, `\`) {
		return true
	}
	return len(v) >= 3 && v[1] == ':' && (v[2] == '\\' || v[2] == '/')
}

// knownSecretsAt is when knownSecretsList was last built.
var knownSecretsAt time.Time
