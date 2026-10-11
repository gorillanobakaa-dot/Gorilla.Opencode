package tools

// GORILLA TOOL (2026-10-11): replay a recorded session's shell commands
// through the permission gate WITHOUT running any of them, and count the
// prompts a person would have seen under the exact-command rule and under
// "Allow family".
//
// This is the measurement behind commandfamily.go, kept as a test so it can be
// re-run on any session in the program's own database:
//
//	GORILLA_REPLAY_DB=%USERPROFILE%\.local\share\gorilla-opencode\gorilla-opencode.db \
//	GORILLA_REPLAY_SESSION=9af3336f-7f3d-4838-93e4-ff27ae687f33 \
//	go test ./internal/llm/tools -run TestReplaySessionPrompts -v -count=1
//
// The database is COPIED before it is opened, so a running program is never
// touched. Skipped when the variables are absent.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

type replayCall struct {
	Session string
	Command string
}

// replaySessionCommands reads every bash tool call of a session and its
// helper sessions, in time order.
func replaySessionCommands(t *testing.T, dbPath, sessionID string) ([]replayCall, string) {
	t.Helper()
	tmp := t.TempDir()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := dbPath + suffix
		b, err := os.ReadFile(src)
		if err != nil {
			if suffix == "" {
				t.Fatalf("read %s: %v", src, err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(tmp, "replay.db"+suffix), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "replay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var startedIn sql.NullString
	if err := db.QueryRow(`SELECT started_in FROM sessions WHERE id = ?`, sessionID).Scan(&startedIn); err != nil {
		t.Fatalf("session %s: %v", sessionID, err)
	}
	ids := []string{sessionID}
	for i := 0; i < len(ids); i++ {
		rows, err := db.Query(`SELECT id FROM sessions WHERE parent_session_id = ?`, ids[i])
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
	}

	var calls []replayCall
	for _, id := range ids {
		rows, err := db.Query(`SELECT parts FROM messages WHERE session_id = ? ORDER BY created_at, id`, id)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var parts string
			rows.Scan(&parts)
			var list []struct {
				Type string `json:"type"`
				Data struct {
					Name  string `json:"name"`
					Input string `json:"input"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(parts), &list) != nil {
				continue
			}
			for _, p := range list {
				if p.Type != "tool_call" || p.Data.Name != BashToolName {
					continue
				}
				var in struct {
					Command string `json:"command"`
				}
				if json.Unmarshal([]byte(p.Data.Input), &in) != nil || in.Command == "" {
					continue
				}
				calls = append(calls, replayCall{Session: id, Command: in.Command})
			}
		}
		rows.Close()
	}
	return calls, startedIn.String
}

func TestReplaySessionPrompts(t *testing.T) {
	dbPath, sessionID := os.Getenv("GORILLA_REPLAY_DB"), os.Getenv("GORILLA_REPLAY_SESSION")
	if dbPath == "" || sessionID == "" {
		t.Skip("set GORILLA_REPLAY_DB and GORILLA_REPLAY_SESSION to replay a recorded session")
	}
	calls, startedIn := replaySessionCommands(t, dbPath, sessionID)
	if len(calls) == 0 {
		t.Fatalf("session %s has no bash calls", sessionID)
	}

	// The working directory decides which paths count as "outside the roots",
	// so the replay runs where the session ran when that folder still exists.
	workDir := t.TempDir()
	if st, err := os.Stat(startedIn); err == nil && st.IsDir() {
		workDir = startedIn
	}
	if _, err := config.Load(workDir, false); err != nil {
		t.Fatalf("config.Load(%s): %v", workDir, err)
	}
	t.Logf("session %s: %d bash calls, replayed with working directory %s", sessionID, len(calls), workDir)

	// --- the old rule: every distinct non-read-only command is a prompt ----
	oldEvery, oldDistinct, banned, readOnly := 0, 0, 0, 0
	seen := map[string]bool{}
	for _, c := range calls {
		v := bashGateVerdict(c.Command)
		switch {
		case v.Banned != "":
			banned++
			continue
		case v.ReadOnly:
			readOnly++
			continue
		}
		oldEvery++
		if !seen[c.Command] {
			seen[c.Command] = true
			oldDistinct++
		}
	}

	// --- the new rule: a person who answers "family" whenever one is offered,
	// and "allow for session" otherwise ------------------------------------
	svc := permission.NewPermissionService().(*permission.PermissionServiceForTest)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := svc.Subscribe(ctx)
	type answer struct {
		key, family string
	}
	answers := make(chan answer, 1024)
	go func() {
		for ev := range events {
			if ev.Type != pubsub.CreatedEvent {
				continue
			}
			p := ev.Payload
			// Recorded BEFORE the grant unblocks Request, so the caller can
			// rely on the record when Request returns.
			answers <- answer{p.GrantKey, p.FamilySuggestion}
			if p.FamilySuggestion != "" {
				svc.GrantFamily(p, p.FamilySuggestion)
			} else {
				svc.GrantPersistant(p)
			}
		}
	}()

	b := NewBashTool(svc).(*bashTool)
	newPrompts := 0
	var report strings.Builder
	for i, c := range calls {
		v := bashGateVerdict(c.Command)
		short := c.Command
		if len(short) > 110 {
			short = short[:107] + "..."
		}
		switch {
		case v.Banned != "":
			fmt.Fprintf(&report, "%3d  BANNED      %s\n", i+1, short)
			continue
		case v.ReadOnly:
			fmt.Fprintf(&report, "%3d  read-only   %s\n", i+1, short)
			continue
		}
		before := len(answers)
		if !svc.Request(b.bashPermissionRequest(c.Session, c.Command, v)) {
			t.Fatalf("call %d was denied in the replay", i+1)
		}
		if len(answers) > before {
			a := <-answers
			newPrompts++
			how := "allow for session"
			if a.family != "" {
				how = "ALLOW FAMILY: " + a.family
				if len(how) > 100 {
					how = how[:97] + "..."
				}
			}
			fmt.Fprintf(&report, "%3d  PROMPT %-3d  %s\n           -> %s\n", i+1, newPrompts, short, how)
		} else {
			fmt.Fprintf(&report, "%3d  covered     %s\n", i+1, short)
		}
	}

	t.Logf("\n%s", report.String())
	t.Logf("bash calls: %d (banned %d, read-only %d)", len(calls), banned, readOnly)
	t.Logf("prompts under the exact-command rule: %d if every call asked, %d with \"allow for session\" on each distinct command", oldEvery, oldDistinct)
	t.Logf("prompts under \"allow family\": %d", newPrompts)
	if newPrompts > oldDistinct {
		t.Errorf("the family rule raised MORE prompts (%d) than the exact rule (%d)", newPrompts, oldDistinct)
	}
}
