#!/bin/bash
# Double-click to bring back the Gorilla OpenCode you had before the last install.
# The installer put it in the Trash as "gorilla-opencode vX.Y.Z". Undo copies
# the newest one back and moves the version installed now to the Trash, so
# running Undo again swaps them back.

BIN="$HOME/.local/bin"
say() { printf '%s\n' "$*"; }
finish() { say ""; read -n1 -rp "Press any key to close."; exit "$1"; }
to_trash() {
  [ -e "$1" ] || return 0
  if osascript -e "tell application \"Finder\" to delete (POSIX file \"$1\" as alias)" >/dev/null 2>&1; then
    return 0
  fi
  mkdir -p "$HOME/.Trash" && mv "$1" "$HOME/.Trash/$(basename "$1") $(date +%Y-%m-%d.%H.%M.%S)"
}

say "Gorilla OpenCode - go back to the previous version"
say "=================================================="
say ""
NOW="$("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
# version_of "…/gorilla-opencode v0.1.147 2026-10-19.22.45.56" -> v0.1.147
version_of() { local v; v="$(basename "$1")"; v="${v#gorilla-opencode }"; printf '%s' "${v%% 20*}"; }
# The newest copy of a DIFFERENT version than the one installed now.
PREV=""
for f in "$HOME/.Trash"/gorilla-opencode\ v* "$BIN"/gorilla-opencode\ v*; do
  [ -f "$f" ] || continue
  [ "$(version_of "$f")" = "$NOW" ] && continue
  if [ -z "$PREV" ] || [ "$f" -nt "$PREV" ]; then PREV="$f"; fi
done
if [ -z "$PREV" ]; then
  say "There is no other version in the Trash to go back to. (Was the Trash emptied?)"
  finish 1
fi
PREV_V="$(version_of "$PREV")"
say "Installed now:  ${NOW:-nothing}"
say "Going back to:  $PREV_V"
say ""
read -rp "Type yes and press Enter to go back: " answer
if [ "$answer" != "yes" ]; then
  say "Nothing was changed."
  finish 1
fi
mkdir -p "$BIN"
cp -p "$PREV" "$BIN/gorilla-opencode.restoring" || { say "STOPPED: could not copy it out of the Trash."; finish 1; }
chmod +x "$BIN/gorilla-opencode.restoring"
if [ -f "$BIN/gorilla-opencode" ]; then
  mv "$BIN/gorilla-opencode" "$BIN/gorilla-opencode ${NOW:-unknown version}" &&
    to_trash "$BIN/gorilla-opencode ${NOW:-unknown version}"
fi
mv -f "$BIN/gorilla-opencode.restoring" "$BIN/gorilla-opencode" || { say "STOPPED: could not put it in place."; finish 1; }
# A copy already in the Trash stays there (nothing is ever deleted for good).
case "$PREV" in "$HOME/.Trash"/*) ;; *) to_trash "$PREV" ;; esac
say "[ok] Now installed: $("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
say "The newer one (${NOW:-unknown}) is in the Trash; double-click Undo again to swap back."
finish 0
