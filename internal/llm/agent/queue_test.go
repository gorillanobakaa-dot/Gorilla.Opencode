package agent

import "testing"

type busyStub struct {
	Service
	busy bool
}

func (b busyStub) IsSessionBusy(string) bool { return b.busy }

func TestQueueOnlyWhileBusyAndKeepsOrder(t *testing.T) {
	if QueueUserMessage(busyStub{busy: false}, "s1", "x") {
		t.Fatal("an idle session must not queue: the text is sent normally")
	}
	b := busyStub{busy: true}
	if !QueueUserMessage(b, "s1", "one") || !QueueUserMessage(b, "s1", "two") {
		t.Fatal("a busy session must queue")
	}
	if QueuedCount("s1") != 2 {
		t.Fatal("count")
	}
	got := TakeQueued("s1")
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("order lost: %v", got)
	}
	if QueuedCount("s1") != 0 || len(TakeQueued("s1")) != 0 {
		t.Fatal("a taken message must be delivered once")
	}
}
