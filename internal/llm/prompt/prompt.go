package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/logging"
)

func GetAgentPrompt(agentName config.AgentName, provider models.ModelProvider) string {
	basePrompt := ""
	switch agentName {
	case config.AgentCoder:
		basePrompt = CoderPrompt(provider)
	case config.AgentTitle:
		basePrompt = TitlePrompt(provider)
	case config.AgentTask:
		basePrompt = TaskPrompt(provider)
	case config.AgentSummarizer:
		basePrompt = SummarizerPrompt(provider)
	default:
		basePrompt = "You are a helpful assistant"
	}

	if agentName == config.AgentCoder || agentName == config.AgentTask {
		// Add context from project-specific instruction files if they exist
		contextContent := getContextFromPaths()
		logging.Debug("Context content", "Context", contextContent)
		if contextContent != "" {
			return fmt.Sprintf("%s\n\n# Project-Specific Context\n Make sure to follow the instructions in the context below\n%s", basePrompt, contextContent)
		}
	}
	return basePrompt
}

// GORILLA OVERRIDE: this cache was a sync.Once, which froze project context for
// the entire process lifetime. sync.Once cannot be reset, so /add-dir, /cd and
// any contextPaths edit were silent no-ops until restart — the control appeared
// to work and changed nothing.
//
// It also made the package's own test lie: TestGetContextFromPaths passes at
// -count=1 but fails at -count=2+, because every later run was served the first
// run's content (built from the first run's temp dir). That failure was the bug,
// not a flaky test.
var (
	contextMu      sync.RWMutex
	contextContent string
	contextLoaded  bool
)

// InvalidateContextCache forces the next system-prompt render to re-read the
// context files. Call it after anything that changes which files are in scope:
// adding or removing a workspace root, changing the working directory, or
// editing contextPaths.
func InvalidateContextCache() {
	contextMu.Lock()
	contextContent = ""
	contextLoaded = false
	contextMu.Unlock()
}

func getContextFromPaths() string {
	contextMu.RLock()
	if contextLoaded {
		defer contextMu.RUnlock()
		return contextContent
	}
	contextMu.RUnlock()

	contextMu.Lock()
	defer contextMu.Unlock()
	// Another goroutine may have loaded it while we waited for the write lock.
	if contextLoaded {
		return contextContent
	}

	// GORILLA OVERRIDE: read context files from EVERY workspace root, not just
	// the primary one. This is what makes /add-dir mean anything — a root whose
	// CLAUDE.md is never read is just a directory the agent was already able to
	// open by absolute path.
	cfg := config.Get()
	primary := config.WorkingDirectory()
	var parts []string
	for _, root := range config.Roots() {
		paths := cfg.ContextPaths

		// GORILLA OVERRIDE (2026-08-19), guards 1 and 3 on AGENTS.md.
		//
		// A context file is spliced into the system prompt automatically, on
		// entering a directory, before the user has typed anything, with no
		// tool call and no permission prompt. `git clone && cd` is therefore
		// prompt injection, and AGENTS.md is a PUBLIC STANDARD an attacker
		// knows is honoured — unlike CLAUDE.md, which is at least a
		// convention of the tools that read it.
		//
		// Guard 1: the primary working directory only. An /add-dir root is a
		// directory you granted READ access to, often somebody else's
		// checkout; it is not a statement that you want its instructions
		// governing your session.
		//
		// Guard 3: not for a repository that is not yours. See
		// config.AutoLoadProjectInstructions.
		if root != primary {
			paths = withoutAgentsFile(paths)
		} else if v := config.AutoLoadProjectInstructions(root); !v.Loaded && v.Bytes > 0 {
			logging.Info("not loading "+config.AgentsFile+" automatically",
				"path", v.Path, "bytes", v.Bytes, "reason", v.Reason)
			paths = withoutAgentsFile(paths)
		}

		if s := processContextPaths(root, paths); s != "" {
			parts = append(parts, s)
		}
	}
	contextContent = strings.Join(parts, "\n")
	contextLoaded = true
	return contextContent
}

// processContextPaths reads every context file found under workDir for the given
// paths and joins them in a DETERMINISTIC order: paths in the order given, and
// within a directory path, the order WalkDir yields (lexical).
//
// GORILLA OVERRIDE: results used to be funnelled through an unbuffered channel
// and appended in arrival order, so the output order depended on goroutine
// scheduling. With the shipped 12 contextPaths that race is mostly hidden by one
// path finishing first, but it is a real race and it becomes visible as soon as
// more paths (or more roots) are in play. Collecting into an index-addressed
// slice keeps the concurrency and removes the non-determinism.
func processContextPaths(workDir string, paths []string) string {
	var wg sync.WaitGroup

	// One slot per path, so a goroutine's output lands at its path's index
	// rather than wherever it happens to finish.
	perPath := make([][]string, len(paths))

	// Track processed files to avoid duplicates (case-insensitive).
	processedFiles := make(map[string]bool)
	var processedMutex sync.Mutex

	// claim reports whether this call is the first to see path; only the
	// claimant reads it, so a file listed twice is included once.
	claim := func(path string) bool {
		lower := strings.ToLower(path)
		processedMutex.Lock()
		defer processedMutex.Unlock()
		if processedFiles[lower] {
			return false
		}
		processedFiles[lower] = true
		return true
	}

	for i, path := range paths {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()

			if strings.HasSuffix(p, "/") {
				filepath.WalkDir(filepath.Join(workDir, p), func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() && claim(path) {
						if result := processFile(path); result != "" {
							perPath[i] = append(perPath[i], result)
						}
					}
					return nil
				})
				return
			}

			fullPath := filepath.Join(workDir, p)
			if claim(fullPath) {
				if result := processFile(fullPath); result != "" {
					perPath[i] = append(perPath[i], result)
				}
			}
		}(i, path)
	}

	wg.Wait()

	results := make([]string, 0, len(paths))
	for _, group := range perPath {
		results = append(results, group...)
	}
	return strings.Join(results, "\n")
}

func processFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return "# From:" + filePath + "\n" + string(content)
}

// withoutAgentsFile copies paths minus AGENTS.md. A copy, not a filter in
// place: cfg.ContextPaths is shared, and mutating it here would silently
// disable the file for the primary root too.
func withoutAgentsFile(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == config.AgentsFile {
			continue
		}
		out = append(out, p)
	}
	return out
}
