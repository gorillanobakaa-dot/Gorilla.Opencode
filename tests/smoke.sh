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
skip() { echo "skip: $1"; }

# A check that cannot run here is not a check that failed. Reporting it as a
# failure trains everyone to ignore the whole suite, which costs more than the
# coverage it pretends to have.

[ -x "$BIN" ] || { echo "build first: go build -o gorilla-opencode ."; exit 2; }

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

# Is a local model server reachable? Gorilla auto-discovers those on purpose,
# so on a developer machine running LM Studio or llama.cpp there IS a provider
# configured and the no-provider path below genuinely cannot be exercised.
# `env -i` clears the environment but cannot clear a listening socket.
LOCAL_UP=no
for port in 1234 8080 11434 8000; do
  if command -v curl >/dev/null 2>&1; then
    curl -s -m 1 -o /dev/null "http://127.0.0.1:$port/v1/models" 2>/dev/null && LOCAL_UP=yes && break
  fi
done

# 1. No provider configured: friendly, actionable, no usage dump, nonzero exit.
OUT="$(cd "$TMP" && env -i HOME="$TMP" PATH="$PATH" TERM=dumb "$BIN" -p hi -q 2>&1)"; RC=$?
if [ "$LOCAL_UP" = yes ]; then
  # A local endpoint is up, so a provider IS configured and this path is
  # unreachable. Two checks below still hold regardless: whatever happens,
  # it must not be the cryptic error and must not dump usage text.
  skip "no-provider message (a local model server is running on this machine)"
  skip "nonzero exit on no-provider (same reason)"
else
  echo "$OUT" | grep -q "no AI provider is configured" \
    && pass "no-provider message is friendly" || fail "friendly no-provider message missing"
  [ "$RC" -ne 0 ] && pass "nonzero exit ($RC)" || fail "exit code is 0 on failure"
fi
echo "$OUT" | grep -q "agent coder not found" \
  && fail "cryptic 'agent coder not found' still surfaces" || pass "cryptic error gone"
echo "$OUT" | grep -q "Usage:" \
  && fail "runtime error still dumps usage text" || pass "no usage dump on runtime error"

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

# 6. Desktop launch parity: the .deb and the self-installer must BOTH
#    use the `launch` wrapper, or GUI launches flash-die (v0.1.1 bug:
#    only the self-installer was fixed).
# scripts/ was deliberately removed from the repository (commit ebfb718,
# "security: remove scripts/ and Documentation.Scripts/"), so this check has
# nothing to read on this branch. Skip rather than fail: the file's absence is
# the intended state, not a regression.
if [ -f "$ROOT/scripts/build-deb.sh" ]; then
  DEB_EXEC="$(grep -A20 'opencode-dino.desktop\|gorilla-opencode.desktop' "$ROOT/scripts/build-deb.sh" | grep '^Exec=' | head -1)"
  echo "$DEB_EXEC" | grep -q 'launch' \
    && pass ".deb desktop entry uses launch wrapper" || fail ".deb desktop entry missing 'launch' (GUI flash-die)"
else
  skip ".deb desktop entry (scripts/build-deb.sh is not part of this repo)"
fi

# The desktop entry moved to cmd/install_unix.go when install.go was split into
# platform halves. Look in both, so this keeps working whichever side it lives
# on, and is not a false alarm the next time the file is reorganised.
if grep -qs 'Exec=` + appBinName + ` launch' "$ROOT/cmd/install_unix.go" "$ROOT/cmd/install.go"; then
  pass "self-installer desktop entry uses launch wrapper"
else
  fail "self-installer missing 'launch'"
fi

# 7. launch self-heals: creates the key file if missing (so .deb users
#    who never run `install` still get onboarded, not flash-died).
grep -q 'ensureEnvTemplate' "$ROOT/cmd/launch.go" \
  && pass "launch creates key file when missing" || fail "launch does not self-heal missing key file"

echo "---"
[ "$FAILS" -eq 0 ] && echo "smoke: all checks passed" || echo "smoke: $FAILS check(s) FAILED"
exit "$FAILS"
