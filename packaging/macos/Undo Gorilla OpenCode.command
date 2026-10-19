#!/bin/bash
# Double-click to put back the Gorilla OpenCode you had before the last install.
# The installer keeps it as ~/.local/bin/gorilla-opencode.previous. Undo swaps
# the two, so running Undo again brings the newer one back.

BIN="$HOME/.local/bin"
say() { printf '%s\n' "$*"; }
finish() { say ""; read -n1 -rp "Press any key to close."; exit "$1"; }

say "Gorilla OpenCode - go back to the previous version"
say "=================================================="
say ""
if [ ! -f "$BIN/gorilla-opencode.previous" ]; then
  say "There is no previous version to go back to ($BIN/gorilla-opencode.previous is missing)."
  finish 1
fi
NOW="$("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
PREV="$(cat "$BIN/gorilla-opencode.previous.version" 2>/dev/null)"
say "Installed now:  ${NOW:-unknown}"
say "Going back to:  ${PREV:-unknown}"
say ""
read -rp "Type yes and press Enter to swap them: " answer
if [ "$answer" != "yes" ]; then
  say "Nothing was changed."
  finish 1
fi

mv -f "$BIN/gorilla-opencode" "$BIN/gorilla-opencode.swap" &&
  mv -f "$BIN/gorilla-opencode.previous" "$BIN/gorilla-opencode" &&
  mv -f "$BIN/gorilla-opencode.swap" "$BIN/gorilla-opencode.previous" || {
  say "STOPPED: the swap did not complete. Look in $BIN."
  finish 1
}
printf '%s\n' "${NOW:-unknown}" > "$BIN/gorilla-opencode.previous.version"
if [ -f "$HOME/Desktop/.Gorilla OpenCode.command.previous" ]; then
  cp -p "$HOME/Desktop/.Gorilla OpenCode.command.previous" "$HOME/Desktop/Gorilla OpenCode.command"
fi
say "[ok] Now installed: $("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
say "Double-click Undo again to return to ${NOW:-the newer version}."
finish 0
