package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

type ToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
}

// UnknownParams returns the parameter names a tool call sent that the tool does
// not declare, sorted, or nil when every name is known.
//
// GORILLA FIX (2026-09-02): a parameter the tool has never heard of used to be
// dropped in silence.
//
// Observed live: Gemma 4 called view with {"file_path": "<a directory>",
// "view": "tree"}. There is no "view" parameter -- the tool takes file_path,
// offset and limit -- so it was discarded without a word. The only feedback the
// model got was "path is a directory", and a turn later it was still theorising
// about the parameter it had invented: "or the tool defaults to reading a single
// file if no specific file is named".
//
// It had no way to learn otherwise. json.Unmarshal ignores unknown fields, so a
// malformed call and a correct one are indistinguishable from the model's side.
//
// This is reported rather than rejected. A hard error would teach faster but a
// model that keeps attaching a harmless field would then loop forever, unable to
// complete a call that would otherwise have worked. Doing the work and saying
// what was ignored costs one line and cannot deadlock.
func UnknownParams(info ToolInfo, rawInput string) []string {
	if len(info.Parameters) == 0 || rawInput == "" {
		// A tool that declares no schema cannot judge what is foreign to it.
		return nil
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(rawInput), &sent); err != nil {
		// Malformed JSON is the tool's own problem to report; guessing at
		// parameter names inside something that does not parse would only
		// add noise to whatever error it is about to produce.
		return nil
	}
	var unknown []string
	for name := range sent {
		if _, declared := info.Parameters[name]; !declared {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// DeclaredParams lists a tool's parameter names, sorted, for telling a model
// what it should have sent.
func DeclaredParams(info ToolInfo) []string {
	names := make([]string, 0, len(info.Parameters))
	for name := range info.Parameters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type toolResponseType string

type (
	sessionIDContextKey string
	messageIDContextKey string
)

const (
	ToolResponseTypeText  toolResponseType = "text"
	ToolResponseTypeImage toolResponseType = "image"

	SessionIDContextKey sessionIDContextKey = "session_id"
	MessageIDContextKey messageIDContextKey = "message_id"
)

type ToolResponse struct {
	Type     toolResponseType `json:"type"`
	Content  string           `json:"content"`
	Metadata string           `json:"metadata,omitempty"`
	IsError  bool             `json:"is_error"`
}

// MaxToolResponseBytes is the LAST line of defence on what a tool may put into
// the conversation.
//
// GORILLA FIX (2026-07-30). Every tool result is appended to the message
// history and re-sent on every subsequent turn, so an unbounded tool result is
// an unbounded, recurring bill. The grep tool proved it: it capped matches at
// 100, honestly reported truncated=true, and returned 2,438,026 bytes, taking a
// conversation from 15.9K tokens to 675K in a single turn.
//
// The lesson generalises past that one tool. A limit must be expressed in the
// unit of the resource being protected. Grep counted MATCHES; the resource was
// BYTES. Counting items is a proxy, and proxies fail exactly when items are
// unusual — which is when the limit was needed. Per-tool caps kept missing this
// because each one measured whatever was natural to that tool: files, lines,
// matches, seconds.
//
// So the bound lives HERE, at the single point every tool passes through,
// rather than in twelve places that each have to remember. It is deliberately
// generous — it is a backstop, not a policy. Tools that need tighter limits
// (bash: 30 KB, ls: 1000 files, view: 2000 lines) keep them. This only catches
// the tool nobody thought about, including ones not written yet.
//
// Sized against real use: a 2000-line source file at ~150 bytes a line is
// ~300 KB, which must still work. view's own worst case (2000 lines x 2000
// chars = 4 MB) is exactly the kind of thing this exists to stop.
const MaxToolResponseBytes = 400 * 1024

// clampToolContent bounds a tool result and SAYS it did.
//
// Silent truncation is worse than the overflow: a model given a cut-off result
// with no marker reasons about the fragment as though it were complete, and
// then reports a conclusion drawn from half the evidence.
func clampToolContent(content string) string {
	if len(content) <= MaxToolResponseBytes {
		return content
	}
	return content[:MaxToolResponseBytes] + fmt.Sprintf(
		"\n\n[TRUNCATED: this tool returned %d bytes; %d were kept. "+
			"The result is incomplete — narrow the request rather than "+
			"drawing conclusions from this fragment.]",
		len(content), MaxToolResponseBytes)
}

func NewTextResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    ToolResponseTypeText,
		Content: clampToolContent(content),
	}
}

func WithResponseMetadata(response ToolResponse, metadata any) ToolResponse {
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return response
		}
		response.Metadata = string(metadataBytes)
	}
	return response
}

func NewTextErrorResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    ToolResponseTypeText,
		Content: clampToolContent(content),
		IsError: true,
	}
}

type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

type BaseTool interface {
	Info() ToolInfo
	Run(ctx context.Context, params ToolCall) (ToolResponse, error)
}

func GetContextValues(ctx context.Context) (string, string) {
	sessionID := ctx.Value(SessionIDContextKey)
	messageID := ctx.Value(MessageIDContextKey)
	if sessionID == nil {
		return "", ""
	}
	if messageID == nil {
		return sessionID.(string), ""
	}
	return sessionID.(string), messageID.(string)
}
