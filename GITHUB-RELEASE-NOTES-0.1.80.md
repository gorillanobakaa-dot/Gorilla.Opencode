## /usage now shows a real meter — bananas included

One feature: `/usage` no longer prints a single cryptic line. It draws the meter.

The old line said `Claude and GPT models: 96%` and left you to guess whether
that meant 96% left or 96% spent. Guessing wrong means either panic or a burned
week. The new panel says both numbers in words and draws the gauge:

```
CLAUDE AND GPT MODELS
  Models within this group: Claude Opus, Claude Sonnet, GPT-OSS

  Weekly Limit Remaining
    [██████████████████████████████████████░░░░░░░░░░░░] 75.15%
    🍌🍌🍌 Loaded up on bananas... let's go nuts. — 75% left, 25% used · resets in 2d
```

- **The bar is a thermometer scale** — red at the left end through orange and
  yellow to green at the right. The fill recedes as your week burns, so the tip
  of the bar is always the colour of what you have left.
- **The bananas are the mood, the numbers are the data.** Three bananas down to
  🦍 "No more bananas for today." as the meter drops. This is Gorilla OpenCode;
  quota is bananas.
- **Paid providers with a real balance endpoint get metered too.** A DeepSeek
  key shows its balance in money; an OpenRouter key shows credits left as a bar
  (or says plainly "no credits purchased — free models only"). A failed check
  is shown as a failed check, never silently missing.
- Providers whose API exposes nothing wallet-shaped to an ordinary key
  (Anthropic, OpenAI, xAI, Groq) are deliberately absent — a meter that guesses
  is worse than no meter.

The automatic one-line reading at session start is unchanged; the panel appears
only when you ask with `/usage`, so it never floods your scrollback.

Also in this release: v0.1.79's `Depends: lynx` packaging fix is the base.

**Not verified:** DeepSeek's balance endpoint is implemented from its API
documentation — no DeepSeek key was available to test live (the OpenRouter
endpoint *was* confirmed against the live API, free-tier response). The paid
OpenRouter response shape is likewise doc-derived. If a provider section ever
renders blank with a valid key, that is where to look.
