# Gorilla OpenCode 0.1.171: plan mode can read everything and write its plan, Hydrogen effort meter, never starts in plan mode

## What is this program?

Gorilla OpenCode is a terminal coding assistant. It uses the models and
connections you choose, including models running on your computer.

## Should you download this version?

Yes, especially if you use `/plan` or Gemini 4 Hydrogen (Sonar). Update from `0.1.169`.

## Why this matters to you

- **`/plan` works like Gemini, Antigravity, Claude and Codex.** The AI can read
  and inspect freely (files, searches, screenshots and OCR, shell commands after
  you approve them) and can write the plan file (for example `fix.me` in your
  Improvements folder; outside the workspace it asks first). Source code stays
  untouched until you approve.
- **The app never starts in plan mode.** A saved plan mode is reset to ask on start.
- **`/plan off` is in `/help`.** So is how to pick a `/work` level. Both
  messages now stay in the transcript instead of vanishing after about 8 seconds.
- **Gemini 4 Hydrogen (Sonar) has a low / medium / high effort meter**,
  and the choice is really sent to the model.
