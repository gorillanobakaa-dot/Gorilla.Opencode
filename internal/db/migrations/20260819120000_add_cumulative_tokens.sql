-- GORILLA OVERRIDE: separate "what is in the context now" from "what this
-- conversation has cost in total".
--
-- prompt_tokens and completion_tokens were being asked to be both, and could
-- only be one. TrackUsage assigned them (`=`), which is correct for the context
-- gauge — the status bar and sidebar compare their sum against the model's
-- context window, and a running total would climb past 100% and show a
-- permanent false warning. Compaction relies on the same reading: it sets
-- prompt_tokens to 0 because the context really has been emptied.
--
-- But three other places read the SAME two columns as lifetime totals. The
-- session export writes "Tokens: N in / M out" and the sidebar labels them
-- "Input" and "Output" — both were reporting the LAST TURN ONLY, silently, in
-- a document written to be kept. cost, sitting one line away in the same
-- function, has always used `+=`, so a session could show £0.31 spent against
-- 4,000 tokens and nothing about the display said which reading was which.
--
-- The proposal that prompted this asked for `+=` instead of `=`. That is the
-- wrong fix: it would repair the export and break the gauge. Both readings are
-- legitimate and they are different numbers, so both are stored.
--
-- Existing rows start at 0 rather than being back-filled from the current
-- values. A back-fill would be a guess presented as a measurement — the last
-- turn's size is not the conversation's total — and a visible zero is honest
-- about being unknown in a way a plausible wrong number never is.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN cumulative_prompt_tokens INTEGER NOT NULL DEFAULT 0 CHECK (cumulative_prompt_tokens >= 0);
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN cumulative_completion_tokens INTEGER NOT NULL DEFAULT 0 CHECK (cumulative_completion_tokens >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN cumulative_prompt_tokens;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN cumulative_completion_tokens;
-- +goose StatementEnd
