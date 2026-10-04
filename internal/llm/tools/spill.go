package tools

// GORILLA OVERRIDE (2026-10-04): what a cap cuts off is kept, not thrown away.
//
// # THE PROBLEM
//
// Two caps protect the conversation from an oversized tool result: bash keeps
// the first and last 15 KB of output, and clampToolContent keeps the first
// 400 KB of anything. Both say that they cut. Neither kept the part they cut.
//
// So the model was told "the result is incomplete — narrow the request" about
// output that no longer existed. For a search that is fair: run it again,
// narrower. For a forty-minute build it is not. The error was in the middle,
// the middle is gone, and the only way to see it is to build again.
//
// # WHAT THIS DOES
//
// Before a cap cuts, the WHOLE text is written to a file under the program's
// state directory, and the notice names that file. The model can then read the
// part it needs with `view` (offset and limit) or search it with `find`, at the
// cost of the lines it asks for rather than of the whole output.
//
// The idea is goose's large-response handler (aaif-goose/goose,
// crates/goose/src/agents/large_response_handler.rs, Apache-2.0) and DeepSeek
// Harness's spill packages (deepseek-ai/deepseek-harness, packages/spill, MIT).
// Both REPLACE the result with a file path. This keeps the bounded excerpt as
// well, because a small model that is handed only a path often stops there.
//
// # WHAT IT DELIBERATELY DOES NOT DO
//
// It does not raise any cap, and it never fails a tool call. If the file cannot
// be written the old notice is used unchanged.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

const (
	// spillKeep is how many spilled outputs are kept. Oldest are removed first.
	spillKeep = 40
	// spillMaxBytes bounds one spilled file. Output past this is not a log
	// anyone will page through, and the disk is the user's, not ours to fill.
	spillMaxBytes = 64 * 1024 * 1024
)

// spillDirOverride lets tests redirect spilled output away from the real
// state directory.
var spillDirOverride string

// SpillDir is where oversized tool output is kept.
func SpillDir() string {
	if spillDirOverride != "" {
		return spillDirOverride
	}
	return filepath.Join(config.StateBase(), "tool-output")
}

// spillOutput writes content to a new file and returns its path, or "" if it
// could not. It never returns an error: losing the overflow is the behaviour
// this replaced, so falling back to it costs nothing new.
func spillOutput(label, content string) string {
	dir := SpillDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return ""
	}
	name := fmt.Sprintf("%s-%s-%s.txt", time.Now().Format("20060102-150405"),
		sanitiseSpillLabel(label), hex.EncodeToString(suffix[:]))
	path := filepath.Join(dir, name)

	if len(content) > spillMaxBytes {
		content = content[:spillMaxBytes]
	}
	// 0600: the output of a command can contain whatever that command printed.
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return ""
	}
	pruneSpillDir(dir)
	return path
}

func sanitiseSpillLabel(label string) string {
	out := make([]byte, 0, len(label))
	for i := 0; i < len(label) && len(out) < 24; i++ {
		c := label[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "output"
	}
	return string(out)
}

// pruneSpillDir keeps the newest spillKeep files. Names begin with a timestamp,
// so sorting by name is sorting by age.
func pruneSpillDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".txt" {
			names = append(names, e.Name())
		}
	}
	if len(names) <= spillKeep {
		return
	}
	sort.Strings(names)
	for _, n := range names[:len(names)-spillKeep] {
		_ = os.Remove(filepath.Join(dir, n))
	}
}

// spillNotice is the sentence appended to a truncation notice. Empty when
// nothing was saved, so the caller's own notice stands alone.
func spillNotice(path string) string {
	if path == "" {
		return ""
	}
	// GORILLA FIX (2026-10-04): hand over the call, do not describe it.
	//
	// Measured twice with a real model (gemma-4-e2b). Given "read the part you
	// need with view or search it with find", it called neither. The first time
	// it reported the line it was asked for as absent; the second time it
	// invented one. A small model carries out a call it is given and does not
	// compose one from a description — the reason every Fieldkit answer ends in
	// a NEXT: line. The path is JSON-encoded here so the suggested call is valid
	// as written: a Windows path pasted with single backslashes is the exact
	// malformed argument toolinput.go exists to repair.
	call, err := json.Marshal(map[string]string{"query": "the text you are looking for", "path": path})
	if err != nil {
		return fmt.Sprintf(" The COMPLETE output is saved at %s .", path)
	}
	return fmt.Sprintf(" The COMPLETE output is saved at %s and can be searched. Do not answer from the "+
		"part shown above and do not re-run the command.\nNEXT: call the find tool with %s", path, call)
}
