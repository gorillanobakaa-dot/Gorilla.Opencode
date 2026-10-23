#!/usr/bin/env python3
"""Pack the macOS 10.13 build: a folder a person double-clicks in, and one .zip
of it for the release page.

WHY
    The 2026-10-14 Mac binary travelled with no checksum and no notes, and
    installing it meant typing commands. Written here (not with Windows tar or
    zip) so every script and the program keep their execute bit and every text
    file has LF line endings; a Mac icon (.icns) is built from the repo's PNGs.

USAGE
    python build_mac_legacy_package.py <repo> <binary> <version> <out-dir>

OUTPUT, in <out-dir>:
    gorilla-opencode-<v>-mac/                      the folder for a USB stick
        Install Gorilla OpenCode.command           double-click to install
        Undo Gorilla OpenCode.command              double-click to go back
        READ-ME-FIRST.txt
        gorilla-opencode-<v>-mac-10.13-x86_64.tar.gz   program + app, checked by the installer
        SHA256SUMS-<v>-mac.txt
    gorilla-opencode-<v>-mac-10.13.zip             that folder, for the release page
"""
import hashlib
import io
import plistlib
import struct
import sys
import tarfile
import time
import zipfile
from pathlib import Path

README = """GORILLA OPENCODE {version} FOR MAC (macOS 10.13 High Sierra and later, Intel)
=============================================================================

TO INSTALL
1. Double-click  "Install Gorilla OpenCode"  in this folder.
   If the Mac says it "can't be opened because it is from an unidentified
   developer": right-click it (or hold Ctrl and click it), choose Open, then
   click Open. This happens once, for a file downloaded with a web browser.
2. A Terminal window shows each step and ends with DONE (or STOPPED and why;
   then nothing was changed). Press any key to close it.
3. Double-click  "Gorilla OpenCode"  on your Desktop to start the program.

WHAT THE INSTALLER DOES
- checks the download is the file that was published (SHA-256);
- tests the new program on this Mac before it replaces anything;
- moves the version you had to the Trash (Finder's "Put Back" restores it);
- installs into your home folder: no password, nothing system-wide;
- tells macOS the files are yours, so it does not block them;
- lets you type  gorilla-opencode  in Terminal;
- puts "Gorilla OpenCode" in your Applications folder, with an icon on the Desktop;
- tells Terminal not to reopen the windows that were open when you quit it,
  so the program starts in one window. Terminal's setting as it was is
  written down first.

TO GO BACK to the version you had: double-click  "Undo Gorilla OpenCode".
It also gives Terminal back its setting for reopening old windows.

THE WINDOW
The app opens Terminal at 167 x 39, or the largest size that fits your screen,
centered, above the Dock. Drag it to any size: every screen follows. For
another starting size, put a line such as
    export GORILLA_WINDOW=144x33
in the file .bash_profile in your home folder (it is still kept on the screen).

CLEARER LETTERS
Terminal > Preferences > Profiles > Text > Font: Menlo, or SF Mono (in
/Applications/Utilities/Terminal.app/Contents/Resources/Fonts: open each file
and click Install Font).

WITHOUT DOUBLE-CLICKING (the same installer, typed once in Terminal):
    bash "<this folder>/Install Gorilla OpenCode.command"
(type  bash  and a space, drag the installer onto the Terminal window, press Enter)

KNOWN ON 10.13: the search tool (find) wants python3 and the text search wants
ripgrep (rg); 10.13 has neither. Both are optional: the program says so and
carries on.
"""


def sha(data):
    return hashlib.sha256(data).hexdigest()


def icns(repo):
    """A Mac icon from PNGs: ic07=128, ic08=256, ic09=512, ic10=1024 (all PNG,
    which macOS reads since 10.7)."""
    parts = b""
    for code, size in (("ic07", 128), ("ic08", 256), ("ic09", 512), ("ic10", 1024)):
        png = (repo / "internal" / "assets" / "icons" / f"gorilla-opencode-{size}.png").read_bytes()
        if png[:8] != b"\x89PNG\r\n\x1a\n":
            sys.exit(f"gorilla-opencode-{size}.png is not a PNG")
        parts += code.encode() + struct.pack(">I", len(png) + 8) + png
    return b"icns" + struct.pack(">I", len(parts) + 8) + parts


def lf(data):
    return data.replace(b"\r\n", b"\n")


def add_tar(tar, name, data, mode):
    info = tarfile.TarInfo(name)
    info.size, info.mode, info.mtime = len(data), mode, int(time.time())
    if name.endswith("/"):
        info.type, info.size = tarfile.DIRTYPE, 0
        tar.addfile(info)
        return
    tar.addfile(info, io.BytesIO(data))


def add_zip(z, name, data, mode):
    zi = zipfile.ZipInfo(name, time.localtime()[:6])
    zi.create_system = 3  # unix, so the mode below is honoured by Archive Utility
    zi.external_attr = (0o100000 | mode) << 16
    zi.compress_type = zipfile.ZIP_DEFLATED
    z.writestr(zi, data)


def main():
    if len(sys.argv) != 5:
        sys.exit(__doc__)
    repo, binary, version, out = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3], Path(sys.argv[4])
    prog = binary.read_bytes()
    if f"v{version}".encode() not in prog:
        sys.exit(f"the binary does not carry the version stamp v{version}")
    mac = repo / "packaging" / "macos"
    plist = (mac / "Info.plist").read_bytes().replace(b"@VERSION@", version.encode())
    plistlib.loads(plist)  # refuses a broken plist here, not on the Mac
    app = "Gorilla OpenCode.app/Contents/"
    top = f"gorilla-opencode-{version}-mac"

    # The archive the installer unpacks: program + app.
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz", format=tarfile.USTAR_FORMAT) as tar:
        add_tar(tar, f"{top}/gorilla-opencode-mac", prog, 0o755)
        for d in ("", "MacOS/", "Resources/"):
            add_tar(tar, f"{top}/{app}{d}", b"", 0o755)
        add_tar(tar, f"{top}/{app}Info.plist", plist, 0o644)
        add_tar(tar, f"{top}/{app}MacOS/gorilla-opencode-launch", lf((mac / "gorilla-opencode-launch").read_bytes()), 0o755)
        add_tar(tar, f"{top}/{app}Resources/gorilla.icns", icns(repo), 0o644)
        add_tar(tar, f"{top}/{app}Resources/Gorilla OpenCode.command", lf((mac / "Gorilla OpenCode.command").read_bytes()), 0o755)
    tgz_name = f"gorilla-opencode-{version}-mac-10.13-x86_64.tar.gz"
    tgz = buf.getvalue()

    files = {
        "Install Gorilla OpenCode.command": (lf((mac / "Install Gorilla OpenCode.command").read_bytes()), 0o755),
        "Undo Gorilla OpenCode.command": (lf((mac / "Undo Gorilla OpenCode.command").read_bytes()), 0o755),
        "READ-ME-FIRST.txt": (README.format(version=version).encode("utf-8"), 0o644),
        tgz_name: (tgz, 0o644),
        f"SHA256SUMS-{version}-mac.txt": (f"{sha(tgz)}  {tgz_name}\n".encode("ascii"), 0o644),
    }
    folder = out / top
    folder.mkdir(parents=True, exist_ok=True)
    for name, (data, _) in files.items():
        (folder / name).write_bytes(data)
    zpath = out / f"gorilla-opencode-{version}-mac-10.13.zip"
    with zipfile.ZipFile(zpath, "w") as z:
        for name, (data, mode) in files.items():
            add_zip(z, f"{top}/{name}", data, mode)
    print(f"folder {folder}")
    print(f"zip    {zpath} ({zpath.stat().st_size:,} bytes, sha256 {sha(zpath.read_bytes())})")
    print(f"tar    {tgz_name} ({len(tgz):,} bytes, sha256 {sha(tgz)})")


if __name__ == "__main__":
    main()
