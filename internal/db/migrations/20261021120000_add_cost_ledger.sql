-- GORILLA (2026-10-21): what the status bar needs to show WHY a session costs
-- what it costs, next to the total that cost already holds.
--
-- cost_reread     the part of cost paid for re-sending old context (cache
--                 reads). Every call sends the whole conversation again; in a
--                 measured 64-hour Claude Code session this was 72% of the bill.
-- calls_priced    calls whose price came from a table (sourced or catalogue).
-- calls_unpriced  calls on a model nobody has a price for. They are counted and
--                 shown, never given a guessed price, and are not in cost.
-- last_call_cost  what the most recent priced call cost, so the bar can show
--                 that a call now costs more than it did an hour ago.
--
-- Existing rows start at 0: a back-fill would be a guess shown as a measurement.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN cost_reread REAL NOT NULL DEFAULT 0 CHECK (cost_reread >= 0);
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN calls_priced INTEGER NOT NULL DEFAULT 0 CHECK (calls_priced >= 0);
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN calls_unpriced INTEGER NOT NULL DEFAULT 0 CHECK (calls_unpriced >= 0);
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN last_call_cost REAL NOT NULL DEFAULT 0 CHECK (last_call_cost >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN cost_reread;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN calls_priced;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN calls_unpriced;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN last_call_cost;
-- +goose StatementEnd
