package agent

import (
	"context"
	"testing"
)

// resetRegistry clears global state between tests.
func resetRegistry() {
	subAgentRegMu.Lock()
	subAgentReg = map[string]*subAgentEntry{}
	subAgentSeq = 0
	subAgentRegMu.Unlock()
}

func TestKillSubAgentCancelsContext(t *testing.T) {
	resetRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	info := RegisterSubAgent("sess-1", "parent-1", "call-1", "find the config", cancel)

	if got := ActiveSubAgentCount(); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}

	if _, ok := KillSubAgent(info.ID); !ok {
		t.Fatalf("KillSubAgent(%q) = false, want true", info.ID)
	}

	select {
	case <-ctx.Done():
		// good: kill cancelled the helper's context
	default:
		t.Fatal("killing the helper did not cancel its context")
	}

	if got := ActiveSubAgentCount(); got != 0 {
		t.Fatalf("count after kill = %d, want 0", got)
	}
	// Second kill of the same handle is a safe no-op.
	if _, ok := KillSubAgent(info.ID); ok {
		t.Fatal("second KillSubAgent should return false")
	}
}

func TestKillAllSubAgents(t *testing.T) {
	resetRegistry()
	var cancels []context.CancelFunc
	ctxs := make([]context.Context, 0, 3)
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		ctxs = append(ctxs, ctx)
		cancels = append(cancels, cancel)
		RegisterSubAgent("s", "p", "c", "task", cancel)
	}
	if got := ActiveSubAgentCount(); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}

	if n := KillAllSubAgents(); n != 3 {
		t.Fatalf("KillAllSubAgents = %d, want 3", n)
	}
	for i, ctx := range ctxs {
		select {
		case <-ctx.Done():
		default:
			t.Fatalf("helper %d context not cancelled by Nuclear Option", i)
		}
	}
	if got := ActiveSubAgentCount(); got != 0 {
		t.Fatalf("count after nuke = %d, want 0", got)
	}
	if n := KillAllSubAgents(); n != 0 {
		t.Fatalf("second nuke = %d, want 0", n)
	}
	_ = cancels
}

func TestUnregisterRemovesEntry(t *testing.T) {
	resetRegistry()
	_, cancel := context.WithCancel(context.Background())
	info := RegisterSubAgent("s", "p", "c", "task", cancel)
	UnregisterSubAgent(info.ID)
	if got := ActiveSubAgentCount(); got != 0 {
		t.Fatalf("count after unregister = %d, want 0", got)
	}
	cancel()
}

func TestListSubAgentsOrdered(t *testing.T) {
	resetRegistry()
	_, c1 := context.WithCancel(context.Background())
	_, c2 := context.WithCancel(context.Background())
	a := RegisterSubAgent("s1", "p", "c", "first", c1)
	b := RegisterSubAgent("s2", "p", "c", "second", c2)
	list := ListSubAgents()
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("order = [%s %s], want [%s %s]", list[0].ID, list[1].ID, a.ID, b.ID)
	}
	KillAllSubAgents()
}
