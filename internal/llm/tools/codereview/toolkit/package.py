#!/usr/bin/env python3
"""Build portable, self-contained bundles of the toolkit -- one per platform.

    python3 package.py                 # build both, into ./dist
    python3 package.py --only linux    # or windows
    python3 package.py --version 1.1.0

WHY TWO DIFFERENT ARCHIVE FORMATS, RATHER THAN TWO ZIPS
-------------------------------------------------------
A zip does not carry the Unix executable bit. Ship Linux users a zip and
their first experience is `permission denied`, fixed only by a chmod they
have to be told about. tar.gz stores the mode, so `./review.sh` works the
moment it is extracted. Windows does not have an executable bit at all and
Explorer opens zips natively, so zip is right there.

The second reason is line endings, which is the same trap in a different
coat. A shell script with CRLF fails on Linux with

    bad interpreter: /bin/sh^M

because the carriage return becomes part of the interpreter path. Meanwhile
.cmd files genuinely want CRLF. So this script does not copy bytes blindly:
it normalises every text file to the endings its target platform expects.
That matters here specifically because the source tree this builds from may
be a Windows checkout where git has already rewritten everything to CRLF.

WHAT MAKES THE BUNDLES PORTABLE
-------------------------------
The toolkit imports nothing outside the standard library, so a bundle needs
no pip, no venv, and no network. Python 3.7 or newer and the archive is the
whole dependency list. The external review tools (ruff, gosec, clang-tidy,
shellcheck...) are a separate matter -- they are what the toolkit SHELLS OUT
to, they are not bundled, and `--doctor` reports which of them this machine
actually has. That is a deliberate split: bundling them would mean shipping
platform-specific binaries and the portability would be a fiction.
"""

from __future__ import annotations

import argparse
import io
import os
import shutil
import sys
import tarfile
import time
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_VERSION = "1.1.0"

# Everything a bundle needs. Directories are taken whole.
PAYLOAD_FILES = [
    "code_review.py", "patch_port.py", "doctor.py", "findings.py",
    "heuristics.py", "install_tools.py", "llm_client.py", "local_tier.py",
    "rules.py", "tools_registry.py", "baseline.py",
    "README.md", "PLAYBOOK.md",
]
PAYLOAD_DIRS = ["rule_docs", "configs", "tests"]

# Suffixes treated as text, and therefore line-ending normalised. Anything
# not listed is copied byte for byte -- guessing wrong on a binary corrupts
# it, so the list is explicit rather than heuristic.
TEXT_SUFFIXES = {".py", ".md", ".json", ".yaml", ".yml", ".txt", ".cfg",
                 ".ini", ".toml", ".sh", ".cmd", ".ps1"}

EXECUTABLE = {".sh"}          # gets mode 0755 in the tarball


# ─────────────────────────────────────────────────────────────────────────────
# launchers
# ─────────────────────────────────────────────────────────────────────────────

WIN_REVIEW = r"""@echo off
setlocal
call :findpy || exit /b 9009
%PY% "%~dp0code_review.py" %*
exit /b %errorlevel%

:findpy
rem Pick a Python that actually RUNS, not merely one that is on PATH.
rem Those are different things on Windows: py.exe is installed into
rem System32 and survives there after the Python it points at has been
rem uninstalled, or when its config names another user's profile. It is
rem then found by `where` and fails at launch with
rem     Unable to create process using '...python.exe'
rem so existence is not a usable test. Each candidate is probed by running
rem it, and the first one that can import sys wins.
set "PY="
py -3 -c "import sys" >nul 2>&1 && (set "PY=py -3" & exit /b 0)
python -c "import sys" >nul 2>&1 && (set "PY=python" & exit /b 0)
python3 -c "import sys" >nul 2>&1 && (set "PY=python3" & exit /b 0)
echo No working Python 3 interpreter was found.
echo.
echo This toolkit needs Python 3.7 or newer on PATH. Install it with:
echo     winget install Python.Python.3.12
echo or from https://www.python.org/downloads/
exit /b 1
"""

WIN_PATCH = r"""@echo off
setlocal
call :findpy || exit /b 9009
%PY% "%~dp0patch_port.py" %*
exit /b %errorlevel%

:findpy
rem See review.cmd -- a Python on PATH is not necessarily a Python that runs.
set "PY="
py -3 -c "import sys" >nul 2>&1 && (set "PY=py -3" & exit /b 0)
python -c "import sys" >nul 2>&1 && (set "PY=python" & exit /b 0)
python3 -c "import sys" >nul 2>&1 && (set "PY=python3" & exit /b 0)
echo No working Python 3 interpreter was found.
echo.
echo This toolkit needs Python 3.7 or newer on PATH. Install it with:
echo     winget install Python.Python.3.12
echo or from https://www.python.org/downloads/
exit /b 1
"""

NIX_REVIEW = """#!/bin/sh
# Resolve the bundle directory even when invoked through a symlink or from
# another working directory, so this can be dropped on PATH.
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# python3 is near-universal, but probe rather than assume: some minimal
# containers ship only `python`, and a broken symlink on PATH should produce
# a clear message here instead of a confusing traceback later.
for c in python3 python; do
    if "$c" -c "import sys; sys.exit(0 if sys.version_info >= (3,7) else 1)" 2>/dev/null; then
        exec "$c" "$here/code_review.py" "$@"
    fi
done
echo "No Python 3.7+ interpreter found (tried python3, python)." >&2
echo "Install one with your package manager, e.g. apt install python3" >&2
exit 1
"""

NIX_PATCH = """#!/bin/sh
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# See review.sh.
for c in python3 python; do
    if "$c" -c "import sys; sys.exit(0 if sys.version_info >= (3,7) else 1)" 2>/dev/null; then
        exec "$c" "$here/patch_port.py" "$@"
    fi
done
echo "No Python 3.7+ interpreter found (tried python3, python)." >&2
echo "Install one with your package manager, e.g. apt install python3" >&2
exit 1
"""

WIN_README = """CODE REVIEW TOOLKIT {ver} -- WINDOWS BUNDLE
{rule}

REQUIREMENTS
    Python 3.7 or newer on PATH. Nothing else. No pip install, no venv.
    Check with:  python --version

START HERE
    review.cmd --doctor

    That scans this machine and tells you which review tools you actually
    have, which ones it can install for you, and which ones do not exist on
    Windows at all. Read it before anything else -- a review is only as good
    as the tools behind it, and the doctor is what stops the toolkit from
    quietly reporting "no issues" when the truth is "no tools".

REVIEWING
    review.cmd C:\\path\\to\\your\\project
    review.cmd . --profile security
    review.cmd . --json > findings.json

PORTING PATCHES
    patch-port.cmd --help
    patch-port.cmd --tree C:\\src\\linux --op forward-port --series ..\\patches

    Forward-port, backport, rebase, refresh, or port a whole series. It
    reports HOW each patch applied, not just whether it did -- clean, merged
    three-way, relocated by fuzz, or conflicted. Those are not the same
    thing and the difference is what tells you how hard to review the result.

A NOTE ON WHAT IS NOT IN HERE
    The external analysers (ruff, gosec, clang-tidy, shellcheck, semgrep...)
    are not bundled. They are separate programs the toolkit runs. Bundling
    them would mean shipping platform-specific binaries and pretending the
    result is portable. `--doctor` tells you where you stand; `--install`
    offers to fetch what winget, scoop or choco can provide.
"""

NIX_README = """CODE REVIEW TOOLKIT {ver} -- LINUX / UNIX BUNDLE
{rule}

REQUIREMENTS
    Python 3.7 or newer. Nothing else. No pip install, no venv.
    Check with:  python3 --version

START HERE
    ./review.sh --doctor

    That scans this machine and tells you which review tools you actually
    have, which ones it can install for you, and which ones are not
    available on this platform. Read it before anything else -- a review is
    only as good as the tools behind it, and the doctor is what stops the
    toolkit from quietly reporting "no issues" when the truth is "no tools".

REVIEWING
    ./review.sh /path/to/your/project
    ./review.sh . --profile security
    ./review.sh . --json > findings.json

PORTING PATCHES
    ./patch-port.sh --help
    ./patch-port.sh --tree ~/src/linux --op forward-port --series ../patches

    Forward-port, backport, rebase, refresh, or port a whole series. It
    reports HOW each patch applied, not just whether it did -- clean, merged
    three-way, relocated by fuzz, or conflicted. Those are not the same
    thing and the difference is what tells you how hard to review the result.

A NOTE ON WHAT IS NOT IN HERE
    The external analysers (ruff, gosec, clang-tidy, shellcheck, semgrep...)
    are not bundled. They are separate programs the toolkit runs. Bundling
    them would mean shipping platform-specific binaries and pretending the
    result is portable. `--doctor` tells you where you stand; `--install`
    offers to fetch what apt, dnf, pacman or brew can provide.

IF THE LAUNCHERS ARE NOT EXECUTABLE
    This tarball stores the executable bit, so they should be. If you
    re-packed it as a zip somewhere along the way, zip drops that bit:
        chmod +x review.sh patch-port.sh
"""


# ─────────────────────────────────────────────────────────────────────────────

def is_text(name):
    return os.path.splitext(name)[1].lower() in TEXT_SUFFIXES


def read_normalised(path, newline):
    """Read a file and return bytes with the requested line endings.

    Text is decoded, stripped of every CR, then re-joined -- rather than
    replacing CRLF with LF -- so a file that has been round-tripped through
    several tools and ended up with CRCRLF still comes out correct.
    """
    with open(path, "rb") as fh:
        raw = fh.read()
    if not is_text(path):
        return raw
    text = raw.decode("utf-8").replace("\r", "")
    if newline == "\r\n":
        text = text.replace("\n", "\r\n")
    return text.encode("utf-8")


def collect(src):
    """The list of (relative path, absolute path) that goes into a bundle."""
    out = []
    for name in PAYLOAD_FILES:
        p = os.path.join(src, name)
        if os.path.exists(p):
            out.append((name, p))
        else:
            print("  warning: missing %s" % name)
    for d in PAYLOAD_DIRS:
        root = os.path.join(src, d)
        if not os.path.isdir(root):
            print("  warning: missing directory %s" % d)
            continue
        for dirpath, dirnames, filenames in os.walk(root):
            dirnames[:] = [x for x in dirnames if x != "__pycache__"]
            for f in sorted(filenames):
                if f.endswith(".pyc"):
                    continue
                ap = os.path.join(dirpath, f)
                out.append((os.path.relpath(ap, src).replace(os.sep, "/"), ap))
    return sorted(out)


def build_windows(src, dist, ver):
    stem = "code-review-toolkit-%s-windows" % ver
    path = os.path.join(dist, stem + ".zip")
    extras = {
        "review.cmd": WIN_REVIEW,
        "patch-port.cmd": WIN_PATCH,
        "README-WINDOWS.txt": WIN_README.format(ver=ver, rule="=" * 44),
    }
    n = 0
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        for rel, ap in collect(src):
            z.writestr(stem + "/" + rel, read_normalised(ap, "\r\n"))
            n += 1
        for rel, body in extras.items():
            z.writestr(stem + "/" + rel, body.replace("\n", "\r\n"))
            n += 1
    return path, n


def build_linux(src, dist, ver):
    stem = "code-review-toolkit-%s-linux" % ver
    path = os.path.join(dist, stem + ".tar.gz")
    extras = {
        "review.sh": NIX_REVIEW,
        "patch-port.sh": NIX_PATCH,
        "README-LINUX.txt": NIX_README.format(ver=ver, rule="=" * 48),
    }
    now = int(time.time())
    n = 0

    def add(tf, rel, data, mode):
        info = tarfile.TarInfo(stem + "/" + rel)
        info.size = len(data)
        info.mode = mode
        info.mtime = now
        # A tarball that records the building machine's user is noise at best
        # and an information leak at worst.
        info.uid = info.gid = 0
        info.uname = info.gname = "root"
        tf.addfile(info, io.BytesIO(data))

    with tarfile.open(path, "w:gz") as tf:
        for rel, ap in collect(src):
            data = read_normalised(ap, "\n")
            mode = 0o755 if os.path.splitext(rel)[1] in EXECUTABLE else 0o644
            add(tf, rel, data, mode)
            n += 1
        for rel, body in extras.items():
            data = body.replace("\r\n", "\n").encode("utf-8")
            mode = 0o755 if os.path.splitext(rel)[1] in EXECUTABLE else 0o644
            add(tf, rel, data, mode)
            n += 1
    return path, n


def main():
    ap = argparse.ArgumentParser(description="Build portable toolkit bundles.")
    ap.add_argument("--only", choices=["windows", "linux"],
                    help="build just one bundle (default: both)")
    ap.add_argument("--version", default=DEFAULT_VERSION)
    ap.add_argument("--src", default=HERE, help="source tree to package")
    # Default OUTSIDE this directory. The toolkit tree is embedded into the
    # Gorilla binary wholesale, so bundles written beside the source would
    # be compiled into it -- a binary carrying a zip of itself, and a
    # content hash that changes every time someone rebuilds the bundles.
    ap.add_argument("--dist",
                    default=os.path.join(os.path.dirname(HERE), "dist"))
    args = ap.parse_args()

    dist = os.path.abspath(args.dist)
    if os.path.commonpath([dist, HERE]) == HERE:
        sys.stderr.write(
            "refusing to build into %s\n"
            "That is inside the toolkit tree, which is embedded into the\n"
            "Gorilla binary wholesale -- the bundles would be compiled into\n"
            "it. Pick a --dist outside the toolkit directory.\n" % dist)
        return 2
    args.dist = dist
    if os.path.isdir(args.dist):
        shutil.rmtree(args.dist)
    os.makedirs(args.dist)

    targets = [args.only] if args.only else ["windows", "linux"]
    for t in targets:
        print("building %s bundle..." % t)
        fn = build_windows if t == "windows" else build_linux
        path, n = fn(args.src, args.dist, args.version)
        size = os.path.getsize(path)
        print("  %s  (%d files, %.1f KB)"
              % (os.path.basename(path), n, size / 1024.0))
    print()
    print("bundles in %s" % args.dist)
    return 0


if __name__ == "__main__":
    sys.exit(main())
