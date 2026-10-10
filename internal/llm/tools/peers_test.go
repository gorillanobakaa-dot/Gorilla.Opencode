package tools

// GORILLA (2026-10-10): the peers and send_message tools. send_message asks
// the person first, per recipient, with the whole text on the dialog; a refusal
// sends nothing; an oversized text is refused before anyone is asked; with
// messaging not running both tools say why instead of failing obscurely.

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/peers"
	"github.com/opencode-ai/opencode/internal/permission"
)

// recordingPerms answers every permission question with answer and keeps the
// questions. Only Request is used by the tool; the embedded interface is nil.
type recordingPerms struct {
	permission.Service
	answer bool
	asked  []permission.CreatePermissionRequest
}

func (r *recordingPerms) Request(o permission.CreatePermissionRequest) bool {
	r.asked = append(r.asked, o)
	return r.answer
}

func peerPair(t *testing.T) (a, b *peers.Session) {
	t.Helper()
	dir := t.TempDir()
	start := func(name, tag string) *peers.Session {
		s, err := peers.Start(peers.Options{Dir: dir, Folder: func() string { return filepath.Join(dir, name) }, Version: "t", Tag: tag})
		if err != nil {
			t.Fatalf("peers.Start(%s): %v", name, err)
		}
		t.Cleanup(s.Close)
		return s
	}
	a = start("alpha", "a")
	b = start("beta", "b")
	peers.SetActive(a)
	t.Cleanup(func() { peers.SetActive(nil) })
	return a, b
}

func sendInput(t *testing.T, to, text string, notify bool) ToolCall {
	t.Helper()
	data, err := json.Marshal(SendMessageParams{To: to, Text: text, NotifyWhenIdle: notify})
	if err != nil {
		t.Fatal(err)
	}
	return ToolCall{ID: "c1", Name: SendMessageToolName, Input: string(data)}
}

func sessionCtx() context.Context {
	return context.WithValue(context.Background(), SessionIDContextKey, "tool-session")
}

func TestPeerToolsSayWhyWhenMessagingIsNotRunning(t *testing.T) {
	peers.SetActive(nil)
	resp, err := NewPeersTool().Run(sessionCtx(), ToolCall{Input: "{}"})
	if err != nil || !resp.IsError || !strings.Contains(resp.Content, "/context") {
		t.Errorf("peers: %+v, %v", resp, err)
	}
	perms := &recordingPerms{answer: true}
	resp, err = NewSendMessageTool(perms).Run(sessionCtx(), sendInput(t, "beta", "hi", false))
	if err != nil || !resp.IsError || !strings.Contains(resp.Content, "not running") {
		t.Errorf("send_message: %+v, %v", resp, err)
	}
	if len(perms.asked) != 0 {
		t.Error("the person was asked about a message that could not be sent")
	}
}

func TestPeersToolListsTheOtherSessions(t *testing.T) {
	peerPair(t)
	resp, err := NewPeersTool().Run(sessionCtx(), ToolCall{Input: "{}"})
	if err != nil || resp.IsError {
		t.Fatalf("peers: %+v, %v", resp, err)
	}
	for _, want := range []string{"This session is called alpha", "- beta", "idle", "version t"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("list lacks %q:\n%s", want, resp.Content)
		}
	}
	resp, _ = NewPeersTool().Run(sessionCtx(), ToolCall{Input: `{"name":"gamma"}`})
	if !strings.Contains(resp.Content, "No session called gamma") {
		t.Errorf("filter for a missing name:\n%s", resp.Content)
	}
}

func TestSendMessageAsksFirstAndSendsOnlyOnYes(t *testing.T) {
	_, b := peerPair(t)

	no := &recordingPerms{answer: false}
	_, err := NewSendMessageTool(no).Run(sessionCtx(), sendInput(t, "beta", "may I start calc.exe?", false))
	if !errors.Is(err, permission.ErrorPermissionDenied) {
		t.Fatalf("a refused send returned %v, want a permission refusal", err)
	}
	if b.Pending() != 0 {
		t.Fatal("the message was sent although the person said no")
	}
	if len(no.asked) != 1 {
		t.Fatalf("asked %d times, want 1", len(no.asked))
	}
	q := no.asked[0]
	if q.ToolName != SendMessageToolName || q.SessionID != "tool-session" || q.Egress {
		t.Errorf("question %+v", q)
	}
	if q.GrantKey != "to:beta" {
		t.Errorf("grant key %q; a grant must cover this recipient only", q.GrantKey)
	}
	for _, want := range []string{"Send a message to session beta", "may I start calc.exe?"} {
		if !strings.Contains(q.Description, want) {
			t.Errorf("the dialog does not show %q:\n%s", want, q.Description)
		}
	}

	yes := &recordingPerms{answer: true}
	resp, err := NewSendMessageTool(yes).Run(sessionCtx(), sendInput(t, "beta", "may I start calc.exe?", true))
	if err != nil || resp.IsError {
		t.Fatalf("approved send: %+v, %v", resp, err)
	}
	if !strings.Contains(resp.Content, "Delivered to session beta") {
		t.Errorf("result %q", resp.Content)
	}
	if !strings.Contains(yes.asked[0].Description, "tell this session once") {
		t.Errorf("the dialog does not mention the idle notice:\n%s", yes.asked[0].Description)
	}
	msgs := b.TakeForAI()
	if len(msgs) != 1 || msgs[0].Text != "may I start calc.exe?" {
		t.Errorf("beta received %+v", msgs)
	}
}

func TestSendMessageRefusesOversizedTextBeforeAsking(t *testing.T) {
	peerPair(t)
	perms := &recordingPerms{answer: true}
	resp, err := NewSendMessageTool(perms).Run(sessionCtx(), sendInput(t, "beta", strings.Repeat("x", peers.MaxTextBytes+1), false))
	if err != nil || !resp.IsError || !strings.Contains(resp.Content, "16 KB") {
		t.Errorf("oversized: %+v, %v", resp, err)
	}
	if len(perms.asked) != 0 {
		t.Error("the person was asked to approve a message that would be refused anyway")
	}
	resp, _ = NewSendMessageTool(perms).Run(sessionCtx(), sendInput(t, "nobody", "hi", false))
	if !resp.IsError || !strings.Contains(resp.Content, "beta") {
		t.Errorf("unknown recipient: %+v", resp)
	}
}

// With no permission service there is nobody to ask, so nothing is sent.
func TestSendMessageWithoutAPermissionServiceRefuses(t *testing.T) {
	_, b := peerPair(t)
	if _, err := NewSendMessageTool(nil).Run(sessionCtx(), sendInput(t, "beta", "hi", false)); !errors.Is(err, permission.ErrorPermissionDenied) {
		t.Errorf("err %v, want a refusal", err)
	}
	if b.Pending() != 0 {
		t.Error("sent without asking anyone")
	}
}

func TestSendMessageDialogShowsCutTextHonestly(t *testing.T) {
	d := sendMessageDialog("beta", "/srv/beta", strings.Repeat("y", dialogTextMax+10), false)
	if !strings.Contains(d, "All of it is sent") {
		t.Errorf("a cut dialog does not say the whole text is sent:\n%s", d[len(d)-200:])
	}
}
