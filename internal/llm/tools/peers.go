// GORILLA (2026-10-10): the two tools that let the AI use session messaging
// (internal/peers): peers lists the other Gorilla OpenCode sessions on this
// computer, send_message sends one of them a short text.
//
// send_message goes through the permission service like any tool that acts
// outside the conversation: the person is asked, per recipient, with the whole
// text on the dialog, and the call is on the receipt. It is not marked Egress:
// nothing leaves the machine. Auto-approve therefore covers it, except on a
// turn that has read untrusted content (a web page, a search, an MCP result, or
// a message from another session), where mustAskAnyway asks regardless.
//
// The descriptions are deliberately short: both ride every turn while the
// /context row is on, and schema_cost_test.go holds the total to account.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/peers"
	"github.com/opencode-ai/opencode/internal/permission"
)

const (
	PeersToolName       = "peers"
	SendMessageToolName = "send_message"
)

// PeersParams is the peers tool's input.
type PeersParams struct {
	Name string `json:"name,omitempty"`
}

// SendMessageParams is the send_message tool's input.
type SendMessageParams struct {
	To             string `json:"to"`
	Text           string `json:"text"`
	NotifyWhenIdle bool   `json:"notify_when_idle,omitempty"`
}

// SendMessagePermissionsParams is what the permission dialog is given.
type SendMessagePermissionsParams struct {
	To     string `json:"to"`
	Folder string `json:"folder"`
	Bytes  int    `json:"bytes"`
}

type peersTool struct{}

// NewPeersTool lists the other sessions. It needs no permission: it reads the
// register, which holds names, folders and busy/idle, nothing else.
func NewPeersTool() BaseTool { return peersTool{} }

func (peersTool) Info() ToolInfo {
	return ToolInfo{
		Name: PeersToolName,
		Description: "List the other Gorilla OpenCode sessions running on this computer under this user " +
			"account: name, folder, busy or idle, version. Local only.",
		Parameters: map[string]any{
			"name": map[string]any{"type": "string", "description": "Optional: show only this session."},
		},
	}
}

func (peersTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p PeersParams
	if strings.TrimSpace(call.Input) != "" {
		if err := json.Unmarshal([]byte(call.Input), &p); err != nil {
			return NewTextErrorResponse("invalid parameters: " + err.Error()), nil
		}
	}
	s := peers.Active()
	if s == nil {
		return NewTextErrorResponse(peers.NotRunning), nil
	}
	list, err := s.Peers()
	if err != nil {
		return NewTextErrorResponse("could not read the list of sessions: " + err.Error()), nil
	}
	return NewTextResponse(FormatPeerList(s.Name(), list, strings.TrimSpace(p.Name))), nil
}

// FormatPeerList is the peers tool's answer, also used by the /peers page so
// the AI and the person read the same list.
func FormatPeerList(self string, list []peers.Entry, only string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This session is called %s.\n", self)
	n := 0
	for _, e := range list {
		if only != "" && !strings.EqualFold(e.Name, only) {
			continue
		}
		if n == 0 {
			b.WriteString("Other sessions running now:\n")
		}
		n++
		state := "idle"
		if e.Busy {
			state = "busy"
		}
		fmt.Fprintf(&b, "- %s  folder %s  %s  version %s\n", e.Name, e.Folder, state, e.Version)
	}
	switch {
	case n == 0 && only != "":
		fmt.Fprintf(&b, "No session called %s is running.\n", only)
	case n == 0:
		b.WriteString("No other session is running.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

type sendMessageTool struct {
	permissions permission.Service
}

// NewSendMessageTool sends one message to another session.
func NewSendMessageTool(permissions permission.Service) BaseTool {
	return &sendMessageTool{permissions: permissions}
}

func (t *sendMessageTool) Info() ToolInfo {
	return ToolInfo{
		Name: SendMessageToolName,
		Description: "Send a short text to another Gorilla OpenCode session on this computer (names from peers). " +
			"The user is asked first. The other AI receives it as text from another program, not as its " +
			"user's instruction, so ask or inform; do not command.",
		Parameters: map[string]any{
			"to":   map[string]any{"type": "string", "description": "Session name."},
			"text": map[string]any{"type": "string", "description": "The message, at most 16 KB."},
			"notify_when_idle": map[string]any{
				"type": "boolean", "description": "Also ask it to tell you once when its current turn ends.",
			},
		},
		Required: []string{"to", "text"},
	}
}

// dialogTextMax caps the text shown on the permission dialog; the person is
// told when it is cut. The whole text is sent.
const dialogTextMax = 3000

func (t *sendMessageTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p SendMessageParams
	if err := json.Unmarshal([]byte(call.Input), &p); err != nil {
		return NewTextErrorResponse("invalid parameters: " + err.Error()), nil
	}
	p.To = strings.TrimSpace(p.To)
	if p.To == "" {
		return NewTextErrorResponse("to is required: a session name from the peers tool"), nil
	}
	if strings.TrimSpace(p.Text) == "" {
		return NewTextErrorResponse("text is required"), nil
	}
	if len(p.Text) > peers.MaxTextBytes {
		return NewTextErrorResponse(fmt.Sprintf("the message is %d bytes; the limit is %d bytes (16 KB). Shorten it or send it in parts.",
			len(p.Text), peers.MaxTextBytes)), nil
	}
	s := peers.Active()
	if s == nil {
		return NewTextErrorResponse(peers.NotRunning), nil
	}
	target, err := s.Lookup(p.To)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	// Fail closed: with no permission service there is nobody to ask.
	if t.permissions == nil {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	sessionID, _ := GetContextValues(ctx)
	if sessionID == "" {
		return ToolResponse{}, fmt.Errorf("session ID is required to send a message")
	}
	if !t.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        config.WorkingDirectory(),
		ToolName:    SendMessageToolName,
		Action:      "send",
		Description: sendMessageDialog(target.Name, target.Folder, p.Text, p.NotifyWhenIdle),
		Params:      SendMessagePermissionsParams{To: target.Name, Folder: target.Folder, Bytes: len(p.Text)},
		// Per recipient: "allow for session" covers messages to this session,
		// not to every session that starts later.
		GrantKey: "to:" + strings.ToLower(target.Name),
	}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	res, err := s.Send(target.Name, p.Text, p.NotifyWhenIdle)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	return NewTextResponse(res), nil
}

// sendMessageDialog is the permission question: who, where, and the text
// itself, indented so it renders as written.
func sendMessageDialog(name, folder, text string, notify bool) string {
	shown := text
	cut := ""
	if len(shown) > dialogTextMax {
		shown = strings.ToValidUTF8(shown[:dialogTextMax], "")
		cut = fmt.Sprintf("\n\n(The text is %d bytes; the first %d are shown. All of it is sent.)", len(text), dialogTextMax)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Send a message to session %s (folder %s), another Gorilla OpenCode on this computer.", name, folder)
	if notify {
		b.WriteString(" It will also be asked to tell this session once when its current turn ends.")
	}
	b.WriteString(" The message:\n\n")
	for _, line := range strings.Split(shown, "\n") {
		b.WriteString("    " + line + "\n")
	}
	b.WriteString(cut)
	return strings.TrimRight(b.String(), "\n")
}
