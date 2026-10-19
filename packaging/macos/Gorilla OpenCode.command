#!/bin/bash
# Double-click to start Gorilla OpenCode in a folder you pick (macOS 10.13 and later).
#
# Terminal opens a new window at its profile size, 80x24 unless changed, which is
# too small for the program's panels. This asks Terminal for a bigger window
# first (an xterm window-size request, which Terminal honours), and only ever
# grows it: a window you have already made bigger is left alone. The program
# follows the window whatever size you drag it to afterwards.
#
# Size: GORILLA_WINDOW in ~/.bash_profile, for example GORILLA_WINDOW=144x33.
# The default, 138x37, fits a maximised 11-inch MacBook Air with SF Mono 12 pt.
[ -f "$HOME/.bash_profile" ] && source "$HOME/.bash_profile"
export PATH="$HOME/.local/bin:/opt/local/bin:$PATH"

if ! command -v gorilla-opencode >/dev/null 2>&1; then
  echo "gorilla-opencode not found in ~/.local/bin"; read -n1 -rp "Press any key to close."; exit 1
fi

want="${GORILLA_WINDOW:-138x37}"
cols="${want%x*}"; rows="${want#*x}"
case "$cols$rows" in *[!0-9]*|"") cols=138; rows=37 ;; esac
have_cols=$(tput cols 2>/dev/null || echo 0); have_rows=$(tput lines 2>/dev/null || echo 0)
if [ "$have_cols" -lt "$cols" ] || [ "$have_rows" -lt "$rows" ]; then
  [ "$have_cols" -gt "$cols" ] && cols="$have_cols"
  [ "$have_rows" -gt "$rows" ] && rows="$have_rows"
  printf '\033[8;%d;%dt' "$rows" "$cols"
fi

DIR=$(osascript -e 'POSIX path of (choose folder with prompt "Work in which folder?" default location (path to home folder))' 2>/dev/null)
cd "${DIR:-$HOME}" || cd "$HOME"
printf '\033]0;Gorilla OpenCode — %s\007' "$(basename "$PWD")"

gorilla-opencode
echo; read -n1 -rp "Gorilla OpenCode exited. Press any key to close."
