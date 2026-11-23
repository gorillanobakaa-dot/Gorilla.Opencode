# For developers

### Summary

- `internal/permission/mode.go`: plan mode allows reads, lets `bash` take the normal ask path, and allows `write`/`edit` of plan documents (`isPlanDocument`: .md/.txt/.me/.json/.yaml/.yml with plan/roadmap/strategy/rfc/todo/improvement in any path component; outside the workspace it asks). Other tools stay refused.
- `internal/llm/agent/agent.go`: `planModeDirective` reworded to match.
- `internal/config/config.go`: a persisted `plan` mode loads as `ask`.
- `internal/commands/registry.go`, `internal/tui/tui.go`: `/plan off` and `/work` pick help; `ReportInfoEcho` keeps the messages in the transcript.
- `internal/llm/models/models.go`, `internal/llm/provider/antigravity.go`: single-id Gemini Antigravity models get `EffortOptions()`, and `thinkingConfig.thinkingBudget` is sent.

### Security posture

No new network calls, telemetry, providers or dependencies. Plan-mode shell commands and out-of-workspace plan writes still ask.

### Not verified

Live TUI run; live Hydrogen API call; the `view` refusal by the owner's Fieldkit tool-router hook is not changed (hook not weakened).
