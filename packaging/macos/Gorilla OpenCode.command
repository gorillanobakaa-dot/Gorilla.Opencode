#!/bin/bash
# Double-click to start Gorilla OpenCode in a folder you pick (macOS 10.13 and later).
#
# Terminal opens a new window at its profile size, 80x24 unless changed, and puts
# it where it likes: on an 11-inch MacBook Air its right part went past the screen
# edge. Before the folder dialog, so the program's first frame is drawn at the
# final size, this sizes the window to 167x39, or the largest that fits the usable
# screen with the current font, and centers it in that area: below the menu bar,
# clear of the Dock on whichever side it sits, all four edges on the screen. The
# program follows the window whatever size you drag it to afterwards.
#
# Size: GORILLA_WINDOW in ~/.bash_profile, for example GORILLA_WINDOW=144x33. It is
# still capped to what fits the screen.

# The placement arithmetic has no macOS calls in it, so it is tested on any system:
# cmd/macos_launcher_test.go sources this file, which stops before the launch.
# Lengths are screen points with the origin at the top left, as in Terminal's
# "bounds"; cell sizes are in hundredths of a point (a Menlo cell is about 6.62).

# _gorilla_floordiv A B: A/B rounded down, also for a negative A (bash truncates).
_gorilla_floordiv() {
  if [ "$1" -ge 0 ]; then echo $(( $1 / $2 )); else echo $(( -((-$1 + $2 - 1) / $2) )); fi
}

# gorilla_cell C0 R0 W0 H0 C1 R1 W1 H1: the cell size from the window measured at
# two sizes (C0xR0 cells, W0xH0 points, then C1xR1, W1xH1), which cancels the title
# bar and the margins. With no change in a direction it falls back to the whole
# window divided by its cells: too big a cell, so too few cells, never too many.
gorilla_cell() {
  local c0=$1 r0=$2 w0=$3 h0=$4 c1=$5 r1=$6 w1=$7 h1=$8 dc dr dw dh cw ch
  dc=$(( c1 - c0 )); dw=$(( w1 - w0 )); dr=$(( r1 - r0 )); dh=$(( h1 - h0 ))
  [ "$dc" -lt 0 ] && { dc=$(( -dc )); dw=$(( -dw )); }
  [ "$dr" -lt 0 ] && { dr=$(( -dr )); dh=$(( -dh )); }
  if [ "$dc" -gt 0 ] && [ "$dw" -gt 0 ]; then cw=$(( (dw * 100 + dc - 1) / dc )); else cw=$(( (w1 * 100 + c1 - 1) / c1 )); fi
  if [ "$dr" -gt 0 ] && [ "$dh" -gt 0 ]; then ch=$(( (dh * 100 + dr - 1) / dr )); else ch=$(( (h1 * 100 + r1 - 1) / r1 )); fi
  echo "$cw $ch"
}

# gorilla_fit WANT_C WANT_R COLS ROWS WIN_W WIN_H CELL_W CELL_H VF_W VF_H: the size
# to ask for, WANT_C x WANT_R capped to what fits a VF_W x VF_H area, given that
# the window measures WIN_W x WIN_H points at COLS x ROWS cells.
gorilla_fit() {
  local want_c=$1 want_r=$2 cols=$3 rows=$4 win_w=$5 win_h=$6 cw=$7 ch=$8 vf_w=$9 vf_h=${10} max_c max_r
  max_c=$(( cols + $(_gorilla_floordiv $(( (vf_w - win_w) * 100 )) "$cw") ))
  max_r=$(( rows + $(_gorilla_floordiv $(( (vf_h - win_h) * 100 )) "$ch") ))
  [ "$want_c" -gt "$max_c" ] && want_c=$max_c
  [ "$want_r" -gt "$max_r" ] && want_r=$max_r
  [ "$want_c" -lt 1 ] && want_c=1
  [ "$want_r" -lt 1 ] && want_r=1
  echo "$want_c $want_r"
}

# gorilla_center VF_X VF_Y VF_W VF_H WIN_W WIN_H: the bounds (left top right bottom)
# of a WIN_W x WIN_H window centered in the visible area. One that fits stays
# inside; one too big for it (only a window gorilla_fit did not size) keeps its
# top-left corner on the screen, so its title bar can still be dragged.
gorilla_center() {
  local vf_x=$1 vf_y=$2 vf_w=$3 vf_h=$4 win_w=$5 win_h=$6 x y
  x=$(( vf_x + (vf_w - win_w) / 2 )); y=$(( vf_y + (vf_h - win_h) / 2 ))
  [ "$x" -lt "$vf_x" ] && x=$vf_x
  [ "$y" -lt "$vf_y" ] && y=$vf_y
  echo "$x $y $(( x + win_w )) $(( y + win_h ))"
}

# gorilla_window_plan VF_X VF_Y VF_W VF_H COLS ROWS WIN_W WIN_H CELL_W CELL_H WANT_C WANT_R:
# both steps at once, "cols rows left top right bottom", the window's new size
# predicted from the cell size. The launcher centers on the size it measures.
gorilla_window_plan() {
  local fit c r w h
  fit=$(gorilla_fit "${11}" "${12}" "$5" "$6" "$7" "$8" "$9" "${10}" "$3" "$4")
  c=${fit% *}; r=${fit#* }
  w=$(( $7 + $(_gorilla_floordiv $(( (c - $5) * $9 + 50 )) 100) ))
  h=$(( $8 + $(_gorilla_floordiv $(( (r - $6) * ${10} + 50 )) 100) ))
  echo "$c $r $(gorilla_center "$1" "$2" "$3" "$4" "$w" "$h")"
}

if [ "${BASH_SOURCE[0]}" != "$0" ]; then return 0; fi

# --- macOS from here on ---

# What the placement measured and any osascript error, rewritten at each launch.
# A local file only: when the window opens in the wrong place, this says why.
GORILLA_WINDOW_LOG="${TMPDIR:-/tmp}/gorilla-opencode-window.log"

# Prints "vf_x vf_y vf_w vf_h left top right bottom cols rows" for the Terminal
# window whose tab runs on tty $1: the visible frame (the screen without the menu
# bar and the Dock, wherever the Dock is and whether it hides) turned to Terminal's
# top-left origin, then the window's bounds and size in cells. Whole points only,
# so no locale's decimal comma gets in.
gorilla_mac_state() {
  osascript - "$1" <<'APPLESCRIPT' 2>>"$GORILLA_WINDOW_LOG"
use framework "AppKit"
use scripting additions
on rect4(r)
  try
    return {item 1 of item 1 of r, item 2 of item 1 of r, item 1 of item 2 of r, item 2 of item 2 of r}
  on error
    return {x of origin of r, y of origin of r, width of |size| of r, height of |size| of r}
  end try
end rect4
on run argv
  set myTty to item 1 of argv
  set fullH to item 4 of my rect4((current application's NSScreen's screens()'s objectAtIndex:0)'s frame())
  set vf to my rect4(current application's NSScreen's mainScreen()'s visibleFrame())
  set vfTop to fullH - (item 2 of vf) - (item 4 of vf)
  set out to {(item 1 of vf) as integer, vfTop as integer, (item 3 of vf) as integer, (item 4 of vf) as integer}
  tell application "Terminal"
    repeat with w in windows
      repeat with t in tabs of w
        if tty of t is myTty then
          set b to bounds of w
          set out to out & {item 1 of b, item 2 of b, item 3 of b, item 4 of b, number of columns of t, number of rows of t}
          set AppleScript's text item delimiters to " "
          return out as text
        end if
      end repeat
    end repeat
  end tell
  error "no Terminal window on " & myTty
end run
APPLESCRIPT
}

# Moves the window on tty $1 to bounds $2..$5. With $6 = 1 (Terminal started for
# this launch) it also closes the empty window Terminal opens at start: one that
# runs nothing, shows no restored session and holds at most two lines of text.
# Every other window, what was seen in it and whether it was closed, goes to the log.
gorilla_mac_place() {
  osascript - "$@" <<'APPLESCRIPT' >>"$GORILLA_WINDOW_LOG" 2>&1
on run argv
  set myTty to item 1 of argv
  set nb to {(item 2 of argv) as integer, (item 3 of argv) as integer, (item 4 of argv) as integer, (item 5 of argv) as integer}
  tell application "Terminal"
    set mine to missing value
    repeat with w in windows
      repeat with t in tabs of w
        if tty of t is myTty then set mine to contents of w
      end repeat
    end repeat
    if mine is missing value then return "placed: no window on " & myTty
    set bounds of mine to nb
    set report to "placed: " & (count of windows) & " window(s)"
    set mineId to id of mine
    set others to id of every window whose id is not mineId
    repeat with wid in others
      set w to window id (contents of wid)
      set empty to true
      set seen to ""
      repeat with t in tabs of w
        if busy of t then
          set empty to false
          set seen to seen & " busy"
        else
          set h to history of t
          if h contains "Restored session" then
            set empty to false
            set seen to seen & " restored"
          end if
          set n to 0
          repeat with p in paragraphs of h
            if length of (contents of p) > 0 then set n to n + 1
          end repeat
          if n > 2 then set empty to false
          set seen to seen & " " & n & " lines"
        end if
      end repeat
      if empty and item 6 of argv is "1" then
        close w
        set report to report & " | window" & seen & ": closed"
      else
        set report to report & " | window" & seen & ": kept"
      end if
    end repeat
    return report
  end tell
end run
APPLESCRIPT
}

# Asks Terminal for COLSxROWS (an xterm window-size request) and waits until the
# tab reports that size or about a second has passed. Prints the state. The
# request goes to the terminal itself: this runs inside $(...), whose output is
# captured, and on 2026-10-23 the request went into the captured state instead.
gorilla_mac_resize() {
  local tty=$1 c=$2 r=$3 s i=0
  printf '\033[8;%d;%dt' "$r" "$c" >"$tty"
  while :; do
    s=$(gorilla_mac_state "$tty") || return 1
    set -- $s
    { [ "$9" = "$c" ] && [ "${10}" = "$r" ]; } || [ "$i" -ge 10 ] && break
    sleep 0.1; i=$(( i + 1 ))
  done
  echo "$s"
}

# Did Terminal start for this launch? Its process is then less than a minute old.
# The process ids and age go to the log: on the Air (2026-10-23) a launch right
# after quitting Terminal still logged "fresh 0".
gorilla_terminal_fresh() {
  local pids et
  pids=$(pgrep -x Terminal 2>/dev/null | tr '\n' ' ')
  et=$(ps -o etime= -p "${pids%% *}" 2>/dev/null | tr -d ' ')
  echo "terminal pids [$pids] age [$et]" >>"$GORILLA_WINDOW_LOG"
  case "$et" in ''|*-*|*:*:*) return 1 ;; esac
  [ $(( 10#${et%%:*} )) -eq 0 ]
}

# Sizes and centers this window. Returns 1, leaving the window as it is, when
# osascript cannot see it: an unmeasured window is never grown past the screen.
gorilla_place_window() {
  local want_c=$1 want_r=$2 tty s0 s1 cell fit bounds fresh=0
  tty=$(tty) || return 1
  s0=$(gorilla_mac_state "$tty") || return 1
  s1=$(gorilla_mac_resize "$tty" "$want_c" "$want_r") || return 1
  set -- $s0 $s1
  # $1-$10 before the resize, $11-$20 after: visible frame, bounds, cols, rows.
  cell=$(gorilla_cell "$9" "${10}" $(( $7 - $5 )) $(( $8 - $6 )) "${19}" "${20}" $(( ${17} - ${15} )) $(( ${18} - ${16} )))
  fit=$(gorilla_fit "$want_c" "$want_r" "${19}" "${20}" $(( ${17} - ${15} )) $(( ${18} - ${16} )) ${cell% *} ${cell#* } "${13}" "${14}")
  if [ "$fit" != "${19} ${20}" ]; then
    s1=$(gorilla_mac_resize "$tty" ${fit% *} ${fit#* }) || return 1
  fi
  set -- $s1
  bounds=$(gorilla_center "$1" "$2" "$3" "$4" $(( $7 - $5 )) $(( $8 - $6 )))
  gorilla_terminal_fresh && fresh=1
  echo "tty $tty want ${want_c}x$want_r | before $s0 | cell $cell fit $fit | after $s1 | bounds $bounds fresh $fresh" >>"$GORILLA_WINDOW_LOG"
  gorilla_mac_place "$tty" $bounds "$fresh"
}

[ -f "$HOME/.bash_profile" ] && source "$HOME/.bash_profile"
export PATH="$HOME/.local/bin:/opt/local/bin:$PATH"

if ! command -v gorilla-opencode >/dev/null 2>&1; then
  echo "gorilla-opencode not found in ~/.local/bin"; read -n1 -rp "Press any key to close."; exit 1
fi

want="${GORILLA_WINDOW:-167x39}"
cols="${want%x*}"; rows="${want#*x}"
case "$cols$rows" in *[!0-9]*|"") cols=167; rows=39 ;; esac
: >"$GORILLA_WINDOW_LOG" 2>/dev/null || GORILLA_WINDOW_LOG=/dev/null
gorilla_place_window "$cols" "$rows"
clear

DIR=$(osascript -e 'POSIX path of (choose folder with prompt "Work in which folder?" default location (path to home folder))' 2>/dev/null)
cd "${DIR:-$HOME}" || cd "$HOME"
printf '\033]0;Gorilla OpenCode — %s\007' "$(basename "$PWD")"

gorilla-opencode
echo; read -n1 -rp "Gorilla OpenCode exited. Press any key to close."
