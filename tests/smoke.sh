#!/bin/sh
# Smoke test — catches the regressions found in the 2026-07-20 community
# review (MiniMax M3) so they cannot come back unnoticed.
# Usage: tests/smoke.sh [expected-version]
# Builds nothing; expects ./gorilla-opencode at the repo root.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/gorilla-opencode"
EXPECT_VERSION="${1:-}"
FAILS=0

fail() { echo "FAIL: $1"; FAILS=$((FAILS+1)); }
pass() { echo "ok:   $1"; }

[ -x "$BIN" ] || { echo "build first: go build -o gorilla-opencode ."; exit 2; }

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

# 1. No provider configured: friendly, actionable, no usage dump, nonzero exit.
OUT="$(cd "$TMP" && env -i HOME="$TMP" PATH="$PATH" TERM=dumb "$BIN" -p hi -q 2>&1)"; RC=$?
echo "$OUT" | grep -q "no AI provider is configured" \
  && pass "no-provider message is friendly" || fail "friendly no-provider message missing"
echo "$OUT" | grep -q "agent coder not found" \
  && fail "cryptic 'agent coder not found' still surfaces" || pass "cryptic error gone"
echo "$OUT" | grep -q "Usage:" \
  && fail "runtime error still dumps usage text" || pass "no usage dump on runtime error"
[ "$RC" -ne 0 ] && pass "nonzero exit ($RC)" || fail "exit code is 0 on failure"

# 2. Runtime provider error in -p mode: error visible, no usage dump.
OUT="$(cd "$TMP" && env -i HOME="$TMP" PATH="$PATH" TERM=dumb \
  LOCAL_ENDPOINT=http://127.0.0.1:9 LOCAL_ENDPOINT_API_KEY=x "$BIN" -p hi -q 2>&1)"; RC=$?
echo "$OUT" | grep -q "Usage:" \
  && fail "-p connection error dumps usage text" || pass "-p error path has no usage dump"
[ "$RC" -ne 0 ] || [ -n "$OUT" ] && pass "-p error is not silent" || fail "-p failed silently"

# 3. Version stamp.
V="$("$BIN" -v 2>/dev/null | tail -1)"
if [ -n "$EXPECT_VERSION" ]; then
  [ "$V" = "$EXPECT_VERSION" ] && pass "version stamped: $V" || fail "version is '$V', expected '$EXPECT_VERSION'"
else
  [ "$V" != "unknown" ] && pass "version not 'unknown': $V" || fail "version is 'unknown' (build with -ldflags)"
fi

# 4. Branding: help says gorilla-opencode, never bare 'opencode' as the command.
HELP="$("$BIN" --help 2>&1)"
echo "$HELP" | grep -q "gorilla-opencode" \
  && pass "help uses gorilla-opencode" || fail "help missing gorilla-opencode"
echo "$HELP" | grep -qE '(^|\s)opencode(\s|$)' \
  && fail "help still says bare 'opencode'" || pass "no bare 'opencode' in help"

# 5. FZF warning must not pollute non-interactive output.
OUT="$(cd "$TMP" && env -i HOME="$TMP" PATH="/usr/bin:/bin" TERM=dumb "$BIN" -p hi -q 2>&1)"
echo "$OUT" | grep -q "FZF not found" \
  && fail "FZF warning still prints in non-interactive mode" || pass "no FZF noise"

echo "---"
[ "$FAILS" -eq 0 ] && echo "smoke: all checks passed" || echo "smoke: $FAILS check(s) FAILED"
exit "$FAILS"
