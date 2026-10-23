#!/bin/bash
# Double-click to install Gorilla OpenCode on a Mac (macOS 10.13 and later, Intel).
#
# Keep this file in the folder with the release (the .tar.gz and its
# SHA256SUMS file) and double-click it. It:
#   1. checks the download is the file that was published (SHA-256);
#   2. unpacks it in a temporary folder and test-runs the new program;
#   3. moves the version you have now to the Trash (Finder's "Put Back"
#      restores it);
#   4. installs the program in your home folder (no password, nothing system-wide);
#   5. tells macOS the files are yours, so it does not block them;
#   6. lets you type  gorilla-opencode  in Terminal;
#   7. installs "Gorilla OpenCode" in your Applications folder, with an icon
#      on the Desktop: double-click it to start;
#   8. tells Terminal not to reopen the windows that were open when it was
#      quit, so the program starts in one window (owner's choice, 2026-10-23,
#      after old windows came back behind it). Terminal's setting as it was
#      before is written down first, and Undo puts it back exactly.
# It stops at the first thing that is wrong and says what. Nothing is replaced
# until the new program has been tested on this Mac.
# "Undo Gorilla OpenCode.command" brings the previous version back.
#
# Where things go (all inside your home folder):
#   ~/.local/bin/gorilla-opencode               the program
#   ~/Applications/Gorilla OpenCode.app         the app you double-click
#   ~/Desktop/Gorilla OpenCode                  an icon pointing at that app
#   ~/.local/share/gorilla-opencode/terminal-window-restore.before
#                                               Terminal's setting before step 8

HERE="$(cd "$(dirname "$0")" && pwd)"
BIN="$HOME/.local/bin"
APPS="$HOME/Applications"
APP="$APPS/Gorilla OpenCode.app"
say() { printf '%s\n' "$*"; }
stop() {
  say ""
  say "STOPPED: $*"
  say "Your current version was not changed after this point."
  say ""
  read -n1 -rp "Press any key to close."
  exit 1
}
# The Trash, the Finder way first so "Put Back" works; a plain move into
# ~/.Trash if Finder cannot be asked.
to_trash() {
  [ -e "$1" ] || [ -L "$1" ] || return 0
  if osascript -e "tell application \"Finder\" to delete (POSIX file \"$1\" as alias)" >/dev/null 2>&1; then
    return 0
  fi
  mkdir -p "$HOME/.Trash" && mv "$1" "$HOME/.Trash/$(basename "$1") $(date +%Y-%m-%d.%H.%M.%S)"
}

# Terminal's own "reopen windows after a quit" setting. It is written down once,
# before the first change ("unset" when Terminal has none, otherwise its type and
# value), so a second install never records the installer's own value as yours;
# Undo puts it back and removes the record. Without a record nothing is changed.
# cmd/macos_launcher_test.go runs this and Undo's terminal_restore_back with a
# stand-in for defaults.
RESTORE_KEY=NSQuitAlwaysKeepsWindows
RESTORE_REC="$HOME/.local/share/gorilla-opencode/terminal-window-restore.before"
terminal_restore_off() {
  local v t
  if [ ! -f "$RESTORE_REC" ]; then
    mkdir -p "$(dirname "$RESTORE_REC")" || return 1
    if v=$(defaults read com.apple.Terminal "$RESTORE_KEY" 2>/dev/null); then
      t=$(defaults read-type com.apple.Terminal "$RESTORE_KEY" 2>/dev/null); t=${t#Type is }
      case "$t" in boolean|integer|float|string) ;; *) return 1 ;; esac
      printf '%s %s\n' "$t" "$v" >"$RESTORE_REC" || return 1
    else
      echo unset >"$RESTORE_REC" || return 1
    fi
  fi
  defaults write com.apple.Terminal "$RESTORE_KEY" -bool false
}

say "Gorilla OpenCode installer"
say "=========================="
say ""

# 1. The release in this folder, and its checksum.
cd "$HERE" || stop "cannot open the folder $HERE"
TGZ=""
for f in gorilla-opencode-*-mac-10.13-x86_64.tar.gz; do
  [ -f "$f" ] && TGZ="$f"
done
[ -n "$TGZ" ] || stop "there is no gorilla-opencode-...-mac-10.13-x86_64.tar.gz next to this installer"
VERSION="${TGZ#gorilla-opencode-}"
VERSION="${VERSION%-mac-10.13-x86_64.tar.gz}"
SUMS="SHA256SUMS-$VERSION-mac.txt"
say "Installing version $VERSION."
[ -f "$SUMS" ] || stop "the checksum file $SUMS is missing, so the download cannot be checked"
shasum -a 256 -c "$SUMS" >/dev/null 2>&1 || stop "the download does not match its checksum: it is damaged or was changed. Download or copy it again."
say "[ok] The download is intact."

# 2. Unpack in a temporary folder and test-run it there.
WORK="$(mktemp -d "${TMPDIR:-/tmp}/gorilla-install.XXXXXX")" || stop "cannot make a temporary folder"
tar xzf "$TGZ" -C "$WORK" || stop "the archive could not be unpacked"
SRC="$WORK/gorilla-opencode-$VERSION-mac"
[ -f "$SRC/gorilla-opencode-mac" ] || stop "the program is not inside the archive"
[ -d "$SRC/Gorilla OpenCode.app" ] || stop "the app is not inside the archive"
xattr -dr com.apple.quarantine "$SRC" 2>/dev/null
chmod +x "$SRC/gorilla-opencode-mac" "$SRC/Gorilla OpenCode.app/Contents/MacOS/gorilla-opencode-launch" \
  "$SRC/Gorilla OpenCode.app/Contents/Resources/Gorilla OpenCode.command"
GOT="$("$SRC/gorilla-opencode-mac" --version 2>/dev/null | tail -1)"
[ "$GOT" = "v$VERSION" ] || stop "the new program does not start on this Mac (it printed: ${GOT:-nothing})"
say "[ok] The new program starts on this Mac ($GOT)."

# 3. The current version goes to the Trash, named with its version.
mkdir -p "$BIN" || stop "cannot create $BIN"
if [ -f "$BIN/gorilla-opencode" ]; then
  OLD="$("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
  OLD="${OLD:-unknown version}"
  if [ "$OLD" = "v$VERSION" ]; then
    # the same version again (a repair): an identical copy in the Trash would
    # only make Undo "go back" to what is already installed
    say "[ok] $OLD is already installed; it is installed again over itself."
  else
    mv "$BIN/gorilla-opencode" "$BIN/gorilla-opencode $OLD" || stop "could not set the current version aside"
    to_trash "$BIN/gorilla-opencode $OLD" || stop "could not move the current version to the Trash"
    say "[ok] Your current version ($OLD) is in the Trash. \"Put Back\" in the Trash, or Undo, restores it."
  fi
fi
for old in "$HOME/Desktop/Gorilla OpenCode.command" "$HOME/Desktop/Gorilla OpenCode" "$APP"; do
  if [ -e "$old" ] || [ -L "$old" ]; then
    to_trash "$old" && say "[ok] The old \"$(basename "$old")\" is in the Trash."
  fi
done

# 4-5. Install, and tell macOS the files are ours.
cp "$SRC/gorilla-opencode-mac" "$BIN/gorilla-opencode" || stop "could not copy the program into $BIN"
chmod +x "$BIN/gorilla-opencode"
xattr -d com.apple.quarantine "$BIN/gorilla-opencode" 2>/dev/null
GOT="$("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
[ "$GOT" = "v$VERSION" ] || stop "installed, but it reports ${GOT:-nothing}. Double-click Undo to go back."
say "[ok] Installed the program: $BIN/gorilla-opencode"

# 6. Terminal can find it by name. Added once, never twice.
for rc in "$HOME/.bash_profile" "$HOME/.zprofile"; do
  case "$rc" in *zprofile) case "$SHELL" in */zsh) ;; *) continue ;; esac ;; esac
  if ! grep -qs 'GORILLA OPENCODE PATH' "$rc"; then
    printf '\n# GORILLA OPENCODE PATH (added by the installer): lets Terminal find gorilla-opencode\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc" \
      && say "[ok] Terminal will find  gorilla-opencode  by name (added to $(basename "$rc"))."
  fi
done

# 7. The app, and its icon on the Desktop.
mkdir -p "$APPS" || stop "cannot create $APPS"
cp -R "$SRC/Gorilla OpenCode.app" "$APPS/" || stop "the program is installed, but the app could not be copied"
xattr -dr com.apple.quarantine "$APP" 2>/dev/null
touch "$APP"   # Finder re-reads the icon
ln -s "$APP" "$HOME/Desktop/Gorilla OpenCode" 2>/dev/null || stop "the app is installed, but the Desktop icon could not be made"
say "[ok] \"Gorilla OpenCode\" is in your Applications folder, with an icon on the Desktop."

# 8. One window: Terminal stops reopening the windows open at its last quit.
if terminal_restore_off; then
  say "[ok] Terminal will not reopen old windows after a quit (Undo turns this back)."
else
  say "[--] Terminal's window setting was left as it is: it could not be read and written down first."
fi

rm -rf "$WORK"
say ""
say "DONE. Gorilla OpenCode $VERSION is installed."
say "Start it: double-click \"Gorilla OpenCode\" on the Desktop."
say "Go back: double-click \"Undo Gorilla OpenCode\" next to this installer."
say ""
read -n1 -rp "Press any key to close."
