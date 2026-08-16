# v0.1.67 — Fix provider portal width clipping on wide terminals

## What was wrong

The every-launch provider picker and the extras screen both hard-capped their
content at **76 columns**, even on terminals that are 150+ columns wide. The
cap came from this line in `contentWidth()`:

```go
return max(minimum, min(fallback, m.width-chrome))
```

`min(fallback, ...)` with `fallback = 76` meant that once the terminal reported
its real width, it was ignored — content was still squeezed into 76 chars and
long provider names were clipped mid-word:

```
> Antigravity free tier - Claude + GPT-OSS + Gemini (Gmail sign-..   ← clipped
  Google - Code Assist free tier (Gemini only, Gmail sign-in, nok.   ← clipped
```

## What was fixed

The `fallback` value is only needed before the terminal reports its size
(`m.width <= 0`). Once width is known, the content uses the full terminal width
minus chrome (4 cols for padding). Fixed in both affected files:

- `internal/tui/startup/provider.go` — the provider portal
- `internal/tui/startup/extras.go` — the extras/toggles screen

```go
// before
return max(minimum, min(fallback, m.width-chrome))

// after
return max(minimum, m.width-chrome)
```

Provider names and descriptions now wrap naturally at the real terminal width
with no mid-word truncation.
