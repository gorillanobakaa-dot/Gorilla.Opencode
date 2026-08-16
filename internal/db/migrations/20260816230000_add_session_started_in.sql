-- GORILLA OVERRIDE: record which folder a session was started in.
--
-- v0.1.85 moved session storage from a per-project `.opencode/opencode.db` to a
-- single XDG store at ~/.local/share/gorilla-opencode. That was the right call —
-- one file to back up, nothing scattered through the user's projects — but the
-- separation between projects had never been a column, it was WHICH FILE the row
-- lived in. Collapsing to one file therefore collapsed every project's history
-- into one flat, unfilterable list.
--
-- This restores the separation as data rather than as filesystem layout, so the
-- storage stays XDG-correct and the PICKER does the scoping. Named started_in,
-- not working_dir: /add-dir and /cd already own the word "dir" and mean
-- something else entirely (which folders the agent may read), so reusing it here
-- would collide with two existing commands.
--
-- Empty string means "unknown" — rows written before this migration. The picker
-- treats those as belonging to every project rather than hiding them, because a
-- session you cannot find is indistinguishable from one that was deleted.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN started_in TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sessions_started_in ON sessions (started_in);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sessions_started_in;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN started_in;
-- +goose StatementEnd
