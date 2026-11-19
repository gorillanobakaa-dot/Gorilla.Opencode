# For developers

## Summary

The picker retains provider and model identities while moving per-model
reasoning settings to the highlighted row. ChatGPT options and defaults come
from the model catalogue. Antigravity keeps sending the exact variant ID.

Explicit ChatGPT usage reads the signed-in account's windows and credits. Its
fallback is scoped to the current account and stored separately from account
snapshots. Antigravity maps the live catalogue to reported group descriptions
and retains ambiguous routes as unresolved.

## Security posture

The usage call goes to the existing ChatGPT origin on demand and rejects
redirects. Quota fallback files contain readings scoped by account. The change
adds no telemetry, provider, or dependency.

## Deployment

Use the installer or package for your platform and verify the published
checksums. `gorilla-opencode --version` should report `v0.1.165`.

## Verification

The tests cover keyboard effort selection, the request JSON, ChatGPT
window and credit parsing, and Antigravity route membership. See the release
evidence for outcomes.
