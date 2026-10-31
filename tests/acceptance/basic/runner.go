package basic

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Call is one tool call the program recorded in its own receipt (never what
// the model said it did).
type Call struct {
	Tool    string `json:"tool"`
	Detail  string `json:"detail"`
	Outcome string `json:"outcome"`
	Times   int    `json:"times"`
}

// JobResult is the record of one job on one model.
type JobResult struct {
	Job   string `json:"job"`
	Title string `json:"title"`
	Model string `json:"model"`
	// Variant is "" (this machine's profile) or "fresh-profile" (see Config).
	Variant string  `json:"variant,omitempty"`
	OS      string  `json:"os"`
	Pass    bool    `json:"pass"`
	Seconds float64 `json:"seconds"`
	// ExitCode is the program's; -1 when it was killed by the job's time limit.
	ExitCode  int      `json:"exit_code"`
	Checks    []Result `json:"checks"`
	ToolCalls int      `json:"tool_calls"`
	Calls     []Call   `json:"calls"`
	Answer    string   `json:"answer"`
	Stderr    string   `json:"stderr,omitempty"`
	// NeedsPresent says, for each program the job names, whether it was on the
	// run's PATH (or embedded) - the evidence for "tool missing".
	NeedsPresent map[string]bool `json:"needs_present,omitempty"`
	// CauseAuto is a first guess at why a failed job failed, from the record
	// alone. The report's causes file overrides it after a person has looked.
	CauseAuto string `json:"cause_auto,omitempty"`
}

// The four causes the report uses (brief P5a, item 4).
const (
	CauseToolMissing = "tool missing"
	CauseNotKnown    = "model did not know the tool exists"
	CauseRefused     = "tool refused by permission or guard"
	CauseModelError  = "model error"
)

// Config says how to run.
type Config struct {
	Exe      string // gorilla-opencode binary
	Model    string // e.g. chatgpt.gpt-5.6-luna
	Root     string // scratch root; each job gets a folder in it
	Fixtures string
	// PathMode: "clean" (default) runs with only the system folders on PATH,
	// what a person who installed nothing else has; "host" keeps this machine's.
	PathMode string
	// Variant "fresh-profile" moves USERPROFILE, HOME, APPDATA and LOCALAPPDATA
	// to an empty folder, so what the owner installed under their profile
	// (scoop, winget, pip) is not where a model would look first - closer to a
	// person who installed nothing. The config folder (login, MCP servers) is
	// kept by pointing XDG_CONFIG_HOME at the real one. Programs under
	// Program Files stay findable by full path; that is stated in the report.
	Variant string
	Timeout time.Duration
	// Embedded says the exe was built with the toolbox (so "toolbox:X" needs
	// count as present).
	Embedded bool
}

// RunJob runs one job on cfg.Model and checks the result.
func RunJob(cfg Config, j Job) (JobResult, error) {
	res := JobResult{Job: j.ID, Title: j.Title, Model: cfg.Model, Variant: cfg.Variant, OS: runtime.GOOS, ExitCode: -1}
	p, err := Prepare(j, cfg.Root, cfg.Fixtures)
	if err != nil {
		return res, err
	}
	defer p.Stop()

	pin := fmt.Sprintf(`{"agents":{"coder":{"model":%q},"task":{"model":%q},"title":{"model":%q},"summarizer":{"model":%q}}}`,
		cfg.Model, cfg.Model, cfg.Model, cfg.Model)
	if err := os.WriteFile(filepath.Join(p.Dir, ".gorilla-opencode.json"), []byte(pin), 0o644); err != nil {
		return res, err
	}

	env := runEnv(cfg.PathMode, cfg.Variant, cfg.Root)
	res.NeedsPresent = map[string]bool{}
	for _, n := range j.Needs {
		if strings.HasPrefix(n, "toolbox:") {
			res.NeedsPresent[n] = cfg.Embedded
		} else {
			res.NeedsPresent[n] = onPath(n, env)
		}
	}

	timeout := cfg.Timeout
	if j.Timeout != "" {
		if d, err := time.ParseDuration(j.Timeout); err == nil {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cfg.Exe, "-p", p.Vars.Expand(j.Prompt), "-c", p.Dir, "-f", "json", "-q")
	cmd.Dir = p.Dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	runErr := cmd.Run()
	res.Seconds = time.Since(start).Seconds()
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		res.ExitCode = -1
		res.Stderr = "killed by the job time limit " + timeout.String() + "\n"
	}
	res.Stderr += tailString(stderr.String(), 1500)
	res.Answer, res.Calls, res.ToolCalls = parseOutput(stdout.Bytes())
	if res.Answer == "" && runErr != nil {
		res.Answer = ""
	}

	// Stop the helpers BEFORE the checks, so a growing log or a listener cannot
	// change what is measured.
	p.Stop()
	res.Checks = RunChecks(j, p.Dir, p.Vars, res.Answer)
	res.Pass = AllPassed(res.Checks)
	if !res.Pass {
		res.CauseAuto = guessCause(res)
	}
	return res, nil
}

// parseOutput reads the -f json document: the answer and the receipt.
func parseOutput(out []byte) (answer string, calls []Call, n int) {
	var doc struct {
		Response string `json:"response"`
		Receipt  struct {
			ToolCalls int    `json:"tool_calls"`
			Calls     []Call `json:"calls"`
		} `json:"receipt"`
	}
	if i := bytes.IndexByte(out, '{'); i >= 0 && json.Unmarshal(out[i:], &doc) == nil {
		return doc.Response, doc.Receipt.Calls, doc.Receipt.ToolCalls
	}
	return strings.TrimSpace(string(out)), nil, 0
}

// guessCause is a first guess at the cause of a failure from the record alone.
// A person confirms it before it goes in the report.
func guessCause(r JobResult) string {
	for _, c := range r.Calls {
		if strings.Contains(c.Outcome, "refused") {
			return CauseRefused
		}
	}
	if r.ExitCode != 0 && r.ToolCalls == 0 {
		return CauseModelError
	}
	var missing, present []string
	for n, ok := range r.NeedsPresent {
		if ok {
			present = append(present, n)
		} else {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return CauseToolMissing
	}
	used := false
	for _, n := range present {
		name := strings.TrimPrefix(n, "toolbox:")
		for _, c := range r.Calls {
			if strings.Contains(strings.ToLower(c.Detail), strings.ToLower(name)) {
				used = true
			}
		}
	}
	if len(present) > 0 && !used {
		return CauseNotKnown
	}
	return CauseModelError
}

// runEnv is the environment of the run. PATH is replaced in "clean" mode.
func runEnv(mode, variant, root string) []string {
	drop := map[string]bool{}
	var add []string
	if mode != "host" {
		drop["PATH"] = true
		add = append(add, "PATH="+cleanPath())
	}
	if variant == "fresh-profile" {
		realConfig := os.Getenv("XDG_CONFIG_HOME")
		if realConfig == "" {
			home, _ := os.UserHomeDir()
			realConfig = filepath.Join(home, ".config")
		}
		fresh := filepath.Join(root, "_fresh-home")
		os.MkdirAll(filepath.Join(fresh, "AppData", "Roaming"), 0o755)
		os.MkdirAll(filepath.Join(fresh, "AppData", "Local"), 0o755)
		for _, k := range []string{"USERPROFILE", "HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME"} {
			drop[k] = true
		}
		add = append(add,
			"USERPROFILE="+fresh, "HOME="+fresh,
			"APPDATA="+filepath.Join(fresh, "AppData", "Roaming"),
			"LOCALAPPDATA="+filepath.Join(fresh, "AppData", "Local"),
			"XDG_CONFIG_HOME="+realConfig)
	}
	var keep []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if drop[strings.ToUpper(name)] {
			continue
		}
		keep = append(keep, kv)
	}
	return append(keep, add...)
}

func cleanPath() string {
	if runtime.GOOS != "windows" {
		return "/usr/local/bin:/usr/bin:/bin"
	}
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return strings.Join([]string{
		filepath.Join(root, "System32"),
		root,
		filepath.Join(root, "System32", "Wbem"),
		filepath.Join(root, "System32", "WindowsPowerShell", "v1.0"),
		filepath.Join(root, "System32", "OpenSSH"),
	}, string(os.PathListSeparator))
}

// onPath looks for a program in the PATH of env.
func onPath(name string, env []string) bool {
	var path string
	for _, kv := range env {
		if strings.HasPrefix(strings.ToUpper(kv), "PATH=") {
			path = kv[5:]
		}
	}
	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{".exe", ".cmd", ".bat", ".com"}
	}
	for _, dir := range filepath.SplitList(path) {
		for _, e := range exts {
			if st, err := os.Stat(filepath.Join(dir, name+e)); err == nil && !st.IsDir() {
				return true
			}
		}
	}
	return false
}

func tailString(s string, n int) string {
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}

// BuildExe builds gorilla-opencode once. On Windows it embeds the pinned
// toolbox (-tags toolbox): the helpers are taken from helpersDir (plain file
// names) or, when that is empty, from the installed copy's cache folder
// (<cache>/gorilla-opencode/toolbox/<sha256>/<name>.exe), and each is checked
// against internal/toolbox/SHA256SUMS before it is embedded - nothing unpinned
// goes in. The embedded copies are removed again afterwards. Elsewhere it is a
// plain build. Returns whether the toolbox is in.
func BuildExe(repo, out, helpersDir string) (embedded bool, err error) {
	args := []string{"build", "-trimpath", "-ldflags", "-s -w", "-o", out, "."}
	if runtime.GOOS == "windows" {
		dest, err := embedToolbox(repo, helpersDir)
		if err != nil {
			return false, err
		}
		defer os.RemoveAll(dest)
		args = []string{"build", "-tags", "toolbox", "-trimpath", "-ldflags", "-s -w", "-o", out, "."}
		embedded = true
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("go build: %v\n%s", err, b)
	}
	return embedded, nil
}

func embedToolbox(repo, helpersDir string) (string, error) {
	sums, err := os.ReadFile(filepath.Join(repo, "internal", "toolbox", "SHA256SUMS"))
	if err != nil {
		return "", err
	}
	dest := filepath.Join(repo, "internal", "toolbox", "bin", "windows-amd64")
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	cache, _ := os.UserCacheDir()
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || strings.HasPrefix(f[0], "#") || !strings.HasPrefix(f[1], "windows-amd64/") {
			continue
		}
		pin, name := f[0], strings.TrimPrefix(f[1], "windows-amd64/")
		src := filepath.Join(helpersDir, name)
		if helpersDir == "" {
			src = filepath.Join(cache, "gorilla-opencode", "toolbox", pin, name)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			os.RemoveAll(dest)
			return "", fmt.Errorf("helper %s: %w", name, err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != pin {
			os.RemoveAll(dest)
			return "", fmt.Errorf("helper %s has sha256 %x, SHA256SUMS pins %s: not embedding it", name, sum, pin)
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write(data)
		zw.Close()
		if err := os.WriteFile(filepath.Join(dest, name+".gz"), buf.Bytes(), 0o644); err != nil {
			return "", err
		}
	}
	return dest, nil
}
