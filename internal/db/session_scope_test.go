package db

// GORILLA OVERRIDE: the first database test in this project, added with the
// started_in column (v0.1.86).
//
// It exists because the release immediately before it shipped a regression test
// that PASSED with its own fix removed — the editor wrapping test, which builds
// a fresh textarea and so never reproduces the stale-viewport state it claims to
// guard. This project's standard is that a test must fail against the bug, so
// this file was written by first asserting the behaviour, then deleting the
// WHERE clause from ListSessionsByDir and confirming TestListSessionsByDirScopes
// FAILS, then restoring it. If you change the query, repeat that.
//
// It runs the real embedded goose migrations against a real (temporary) SQLite
// file, so it also proves 20260816230000_add_session_started_in.sql applies
// cleanly on top of the existing schema rather than assuming it.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/pressly/goose/v3"
)

// newTestDB opens a throwaway database with every migration applied.
func newTestDB(t *testing.T) *Queries {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
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
		t.Fatalf("migrations did not apply: %v", err)
	}
	return New(sqlDB)
}

func mustCreate(t *testing.T, q *Queries, id, title, startedIn string) {
	t.Helper()
	if _, err := q.CreateSession(context.Background(), CreateSessionParams{
		ID:        id,
		Title:     title,
		StartedIn: startedIn,
	}); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func ids(sessions []Session) map[string]bool {
	got := map[string]bool{}
	for _, s := range sessions {
		got[s.ID] = true
	}
	return got
}

// TestListSessionsByDirScopes is the non-vacuous one: delete the
// "AND (started_in = ? OR started_in = ”)" clause and it fails.
func TestListSessionsByDirScopes(t *testing.T) {
	q := newTestDB(t)
	ctx := context.Background()

	mustCreate(t, q, "kernel-1", "kernel work", "/home/u/Kernel")
	mustCreate(t, q, "kernel-2", "more kernel", "/home/u/Kernel")
	mustCreate(t, q, "firefox-1", "firefox work", "/home/u/Firefox")

	got, err := q.ListSessionsByDir(ctx, "/home/u/Kernel")
	if err != nil {
		t.Fatalf("ListSessionsByDir: %v", err)
	}
	have := ids(got)
	if !have["kernel-1"] || !have["kernel-2"] {
		t.Errorf("scoped list dropped its own folder's sessions: %v", have)
	}
	if have["firefox-1"] {
		t.Errorf("scoped list leaked another folder's session: %v", have)
	}
	if len(got) != 2 {
		t.Errorf("want 2 sessions for /home/u/Kernel, got %d", len(got))
	}
}

// TestListSessionsByDirIncludesUnknownOrigin pins the deliberate choice that
// rows written before the column existed (started_in = "") appear under EVERY
// folder rather than becoming invisible. A session the user cannot find is
// indistinguishable from one that was deleted.
func TestListSessionsByDirIncludesUnknownOrigin(t *testing.T) {
	q := newTestDB(t)

	mustCreate(t, q, "legacy", "from before the column", "")
	mustCreate(t, q, "kernel-1", "kernel work", "/home/u/Kernel")

	for _, dir := range []string{"/home/u/Kernel", "/home/u/Somewhere/Else"} {
		got, err := q.ListSessionsByDir(context.Background(), dir)
		if err != nil {
			t.Fatalf("ListSessionsByDir(%s): %v", dir, err)
		}
		if !ids(got)["legacy"] {
			t.Errorf("unknown-origin session hidden from %s — it can never be found again", dir)
		}
	}
}

// TestListSessionsStillReturnsEverything guards the other half: the unscoped
// list must not start filtering. It is what ctrl+a in the picker shows.
func TestListSessionsStillReturnsEverything(t *testing.T) {
	q := newTestDB(t)

	mustCreate(t, q, "kernel-1", "kernel work", "/home/u/Kernel")
	mustCreate(t, q, "firefox-1", "firefox work", "/home/u/Firefox")
	mustCreate(t, q, "legacy", "from before the column", "")

	got, err := q.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want all 3 sessions, got %d", len(got))
	}
}

// TestStartedInRoundTrips proves the column is actually persisted and read back,
// not silently dropped by the INSERT column list.
func TestStartedInRoundTrips(t *testing.T) {
	q := newTestDB(t)
	ctx := context.Background()

	mustCreate(t, q, "s1", "a session", "/home/u/Project")

	got, err := q.GetSessionByID(ctx, "s1")
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if got.StartedIn != "/home/u/Project" {
		t.Errorf("started_in did not survive the round trip: got %q", got.StartedIn)
	}
}
