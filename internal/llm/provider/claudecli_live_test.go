package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

// Live tests against the real, signed-in `claude` on this machine. Everything
// else in claudecli_test.go replays a recording; only this turns inference into
// fact. Gated because it spends the developer's subscription window and needs an
// account, so it must never run in CI or on a contributor's clone.
//
//	CLAUDECLI_LIVE=1 go test ./internal/llm/provider/ -run TestClaudeCLILive -v
//
// Haiku is used throughout: it is the cheapest model against the five-hour
// window, which is the resource this route actually runs out of.
func liveCLIClient(t *testing.T, model models.ModelID) *claudeCLIClient {
	t.Helper()
	if os.Getenv("CLAUDECLI_LIVE") != "1" {
		t.Skip("set CLAUDECLI_LIVE=1 to run against the real claude CLI")
	}
	if !ClaudeCLIAvailable() {
		t.Skip("the claude program is not on PATH")
	}
	c := newClaudeCLIClient(providerClientOptions{
		model:         models.ClaudeCLIModels[model],
		systemMessage: "You are a terse test agent. Answer in as few words as possible.",
	})
	return c.(*claudeCLIClient)
}

// Proves the whole path: a real child process, a real subscription, real
// streaming. Asserting that deltas ARRIVED matters as much as the text, because
// the answer is also present on the result line and a provider that never
// streams still produces the right final string.
func TestClaudeCLILiveText(t *testing.T) {
	c := liveCLIClient(t, models.ClaudeCLIHaiku)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var deltas int
	var final *ProviderResponse
	for ev := range c.stream(ctx, []message.Message{cliUserMsg("live-1", "Reply with exactly: PONG")}, nil) {
		switch ev.Type {
		case EventContentDelta:
			deltas++
		case EventComplete:
			final = ev.Response
		case EventError:
			t.Fatalf("live stream failed: %v", ev.Error)
		}
	}
	if final == nil {
		t.Fatal("no EventComplete")
	}
	if !strings.Contains(strings.ToUpper(final.Content), "PONG") {
		t.Errorf("content = %q, expected it to contain PONG", final.Content)
	}
	if deltas == 0 {
		t.Error("the answer arrived without any streamed deltas")
	}
	if final.Usage.OutputTokens == 0 {
		t.Error("no usage was reported, so this turn would be invisible in the receipt")
	}
	t.Logf("content=%q deltas=%d usage=%+v finish=%s",
		final.Content, deltas, final.Usage, final.FinishReason)
}

// The second turn must reach a session that already holds the first. This is
// the assertion that proves --resume works: without it the model is asked the
// follow-up with no idea what came before, and answers something plausible and
// wrong.
func TestClaudeCLILiveRemembersTheConversation(t *testing.T) {
	c := liveCLIClient(t, models.ClaudeCLIHaiku)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	const sess = "live-2"
	history := []message.Message{
		cliUserMsg(sess, "Remember this number: 8713. Reply with just: OK"),
	}

	first, err := c.send(ctx, history, nil)
	if err != nil {
		t.Fatalf("first turn: %v", err)
	}
	t.Logf("turn 1: %q", first.Content)

	// What the agent loop would hand back: the assistant's turn, then the
	// follow-up. The provider must forward only the follow-up.
	assistant := message.Message{Role: message.Assistant, SessionID: sess}
	assistant.Parts = append(assistant.Parts, message.TextContent{Text: first.Content})
	history = append(history, assistant, cliUserMsg(sess, "What number did I ask you to remember? Digits only."))

	second, err := c.send(ctx, history, nil)
	if err != nil {
		t.Fatalf("second turn: %v", err)
	}
	t.Logf("turn 2: %q", second.Content)

	if !strings.Contains(second.Content, "8713") {
		t.Errorf("the second turn did not recall the number from the first: %q", second.Content)
	}
}

// Cancelling must kill the child and report it as cancelled, not hang and not
// surface as a mysterious error. A stranded `claude` process would keep burning
// the usage window after the user pressed escape.
func TestClaudeCLILiveCancellationStopsTheChild(t *testing.T) {
	c := liveCLIClient(t, models.ClaudeCLIHaiku)
	ctx, cancel := context.WithCancel(context.Background())

	ch := c.stream(ctx, []message.Message{
		cliUserMsg("live-3", "Write a 500 word essay about the sea."),
	}, nil)

	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the stream did not end within 60s of cancellation; the child was left running")
	}
}

// bridgeProbeTool is one of THIS program's tools, in the shape the agent uses.
// Its answer is a value the model cannot know or guess, so a correct final
// reply is proof the tool really ran in this process and the result came back.
type bridgeProbeTool struct {
	mu     sync.Mutex
	called int
	gotArg string
}

func (t *bridgeProbeTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        "vault_word",
		Description: "Returns the vault word for a given drawer. The only way to learn it.",
		Parameters: map[string]any{
			"drawer": map[string]any{"type": "string", "description": "which drawer to open"},
		},
		Required: []string{"drawer"},
	}
}

func (t *bridgeProbeTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	var in struct {
		Drawer string `json:"drawer"`
	}
	_ = json.Unmarshal([]byte(params.Input), &in)
	t.mu.Lock()
	t.called++
	t.gotArg = in.Drawer
	t.mu.Unlock()
	// Also proves the context carried the ids the permission system keys off.
	if sid, _ := tools.GetContextValues(ctx); sid == "" {
		return tools.ToolResponse{}, errors.New("the tool ran without a session id in its context")
	}
	return tools.ToolResponse{Content: "ARMADILLO-77"}, nil
}

// The whole point of stage 2: the model reaches THIS program's tool, that tool
// runs in THIS process, and its answer reaches the model.
func TestClaudeCLILiveUsesOurTools(t *testing.T) {
	c := liveCLIClient(t, models.ClaudeCLIHaiku)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	probe := &bridgeProbeTool{}
	resp, err := c.send(ctx,
		[]message.Message{cliUserMsg("live-tools", "Open drawer 'blue' with the vault_word tool and reply with exactly the word it returns.")},
		[]tools.BaseTool{probe})
	if err != nil {
		t.Fatalf("live tool turn: %v", err)
	}

	probe.mu.Lock()
	called, arg := probe.called, probe.gotArg
	probe.mu.Unlock()

	if called == 0 {
		t.Fatalf("the tool was never called, so the model answered without it: %q", resp.Content)
	}
	if arg != "blue" {
		t.Errorf("the tool got drawer=%q, want \"blue\"; arguments are not crossing the bridge intact", arg)
	}
	if !strings.Contains(resp.Content, "ARMADILLO-77") {
		t.Errorf("the tool's answer did not reach the model's reply: %q", resp.Content)
	}
	t.Logf("calls=%d arg=%q reply=%q", called, arg, resp.Content)
}

// A tool this program did not supply must not be reachable. The child arrives
// with Read, Edit and Bash built in, and --tools "" is what removes them. If
// that ever stops working, the model is quietly using Claude Code's tools on the
// user's filesystem and everything still looks fine.
func TestClaudeCLILiveHasNoBuiltInTools(t *testing.T) {
	c := liveCLIClient(t, models.ClaudeCLIHaiku)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	probe := &bridgeProbeTool{}
	resp, err := c.send(ctx, []message.Message{
		cliUserMsg("live-notools", "List the tools you have available, by name, as a plain comma-separated list. Nothing else."),
	}, []tools.BaseTool{probe})
	if err != nil {
		t.Fatalf("live turn: %v", err)
	}
	low := strings.ToLower(resp.Content)
	for _, banned := range []string{"bash", "webfetch", "notebookedit", "todowrite"} {
		if strings.Contains(low, banned) {
			t.Errorf("the child still has Claude Code's %q tool; --tools \"\" is not taking effect: %q", banned, resp.Content)
		}
	}
	t.Logf("reported tools: %q", resp.Content)
}
