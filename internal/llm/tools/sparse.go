// GORILLA OVERRIDE: this file did not exist upstream. `sparse` is the Linux
// kernel's own semantic checker, originally by Linus Torvalds. It catches a
// class of bug no language server sees, because the checks encode kernel
// conventions rather than C semantics:
//
//   - __user / __kernel address-space confusion (dereferencing user pointers)
//   - endianness violations (__le32 assigned from a __be32)
//   - lock imbalance (a path that acquires and returns without releasing)
//   - __init / __exit section misuse
//   - bitfield truncation
//
// clangd will happily compile all of those. sparse will not. Exposing it as a
// tool lets the agent check its own kernel edit BEFORE yielding, instead of the
// user discovering it after a `make C=1` or, worse, at runtime.
//
// Default OFF in the loadout — only useful on kernel work, and its schema
// should not ride every request for a user who never touches the kernel.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
)

type SparseParams struct {
	FilePath string   `json:"file_path"`
	Includes []string `json:"includes,omitempty"`
	Defines  []string `json:"defines,omitempty"`
}

type SparseResponseMetadata struct {
	Warnings int    `json:"warnings"`
	Errors   int    `json:"errors"`
	Clean    bool   `json:"clean"`
	Binary   string `json:"binary"`
}

type sparseTool struct {
	permissions permission.Service
}

const (
	SparseToolName = "sparse"
	// Kept deliberately short. This tool's schema rides every request when the
	// loadout row is on, so the description states when to reach for it and what
	// it uniquely catches, and stops.
	sparseToolDescription = `Run sparse, the Linux kernel's semantic checker, on one C file.

WHEN TO USE:
- After editing kernel C code, before reporting the work done.
- Catches what a compiler and clangd do NOT: __user/__kernel pointer confusion,
  endianness violations (__le32 vs __be32), lock imbalance across return paths,
  __init/__exit section misuse, bitfield truncation.

HOW TO USE:
- file_path: absolute path to a .c or .h file inside the kernel tree.
- includes: optional extra -I paths, when the file needs headers sparse cannot
  find from the tree layout alone.
- defines: optional extra -D values.

LIMITATIONS:
- Kernel C only. Meaningless on userspace code, Rust, or non-C files.
- Warnings on a file you did not edit are usually pre-existing kernel style,
  not bugs you introduced. Read the line numbers against your own change.
- Not a substitute for building. It parses; it does not link or run.`
)

// sparseTimeout bounds a single file check. sparse is fast (sub-second on most
// files) but a pathological macro expansion can hang; a stuck tool call is worse
// than a missing check.
const sparseTimeout = 45 * time.Second

func NewSparseTool(permissions permission.Service) BaseTool {
	return &sparseTool{permissions: permissions}
}

func (s *sparseTool) Info() ToolInfo {
	return ToolInfo{
		Name:        SparseToolName,
		Description: sparseToolDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the C source or header file to check",
			},
			"includes": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional additional include directories (-I)",
			},
			"defines": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional additional preprocessor defines (-D), e.g. __KERNEL__",
			},
		},
		Required: []string{"file_path"},
	}
}

func (s *sparseTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params SparseParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("failed to parse sparse parameters: " + err.Error()), nil
	}
	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	// sparse is a binary the user may not have installed. Say so plainly with
	// the fix, rather than returning an opaque exec error.
	binary, err := exec.LookPath("sparse")
	if err != nil {
		return NewTextErrorResponse(
			"sparse is not installed. On Debian/Ubuntu: sudo apt install sparse\n" +
				"See readme.before.compiling.md in the kernel work directory."), nil
	}

	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(config.WorkingDirectory(), filePath)
	}
	if _, err := os.Stat(filePath); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("cannot read %s: %v", filePath, err)), nil
	}

	// Permission gate. sparse only reads the file and writes nothing, but it
	// executes a binary and can take real time on a big translation unit, so it
	// follows the same consent rule as every other tool that spawns a process.
	sessionID, _ := GetContextValues(ctx)
	if sessionID == "" {
		return ToolResponse{}, fmt.Errorf("session ID is required to run sparse")
	}
	if s.permissions != nil {
		granted := s.permissions.Request(permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        permissionScope(filePath),
			ToolName:    SparseToolName,
			Action:      "execute",
			Description: fmt.Sprintf("Run sparse semantic checks on %s", filePath),
			Params:      params,
		})
		if !granted {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	args := []string{}
	// Kernel files assume __KERNEL__ and the tree's include layout. Callers can
	// add more, but these two make the common case work without every call
	// having to spell them out.
	args = append(args, "-D__KERNEL__")
	if root, ok := config.RootFor(filePath); ok {
		args = append(args, "-I"+filepath.Join(root, "include"))
		args = append(args, "-I"+filepath.Join(root, "arch", "x86", "include"))
	}
	for _, inc := range params.Includes {
		args = append(args, "-I"+inc)
	}
	for _, def := range params.Defines {
		args = append(args, "-D"+def)
	}
	args = append(args, filePath)

	runCtx, cancel := context.WithTimeout(ctx, sparseTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, binary, args...)
	// sparse writes its findings to stderr and nothing useful to stdout.
	out, runErr := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))

	if runCtx.Err() == context.DeadlineExceeded {
		return NewTextErrorResponse(fmt.Sprintf(
			"sparse timed out after %s on %s — likely a macro expansion it cannot parse. Skip this file.",
			sparseTimeout, filePath)), nil
	}

	warnings := strings.Count(text, ": warning:")
	errors := strings.Count(text, ": error:")

	if text == "" {
		// A non-zero exit with no output means sparse itself failed to run.
		if runErr != nil {
			return NewTextErrorResponse(fmt.Sprintf("sparse failed to run: %v", runErr)), nil
		}
		return WithResponseMetadata(
			NewTextResponse(fmt.Sprintf("sparse: clean, no findings for %s", filePath)),
			SparseResponseMetadata{Clean: true, Binary: binary},
		), nil
	}

	// A non-zero exit code with output is the NORMAL case — sparse exits non-zero
	// when it finds anything. Do not treat that as a tool error; the findings ARE
	// the result.
	header := fmt.Sprintf("sparse on %s — %d warning(s), %d error(s)\n\n", filePath, warnings, errors)
	return WithResponseMetadata(
		NewTextResponse(header+text),
		SparseResponseMetadata{Warnings: warnings, Errors: errors, Binary: binary},
	), nil
}
