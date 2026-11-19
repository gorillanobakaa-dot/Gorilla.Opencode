# Gorilla OpenCode 0.1.165: keyboard model selection and clearer usage

## What is this program?

Gorilla OpenCode is a terminal coding assistant. It uses the models and
connections you choose, including models running on your computer.

## Should you download this version?

Update if you switch models frequently or use ChatGPT or Antigravity sign-in.

## Why this matters to you

Choose a model and adjust its reasoning with the arrow keys. See ChatGPT's
account limits and credits and find which Antigravity models share a quota.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### What this is

A shorter model list with a single reasoning control, and clearer information
about the allowance your subscription has left.

### Should you download it?

Yes if you use sign-in or change models from the terminal.

### Why this matters

- Use up and down to browse, left and right to set reasoning, and Enter to
  apply. Use slash to search and Ctrl plus the arrow keys to switch providers.
- ChatGPT usage shows the time windows and credits your account reports.
- Antigravity usage names models with shared allowances and identifies models
  whose quota group is unavailable.

### What was checked

See the release evidence for platform, CI, and live-account verification.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

The picker retains provider and model identities while moving per-model
reasoning settings to the highlighted row. ChatGPT options and defaults come
from the model catalogue. Antigravity keeps sending the exact variant ID.

Explicit ChatGPT usage reads the signed-in account's windows and credits. Its
fallback is scoped to the current account and stored separately from account
snapshots. Antigravity maps the live catalogue to reported group descriptions
and retains ambiguous routes as unresolved.

### Security posture

The usage call goes to the existing ChatGPT origin on demand and rejects
redirects. Quota fallback files contain readings scoped by account. The change
adds no telemetry, provider, or dependency.

### Deployment

Use the installer or package for your platform and verify the published
checksums. `gorilla-opencode --version` should report `v0.1.165`.

### Verification

The tests cover keyboard effort selection, the request JSON, ChatGPT
window and credit parsing, and Antigravity route membership. See the release
evidence for outcomes.
