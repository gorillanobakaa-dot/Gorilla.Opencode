#!/bin/bash
# Double-click to install Gorilla OpenCode on a Mac (macOS 10.13 and later, Intel).
#
# Run from the folder that holds the release: the .tar.gz and its SHA256SUMS
# file. It checks the download, unpacks it in your home folder, keeps a backup
# of the version you have now, installs the new one, checks it starts, and puts
# the double-click launcher on the Desktop. It stops at the first thing that is
# wrong and says what it was; nothing is installed after a failed check.
# "Undo Gorilla OpenCode.command" puts the previous version back.
#
# Nothing here asks for your password or touches anything outside your home
# folder: ~/.local/bin, ~/Downloads and ~/Desktop.

HERE="$(cd "$(dirname "$0")" && pwd)"
BIN="$HOME/.local/bin"
say() { printf '%s\n' "$*"; }
stop() {
  say ""
  say "STOPPED: $*"
  say "Nothing was changed after this point."
  say ""
  read -n1 -rp "Press any key to close."
  exit 1
}

say "Gorilla OpenCode installer"
say "=========================="
say ""

# 1. Find the release in this folder.
cd "$HERE" || stop "cannot open the folder $HERE"
TGZ=""
for f in gorilla-opencode-*-mac-10.13-x86_64.tar.gz; do
  [ -f "$f" ] && TGZ="$f"
done
[ -n "$TGZ" ] || stop "no gorilla-opencode-...-mac-10.13-x86_64.tar.gz in $HERE"
VERSION="${TGZ#gorilla-opencode-}"
VERSION="${VERSION%-mac-10.13-x86_64.tar.gz}"
SUMS="SHA256SUMS-$VERSION-mac.txt"
say "Found version $VERSION."

# 2. Check the download is the file that was published.
[ -f "$SUMS" ] || stop "the checksum file $SUMS is missing, so the download cannot be checked"
if ! shasum -a 256 -c "$SUMS" >/dev/null 2>&1; then
  stop "the download does not match its checksum: it was damaged or changed. Copy it again."
fi
say "[ok] The download is intact (checksum matches)."

# 3. Unpack in the home folder, never on the stick.
WORK="$HOME/Downloads/gorilla-$VERSION"
mkdir -p "$WORK" || stop "cannot create $WORK"
tar xzf "$TGZ" -C "$WORK" || stop "the archive could not be unpacked"
SRC="$WORK/gorilla-opencode-$VERSION-mac"
[ -f "$SRC/gorilla-opencode-mac" ] || stop "the program is not inside the archive"
xattr -d com.apple.quarantine "$SRC/gorilla-opencode-mac" "$SRC/Gorilla OpenCode.command" 2>/dev/null
chmod +x "$SRC/gorilla-opencode-mac" "$SRC/Gorilla OpenCode.command"
say "[ok] Unpacked into $WORK"

# 4. Check the new program starts on this Mac BEFORE replacing anything.
GOT="$("$SRC/gorilla-opencode-mac" --version 2>/dev/null | tail -1)"
[ "$GOT" = "v$VERSION" ] || stop "the new program did not start on this Mac (it printed: ${GOT:-nothing}). Your current version is untouched."
say "[ok] The new program starts and reports $GOT."

# 5. Keep the current version.
mkdir -p "$BIN" || stop "cannot create $BIN"
if [ -f "$BIN/gorilla-opencode" ]; then
  OLD="$("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
  cp -p "$BIN/gorilla-opencode" "$BIN/gorilla-opencode.previous" || stop "could not back up the current version"
  printf '%s\n' "${OLD:-unknown}" > "$BIN/gorilla-opencode.previous.version"
  say "[ok] Your current version (${OLD:-unknown}) is kept as $BIN/gorilla-opencode.previous"
fi

# 6. Install.
cp "$SRC/gorilla-opencode-mac" "$BIN/gorilla-opencode.new" && mv -f "$BIN/gorilla-opencode.new" "$BIN/gorilla-opencode" \
  || stop "could not copy the program into $BIN"
chmod +x "$BIN/gorilla-opencode"
GOT="$("$BIN/gorilla-opencode" --version 2>/dev/null | tail -1)"
[ "$GOT" = "v$VERSION" ] || stop "installed, but it reports ${GOT:-nothing} instead of v$VERSION. Double-click Undo to go back."
say "[ok] Installed: $BIN/gorilla-opencode ($GOT)"

# 7. The launcher on the Desktop (an older copy is kept beside it).
if [ -f "$HOME/Desktop/Gorilla OpenCode.command" ]; then
  cp -p "$HOME/Desktop/Gorilla OpenCode.command" "$HOME/Desktop/.Gorilla OpenCode.command.previous" 2>/dev/null
fi
cp "$SRC/Gorilla OpenCode.command" "$HOME/Desktop/" && chmod +x "$HOME/Desktop/Gorilla OpenCode.command" \
  || stop "the program is installed, but the launcher could not be put on the Desktop"
say "[ok] \"Gorilla OpenCode\" is on your Desktop."

say ""
say "DONE. Gorilla OpenCode $VERSION is installed."
say "Double-click \"Gorilla OpenCode\" on the Desktop to start it."
say "To go back to the version you had before, double-click \"Undo Gorilla OpenCode\"."
say ""
read -n1 -rp "Press any key to close."
