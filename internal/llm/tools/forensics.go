package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/forensics"
	"github.com/opencode-ai/opencode/internal/permission"
)

// GORILLA (2026-10-21): binary_inspect and archive, phase 1 of the binary and
// forensics toolkit. Windows and macOS 10.13 have no readelf, objdump, nm,
// strings or unzip, so the model was half blind there; both tools are pure Go
// (internal/forensics), add no door (no network, no process, no download) and
// read without running anything. Descriptions are kept short on purpose: they
// ride every turn, and the bar is Gemma 4 E2B on CPU.

const (
	BinaryInspectToolName = "binary_inspect"
	ArchiveToolName       = "archive"
)

type binaryInspectParams struct {
	Path   string `json:"path"`
	Part   string `json:"part"`
	Filter string `json:"filter"`
	MinLen int    `json:"min_len"`
}

type binaryInspectTool struct{}

// NewBinaryInspectTool builds the read-only executable inspector.
func NewBinaryInspectTool() BaseTool { return binaryInspectTool{} }

func (binaryInspectTool) Info() ToolInfo {
	return ToolInfo{
		Name: BinaryInspectToolName,
		Description: "Inspect an executable or library without running it: ELF, PE (.exe/.dll), Mach-O (fat too). " +
			"Default summary: arch, type, needed libraries, rpath, signature, sections with entropy, " +
			"Mach-O minimum macOS with a 10.13 check. Lists are capped and say so. Read-only.",
		Parameters: map[string]any{
			"path": map[string]any{"type": "string", "description": "File to inspect"},
			"part": map[string]any{
				"type": "string", "enum": []string{"summary", "sections", "imports", "symbols", "strings"},
				"description": "What to list (default summary)",
			},
			"filter":  map[string]any{"type": "string", "description": "Only lines containing this text"},
			"min_len": map[string]any{"type": "integer", "description": "Shortest string for part=strings (default 6)"},
		},
		Required: []string{"path"},
	}
}

func (binaryInspectTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p binaryInspectParams
	if err := UnmarshalToolInput(call.Input, &p); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	path, refusal := forensicsInputPath(p.Path)
	if refusal != "" {
		return NewTextErrorResponse(refusal), nil
	}
	out, err := forensics.Inspect(path, forensics.Options{Part: p.Part, Filter: p.Filter, MinLen: p.MinLen})
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if p.Part == forensics.PartStrings {
		// Strings are whatever the binary's author put there, including text
		// addressed to a model. Fenced like a web page.
		return NewUntrustedTextResponse("binary strings", path, "", out), nil
	}
	return NewTextResponse(out), nil
}

type archiveParams struct {
	Path    string `json:"path"`
	Action  string `json:"action"`
	Pattern string `json:"pattern"`
	Entry   string `json:"entry"`
	Dest    string `json:"dest"`
}

type archiveTool struct {
	permissions permission.Service
}

// NewArchiveTool builds the archive lister/extractor. Extraction asks first.
func NewArchiveTool(permissions permission.Service) BaseTool {
	return archiveTool{permissions: permissions}
}

func (archiveTool) Info() ToolInfo {
	return ToolInfo{
		Name: ArchiveToolName,
		Description: "List, find or extract ONE file from a zip, jar, xpi, omni.ja, tar, tar.gz, tar.bz2, gz or bz2 " +
			"without unpacking it all. extract writes into the project only (default extracted/<archive>), " +
			"asks first, never overwrites, refuses names that climb out.",
		Parameters: map[string]any{
			"path": map[string]any{"type": "string", "description": "Archive file"},
			"action": map[string]any{
				"type": "string", "enum": []string{"list", "find", "extract"},
				"description": "Default list",
			},
			"pattern": map[string]any{"type": "string", "description": "find: glob (*.js) or text"},
			"entry":   map[string]any{"type": "string", "description": "extract: exact entry name"},
			"dest":    map[string]any{"type": "string", "description": "extract: folder inside the project"},
		},
		Required: []string{"path"},
	}
}

func (t archiveTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p archiveParams
	if err := UnmarshalToolInput(call.Input, &p); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	path, refusal := forensicsInputPath(p.Path)
	if refusal != "" {
		return NewTextErrorResponse(refusal), nil
	}
	switch strings.ToLower(strings.TrimSpace(p.Action)) {
	case "", "list":
		return archiveText(forensics.ListArchive(path, ""))
	case "find":
		if strings.TrimSpace(p.Pattern) == "" {
			return NewTextErrorResponse("find needs a pattern, e.g. *.js or a word in the name"), nil
		}
		return archiveText(forensics.ListArchive(path, p.Pattern))
	case "extract":
		return t.extract(ctx, path, p)
	}
	return NewTextErrorResponse(fmt.Sprintf("unknown action %q: use list, find or extract", p.Action)), nil
}

func archiveText(out string, err error) (ToolResponse, error) {
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	return NewTextResponse(out), nil
}

func (t archiveTool) extract(ctx context.Context, archivePath string, p archiveParams) (ToolResponse, error) {
	if strings.TrimSpace(p.Entry) == "" {
		return NewTextErrorResponse("extract needs entry: the exact name from action=list or find"), nil
	}
	rel, err := forensics.SafeEntryPath(p.Entry)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	dest := strings.TrimSpace(p.Dest)
	if dest == "" {
		dest = filepath.Join("extracted", filepath.Base(archivePath))
	}
	resolved, inside, err := ResolveWriteTarget(dest)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if !inside {
		return NewTextErrorResponse(fmt.Sprintf(
			"refused: %s is outside the project; extraction writes only inside %s", resolved, config.WorkingDirectory())), nil
	}
	target := filepath.Join(resolved, filepath.FromSlash(rel))
	if _, err := os.Lstat(target); err == nil {
		return NewTextErrorResponse(fmt.Sprintf("refused: %s already exists; nothing is overwritten", target)), nil
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}
	if !t.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        permissionScope(target),
		ToolName:    ArchiveToolName,
		GrantKey:    target,
		Action:      "write",
		Description: fmt.Sprintf("Extract %s from %s to %s", rel, filepath.Base(archivePath), DescribeWriteTarget(target)),
		Params:      map[string]string{"archive": archivePath, "entry": p.Entry, "dest": target},
	}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	// Created only after permission: a denied extraction leaves no folder behind.
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	return archiveText(forensics.ExtractEntry(archivePath, p.Entry, resolved, forensics.DefaultMaxCopy))
}

// forensicsInputPath resolves a path to read and applies the credential guard
// every reading tool uses.
func forensicsInputPath(p string) (string, string) {
	if strings.TrimSpace(p) == "" {
		return "", "path is required"
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(config.WorkingDirectory(), p)
	}
	if why := RefuseSensitiveRead(p); why != "" {
		return "", why
	}
	return p, ""
}
