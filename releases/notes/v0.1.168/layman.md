# In plain language

## What this is

A quick hotfix release for `0.1.167` that fixes the `Enter` key inside `/context`
so pressing `Enter` locks in your agent limit or Nuclear Option and closes the
menu instead of knocking the dial one step to the right.

## Should you download it?

Yes — especially if you use `/context` to set a sub-agent cap or turn on
**Gorilla Nuclear Option** (solo mode).

## Why this matters

- **`Enter` now cements your choices and closes `/context`:** Move the top
  dials (AI request pace and agent/sub-agent cap) with `←` and `→`, flip any
  feature `ON`/`OFF` with `Space` (pressing `Space` on a dial also holds its
  current setting instead of nudging it), and press **`Enter`** to confirm,
  save, and close the screen.
- **Nuclear Option stays locked on:** Dialing left to
  `GORILLA NUCLEAR — ALL FUCKING AGENTS/SUBAGENTS DISABLED` and pressing
  `Enter` now closes `/context`, shows a clear confirmation toast, and keeps
  the red `SOLO (NUCLEAR)` indicator active in the bottom bar.
- **Clear on-screen key guide:** The bottom help line in `/context` now reads
  `up/down pick | left/right dial | space toggle/select | enter confirm & close | ... | esc close`.

## What you are looking at (Screenshots)

### 1. Tiered `/model` picker with Newer vs Legacy models & vendor dividers

[![Tiered Antigravity model picker showing Newer models on top with launch dates and color coding, Legacy models underneath separated by Anthropic, Gemini, and OpenAI dividers, and red SOLO (NUCLEAR) status in the bottom bar](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.168/docs/screenshots/gallery/v0168-model-picker-tiers.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.168/docs/screenshots/gallery/v0168-model-picker-tiers.png)

- **What you are seeing:** The `/model` picker after the `0.1.167`/`0.1.168`
  reorganization. The newest generation models sit at the top under
  **Newer models** with color coding, `[YYYY-MM]` launch dates, and short plain
  descriptions (such as `next-gen multimodal` and `fast high-speed workhorse`).
  Older models sit below a clear **`-- Legacy models --`** divider, split by
  vendor (`-- Gemini --`, `-- OpenAI --`) so you never confuse an older model
  with a current flagship. Notice the red **`SOLO (NUCLEAR)`** pill at the
  bottom right showing Nuclear Option is active while you browse models.

### 2. Setting a 3-agent cap in `/context` with instant confirmation

[![Context loadout dialog with the Agents/Subagents dial set to up to 3 agents per turn, showing the inline [SET] badge and green [APPLIED] SAVED confirmation banner at the top](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.168/docs/screenshots/gallery/v0168-context-agents-3.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.168/docs/screenshots/gallery/v0168-context-agents-3.png)

- **What you are seeing:** The `/context` menu when you dial the
  **GORILLA AGENTS/SUBAGENTS** limit to **3 agents per turn** using `←`/`→`.
  Two clear confirmations prove the limit actually took effect: the inline
  **`[SET]`** tag directly on the highlighted orange bar, and the green
  **`[APPLIED] SAVED: up to 3 agent(s)/subagent(s) per turn`** banner near the
  top of the screen. Press **`Enter`** to hold it in place and close.

### 3. Raising the cap to 12 agents/subagents in `/context`

[![Context loadout dialog dialed up to 12 agents/subagents per turn with [SET] inline tag, green [APPLIED] SAVED banner, and live per-turn token count in the bottom bar](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.168/docs/screenshots/gallery/v0168-context-agents-12.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.168/docs/screenshots/gallery/v0168-context-agents-12.png)

- **What you are seeing:** The same `/context` menu dialed up to **12
  agents/subagents per turn** for heavier parallel jobs. The green
  **`[APPLIED] SAVED`** banner at the top updates immediately, the blue bottom
  status bar shows the exact per-turn prompt overhead (`Loadout: ~16778
  tokens/turn`), and every feature row below tells you in plain English what
  turning it off saves and what stops working.

### 4. Gorilla Nuclear Option active (zero sub-agents, solo mode)

[![Context loadout dialog with Gorilla Nuclear Option enabled, displaying the red NUCLEAR OPTION ACTIVE warning banner with Enter to confirm and close, green [APPLIED] SAVED Nuclear confirmation, inline [SET] tag, and enter confirm and close footer](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/docs/screenshots/gallery/v0168-context-nuclear-option.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/docs/screenshots/gallery/v0168-context-nuclear-option.png)

- **What you are seeing:** **Gorilla Nuclear Option** turned on (dial all the
  way left with `←`, then press **`Enter`** to confirm and close). You get
  three unmistakable signals that no background sub-agents can spawn:
  1. The bright red **`NUCLEAR OPTION ACTIVE: ALL sub-agents disabled (solo mode). Press Enter to confirm & close, or -> on Agents to re-enable.`**
     banner at the top.
  2. The green **`[APPLIED] SAVED: GORILLA NUCLEAR`** confirmation and inline
     **`[SET]`** tag on the highlighted row.
  3. The updated bottom key bar showing **`enter confirm & close`** alongside
     `space toggle/select` and `left/right dial`, so hitting `Enter` locks
     Nuclear Option in place and closes `/context`.

## What was checked

See the release evidence for GitHub CI results across Linux and Windows and
reproducible multi-platform builds (Windows, Linux, and macOS).
