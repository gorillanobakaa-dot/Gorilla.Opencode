-- GORILLA (2026-10-21): how much of the input was the conversation sent AGAIN.
--
-- cost_reread counts only what the provider served from its cache. A session
-- on Antigravity Gemini 3.8 Flash measured the difference on the same day:
-- 596,340 input tokens over 33 calls for a conversation of ~21K, so roughly
-- nine tenths of the input was repeats, while the cache caught about a fifth of
-- them and the bar read "2% re-reading". The repeats the cache missed were
-- billed as fresh input, and nothing on screen said so.
--
-- resent_tokens adds, for every call, the part of its prompt that was already
-- sent by the previous call (the previous prompt plus the previous reply,
-- capped at this call's prompt). Divided by cumulative_prompt_tokens it is the
-- share of input that was repetition, whatever the provider's cache did.
-- Existing rows start at 0: a back-fill would be a guess.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN resent_tokens INTEGER NOT NULL DEFAULT 0 CHECK (resent_tokens >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN resent_tokens;
-- +goose StatementEnd
