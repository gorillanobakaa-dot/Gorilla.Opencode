package db

// GORILLA OVERRIDE (2026-08-18): the storage-management tests.
//
// These assert the two claims that are easy to make and easy to get wrong, and
// both of them matter on a device with 1 GB free rather than 1 TB:
//
//   1. Deleting a conversation removes the helper sessions it spawned. SQLite
//      will not do it — sessions have no foreign key on parent_session_id — so
//      a plain delete leaves seventeen orphans behind per supervised run.
//   2. Reclaiming actually shrinks the FILE. DELETE marks pages free and reuses
//      them later; without VACUUM the bytes never come back, and an erase that
//      frees nothing is not an erase.
//
// The second is asserted by stat-ing the file, not by trusting the call to
// return nil. A tool reporting success having done nothing is the failure this
// project's rules are written against.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/pressly/goose/v3"
)

// newTestDBAt is newTestDB with the path handed back, so a test can stat the
// file it is meant to be shrinking.
func newTestDBAt(t *testing.T) (*Queries, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.db")
	sqlDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	goose.SetBaseFS(FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return New(sqlDB), path
}

// addMessage stores one message with a body of the requested size, so a test
// can put a known number of bytes into the store.
func addMessage(t *testing.T, q *Queries, sessionID, id, body string) {
	t.Helper()
	parts, err := json.Marshal([]map[string]any{
		{"type": "text", "data": map[string]string{"text": body}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateMessage(context.Background(), CreateMessageParams{
		ID:        id,
		SessionID: sessionID,
		Role:      "assistant",
		Parts:     string(parts),
	}); err != nil {
		t.Fatalf("create message %s: %v", id, err)
	}
}

// A research run's helpers hold most of its bytes. Deleting the conversation
// must take them with it, or the space is unreachable: nothing in the interface
// can name an orphaned helper afterwards.
func TestDeletingAConversationTakesItsHelpersAndTheirMessages(t *testing.T) {
	q, _ := newTestDBAt(t)
	ctx := context.Background()

	mustCreate(t, q, "conv", "A research run", "/home/x")
	addMessage(t, q, "conv", "m-conv", "the question")
	for i := range 10 {
		id := fmt.Sprintf("call_abc-lane%d", i)
		if _, err := q.CreateSession(ctx, CreateSessionParams{
			ID: id, Title: "Research: lane", ParentSessionID: sql.NullString{String: "conv", Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
		addMessage(t, q, id, "m-"+id, strings.Repeat("findings ", 200))
	}

	before, err := q.SessionStorageFor(ctx, "conv")
	if err != nil {
		t.Fatal(err)
	}
	if before.Helpers != 10 {
		t.Fatalf("expected 10 helpers attributed to the conversation, got %d", before.Helpers)
	}
	if before.Messages != 11 {
		t.Errorf("expected 11 messages counted against it, got %d", before.Messages)
	}
	if before.Bytes < 10000 {
		t.Errorf("the helpers' bytes are not attributed to the conversation: %d", before.Bytes)
	}

	removed, err := q.DeleteSessionTree(ctx, "conv")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 11 {
		t.Errorf("removed %d session rows, want 11 (the conversation and its ten helpers)", removed)
	}

	var sessions, messages int
	if err := q.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := q.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Errorf("%d session rows survived the delete — that space cannot be reached again", sessions)
	}
	if messages != 0 {
		t.Errorf("%d messages survived; the cascade did not fire", messages)
	}
}

// The claim under test is "you get the space back". Assert the FILE, because
// DELETE alone leaves it exactly the same size.
func TestReclaimActuallyShrinksTheFileOnDisk(t *testing.T) {
	q, path := newTestDBAt(t)
	ctx := context.Background()

	mustCreate(t, q, "big", "A long conversation", "/home/x")
	// ~1 MB of message bodies: enough that page reuse cannot hide the change.
	for i := range 40 {
		addMessage(t, q, "big", fmt.Sprintf("m%d", i), strings.Repeat("x", 25_000))
	}
	if _, err := q.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	full := sizeOf(t, path)

	if _, err := q.DeleteSessionTree(ctx, "big"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	afterDelete := sizeOf(t, path)

	// This is the part people assume away. If it ever stops being true, the
	// assertion below is measuring nothing and should be revisited.
	if afterDelete < full/2 {
		t.Fatalf("DELETE alone already shrank the file (%d -> %d); this test no longer proves what it claims",
			full, afterDelete)
	}

	if err := q.Reclaim(ctx); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	afterReclaim := sizeOf(t, path)

	if afterReclaim >= afterDelete {
		t.Errorf("the file did not shrink: %d bytes before reclaim, %d after — "+
			"on a device with 1 GB free that is an erase that frees nothing",
			afterDelete, afterReclaim)
	}
	t.Logf("full %d B → after delete %d B → after reclaim %d B", full, afterDelete, afterReclaim)
}

func sizeOf(t *testing.T, path string) int64 {
	t.Helper()
	total := int64(0)
	for _, p := range []string{path, path + "-wal"} {
		if info, err := os.Stat(p); err == nil {
			total += info.Size()
		}
	}
	return total
}

// Searching by title alone is not enough: this program writes "New Session" as
// a real title, and someone looking for a past run weeks later searches for
// what was discussed, not for what the summariser called the day.
func TestSearchMatchesMessageContentNotJustTitles(t *testing.T) {
	q, _ := newTestDBAt(t)
	ctx := context.Background()

	mustCreate(t, q, "s1", "New Session", "/home/x")
	addMessage(t, q, "s1", "m1", "the VA-API decode path was the problem")
	mustCreate(t, q, "s2", "Kernel work", "/home/x")
	addMessage(t, q, "s2", "m2", "nothing relevant here")

	hits, err := q.SearchSessions(ctx, "va-api")
	if err != nil {
		t.Fatal(err)
	}
	if !hits["s1"] {
		t.Error("a session whose CONTENT matches was not found; its title is 'New Session' and unsearchable")
	}
	if hits["s2"] {
		t.Error("a session that does not match was returned")
	}

	// Titles still work, and matching is case-insensitive on both.
	if hits, _ := q.SearchSessions(ctx, "kernel"); !hits["s2"] {
		t.Error("title search stopped working")
	}
}

// Helper sessions must never appear as conversations in their own right — the
// list would show seventeen rows for one research run.
func TestStorageAndSearchOnlyReportConversations(t *testing.T) {
	q, _ := newTestDBAt(t)
	ctx := context.Background()

	mustCreate(t, q, "conv", "A run about widgets", "/home/x")
	if _, err := q.CreateSession(ctx, CreateSessionParams{
		ID: "call_x-local", Title: "Research: local",
		ParentSessionID: sql.NullString{String: "conv", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	addMessage(t, q, "call_x-local", "m1", "widgets are made of widgets")

	per, err := q.AllSessionStorage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := per["call_x-local"]; ok {
		t.Error("a helper session was listed as a conversation of its own")
	}
	if per["conv"].Bytes == 0 {
		t.Error("the helper's bytes were not attributed to the conversation that spawned it")
	}

	totals, err := q.StoreTotals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Sessions != 1 || totals.Helpers != 1 {
		t.Errorf("totals said %d conversations and %d helpers, want 1 and 1", totals.Sessions, totals.Helpers)
	}

	hits, err := q.SearchSessions(ctx, "widgets")
	if err != nil {
		t.Fatal(err)
	}
	if hits["call_x-local"] {
		t.Error("a helper session was returned as a search result")
	}
}
