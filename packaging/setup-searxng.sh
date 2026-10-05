#!/usr/bin/env bash
# GORILLA OVERRIDE: one-shot installer for a private SearXNG, so the agent can
# offer to set up web search instead of only explaining how.
#
# WHY THIS IS A SCRIPT AND NOT INSTRUCTIONS THE AGENT FOLLOWS
#
# On 2026-08-08 a model was handed these exact steps as text and asked to relay
# them. It dropped "pyyaml" from the pip line - one of the only two traps in the
# whole procedure - while merely PARAPHRASING. A model improvising the install
# itself produces a half-built venv and then reports success, and a half-working
# SearXNG is worse than none, because it answers and the agent summarises
# whatever it answered with.
#
# So the agent's job is to ask the user and run ONE command. Every decision that
# can be wrong is made here, once, in code that does not improvise.
#
# NOTHING here needs root. SearXNG is not packaged in Debian, so it is a git
# clone plus a venv in the user's own home. The single apt call is guarded and
# only fires if python3-venv is genuinely missing.
#
# Exit codes: 0 only if a real query returned real JSON. Anything else is a
# non-zero exit naming the step that failed. Never "probably fine".
set -uo pipefail

PREFIX="${SEARXNG_PREFIX:-$HOME/.local/share/searxng}"
PORT="${SEARXNG_PORT:-8888}"
REPO="https://github.com/searxng/searxng.git"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/gorilla-opencode"
UNIT_DIR="$HOME/.config/systemd/user"
URL="http://127.0.0.1:${PORT}"

step() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
ok()   { printf '    \033[32mok\033[0m %s\n' "$*"; }
die()  { printf '\n\033[31mFAILED at: %s\033[0m\n%s\n' "$1" "${2:-}" >&2; exit 1; }

step "Checking prerequisites"
command -v git >/dev/null || die "git not found" "Install it: sudo apt install git"
command -v python3 >/dev/null || die "python3 not found" "Install it: sudo apt install python3"

# python3-venv is the one thing that may need root, and only sometimes.
if ! python3 -c 'import venv, ensurepip' 2>/dev/null; then
    step "Installing python3-venv (the only step needing sudo)"
    sudo apt-get install -y python3-venv || die "installing python3-venv" \
        "Install it manually: sudo apt install python3-venv"
fi
ok "git, python3, venv"

step "Fetching SearXNG into $PREFIX"
mkdir -p "$PREFIX" || die "creating $PREFIX"
if [ -d "$PREFIX/src/.git" ]; then
    git -C "$PREFIX/src" pull --ff-only --quiet || ok "existing checkout kept (pull skipped)"
    ok "updated existing checkout"
else
    git clone --depth 1 --quiet "$REPO" "$PREFIX/src" || die "git clone" \
        "Could not reach github.com. Check your connection and retry."
    ok "cloned"
fi

step "Building the virtualenv"
[ -d "$PREFIX/venv" ] || python3 -m venv "$PREFIX/venv" || die "creating venv"
# THE TRAP: do NOT "pip install -e ." — SearXNG's build backend imports msgspec
# and yaml before those get installed, so it fails at metadata generation.
# Running from source via PYTHONPATH skips packaging entirely. pyyaml is listed
# explicitly because it is needed to READ settings.yml at startup.
"$PREFIX/venv/bin/pip" install --quiet --upgrade pip 2>/dev/null
"$PREFIX/venv/bin/pip" install --quiet pyyaml -r "$PREFIX/src/requirements.txt" \
    || die "pip install" "Re-run with output: $PREFIX/venv/bin/pip install pyyaml -r $PREFIX/src/requirements.txt"
"$PREFIX/venv/bin/python" -c 'import searx' 2>/dev/null || {
    PYTHONPATH="$PREFIX/src" "$PREFIX/venv/bin/python" -c 'import searx' \
        || die "importing searx" "Dependencies installed but the package will not import."
}
ok "dependencies installed"

step "Writing settings.yml"
# THE OTHER TRAP: json is NOT in search.formats by default, which is why public
# instances answer 403 to an API request; and the bot limiter 429s local
# automation. Both are set here deliberately. The instance binds to loopback
# only, so neither weakens anything reachable from outside this machine.
if [ ! -f "$PREFIX/settings.yml" ]; then
    cat > "$PREFIX/settings.yml" <<YML
use_default_settings: true

general:
  debug: false
  instance_name: "gorilla-opencode private instance"

server:
  secret_key: "$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  limiter: false
  public_instance: false
  bind_address: "127.0.0.1"
  port: ${PORT}
  image_proxy: false

search:
  safe_search: 0
  autocomplete: ""
  formats:
    - html
    - json
YML
    ok "written"
else
    grep -q "json" "$PREFIX/settings.yml" || die "existing settings.yml has no json format" \
        "Add '- json' under search.formats in $PREFIX/settings.yml"
    ok "kept existing settings.yml"
fi

step "Installing the systemd user service"
if command -v systemctl >/dev/null && [ -d /run/systemd/system ]; then
    mkdir -p "$UNIT_DIR"
    cat > "$UNIT_DIR/searxng.service" <<UNIT
[Unit]
Description=SearXNG (private, for gorilla-opencode)
After=network-online.target

[Service]
Environment=SEARXNG_SETTINGS_PATH=${PREFIX}/settings.yml
Environment=PYTHONPATH=${PREFIX}/src
ExecStart=${PREFIX}/venv/bin/python -m searx.webapp
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
UNIT
    systemctl --user daemon-reload
    systemctl --user enable --now searxng.service >/dev/null 2>&1 \
        || die "starting the service" "Check: systemctl --user status searxng"
    ok "service enabled and started"
else
    step "No systemd user session — starting in the background instead"
    SEARXNG_SETTINGS_PATH="$PREFIX/settings.yml" PYTHONPATH="$PREFIX/src" \
        nohup "$PREFIX/venv/bin/python" -m searx.webapp > "$PREFIX/searxng.log" 2>&1 &
    ok "started (will not survive a reboot; re-run this script after one)"
fi

step "Waiting for it to answer"
up=""
for _ in $(seq 1 30); do
    if curl -sf -o /dev/null -m 2 "$URL/" 2>/dev/null; then up=1; break; fi
    sleep 1
done
[ -n "$up" ] || die "waiting for SearXNG to start" \
    "It did not answer on $URL within 30s. Logs: journalctl --user -u searxng -n 50"
ok "listening on $URL"

step "Verifying a real query returns real JSON"
# The whole point of this script. An installer that reports success without
# issuing a query is guessing, and this project has been bitten by that in four
# separate subsystems. Success means RESULTS CAME BACK - nothing less counts.
body=$(curl -s -m 30 "$URL/search?q=debian&format=json" 2>/dev/null)
case "$body" in
    '{'*) ;;
    *) die "the JSON API" "Got non-JSON back. 'json' is probably missing from search.formats in $PREFIX/settings.yml" ;;
esac
n=$(printf '%s' "$body" | python3 -c 'import sys,json; print(len(json.load(sys.stdin).get("results",[])))' 2>/dev/null || echo 0)
[ "${n:-0}" -gt 0 ] || die "the test query" \
    "SearXNG answered with JSON but zero results, which usually means every upstream engine refused. Try again in a minute; if it persists check $PREFIX/settings.yml"
ok "$n results for a test query"

step "Pointing gorilla-opencode at it"
mkdir -p "$CONFIG_DIR"
CONFIG="$CONFIG_DIR/config.json"
[ -f "$CONFIG" ] || echo '{}' > "$CONFIG"
python3 - "$CONFIG" "$URL" <<'PY' || die "writing config.json" "Set \"searxngURL\" by hand."
import json, sys
path, url = sys.argv[1], sys.argv[2]
try:
    with open(path) as f:
        cfg = json.load(f)
except Exception:
    cfg = {}
cfg["searxngURL"] = url
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
PY
ok "searxngURL = $URL"

cat <<DONE

  SearXNG is installed, running, and verified with a live query.
  gorilla-opencode will use it for web_search with source: web.

    stop:      systemctl --user stop searxng
    start:     systemctl --user start searxng
    logs:      journalctl --user -u searxng -n 50
    remove:    systemctl --user disable --now searxng && rm -rf $PREFIX
               (and delete "searxngURL" from $CONFIG)

DONE
