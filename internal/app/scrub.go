package app

// GORILLA FIX (2026-10-21): credentials already stored are masked at start-up.
//
// Tool results are masked before they are stored (agent/aftertool.go), but the
// metadata beside them was not until 2026-10-21, and a key that is configured
// AFTER it was printed was never on the list when the row was written. MEASURED:
// the owner's session database held an Antigravity access and refresh token in
// a view result's metadata. Every start passes the stored messages through the
// same known-value masking once more.
//
// Known VALUES only (toolName ""): the shape rules are for command output, and
// a stored file read must not have its fixtures rewritten. Replacing a token in
// the raw JSON is safe: the tokens this machine holds contain no character that
// JSON escapes. A 7 MB database is read in well under a second.

import (
	"context"
	"database/sql"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
)

// scrubStoredSecrets masks every stored message with mask and reports how many
// rows it changed.
func scrubStoredSecrets(ctx context.Context, conn *sql.DB, mask func(string) (string, int)) (int, error) {
	rows, err := conn.QueryContext(ctx, `SELECT id, parts FROM messages`)
	if err != nil {
		return 0, err
	}
	type change struct{ id, parts string }
	var changes []change
	for rows.Next() {
		var id, parts string
		if err := rows.Scan(&id, &parts); err != nil {
			rows.Close()
			return 0, err
		}
		if masked, n := mask(parts); n > 0 {
			changes = append(changes, change{id, masked})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, c := range changes {
		if _, err := conn.ExecContext(ctx, `UPDATE messages SET parts = ? WHERE id = ?`, c.parts, c.id); err != nil {
			return 0, err
		}
	}
	return len(changes), nil
}

func scrubStoredSecretsAtStart(ctx context.Context, conn *sql.DB) {
	n, err := scrubStoredSecrets(ctx, conn, func(s string) (string, int) { return tools.MaskSecrets("", s) })
	if err != nil {
		logging.Warn("masking stored credentials failed", "error", err)
		return
	}
	if n > 0 {
		logging.Info("masked credentials in stored messages", "messages", n)
	}
}
