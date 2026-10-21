package app

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// A stored view result whose metadata holds a sign-in token is masked at start;
// a row with nothing to mask is not rewritten.
func TestStoredCredentialsAreMaskedAtStart(t *testing.T) {
	conn, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "t.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE messages (id TEXT PRIMARY KEY, parts TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	const token = "ya29.a0TestTokenValue1234567890"
	leak := `[{"type":"tool_result","data":{"content":"[REDACTED]","metadata":"{\"content\":\"{\\\"access_token\\\": \\\"` + token + `\\\"}\"}"}}]`
	clean := `[{"type":"text","data":{"text":"hello"}}]`
	if _, err := conn.Exec(`INSERT INTO messages VALUES ('a', ?), ('b', ?)`, leak, clean); err != nil {
		t.Fatal(err)
	}
	mask := func(s string) (string, int) {
		n := strings.Count(s, token)
		return strings.ReplaceAll(s, token, "[REDACTED: sign-in token]"), n
	}
	n, err := scrubStoredSecrets(context.Background(), conn, mask)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("changed %d rows, want 1", n)
	}
	var got string
	if err := conn.QueryRow(`SELECT parts FROM messages WHERE id = 'a'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, token) || !strings.Contains(got, "[REDACTED: sign-in token]") {
		t.Errorf("token still stored: %s", got)
	}
	if err := conn.QueryRow(`SELECT parts FROM messages WHERE id = 'b'`).Scan(&got); err != nil || got != clean {
		t.Errorf("clean row changed: %q %v", got, err)
	}
}
