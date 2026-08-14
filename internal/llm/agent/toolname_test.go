package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// THESE TESTS ARE THE LOCK. toolname.go is the door.
//
// The change under test repairs a model that misspells its own tool names
// ("ls<|message|>" instead of "ls"), measured 2026-08-14 when 30 of 44 tool
// calls in a research run failed that way and every helper returned nothing.
//
// The owner's requirement, verbatim: make sure "no one, no agent, no model, no
// other moron developer can use this to escalate anything".
//
// So every test below is an ATTACK, not a feature check. If you are here
// because one of them failed after you edited toolname.go: the test is not
// wrong. You have opened a hole.

type fakeTool string

func (f fakeTool) ToolName() string { return string(f) }

// The real tool set a coder agent has, including the dangerous ones.
var dangerousToolset = []toolNamer{
	fakeTool("bash"), fakeTool("edit"), fakeTool("write"), fakeTool("view"),
	fakeTool("ls"), fakeTool("grep"), fakeTool("glob"), fakeTool("patch"),
}

// ATTACK 1: prefix / suffix / substring smuggling.
//
// If any of these resolve, an attacker who can steer the model — via a poisoned
// README, a fetched page, a crafted filename — chooses which tool runs.
func TestNoPrefixOrFuzzyNameEverResolves(t *testing.T) {
	for _, attack := range []string{
		"bash_readonly", "bash-safe", "bashx", "bas", "BASH", "Bash",
		"bash bash", "bash;rm -rf /", "bash\nedit", "bash/../edit",
		"editx", "wr1te", "vieww", "b a s h", "bash ", " bash",
		"bash()", "bash{}", "bash|edit", "bash&&edit", "bash$(id)",
		"../bash", "./bash", "tools/bash", "bash.exe",
	} {
		if idx, _, _ := findTool(dangerousToolset, attack); idx >= 0 {
			t.Errorf("SECURITY: %q resolved to %q — a near-miss name reached a real tool",
				attack, dangerousToolset[idx].ToolName())
		}
	}
}

// ATTACK 2: the control token in places other than a trailing position.
//
// A token in the MIDDLE is not a stutter, it is probing. It must be refused,
// not cleaned "harder".
func TestOnlyATrailingControlTokenIsEverStripped(t *testing.T) {
	for _, attack := range []string{
		"ba<|x|>sh",       // embedded
		"<|message|>bash", // leading
		"<|bash|>",        // the whole name is a token
		"<|>",
		"<|message|>",
		"bash<|x|>edit",
		"b<|a|>s<|h|>",
	} {
		cleaned, _, ok := normaliseToolName(attack)
		if ok && cleaned == "bash" {
			t.Errorf("SECURITY: %q was cleaned into %q — cleaning must not reach inside a name",
				attack, cleaned)
		}
		if idx, _, _ := findTool(dangerousToolset, attack); idx >= 0 {
			t.Errorf("SECURITY: %q resolved to %q", attack, dangerousToolset[idx].ToolName())
		}
	}
}

// ATTACK 3: cleaning must never produce something that was not already there.
//
// The output must be a SUBSTRING of the input with only trailing tokens and
// whitespace removed. If it can add or alter a character, it can forge a name.
func TestCleaningOnlyEverRemovesTrailingTokensAndSpace(t *testing.T) {
	for _, raw := range []string{
		"ls<|message|>", "view<|end|>", "grep <|im_end|> ", "bash<|a|><|b|>",
		"bash", "  bash  ", "ls<|message|><|message|><|message|>",
		"nonsense<|x|>", "", "<|x|>", "ls<|message|>extra",
	} {
		cleaned, _, ok := normaliseToolName(raw)
		if !ok {
			continue // refused, which is always a safe outcome
		}
		// Every character of the result must appear, in order, in the input.
		if !strings.Contains(raw, cleaned) {
			t.Errorf("SECURITY: %q produced %q, which is not present in the input — "+
				"cleaning invented characters and can therefore forge a tool name", raw, cleaned)
		}
		if len(cleaned) > len(raw) {
			t.Errorf("SECURITY: %q grew into %q", raw, cleaned)
		}
	}
}

// ATTACK 4: a name that cleans to a tool the CALLING AGENT does not have.
//
// A research helper has no bash. No spelling may give it one.
func TestCleaningCannotReachAToolThisAgentDoesNotHave(t *testing.T) {
	// Exactly ResearchAgentTools' surface: read-only plus web.
	helperToolset := []toolNamer{
		fakeTool("fetch"), fakeTool("web_search"), fakeTool("glob"),
		fakeTool("grep"), fakeTool("ls"), fakeTool("view"), fakeTool("diagnostics"),
	}
	for _, attack := range []string{
		"bash", "bash<|message|>", "edit<|message|>", "write<|message|>",
		"patch<|message|>", "sparse<|message|>", "agent<|message|>",
	} {
		if idx, _, _ := findTool(helperToolset, attack); idx >= 0 {
			t.Errorf("SECURITY: a research helper resolved %q to %q — helpers must be read-only",
				attack, helperToolset[idx].ToolName())
		}
	}
	// ...while the tools it DOES have still work, or the fix is pointless.
	for _, good := range []string{"ls<|message|>", "grep<|message|>", "view<|end|>"} {
		if idx, _, cleaned := findTool(helperToolset, good); idx < 0 || !cleaned {
			t.Errorf("%q did not resolve; the bug this exists to fix is back", good)
		}
	}
}

// The repair must actually work, or all of the above is guarding nothing.
// These are the exact strings measured in the failed run.
func TestTheMeasuredMalformedNamesAreRepaired(t *testing.T) {
	for raw, want := range map[string]string{
		"ls<|message|>":    "ls",
		"view<|message|>":  "view",
		"glob<|message|>":  "glob",
		"grep<|message|>":  "grep",
		"bash<|message|>":  "bash",
		"edit<|message|> ": "edit",
	} {
		idx, used, cleaned := findTool(dangerousToolset, raw)
		if idx < 0 {
			t.Errorf("%q still does not resolve — 30 of 44 calls failed this way", raw)
			continue
		}
		if used != want {
			t.Errorf("%q resolved to %q, want %q", raw, used, want)
		}
		if !cleaned {
			t.Errorf("%q resolved without being flagged as repaired; the repair must be visible", raw)
		}
	}
}

// An exact name must never be flagged as repaired — otherwise the log fills
// with noise and a real repair stops standing out.
func TestExactNamesResolveUntouched(t *testing.T) {
	for _, name := range []string{"bash", "edit", "ls", "view"} {
		idx, used, cleaned := findTool(dangerousToolset, name)
		if idx < 0 || used != name {
			t.Fatalf("exact name %q failed to resolve", name)
		}
		if cleaned {
			t.Errorf("exact name %q was reported as repaired", name)
		}
	}
}

// SOURCE GUARD 1: no fuzzy matching may appear in the dispatch path, ever.
// Checked against the source because a fuzzy fallback added tomorrow would pass
// every behavioural test above by simply not being reached by those inputs.
func TestNoFuzzyMatchingPrimitivesInTheDispatchPath(t *testing.T) {
	banned := []string{
		"strings.HasPrefix(toolCall", "strings.Contains(toolCall",
		"strings.EqualFold", "levenshtein", "Levenshtein",
		"strings.ToLower(toolCall", "closestTool", "didYouMean",
	}
	for _, file := range []string{"toolname.go", "agent.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		body := string(src)
		for _, b := range banned {
			// Comments naming the banned technique are the point of the file.
			for _, line := range strings.Split(body, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(line, b) {
					t.Errorf("SECURITY: %s contains %q in live code — fuzzy tool matching is a "+
						"privilege-escalation primitive. Read the header of toolname.go.", file, b)
				}
			}
		}
	}
}

// SOURCE GUARD 2: the defence-in-depth layer. Every permission request must
// name the tool with a CONSTANT, never with a value derived from model output.
// If a tool ever passes a caller-supplied name here, an alias could slip past a
// deny-list.
func TestPermissionRequestsUseTheToolsOwnConstantName(t *testing.T) {
	dir := filepath.Join("..", "tools")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "ToolName" {
				return true
			}
			checked++
			// Must be a bare identifier (a constant like BashToolName) or a
			// qualified one. Anything computed is a finding.
			switch v := kv.Value.(type) {
			case *ast.Ident, *ast.SelectorExpr:
				return true
			default:
				t.Errorf("SECURITY: %s sets permission ToolName to a computed value (%T); "+
					"it must be the tool's own constant, or a deny-list can be bypassed by alias",
					path, v)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("found no permission ToolName assignments at all — this guard is inspecting nothing")
	}
	t.Logf("checked %d permission ToolName assignments", checked)
}
