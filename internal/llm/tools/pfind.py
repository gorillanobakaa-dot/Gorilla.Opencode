#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# pfind — Unified hybrid local search, content extraction, and filesystem exploration.
# Copyright (C) 2026  gorillanobakaa
#
# This program is free software: you can redistribute it and/or modify it under
# the terms of the GNU Affero General Public License as published by the Free
# Software Foundation, either version 3 of the License, or (at your option) any
# later version.
#
# This program is distributed in the hope that it will be useful, but WITHOUT ANY
# WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
# PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License along
# with this program. If not, see <https://www.gnu.org/licenses/>.
#
# The deal (copyleft, in one line): use it, fork it, change it, sell it — but any
# version you SHIP or RUN AS A SERVICE must stay open under this same license, so
# every improvement can flow back and the tool keeps growing. Improvements welcome
# upstream: https://github.com/gorillanobakaa-dot/pfind
#
# VERSION: 3.2.0 | UPDATED: 2026-08-17 | STATUS: live
# CHANGELOG:
#   3.2.0 (2026-08-17) — Portability: nothing machine-specific is hardcoded any more. Hostname from
#       socket.gethostname(), workers from os.cpu_count(), --brain/--work/--src preset roots and the
#       Chroma store from ~/.config/pfind/presets.json or PFIND_* env vars. An unconfigured preset
#       refuses with setup instructions instead of guessing. --max now also caps match blocks in
#       context mode (one kernel file with 36 hits printed all 36; 35 KB -> 16.5 KB) and prints
#       '… N more match(es)' when it bites. This version ships embedded inside gorilla-opencode as
#       the engine of its `find` tool.
#   3.1.0 (2026-08-17) — Dead-flag repair + flag-surface completion against eza 0.23.5 / ripgrep 15.2.0.
#       (1) DEAD FLAG REPAIR: fourteen flags were parsed by argparse and never read by any code path
#           (the same class of bug as the 3.0.0 --glob no-op). Now wired end-to-end, rg side and
#           pure-Python side: -o/--only-matching, -m/--max-count, -a/--text, --binary, --max-filesize,
#           --one-file-system, --no-ignore-dot, --no-ignore-global, --no-ignore-parent, --no-ignore-vcs,
#           --color-scale-mode, --time, --show-symlinks, --follow-symlinks.
#       (2) EZA COMPLETION: --across, --width, --numeric, --group, --smart-group, --classify, --absolute,
#           --extended, --flags, --mounts, --short-nix, --stdin, --dereference, --treat-dirs-as-files,
#           --no-permissions, --no-filesize, --no-time, --no-user, --no-quotes, --git-repos,
#           --git-repos-no-status, --git-ignore, --changed; --time-style gains long-iso/full-iso/+FORMAT;
#           --sort gains eza's full vocabulary; --loc/--code gain =lines|percent|both.
#       (3) RIPGREP COMPLETION: --iglob, --glob-case-insensitive, --ignore-file, --no-ignore-exclude,
#           --no-ignore-messages, --no-require-git, --encoding, --engine, --multiline, --line-regexp,
#           --max-columns, --max-columns-preview, --byte-offset, --with-filename, --no-filename, --trim,
#           --passthru, --stop-on-nonmatch, --colors, --hostname-bin, --crlf, --null-data, --no-unicode,
#           --mmap/--no-mmap, --block-buffered, --line-buffered, --dfa-size-limit, --regex-size-limit,
#           --no-config.
#   3.0.0 (2026-08-15) — The Super-Merge: unified ripgrep-15.2.0 + eza-0.23.5 capabilities into pfind.
#       (1) RIPGREP INGESTION: Context lines (-C, -A, -B) with line-range deduplication & '--' markers;
#           archive/compressed search (-z/--search-zip for .gz, .bz2, .xz, .lzma, .zst, .tar, .zip);
#           50+ language type definitions (-t/--type, -T/--type-not); binary NUL-byte heuristic skipping;
#           invert-match (-v), only-matching (-o), regex replacement preview (-r/--replace), count (-c),
#           and search performance statistics (--stats).
#       (2) EZA INGESTION: Comment-aware syntax & LOC counting (--loc, --code) across 50+ languages;
#           Git working-tree status flags ([M], [A], [?], [D], [U], [I]) + --git-dirty filter & boost;
#           Hierarchical tree view (-T/--tree, --level); relative timestamps (--time-style=relative,iso);
#           extended metadata (-l/--long, --octal-permissions, --blocks, --inode, --links, --binary-units);
#           OSC 8 clickable terminal hyperlinks (--hyperlink); Nerd Font icons (--icons); adaptive color scales.
#       (3) EXPLORATORY MODE: Running without a query switches to smart directory/tree/metadata listing.
#       (4) BASELINE PRESERVATION: 100% backward-compatible with 2.x hybrid RRF ranking, coverage scoring,
#           exact/loose snippet search (-x, --loose), architecture presets (--brain/--work/--src/--all),
#           Chroma Second Brain semantic seam, and zero-dependency pure-Python fallback.
#   2.1.0 (2026-07-22) — Needle-in-a-haystack-of-needles upgrade. Exact multi-line snippet search (-x/--loose).
#   2.0.0 (2026-07-22) — Full rewrite. Hybrid search: filename + content + fuzzy, fused with RRF.
#   1.0.0 — original parallel mmap+re grep.
"""
pfind — find stuff, analyze code, and explore filesystems in one shot.

Combines:
  1. ripgrep 15.2.0: SIMD/mmap regex, context windowing, archive decomp, language types, binary heuristics.
  2. eza 0.23.5: Git status badges, syntax-aware LOC lexer, visual trees, relative times, hyperlinks, icons.
  3. pfind 2.1.0: Hybrid RRF fusion, term coverage ranking, snippet matching, Chroma Second Brain semantic seam.
"""

import argparse
import bz2
import datetime
import fnmatch
import gzip
import json
import lzma
import math
import os
try:
    import pwd
    import grp
except ImportError:
    pwd = None
    grp = None
import re
import shlex
import shutil
import socket
import stat
import subprocess
import sys
import tarfile
import time
import zipfile
from collections import defaultdict
from difflib import SequenceMatcher
from pathlib import Path

# GORILLA OVERRIDE (2026-09-01): force UTF-8 on the output streams.
#
# On Windows, Python picks the console's legacy code page for stdout — cp1252 on
# a UK/US install — and pfind's output is full of characters that page cannot
# represent: the tree connectors, the box-drawing rules, the arrows in the long
# view. Printing one of them raises
#
#   UnicodeEncodeError: 'charmap' codec can't encode characters in position 0-2
#
# which kills the whole search. The caller sees "search FAILED", so the failure
# is at least honest, but the tree view simply did not work on Windows at all.
#
# errors="replace" rather than "strict": a single unrepresentable byte in a
# matched line must never cost the user the entire result set. Guarded because
# .reconfigure() is Python 3.7+ and the streams may be replaced by a harness.
for _stream in (sys.stdout, sys.stderr):
    try:
        if (_stream.encoding or "").lower().replace("-", "") != "utf8":
            _stream.reconfigure(encoding="utf-8", errors="replace")
    except (AttributeError, ValueError):
        pass

# ---------------------------------------------------------------------------
# Machine & Preset Resolution — NOTHING machine-specific is hardcoded.
#
# pfind ships inside gorilla-opencode and must behave identically on any
# computer it lands on. The hostname comes from the system, the worker count
# from the CPU, and the --brain/--work/--src preset roots from the user's own
# config file (or environment), never from a path baked in here. A preset that
# is not configured refuses with instructions rather than guessing — silence
# and success must never look alike.
# ---------------------------------------------------------------------------
MACHINE = os.environ.get("PFIND_MACHINE") or socket.gethostname() or "localhost"
LOGICAL_CPUS = os.cpu_count() or 4

HOME = Path.home()
PRESETS_CONFIG = (
    Path(os.environ.get("XDG_CONFIG_HOME") or (HOME / ".config")) / "pfind" / "presets.json"
)

def _load_preset_config():
    """Read the preset map. Environment variables override the config file;
    both are optional. Returns (roots, chroma_path, collection)."""
    roots, chroma, collection = {}, None, "core_memory"
    try:
        with open(PRESETS_CONFIG, "r", encoding="utf-8") as f:
            data = json.load(f)
        for name in ("brain", "work", "src"):
            if data.get(name):
                roots[name] = Path(str(data[name])).expanduser()
        if data.get("chroma"):
            chroma = Path(str(data["chroma"])).expanduser()
        if data.get("collection"):
            collection = str(data["collection"])
    except (OSError, json.JSONDecodeError, ValueError):
        pass  # no config file is a normal state; presets are simply unset
    for name, env in (("brain", "PFIND_BRAIN"), ("work", "PFIND_WORK"), ("src", "PFIND_SRC")):
        if os.environ.get(env):
            roots[name] = Path(os.environ[env]).expanduser()
    if os.environ.get("PFIND_CHROMA"):
        chroma = Path(os.environ["PFIND_CHROMA"]).expanduser()
    if os.environ.get("PFIND_COLLECTION"):
        collection = os.environ["PFIND_COLLECTION"]
    return roots, chroma, collection

PRESET_ROOTS, BRAIN_CHROMA, BRAIN_DEFAULT_COLLECTION = _load_preset_config()

_PRESET_HELP = (
    f"configure it in {PRESETS_CONFIG} "
    '(e.g. {"brain": "~/notes", "work": "~/projects", "src": "~/src", '
    '"chroma": "~/notes/chroma_db"}) or set PFIND_BRAIN / PFIND_WORK / PFIND_SRC'
)

EXCLUDE_GLOBS = [
    ".git", ".hg", ".svn", "node_modules", "__pycache__",
    ".mozbuild", "venv", ".venv", "vector_env", "dist", "build",
    "obj-*",
    "chroma_db", "chroma_fx154",
    "*.sqlite3", "*.bin", "*.parquet", "*.pyc",
    "*.min.js", "*.map",
]

RRF_K_DEFAULT = 60
RRF_WEIGHTS_DEFAULT = {
    "name": 2.0,
    "semantic": 1.5,
    "content": 1.0,
    "git": 0.8,
    "recency": 0.5,
}

HAVE_RG = shutil.which("rg") is not None
HAVE_GIT = shutil.which("git") is not None

# ---------------------------------------------------------------------------
# Language Type Definitions (from ripgrep-15.2.0 crates/ignore/src/default_types.rs)
# ---------------------------------------------------------------------------
LANGUAGE_TYPES = {
    "c": [".c", ".h"],
    "cpp": [".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h", ".C", ".H"],
    "rust": [".rs"],
    "python": [".py", ".pyi", ".pyx"],
    "py": [".py", ".pyi", ".pyx"],
    "go": [".go"],
    "javascript": [".js", ".jsx", ".mjs", ".cjs"],
    "js": [".js", ".jsx", ".mjs", ".cjs"],
    "typescript": [".ts", ".tsx", ".mts", ".cts"],
    "ts": [".ts", ".tsx", ".mts", ".cts"],
    "html": [".html", ".htm", ".xhtml"],
    "css": [".css", ".scss", ".sass", ".less"],
    "json": [".json", ".jsonc", ".jsonl"],
    "yaml": [".yaml", ".yml"],
    "toml": [".toml"],
    "markdown": [".md", ".markdown", ".mdown", ".mkdn"],
    "md": [".md", ".markdown"],
    "sh": [".sh", ".bash", ".zsh", ".ksh"],
    "bash": [".sh", ".bash", ".zsh"],
    "java": [".java", ".jsp"],
    "kotlin": [".kt", ".kts"],
    "swift": [".swift"],
    "ruby": [".rb", ".rake", ".gemspec"],
    "php": [".php", ".phtml", ".php3", ".php4", ".php5"],
    "perl": [".pl", ".pm", ".t"],
    "lua": [".lua"],
    "zig": [".zig"],
    "nim": [".nim", ".nims"],
    "scala": [".scala", ".sc"],
    "sql": [".sql"],
    "xml": [".xml", ".xsl", ".xslt", ".svg", ".plist"],
    "docker": ["Dockerfile", ".dockerignore"],
    "make": ["Makefile", "makefile", ".mk", ".mak"],
    "cmake": ["CMakeLists.txt", ".cmake"],
    "assembly": [".s", ".S", ".asm"],
    "asm": [".s", ".S", ".asm"],
}

# ---------------------------------------------------------------------------
# Comment Syntax Lexer Definitions (from eza-0.23.5 src/loc/mod.rs)
# ---------------------------------------------------------------------------
class LanguageSyntax:
    def __init__(self, name, line_comments, block_comments, extensions, filenames=None):
        self.name = name
        self.line_comments = line_comments
        self.block_comments = block_comments  # list of (open, close)
        self.extensions = set(extensions)
        self.filenames = set(filenames or [])

SYNTAX_TABLE = [
    LanguageSyntax("C/C++", ["//"], [("/*", "*/")], [".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".C", ".H"]),
    LanguageSyntax("Rust", ["//"], [("/*", "*/")], [".rs"]),
    LanguageSyntax("Python", ["#"], [('"""', '"""'), ("'''", "'''")], [".py", ".pyi", ".pyx", ".pyw"]),
    LanguageSyntax("Go", ["//"], [("/*", "*/")], [".go"]),
    LanguageSyntax("JavaScript/TypeScript", ["//"], [("/*", "*/")], [".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts"]),
    LanguageSyntax("Shell", ["#"], [], [".sh", ".bash", ".zsh", ".ksh", ".ebuild", ".eclass"]),
    LanguageSyntax("HTML/XML", [], [("<!--", "-->")], [".html", ".htm", ".xhtml", ".xml", ".svg", ".plist"]),
    LanguageSyntax("CSS", ["//"], [("/*", "*/")], [".css", ".scss", ".sass", ".less"]),
    LanguageSyntax("JSON", ["//"], [("/*", "*/")], [".json", ".jsonc"]),
    LanguageSyntax("YAML/TOML", ["#"], [], [".yaml", ".yml", ".toml"]),
    LanguageSyntax("Markdown", [], [("<!--", "-->")], [".md", ".markdown"]),
    LanguageSyntax("Java/Kotlin", ["//"], [("/*", "*/")], [".java", ".kt", ".kts"]),
    LanguageSyntax("Swift", ["//"], [("/*", "*/")], [".swift"]),
    LanguageSyntax("Ruby", ["#"], [("=begin", "=end")], [".rb", ".rake", ".gemspec"]),
    LanguageSyntax("PHP", ["//", "#"], [("/*", "*/")], [".php", ".phtml"]),
    LanguageSyntax("Perl", ["#"], [("=pod", "=cut")], [".pl", ".pm", ".t"]),
    LanguageSyntax("Lua", ["--"], [("--[[", "]]")], [".lua"]),
    LanguageSyntax("Zig", ["//"], [], [".zig"]),
    LanguageSyntax("Nim", ["#"], [("#[", "]#")], [".nim", ".nims"]),
    LanguageSyntax("SQL", ["--"], [("/*", "*/")], [".sql"]),
    LanguageSyntax("Makefile", ["#"], [], [".mk", ".mak"], ["Makefile", "makefile", "GNUmakefile"]),
    LanguageSyntax("CMake", ["#"], [("#[[", "]]")], [".cmake"], ["CMakeLists.txt"]),
    LanguageSyntax("Assembly", [";", "//", "#", "@"], [("/*", "*/")], [".s", ".S", ".asm"]),
]

EXT_TO_SYNTAX = {}
FILE_TO_SYNTAX = {}
for syn in SYNTAX_TABLE:
    for ext in syn.extensions:
        EXT_TO_SYNTAX[ext.lower()] = syn
    for fn in syn.filenames:
        FILE_TO_SYNTAX[fn.lower()] = syn

# ---------------------------------------------------------------------------
# Nerd Font Icons Map (from eza-0.23.5 src/output/icons.rs)
# ---------------------------------------------------------------------------
NERD_ICONS = {
    # File extensions
    ".rs": "\ue68b", ".py": "\ue606", ".c": "\ue61e", ".h": "\ue61e",
    ".cpp": "\ue61d", ".hpp": "\ue61d", ".go": "\ue627", ".js": "\ue74e",
    ".ts": "\ue628", ".tsx": "\ue7ba", ".jsx": "\ue7ba", ".html": "\ue736",
    ".css": "\ue749", ".scss": "\ue749", ".json": "\ue60b", ".yaml": "\ue6a8",
    ".yml": "\ue6a8", ".toml": "\ue6b2", ".md": "\ue609", ".sh": "\ue795",
    ".bash": "\ue795", ".zsh": "\ue795", ".java": "\ue738", ".kt": "\ue634",
    ".swift": "\ue755", ".rb": "\ue739", ".php": "\ue73d", ".lua": "\ue620",
    ".zig": "\ue6a9", ".nim": "\ue677", ".sql": "\ue706", ".xml": "\ue796",
    ".tar": "\uf410", ".gz": "\uf410", ".zip": "\uf410", ".xz": "\uf410",
    ".zst": "\uf410", ".png": "\uf1c5", ".jpg": "\uf1c5", ".jpeg": "\uf1c5",
    ".svg": "\uf1c5", ".pdf": "\uf1c1", ".deb": "\ue77d", ".lock": "\uf023",
    # Special filenames
    "makefile": "\ue660", "cmakelists.txt": "\ue660", "dockerfile": "\ue650",
    "license": "\uf02d", "readme.md": "\uf02d", ".gitignore": "\ue725",
    # Default fallbacks
    "dir": "\ue5ff", "file": "\uf15b", "symlink": "\uf0c1", "binary": "\ueae8",
}

# ---------------------------------------------------------------------------
# Helper: ANSI Colors & Formatting
# ---------------------------------------------------------------------------
def c(code, s, use_color):
    return f"\033[{code}m{s}\033[0m" if use_color else s

# Hostname token used in OSC 8 file:// targets. Overridable with --hostname-bin,
# mirroring ripgrep-15.2.0 crates/core/flags/defs.rs HostnameBin.
_HYPERLINK_HOST = MACHINE

def resolve_hostname_bin(cmd):
    """Run ``cmd`` and use its first output line as the OSC 8 hostname token."""
    global _HYPERLINK_HOST
    if not cmd:
        return
    try:
        # ripgrep runs this as a program, not a shell line; shlex keeps argument
        # handling predictable without invoking a shell.
        out = subprocess.run(shlex.split(cmd), capture_output=True, text=True, timeout=5)
        host = (out.stdout or "").strip().splitlines()
        if out.returncode == 0 and host:
            _HYPERLINK_HOST = host[0].strip()
        else:
            print(f"pfind: --hostname-bin {cmd!r} produced no hostname; keeping {MACHINE}", file=sys.stderr)
    except (OSError, subprocess.SubprocessError) as e:
        print(f"pfind: --hostname-bin failed ({e}); keeping {MACHINE}", file=sys.stderr)

def make_hyperlink(path, line=0, display_text=None, use_hyperlink=False, format_type="file"):
    text = display_text or path
    if not use_hyperlink:
        return text

    abs_path = os.path.abspath(path)

    if format_type == "vscode":
        # VSCode format: vscode://file/path:line
        target = f"vscode://file{abs_path}"
        if line > 0:
            target += f":{line}"
    elif format_type == "grep+":
        # grep+ format: file://path with #L line anchor
        target = f"file://{abs_path}"
        if line > 0:
            target += f"#L{line}"
    else:  # file format (default)
        target = f"file://{_HYPERLINK_HOST}{abs_path}"
        if line > 0:
            target += f"#{line}"

    return f"\033]8;;{target}\033\\{text}\033]8;;\033\\"

# ---------------------------------------------------------------------------
# --colors COLOR_SPEC (ripgrep-15.2.0 crates/core/flags/defs.rs, Colors)
# ---------------------------------------------------------------------------
# Spec grammar: {type}:{attribute}:{value} where type is path|line|column|match,
# attribute is fg|bg|style, and value is a colour name, 0xRRGGBB, or a style
# name. `{type}:none` clears the type. pfind renders its own output rather than
# letting rg colour it, so the spec is resolved to ANSI codes here.
_COLOR_NAMES = {
    "black": 0, "red": 1, "green": 2, "yellow": 3,
    "blue": 4, "magenta": 5, "cyan": 6, "white": 7,
}
_STYLE_NAMES = {
    "bold": "1", "nobold": "22", "intense": "1", "nointense": "22",
    "underline": "4", "nounderline": "24",
}
# Defaults match the codes pfind 3.0.0 hardcoded, so an unspecified type is unchanged.
COLOR_SPEC = {"path": "1;32", "line": "1;33", "column": "1;33", "match": "1;31"}

def _colour_value_to_code(value, is_bg):
    value = value.strip().lower()
    base = 40 if is_bg else 30
    if value in _COLOR_NAMES:
        return str(base + _COLOR_NAMES[value])
    if value.startswith("0x") and len(value) == 8:
        try:
            r, g, b = int(value[2:4], 16), int(value[4:6], 16), int(value[6:8], 16)
        except ValueError:
            return None
        return f"{base + 8};2;{r};{g};{b}"
    if value.isdigit() and 0 <= int(value) <= 255:
        return f"{base + 8};5;{value}"
    return None

def parse_color_specs(specs):
    """Fold --colors specs into COLOR_SPEC. Invalid specs warn and are skipped."""
    for raw in specs or []:
        parts = raw.split(":")
        if len(parts) < 2:
            print(f"pfind: unrecognised --colors spec {raw!r} (want type:attribute:value)", file=sys.stderr)
            continue
        ctype = parts[0].strip().lower()
        if ctype not in COLOR_SPEC:
            print(f"pfind: unknown --colors type {ctype!r} (want path|line|column|match)", file=sys.stderr)
            continue
        if len(parts) == 2 and parts[1].strip().lower() == "none":
            COLOR_SPEC[ctype] = ""
            continue
        if len(parts) < 3:
            print(f"pfind: unrecognised --colors spec {raw!r} (want type:attribute:value)", file=sys.stderr)
            continue
        attr, value = parts[1].strip().lower(), ":".join(parts[2:])
        existing = [x for x in COLOR_SPEC[ctype].split(";") if x]
        if attr == "style":
            code = _STYLE_NAMES.get(value.strip().lower())
        elif attr in ("fg", "bg"):
            code = _colour_value_to_code(value, attr == "bg")
        else:
            code = None
        if code is None:
            print(f"pfind: unrecognised --colors value in {raw!r}", file=sys.stderr)
            continue
        COLOR_SPEC[ctype] = ";".join(existing + [code]) if attr == "style" else code

def cc(ctype, s, use_color):
    """Colour ``s`` using the --colors entry for ``ctype``."""
    code = COLOR_SPEC.get(ctype, "")
    return c(code, s, use_color) if code else s

# ---------------------------------------------------------------------------
# Root & Scope Resolution
# ---------------------------------------------------------------------------
def resolve_roots(paths, presets):
    roots = []
    for name in presets:
        p = PRESET_ROOTS.get(name)
        if p is None:
            print(f"pfind: preset --{name} is not configured on this machine; {_PRESET_HELP}", file=sys.stderr)
            continue
        if p.exists():
            roots.append(p)
        else:
            print(f"pfind: preset --{name} points at {p}, which does not exist; {_PRESET_HELP}", file=sys.stderr)
    for raw in paths:
        p = Path(raw).expanduser().resolve()
        if p.exists():
            roots.append(p)
        else:
            print(f"pfind: path does not exist: {p}", file=sys.stderr)
    if not roots:
        roots = [Path.cwd()]
    seen, out = set(), []
    for r in roots:
        rs = str(r)
        if rs not in seen:
            seen.add(rs)
            out.append(r)
    return out

def resolve_type_globs(types, types_not):
    globs = []
    if types:
        for t in types:
            t_lower = t.lower()
            if t_lower in LANGUAGE_TYPES:
                for ext in LANGUAGE_TYPES[t_lower]:
                    globs.append(f"*{ext}")
            else:
                globs.append(f"*.{t}" if not t.startswith(".") else f"*{t}")
    if types_not:
        for t in types_not:
            t_lower = t.lower()
            if t_lower in LANGUAGE_TYPES:
                for ext in LANGUAGE_TYPES[t_lower]:
                    globs.append(f"!*{ext}")
            else:
                globs.append(f"!*.{t}" if not t.startswith(".") else f"!*{t}")
    return globs

def rg_common_globs(ext_filter, extra_excludes, type_globs, globs=()):
    """Build ripgrep -g override globs. ``globs`` are user-supplied include
    globs (e.g. ``*.md``) or excludes prefixed with ``!``. When only includes
    are given, ripgrep's override semantics make every other path ignored,
    matching ripgrep-15.2.0/crates/ignore/src/overrides.rs behaviour."""
    out = []
    for g in EXCLUDE_GLOBS:
        out += ["-g", f"!{g}"]
    for g in extra_excludes:
        out += ["-g", f"!{g}"]
    if ext_filter:
        for e in ext_filter:
            e = e if e.startswith(".") else "." + e
            out += ["-g", f"*{e}"]
    for tg in type_globs:
        out += ["-g", tg]
    for g in globs:
        out += ["-g", g]
    return out


def _apply_user_globs(paths, globs, is_dir=False):
    """Filter ``paths`` by user-supplied ``-g`` globs using ripgrep override
    semantics (see ripgrep-15.2.0 crates/ignore/src/overrides.rs): a leading
    ``!`` turns a glob into a re-include (whitelist); when at least one plain
    include glob is present, every path that matches none of the includes is
    ignored. This is the pure-Python mirror used when ripgrep is unavailable;
    the rg path hands the same ``-g`` args straight to ripgrep.
    """
    if not globs:
        return paths
    includes = [g for g in globs if not g.startswith("!")]
    excludes = [g[1:] for g in globs if g.startswith("!")]

    def gmatch(path, pattern):
        # Match on the basename for patterns that do not contain a slash
        # (ripgrep matches *-only globs against the basename), else the full
        # path, which is how globset behaves.
        if "/" not in pattern:
            return (fnmatch.fnmatch(os.path.basename(path), pattern)
                    or fnmatch.fnmatch(path, pattern))
        return fnmatch.fnmatch(path, pattern)

    filtered = []
    for p in paths:
        if any(gmatch(p, e) for e in excludes):
            continue
        if includes and not any(gmatch(p, g) for g in includes):
            continue
        filtered.append(p)
    return filtered

# ---------------------------------------------------------------------------
# Comment-Aware LOC Classifier (from eza-0.23.5 src/loc/mod.rs)
# ---------------------------------------------------------------------------
class LocCounts:
    __slots__ = ("lines", "code", "comments", "blanks")
    def __init__(self, lines=0, code=0, comments=0, blanks=0):
        self.lines = lines
        self.code = code
        self.comments = comments
        self.blanks = blanks

    def add(self, other):
        self.lines += other.lines
        self.code += other.code
        self.comments += other.comments
        self.blanks += other.blanks

def classify_file_loc(path):
    fn = os.path.basename(path).lower()
    ext = os.path.splitext(fn)[1].lower()
    syntax = FILE_TO_SYNTAX.get(fn) or EXT_TO_SYNTAX.get(ext)
    if not syntax:
        return None

    try:
        with open(path, "r", encoding="utf-8", errors="ignore") as f:
            lines = f.readlines()
    except (OSError, UnicodeError):
        return None

    counts = LocCounts(lines=len(lines))
    active_block_closer = None

    for line in lines:
        rest = line.strip()
        if not rest:
            counts.blanks += 1
            continue

        has_code = False
        has_comment = False

        while rest:
            if active_block_closer:
                has_comment = True
                idx = rest.find(active_block_closer)
                if idx != -1:
                    rest = rest[idx + len(active_block_closer):].strip()
                    active_block_closer = None
                    continue
                else:
                    rest = ""
                    break

            if not rest:
                break

            # check line comments
            is_line_comment = False
            for lc in syntax.line_comments:
                if rest.startswith(lc):
                    has_comment = True
                    is_line_comment = True
                    rest = ""
                    break
            if is_line_comment:
                break

            # check block comment openers
            found_block = False
            for b_open, b_close in syntax.block_comments:
                if rest.startswith(b_open):
                    has_comment = True
                    active_block_closer = b_close
                    rest = rest[len(b_open):].strip()
                    found_block = True
                    break
            if found_block:
                continue

            # otherwise character is code
            has_code = True
            rest = rest[1:].strip()

        if has_code:
            counts.code += 1
        elif has_comment:
            counts.comments += 1
        else:
            counts.blanks += 1

    return syntax.name, counts

# ---------------------------------------------------------------------------
# Git Status Engine (from eza-0.23.5 src/fs/feature/git.rs)
# ---------------------------------------------------------------------------
class GitStatusCache:
    def __init__(self, roots):
        self.status_map = {}  # abs_path -> status_str ("M", "A", "?", "U", "D")
        if not HAVE_GIT:
            return
        repo_roots = set()
        for r in roots:
            try:
                out = subprocess.run(
                    ["git", "-C", str(r), "rev-parse", "--show-toplevel"],
                    capture_output=True, text=True, timeout=5
                )
                if out.returncode == 0 and out.stdout.strip():
                    repo_roots.add(out.stdout.strip())
            except Exception:
                pass

        for repo in repo_roots:
            try:
                proc = subprocess.run(
                    ["git", "-C", repo, "status", "--porcelain=v1", "-z", "-uall"],
                    capture_output=True, text=True, timeout=10
                )
                if proc.returncode == 0:
                    raw_entries = proc.stdout.split("\0")
                    for entry in raw_entries:
                        if len(entry) >= 4:
                            st = entry[:2].strip()
                            rel_path = entry[3:]
                            abs_p = str(Path(repo) / rel_path)
                            self.status_map[abs_p] = st
            except Exception:
                pass

    def get_status(self, path):
        return self.status_map.get(os.path.abspath(path))

# ---------------------------------------------------------------------------
# Filesystem Metadata & Formatting Engine (from eza-0.23.5 src/output/)
# ---------------------------------------------------------------------------
def format_size(size_bytes, binary_units=True):
    if size_bytes is None:
        return "     -"
    base = 1024.0 if binary_units else 1000.0
    suffixes = ["B", "KiB", "MiB", "GiB", "TiB"] if binary_units else ["B", "KB", "MB", "GB", "TB"]
    val = float(size_bytes)
    for s in suffixes:
        if val < base or s == suffixes[-1]:
            if s == "B":
                return f"{int(val):>5d} B"
            return f"{val:>5.1f} {s}"
        val /= base
    return f"{val:.1f} B"

def format_permissions(mode, octal=False):
    if octal:
        return f"{stat.S_IMODE(mode):04o}"
    is_dir = "d" if stat.S_ISDIR(mode) else "-"
    flags = [
        "r" if mode & stat.S_IRUSR else "-",
        "w" if mode & stat.S_IWUSR else "-",
        "x" if mode & stat.S_IXUSR else "-",
        "r" if mode & stat.S_IRGRP else "-",
        "w" if mode & stat.S_IWGRP else "-",
        "x" if mode & stat.S_IXGRP else "-",
        "r" if mode & stat.S_IROTH else "-",
        "w" if mode & stat.S_IWOTH else "-",
        "x" if mode & stat.S_IXOTH else "-",
    ]
    return is_dir + "".join(flags)

def format_timestamp(ts, style="relative"):
    dt = datetime.datetime.fromtimestamp(ts)
    # eza-0.23.5 src/output/time.rs: default|iso|long-iso|full-iso|relative|+FORMAT
    if style and style.startswith("+"):
        try:
            return dt.strftime(style[1:])
        except ValueError:
            return dt.strftime("%Y-%m-%d %H:%M")
    if style == "iso":
        return dt.strftime("%Y-%m-%d %H:%M:%S")
    if style == "long-iso":
        return dt.strftime("%Y-%m-%d %H:%M")
    if style == "full-iso":
        return dt.astimezone().strftime("%Y-%m-%d %H:%M:%S.%f %z")
    now = datetime.datetime.now()
    delta = now - dt
    secs = delta.total_seconds()
    if secs < 0:
        return dt.strftime("%Y-%m-%d")
    if secs < 60:
        return f"{int(secs)}s ago"
    if secs < 3600:
        return f"{int(secs // 60)}m ago"
    if secs < 86400:
        return f"{int(secs // 3600)}h ago"
    if secs < 86400 * 30:
        return f"{int(secs // 86400)}d ago"
    if secs < 86400 * 365:
        return f"{int(secs // (86400 * 30))}mo ago"
    return f"{int(secs // (86400 * 365))}y ago"

def get_file_icon(path, is_dir=False):
    if is_dir:
        return NERD_ICONS["dir"]
    fn = os.path.basename(path).lower()
    if fn in NERD_ICONS:
        return NERD_ICONS[fn]
    ext = os.path.splitext(fn)[1].lower()
    return NERD_ICONS.get(ext, NERD_ICONS["file"])

def is_binary_file(path, sample_size=8192):
    try:
        with open(path, "rb") as f:
            chunk = f.read(sample_size)
            return b"\x00" in chunk
    except OSError:
        return False

# ---------------------------------------------------------------------------
# Metadata Selection Helpers (eza-0.23.5 src/output/render/, src/fs/file.rs)
# ---------------------------------------------------------------------------
def get_file_stat(path, dereference=False):
    """os.stat when --dereference (report the symlink target's metadata),
    os.lstat otherwise (report the link itself), matching eza's -X."""
    if dereference:
        try:
            return os.stat(path)
        except OSError:
            pass
    return os.lstat(path)

def pick_timestamp(st, field="modified"):
    """Select the timestamp field named by --time / --changed / --accessed.
    POSIX has no true birth time in os.stat_result on most Linux filesystems;
    st_birthtime is used when the platform exposes it, else ctime, and the
    substitution is visible here rather than silently presented as creation."""
    if field == "accessed":
        return st.st_atime
    if field == "changed":
        return st.st_ctime
    if field == "created":
        return getattr(st, "st_birthtime", st.st_ctime)
    return st.st_mtime

def classify_indicator(path, st=None):
    """eza -F/--classify: append a type indicator to the name."""
    try:
        st = st or os.lstat(path)
    except OSError:
        return ""
    mode = st.st_mode
    if stat.S_ISDIR(mode):
        return "/"
    if stat.S_ISLNK(mode):
        return "@"
    if stat.S_ISFIFO(mode):
        return "|"
    if stat.S_ISSOCK(mode):
        return "="
    if mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH):
        return "*"
    return ""

_QUOTE_NEEDED = re.compile(r"[\s'\"\\$`|&;<>()*?\[\]{}!#~]")

def quote_name(name, quotes_enabled):
    """eza quotes names containing shell-significant characters; --no-quotes
    turns that off (eza-0.23.5 src/output/file_name.rs)."""
    if not quotes_enabled or not _QUOTE_NEEDED.search(name):
        return name
    if '"' in name:
        return "'" + name + "'"
    return '"' + name + '"'

_NIX_STORE = re.compile(r"(/nix/store/)([a-z0-9]{32})-")

def short_nix(path, enabled):
    """eza --short-nix: elide the 32-char Nix store hash."""
    if not enabled:
        return path
    return _NIX_STORE.sub(r"\1…-", path)

def display_path(path, args):
    """Apply --absolute and --short-nix to a path before display."""
    mode = getattr(args, "absolute", "off") or "off"
    if mode == "follow":
        try:
            path = os.path.realpath(path)
        except OSError:
            path = os.path.abspath(path)
    elif mode == "on":
        path = os.path.abspath(path)
    return short_nix(path, getattr(args, "short_nix", False))

def get_xattrs(path):
    """eza -@/--extended: list extended attributes as (name, value_size)."""
    if not hasattr(os, "listxattr"):
        return []
    try:
        out = []
        for name in os.listxattr(path, follow_symlinks=False):
            try:
                out.append((name, len(os.getxattr(path, name, follow_symlinks=False))))
            except OSError:
                out.append((name, 0))
        return out
    except OSError:
        return []

# BSD/macOS file flags (eza -O/--flags). Linux stat() carries no st_flags; the
# nearest equivalent is the ext-family attribute set, read via lsattr when
# present. Absence is reported as "-", never as "no flags set".
_HAVE_LSATTR = shutil.which("lsattr") is not None

def get_file_flags(path):
    st_flags = None
    try:
        st_flags = getattr(os.lstat(path), "st_flags", None)
    except OSError:
        pass
    if st_flags is not None:
        return f"{st_flags:o}"
    if _HAVE_LSATTR:
        try:
            out = subprocess.run(["lsattr", "-d", "--", path],
                                 capture_output=True, text=True, timeout=5)
            if out.returncode == 0 and out.stdout.strip():
                attrs = out.stdout.split()[0].replace("-", "")
                return attrs or "-"
        except (OSError, subprocess.SubprocessError):
            pass
    return "-"

_MOUNT_CACHE = None

def get_mount_info(path):
    """eza -M/--mounts: device + filesystem type when ``path`` is a mount point."""
    global _MOUNT_CACHE
    if _MOUNT_CACHE is None:
        _MOUNT_CACHE = {}
        for src in ("/proc/self/mounts", "/etc/mtab"):
            try:
                with open(src, "r", encoding="utf-8", errors="replace") as f:
                    for line in f:
                        parts = line.split()
                        if len(parts) >= 3:
                            _MOUNT_CACHE[parts[1].replace("\\040", " ")] = (parts[0], parts[2])
                break
            except OSError:
                continue
    return _MOUNT_CACHE.get(os.path.abspath(path))

# ---------------------------------------------------------------------------
# Git Repository Summary (eza-0.23.5 --git-repos / --git-repos-no-status)
# ---------------------------------------------------------------------------
_GIT_REPO_CACHE: dict = {}

def get_repo_summary(path, with_status=True):
    """Branch name (+ dirty/clean when with_status) for a directory that is a
    git work tree. Returns None for anything that is not one."""
    if not HAVE_GIT or not os.path.isdir(path):
        return None
    key = (os.path.abspath(path), with_status)
    if key in _GIT_REPO_CACHE:
        return _GIT_REPO_CACHE[key]
    result = None
    try:
        top = subprocess.run(["git", "-C", path, "rev-parse", "--show-toplevel"],
                             capture_output=True, text=True, timeout=5)
        # Only label the repository root itself, as eza does — not every
        # directory that happens to sit inside a checkout.
        if top.returncode == 0 and os.path.abspath(top.stdout.strip()) == os.path.abspath(path):
            branch = subprocess.run(["git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD"],
                                    capture_output=True, text=True, timeout=5)
            name = branch.stdout.strip() if branch.returncode == 0 else "?"
            if not with_status:
                result = name
            else:
                dirty = subprocess.run(["git", "-C", path, "status", "--porcelain"],
                                       capture_output=True, text=True, timeout=10)
                state = "dirty" if (dirty.returncode == 0 and dirty.stdout.strip()) else "clean"
                result = f"{name}|{state}"
    except (OSError, subprocess.SubprocessError):
        result = None
    _GIT_REPO_CACHE[key] = result
    return result

def git_ignored_filter(paths):
    """Drop paths that git would ignore. Used by the pure-Python listing path
    for --git-ignore; the ripgrep path already honours .gitignore natively."""
    if not HAVE_GIT or not paths:
        return paths
    by_repo = defaultdict(list)
    for p in paths:
        d = p if os.path.isdir(p) else os.path.dirname(p) or "."
        try:
            top = subprocess.run(["git", "-C", d, "rev-parse", "--show-toplevel"],
                                 capture_output=True, text=True, timeout=5)
        except (OSError, subprocess.SubprocessError):
            continue
        if top.returncode == 0 and top.stdout.strip():
            by_repo[top.stdout.strip()].append(p)
    ignored = set()
    for repo, group in by_repo.items():
        try:
            proc = subprocess.run(["git", "-C", repo, "check-ignore", "--stdin"],
                                  input="\n".join(group), capture_output=True,
                                  text=True, timeout=30)
            # exit 0 = some paths ignored, 1 = none, >1 = real error
            if proc.returncode in (0, 1):
                ignored.update(line for line in proc.stdout.splitlines() if line)
        except (OSError, subprocess.SubprocessError):
            continue
    return [p for p in paths if p not in ignored]

# ---------------------------------------------------------------------------
# Search Engine: ripgrep 15.2.0 Ingestion with Context Windowing
# ---------------------------------------------------------------------------
def rg_walk_flags(opts):
    """Translate the directory-traversal / ignore-layer / resource options into
    ripgrep arguments. Applies to both `rg --files` and `rg --json` so that a
    listing and a search see exactly the same file set."""
    if opts is None:
        return []
    g = lambda n, d=False: getattr(opts, n, d)
    out = []
    for name, flag in (
        ("no_ignore_dot", "--no-ignore-dot"),
        ("no_ignore_global", "--no-ignore-global"),
        ("no_ignore_parent", "--no-ignore-parent"),
        ("no_ignore_vcs", "--no-ignore-vcs"),
        ("no_ignore_exclude", "--no-ignore-exclude"),
        ("no_ignore_messages", "--no-ignore-messages"),
        ("no_require_git", "--no-require-git"),
        ("one_file_system", "--one-file-system"),
        ("glob_case_insensitive", "--glob-case-insensitive"),
        ("no_config", "--no-config"),
        ("follow_symlinks", "--follow"),
    ):
        if g(name):
            out.append(flag)
    if g("max_filesize", None):
        out += ["--max-filesize", str(g("max_filesize"))]
    for ig in (g("iglobs", None) or []):
        out += ["--iglob", ig]
    for f in (g("ignore_files", None) or []):
        out += ["--ignore-file", f]
    if g("ignore_file_case_insensitive"):
        out.append("--ignore-file-case-insensitive")
    if g("mmap"):
        out.append("--mmap")
    if g("no_mmap"):
        out.append("--no-mmap")
    return out

def rg_match_flags(opts):
    """Translate the matcher / engine / I/O options into ripgrep arguments.
    Search-only; these have no meaning for `rg --files`."""
    if opts is None:
        return []
    g = lambda n, d=False: getattr(opts, n, d)
    out = []
    for name, flag in (
        ("text", "--text"),
        ("binary", "--binary"),
        ("line_regexp", "--line-regexp"),
        ("crlf", "--crlf"),
        ("null_data", "--null-data"),
        ("no_unicode", "--no-unicode"),
        ("stop_on_nonmatch", "--stop-on-nonmatch"),
        ("passthru", "--passthru"),
        ("block_buffered", "--block-buffered"),
        ("line_buffered", "--line-buffered"),
    ):
        if g(name):
            out.append(flag)
    if g("max_count", 0):
        out += ["--max-count", str(g("max_count"))]
    if g("encoding", None):
        out += ["--encoding", str(g("encoding"))]
    if g("engine", None) and g("engine") != "default":
        out += ["--engine", str(g("engine"))]
    if g("dfa_size_limit", None):
        out += ["--dfa-size-limit", str(g("dfa_size_limit"))]
    if g("regex_size_limit", None):
        out += ["--regex-size-limit", str(g("regex_size_limit"))]
    return out

def rg_list_files(roots, ext_filter, extra_excludes, type_globs, include_hidden, no_ignore, workers, max_depth=None, globs=(), opts=None):
    cmd = ["rg", "--files", "--threads", str(workers)]
    if include_hidden:
        cmd.append("--hidden")
    if no_ignore:
        cmd.append("--no-ignore")
    if max_depth:
        cmd += ["--max-depth", str(max_depth)]
    cmd += rg_walk_flags(opts)
    cmd += rg_common_globs(ext_filter, extra_excludes, type_globs, globs)
    cmd += [str(r) for r in roots]
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
    except (subprocess.TimeoutExpired, OSError) as e:
        print(f"pfind: rg --files failed ({e}); fallback enumeration", file=sys.stderr)
        return []
    if out.returncode not in (0, 1) and not getattr(opts, "no_messages", False):
        msg = (out.stderr or "").strip().splitlines()
        if msg:
            print(f"pfind: rg --files: {msg[0]}", file=sys.stderr)
    return [line for line in out.stdout.splitlines() if line]

def rg_content(pattern, roots, regex, ignore_case, case_sensitive, word_regexp,
               ext_filter, extra_excludes, type_globs, include_hidden, no_ignore,
               workers, max_per_file, fixed=None, multiline=False, search_zip=False,
               context_before=0, context_after=0, max_depth=None, pcre2=False,
               invert_match=False, timeout=600, extract_column=False,
               pre=None, pre_glob=None, globs=(), opts=None, only_matching=False,
               smart_case=False):
    if fixed is None:
        fixed = not regex
    cmd = ["rg", "--json", "--threads", str(workers)]
    # -s wins over -i wins over -S, matching ripgrep's own precedence.
    if case_sensitive:
        cmd.append("-s")
    elif ignore_case and not smart_case:
        cmd.append("-i")
    else:
        cmd.append("-S")
    if fixed:
        cmd.append("-F")
    if word_regexp:
        cmd.append("-w")
    if multiline:
        cmd.append("-U")
        # -x/--loose need dotall to span lines; -U alone does not imply it.
        if getattr(opts, "multiline_dotall", False) or getattr(opts, "exact", False) or getattr(opts, "loose", False):
            cmd.append("--multiline-dotall")
    if search_zip:
        cmd.append("-z")
    if pcre2:
        cmd.append("--pcre2")
    if invert_match:
        cmd.append("-v")
    if include_hidden:
        cmd.append("--hidden")
    if no_ignore:
        cmd.append("--no-ignore")
    if max_depth:
        cmd += ["--max-depth", str(max_depth)]
    if context_before > 0:
        cmd += ["-B", str(context_before)]
    if context_after > 0:
        cmd += ["-A", str(context_after)]
    if extract_column:
        cmd.append("--column")
    if pre:
        cmd += ["--pre", pre]
    if pre_glob:
        cmd += ["--pre-glob", pre_glob]

    cmd += rg_walk_flags(opts)
    cmd += rg_match_flags(opts)
    cmd += rg_common_globs(ext_filter, extra_excludes, type_globs, globs)
    cmd += ["-e", pattern, "--"]
    cmd += [str(r) for r in roots]

    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    except (subprocess.TimeoutExpired, OSError) as e:
        print(f"pfind: rg content search failed: {e}", file=sys.stderr)
        return {}
    if proc.returncode not in (0, 1) and not getattr(opts, "no_messages", False):
        msg = (proc.stderr or "").strip().splitlines()
        if msg:
            print(f"pfind: rg: {msg[0]}", file=sys.stderr)

    hits = defaultdict(lambda: {"count": 0, "samples": [], "terms": set(),
                                "lines_data": [], "offsets": {}})
    for line in proc.stdout.splitlines():
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        msg_type = obj.get("type")
        if msg_type not in ("match", "context"):
            continue
        d = obj.get("data", {})
        path = d.get("path", {}).get("text")
        if not path:
            continue
        rec = hits[path]
        text = (d.get("lines", {}).get("text") or "").rstrip("\n")
        lineno = d.get("line_number", 0)
        if d.get("absolute_offset") is not None:
            rec["offsets"][lineno] = d["absolute_offset"]

        if msg_type == "match":
            rec["count"] += 1
            col = 0
            if extract_column and d.get("submatches"):
                # Get column of first match
                col = d["submatches"][0].get("start", 0) + 1  # 1-indexed

            for sm in d.get("submatches", []):
                mt = (sm.get("match") or {}).get("text")
                if mt:
                    rec["terms"].add(mt.lower()[:40])

            if only_matching:
                # -o: the matched substrings replace the whole line, which is
                # what `rg -o` prints (one per match; joined here because pfind
                # renders one row per source line).
                parts = [(sm.get("match") or {}).get("text") or "" for sm in d.get("submatches", [])]
                parts = [p for p in parts if p]
                if parts:
                    text = " ".join(parts)
            if len(rec["samples"]) < max_per_file:
                sample = (lineno, text[:220] + "…" if len(text) > 220 else text, col) if extract_column else (lineno, text[:220] + "…" if len(text) > 220 else text)
                rec["samples"].append(sample)
            rec["lines_data"].append((lineno, text, True, col) if extract_column else (lineno, text, True))
        else: # context
            rec["lines_data"].append((lineno, text, False, 0) if extract_column else (lineno, text, False))

    return dict(hits)

# ---------------------------------------------------------------------------
# Pure-Python Fallback Engine (with Decompression & Context Support)
# ---------------------------------------------------------------------------
def parse_size_arg(text):
    """Parse ripgrep's NUM[KMG] size syntax (--max-filesize, --*-size-limit)."""
    if text is None:
        return None
    s = str(text).strip().upper()
    mult = 1
    if s and s[-1] in "KMG":
        mult = {"K": 1024, "M": 1024 ** 2, "G": 1024 ** 3}[s[-1]]
        s = s[:-1]
    try:
        return int(float(s) * mult)
    except ValueError:
        print(f"pfind: unrecognised size value {text!r}; ignoring", file=sys.stderr)
        return None

def py_fallback_files(roots, ext_filter, extra_excludes, type_globs, max_depth=None,
                      include_dirs=False, include_hidden=False, globs=(), opts=None):
    excl_names = {g for g in EXCLUDE_GLOBS if "*" not in g} | set(extra_excludes)
    excl_prefix = tuple(g[:-1] for g in EXCLUDE_GLOBS if g.endswith("*"))
    g = (lambda n, d=False: getattr(opts, n, d)) if opts is not None else (lambda n, d=False: d)
    max_bytes = parse_size_arg(g("max_filesize", None))
    one_fs = g("one_file_system")
    follow = g("follow_symlinks")
    treat_dirs_as_files = g("treat_dirs_as_files")
    iglobs = [ig.lower() for ig in (g("iglobs", None) or [])]
    files = []
    for root in roots:
        root_path = Path(root)
        root_depth = len(root_path.parts)
        try:
            root_dev = os.stat(root).st_dev if one_fs else None
        except OSError:
            root_dev = None
        if treat_dirs_as_files and os.path.isdir(root):
            # --treat-dirs-as-files: the directory is the entry; do not descend.
            files.append(str(root))
            continue
        for dirpath, dirnames, filenames in os.walk(root, followlinks=follow):
            if not include_hidden:
                dirnames[:] = [d for d in dirnames if not d.startswith('.')]
                filenames = [f for f in filenames if not f.startswith('.')]
            if max_depth is not None:
                cur_depth = len(Path(dirpath).parts) - root_depth
                if cur_depth >= max_depth:
                    dirnames.clear()
            dirnames[:] = [d for d in dirnames if d not in excl_names and not d.startswith(excl_prefix)]
            if one_fs and root_dev is not None:
                kept = []
                for d in dirnames:
                    try:
                        if os.stat(os.path.join(dirpath, d)).st_dev == root_dev:
                            kept.append(d)
                    except OSError:
                        pass
                dirnames[:] = kept
            if include_dirs and dirpath != str(root):
                files.append(dirpath)
            for fn in filenames:
                ext = Path(fn).suffix.lower()
                if ext_filter and ext not in ext_filter:
                    continue
                fp = os.path.join(dirpath, fn)
                if max_bytes is not None:
                    try:
                        if os.lstat(fp).st_size > max_bytes:
                            continue
                    except OSError:
                        continue
                files.append(fp)
    if type_globs:
        # -t/--type resolves to the same override globs ripgrep receives; the
        # pure-Python path applied them nowhere before, so -t silently matched
        # everything whenever ripgrep was absent.
        files = _apply_user_globs(files, list(type_globs), is_dir=False)
    if globs:
        files = _apply_user_globs(files, globs, is_dir=False)
    if iglobs:
        # --iglob is --glob with case folded on both sides; reuse the same
        # override semantics rather than duplicating them.
        lowered = [f.lower() for f in files]
        keep = set(_apply_user_globs(lowered, iglobs, is_dir=False))
        files = [f for f, low in zip(files, lowered) if low in keep]
    if g("git_ignore"):
        files = git_ignored_filter(files)
    return files

# GORILLA OVERRIDE (2026-08-20): a decompression budget for --search-zip.
#
# THE HOLE THIS CLOSES. _read_lines() decompressed archive members straight into
# a list with no cap. The --max-filesize guard does not help: it checks
# os.lstat(fp).st_size, which is the COMPRESSED size on disk. A 42 KB zip that
# expands to gigabytes passes any cap you set and then exhausts memory. That is
# the classic decompression bomb, and on the 2012 laptops this is built for the
# machine dies rather than degrades.
#
# WHY A SHARED BUDGET AND NOT A PER-MEMBER ONE: a per-member cap stops one huge
# member and does nothing about ten thousand small ones. The budget is spent
# across the WHOLE archive.
#
# WHY IT IS LOUD. Truncating results silently would make "no matches" and "I
# stopped reading" look identical - the failure this project cares most about.
# When the budget runs out it says so on stderr, naming the file.
MAX_DECOMPRESSED_BYTES = 256 * 1024 * 1024


class _Budget:
    """Bytes remaining for one archive. take() returns how many may still be read."""

    def __init__(self, limit):
        self.limit = limit
        self.left = limit
        self.blown = False

    def offer(self, n):
        """How many bytes may be attempted next. Does NOT charge for them."""
        if n <= 0 or self.left <= 0:
            return 0
        return min(n, self.left)

    def spend(self, n):
        """Charge for bytes ACTUALLY read.

        Charging on offer instead of on spend was a real bug: a chunk size of
        1 MB against a 100 KB tar member burned 1 MB of budget for 100 KB of
        data, so a 64 MB budget truncated a legitimate archive after ~3 MB. Only
        bytes that arrived are billed.
        """
        self.left -= n


def _read_capped(handle, budget, chunk=1 << 20):
    """Read a binary handle in chunks until it ends or the budget is spent."""
    out = []
    while True:
        room = budget.offer(chunk)
        if room == 0:
            budget.blown = True
            break
        block = handle.read(room)
        if not block:
            break
        budget.spend(len(block))
        out.append(block)
    return b"".join(out)


def _text_lines_capped(f, budget, chunk=1 << 20):
    """Same, for an already-decoded text stream."""
    out = []
    while True:
        room = budget.offer(chunk)
        if room == 0:
            budget.blown = True
            break
        block = f.read(room)
        if not block:
            break
        # Text mode: read() returns CHARACTERS. Bill the encoded length so the
        # budget stays a byte budget in both paths.
        budget.spend(len(block.encode("utf-8", "ignore")))
        out.append(block)
    return "".join(out).splitlines(keepends=True)


def _max_decompressed():
    """Budget in bytes. PFIND_MAX_DECOMPRESSED_MB overrides it; 0 means no cap,
    for someone who genuinely knows the archive and has the memory."""
    raw = os.environ.get("PFIND_MAX_DECOMPRESSED_MB", "").strip()
    if raw:
        try:
            mb = float(raw)
            if mb <= 0:
                return float("inf")
            return int(mb * 1024 * 1024)
        except ValueError:
            pass
    return MAX_DECOMPRESSED_BYTES


def _budget_note(fp, budget):
    # Report the limit ACTUALLY in force, not the module default. The first
    # version printed MAX_DECOMPRESSED_BYTES while a smaller env override was
    # active, so the warning named a number the run had never used - a message
    # that lies about its own cause is worse than no message.
    if budget.blown:
        print("pfind: %s exceeded the %d MB decompression budget - results are "
              "PARTIAL. Raise it with PFIND_MAX_DECOMPRESSED_MB if you meant it."
              % (fp, budget.limit // (1024 * 1024)), file=sys.stderr)


def _read_lines(fp, search_zip, encoding):
    """Read a file as text lines, transparently decompressing when --search-zip
    is on. gz/bz2/xz/lzma stream directly; zip and tar are containers, so their
    members are concatenated in archive order."""
    enc = encoding or "utf-8"
    if search_zip:
        budget = _Budget(_max_decompressed())
        low = fp.lower()
        if low.endswith((".gz", ".tgz")) and not low.endswith(".tar.gz"):
            with gzip.open(fp, "rt", encoding=enc, errors="ignore") as f:
                lines = _text_lines_capped(f, budget)
            _budget_note(fp, budget)
            return lines
        if low.endswith(".bz2") and not low.endswith(".tar.bz2"):
            with bz2.open(fp, "rt", encoding=enc, errors="ignore") as f:
                lines = _text_lines_capped(f, budget)
            _budget_note(fp, budget)
            return lines
        if low.endswith((".xz", ".lzma")) and not low.endswith(".tar.xz"):
            with lzma.open(fp, "rt", encoding=enc, errors="ignore") as f:
                lines = _text_lines_capped(f, budget)
            _budget_note(fp, budget)
            return lines
        if low.endswith(".zip"):
            lines = []
            with zipfile.ZipFile(fp) as z:
                for name in z.namelist():
                    if name.endswith("/"):
                        continue
                    with z.open(name) as member:
                        raw = _read_capped(member, budget)
                    lines.extend(raw.decode(enc, "ignore").splitlines(keepends=True))
                    if budget.blown:
                        break
            _budget_note(fp, budget)
            return lines
        if tarfile.is_tarfile(fp):
            lines = []
            with tarfile.open(fp) as t:
                for member in t.getmembers():
                    if not member.isfile():
                        continue
                    handle = t.extractfile(member)
                    if handle is None:
                        continue
                    raw = _read_capped(handle, budget)
                    lines.extend(raw.decode(enc, "ignore").splitlines(keepends=True))
                    if budget.blown:
                        break
            _budget_note(fp, budget)
            return lines
    with open(fp, "r", encoding=enc, errors="ignore") as f:
        return f.readlines()

def py_fallback_content(pattern, files, regex, ignore_case, case_sensitive, word_regexp,
                        max_per_file, search_zip=False, context_before=0, context_after=0,
                        invert_match=False, opts=None, only_matching=False):
    g = (lambda n, d=False: getattr(opts, n, d)) if opts is not None else (lambda n, d=False: d)
    max_count = g("max_count", 0) or 0
    encoding = g("encoding", None)
    passthru = g("passthru")
    line_regexp = g("line_regexp")
    treat_as_text = g("text") or g("binary")
    stop_on_nonmatch = g("stop_on_nonmatch")
    no_messages = g("no_messages")
    max_bytes = parse_size_arg(g("max_filesize", None))

    flags = re.IGNORECASE if (ignore_case and not case_sensitive) else 0
    if word_regexp:
        rx_src = r"\b" + (pattern if regex else re.escape(pattern)) + r"\b"
        compiled = re.compile(rx_src, flags)
    elif line_regexp:
        rx_src = r"^(?:" + (pattern if regex else re.escape(pattern)) + r")$"
        compiled = re.compile(rx_src, flags)
    elif regex:
        compiled = re.compile(pattern, flags)
    else:
        compiled = re.compile(re.escape(pattern), flags)
    matcher = compiled.search

    hits = {}
    for fp in files:
        if not search_zip and not treat_as_text and is_binary_file(fp):
            continue
        if max_bytes is not None:
            try:
                if os.lstat(fp).st_size > max_bytes:
                    continue
            except OSError:
                continue
        try:
            lines = _read_lines(fp, search_zip, encoding)

            match_indices = []
            for i, line in enumerate(lines):
                is_m = bool(matcher(line.rstrip("\n") if line_regexp else line))
                if invert_match:
                    is_m = not is_m
                if is_m:
                    match_indices.append(i)
                    if max_count and len(match_indices) >= max_count:
                        break
                elif stop_on_nonmatch and match_indices:
                    break

            if not match_indices:
                continue

            def render_line(idx):
                """-o prints the matched substrings in place of the line."""
                raw = lines[idx].rstrip("\n")
                if not only_matching:
                    return raw
                found = [m.group(0) for m in compiled.finditer(raw)]
                return " ".join(found) if found else raw

            samples = []
            lines_data = []
            for idx in match_indices:
                if len(samples) < max_per_file:
                    s = render_line(idx).strip()
                    samples.append((idx + 1, s[:220] + "…" if len(s) > 220 else s))

            # build context windows (--passthru widens the window to the file)
            included_lines = set()
            if passthru:
                included_lines.update(range(len(lines)))
            else:
                for idx in match_indices:
                    start = max(0, idx - context_before)
                    end = min(len(lines), idx + context_after + 1)
                    included_lines.update(range(start, end))

            match_set = set(match_indices)
            offsets, running = {}, 0
            need_offsets = bool(g("byte_offset"))
            if need_offsets:
                for i, line in enumerate(lines):
                    offsets[i + 1] = running
                    running += len(line.encode(encoding or "utf-8", "ignore"))

            for li in sorted(included_lines):
                is_match = li in match_set
                text = render_line(li) if is_match else lines[li].rstrip("\n")
                lines_data.append((li + 1, text, is_match))

            hits[fp] = {
                "count": len(match_indices),
                "samples": samples,
                "terms": {pattern.lower()[:40]},
                "lines_data": lines_data,
                "offsets": offsets,
            }
        except (OSError, UnicodeError, EOFError, zipfile.BadZipFile, tarfile.TarError, lzma.LZMAError) as e:
            if not no_messages:
                print(f"pfind: {fp}: {e}", file=sys.stderr)
            continue
    return hits

# ---------------------------------------------------------------------------
# Name Matching Engine
# ---------------------------------------------------------------------------
def match_names(query, files, regex, ignore_case, fuzzy):
    scored = []
    rx = None
    if regex:
        rx = re.compile(query, re.IGNORECASE if ignore_case else 0)
    q = query.lower() if (ignore_case or fuzzy) else query
    for fp in files:
        base = os.path.basename(fp)
        b = base.lower() if (ignore_case or fuzzy) else base
        p = fp.lower() if (ignore_case or fuzzy) else fp
        score = 0.0
        if regex and rx:
            if rx.search(base):
                score = 3.0
            elif rx.search(fp):
                score = 1.5
        else:
            if q == b or q + Path(base).suffix.lower() == b:
                score = 5.0
            elif q in b:
                score = 3.0
            elif q in p:
                score = 1.5
            elif fuzzy:
                r = SequenceMatcher(None, q, b).ratio()
                if r >= 0.6 or all(c in iter(b) for c in q):
                    score = 0.5 + r
        if score > 0:
            scored.append((fp, score))
    scored.sort(key=lambda t: (-t[1], t[0]))
    return scored

# ---------------------------------------------------------------------------
# Multi-Signal RRF Fusion Engine
# ---------------------------------------------------------------------------
def rrf_fuse(rankings, k, weights=None):
    w_map = weights or RRF_WEIGHTS_DEFAULT
    fused = defaultdict(float)
    sources = defaultdict(list)
    for label, ranked in rankings.items():
        w = w_map.get(label, 1.0)
        for rank, path in enumerate(ranked, start=1):
            fused[path] += w / (k + rank)
            sources[path].append(label)
    out = [(p, s, sources[p]) for p, s in fused.items()]
    out.sort(key=lambda t: (-t[1], t[0]))
    return out

# ---------------------------------------------------------------------------
# Chroma Second Brain Semantic Seam
# ---------------------------------------------------------------------------
def semantic_brain(query, collection, top_k):
    try:
        import chromadb
    except ImportError:
        print("pfind: --brain semantic needs chromadb (import failed); using lexical only.", file=sys.stderr)
        return []
    if BRAIN_CHROMA is None:
        print(f"pfind: no Chroma store configured for --brain; {_PRESET_HELP}; lexical only.", file=sys.stderr)
        return []
    if not BRAIN_CHROMA.exists():
        print(f"pfind: brain store not found at {BRAIN_CHROMA}; lexical only.", file=sys.stderr)
        return []
    try:
        client = chromadb.PersistentClient(path=str(BRAIN_CHROMA))
        col = client.get_collection(collection)
        res = col.query(query_texts=[query], n_results=top_k)
        docs = (res.get("documents") or [[]])[0]
        metas = (res.get("metadatas") or [[]])[0]
        ids = (res.get("ids") or [[]])[0]
        out = []
        for doc, meta, _id in zip(docs, metas, ids):
            meta = meta or {}
            raw = meta.get("source") or meta.get("path") or meta.get("file")
            path = None
            if raw and Path(str(raw)).expanduser().exists():
                path = str(Path(str(raw)).expanduser().resolve())
            snippet = " ".join((doc or "").split())
            if len(snippet) > 240:
                snippet = snippet[:240] + "…"
            out.append({"id": _id, "snippet": snippet or "(empty chunk)", "path": path})
        return out
    except Exception as e:
        print(f"pfind: semantic query degraded ({type(e).__name__}: {str(e)[:100]}); lexical only.", file=sys.stderr)
        return []

# ---------------------------------------------------------------------------
# Tree View Engine (from eza-0.23.5 src/output/tree.rs)
# ---------------------------------------------------------------------------
def render_tree_view(fused_paths, content_hits, git_cache, args, use_color):
    if not fused_paths:
        return
    # Build Trie
    tree = {}
    for p in fused_paths:
        parts = Path(p).parts
        curr = tree
        for part in parts:
            curr = curr.setdefault(part, {})

    def walk_tree(node, prefix="", current_path=Path(), depth=0):
        # Apply level limiting if specified
        if args.level is not None and depth >= args.level:
            return
        
        items = sorted(node.keys())
        for i, name in enumerate(items):
            is_last = (i == len(items) - 1)
            connector = "└── " if is_last else "├── "
            child_prefix = "    " if is_last else "│   "
            full_path = current_path / name
            full_path_str = str(full_path)

            icon = get_file_icon(full_path_str, is_dir=bool(node[name])) if args.icons else ""
            git_st = git_cache.get_status(full_path_str) if git_cache and args.git else None
            git_badge = f" {c('31;1', f'[{git_st}]', use_color)}" if git_st else ""

            display_name = c("1;34", name, use_color) if node[name] else c("1;32", name, use_color)
            if args.hyperlink:
                display_name = make_hyperlink(full_path_str, 0, display_name, True, args.hyperlink_format)

            print(f"{prefix}{connector}{icon + ' ' if icon else ''}{display_name}{git_badge}")

            # print match samples under leaf files
            if not node[name] and full_path_str in content_hits:
                samples = content_hits[full_path_str].get("samples", [])
                for lineno, text in samples:
                    print(f"{prefix}{child_prefix}    {c('90', f'{lineno}:', use_color)} {text}")

            walk_tree(node[name], prefix + child_prefix, full_path, depth + 1)

    # find common roots
    walk_tree(tree)

# ---------------------------------------------------------------------------
# Code Summary View Engine (from eza-0.23.5 src/loc/mod.rs)
# ---------------------------------------------------------------------------
def render_code_summary_table(files, use_color, mode="both"):
    """--code[=lines|percent|both]: aggregate LOC by language. ``lines`` prints
    raw counts, ``percent`` prints each column as a share of that language's
    total lines, ``both`` prints counts with the share alongside."""
    mode = mode if mode in ("lines", "percent", "both") else "both"
    summary = defaultdict(LocCounts)
    for fp in files:
        res = classify_file_loc(fp)
        if res:
            lang_name, counts = res
            summary[lang_name].add(counts)

    if not summary:
        print("pfind: no source code files identified for --code summary.", file=sys.stderr)
        return

    print(f"\n{c('1;37', 'Language Breakdown & Lines of Code Summary:', use_color)}")
    print(f"{'-' * 70}")
    print(f"{'Language':<24} {'Total Lines':>12} {'Code':>10} {'Comments':>10} {'Blanks':>10}")
    print(f"{'-' * 70}")

    def cell(value, total):
        if mode == "percent":
            return f"{(value * 100 / max(total, 1)):>9.1f}%"
        if mode == "both":
            return f"{value:>5,d} ({value * 100 // max(total, 1):>2d}%)"
        return f"{value:>10,d}"

    grand_total = LocCounts()
    for lang, counts in sorted(summary.items(), key=lambda kv: -kv[1].code):
        grand_total.add(counts)
        lang_str = f"{lang:<24}"
        code_str = cell(counts.code, counts.lines)
        comm_str = cell(counts.comments, counts.lines)
        blnk_str = cell(counts.blanks, counts.lines)
        print(f"{c('1;36', lang_str, use_color)} {counts.lines:>12,d} "
              f"{c('32', code_str, use_color)} "
              f"{c('33', comm_str, use_color)} "
              f"{c('90', blnk_str, use_color)}")

    print(f"{'=' * 70}")
    tot_lang = f"{'TOTAL':<24}"
    tot_code = cell(grand_total.code, grand_total.lines)
    tot_comm = cell(grand_total.comments, grand_total.lines)
    tot_blnk = cell(grand_total.blanks, grand_total.lines)
    print(f"{c('1;37', tot_lang, use_color)} {grand_total.lines:>12,d} "
          f"{c('1;32', tot_code, use_color)} "
          f"{c('1;33', tot_comm, use_color)} "
          f"{c('1;90', tot_blnk, use_color)}\n")

# ---------------------------------------------------------------------------
# Directory Size & Color Scale Engines (from eza-0.23.5)
# ---------------------------------------------------------------------------
_DIR_SIZE_CACHE: dict = {}
def get_dir_total_size(path):
    """Recursively calculate total bytes contained in a directory."""
    if path in _DIR_SIZE_CACHE:
        return _DIR_SIZE_CACHE[path]
    total = 0
    try:
        for dirpath, _, filenames in os.walk(path):
            for fn in filenames:
                fp = os.path.join(dirpath, fn)
                try:
                    total += os.lstat(fp).st_size
                except OSError:
                    pass
    except OSError:
        pass
    _DIR_SIZE_CACHE[path] = total
    return total

def get_scale_color(val, kind="size", use_color=True, mode="fixed"):
    """ANSI colour for a size magnitude or a recency delta.

    ``fixed`` picks from discrete bands (the 3.0.0 behaviour). ``gradient``
    interpolates a 256-colour ramp across the magnitude, which is what eza's
    --color-scale-mode=gradient does (eza-0.23.5 src/output/render/size.rs).
    """
    if not use_color:
        return ""
    if mode == "gradient":
        if kind == "size":
            # log10 ramp: 0 B .. 1 GiB spread over the green->red arc
            frac = min(1.0, math.log10(max(float(val), 1.0)) / 9.0)
        else:
            # 0 .. 90 days
            frac = min(1.0, float(val) / (86400.0 * 90.0))
        ramp = [46, 82, 118, 154, 190, 226, 220, 214, 208, 202, 196]
        return f"38;5;{ramp[int(frac * (len(ramp) - 1))]}"
    if kind == "size":
        if val < 1024:
            return "32"           # < 1 KiB: green
        elif val < 1024 * 1024:
            return "36"           # < 1 MiB: cyan
        elif val < 100 * 1024 * 1024:
            return "33"           # < 100 MiB: yellow
        else:
            return "31"           # >= 100 MiB: red
    elif kind == "age":
        if val < 3600:
            return "1;32"         # < 1 hour: bright green
        elif val < 86400:
            return "32"           # < 1 day: green
        elif val < 86400 * 7:
            return "36"           # < 1 week: cyan
        elif val < 86400 * 30:
            return "33"           # < 1 month: yellow
        else:
            return "90"           # > 1 month: dim
    return ""

# ---------------------------------------------------------------------------
# Long Details View Engine (from eza-0.23.5 src/output/table.rs)
# ---------------------------------------------------------------------------
def format_loc_badge(counts, mode):
    """--loc=lines|percent|both (eza-0.23.5 CodeContent)."""
    total = max(counts.lines, 1)
    if mode == "lines":
        return f"[code:{counts.code} com:{counts.comments}]"
    if mode == "percent":
        return f"[code:{counts.code * 100 // total}% com:{counts.comments * 100 // total}%]"
    return (f"[code:{counts.code} ({counts.code * 100 // total}%) "
            f"com:{counts.comments} ({counts.comments * 100 // total}%)]")

def render_long_table(paths, content_hits, git_cache, args, use_color):
    show_perms = not getattr(args, "no_permissions", False)
    show_size = not getattr(args, "no_filesize", False)
    show_user = not getattr(args, "no_user", False)
    show_time = not getattr(args, "no_time", False)
    # pfind's long view always carries a group column (eza gates it behind -g);
    # --group asks for it explicitly and so overrides --smart-group's hiding.
    smart_group = getattr(args, "smart_group", False) and not getattr(args, "group", False)
    numeric = getattr(args, "numeric", False)
    deref = getattr(args, "dereference", False)
    time_field = getattr(args, "time", "modified")
    scale_mode = getattr(args, "color_scale_mode", "fixed")
    quotes = not getattr(args, "no_quotes", False)
    classify = getattr(args, "classify", False)
    repos_mode = getattr(args, "git_repos", False) or getattr(args, "git_repos_no_status", False)

    # Print header if requested
    if args.header:
        header_parts = []
        if args.inode:
            header_parts.append(f"{'INODE':<10}")
        if args.blocks:
            header_parts.append(f"{'BLOCKS':<10}")
        if args.links:
            header_parts.append(f"{'LINKS':<6}")
        if getattr(args, "flags", False):
            header_parts.append(f"{'FLAGS':<8}")
        if show_perms:
            header_parts.append(f"{'PERMISSIONS':<10}")
        header_parts.append(f"{'GIT':<3}")
        if show_user:
            header_parts.append(f"{'USER':<8}")
        header_parts.append(f"{'GROUP':<8}")
        if show_size:
            header_parts.append(f"{'SIZE':>9}")
        if show_time:
            header_parts.append(f"{time_field.upper():<12}")
        if repos_mode:
            header_parts.append(f"{'REPO':<20}")
        header_parts.append("NAME")
        print(c("1;37", " ".join(header_parts), use_color))

    for path in paths:
        try:
            st = get_file_stat(path, deref)
            is_dir = stat.S_ISDIR(st.st_mode)

            # Build metadata columns
            meta_cols = []
            if args.inode:
                meta_cols.append(f"{st.st_ino:<10}")
            if args.blocks:
                blocks = st.st_blocks if hasattr(st, 'st_blocks') else 0
                meta_cols.append(f"{blocks:<10}")
            if args.links:
                meta_cols.append(f"{st.st_nlink:<6}")
            if getattr(args, "flags", False):
                meta_cols.append(f"{get_file_flags(path):<8}")

            mode_str = format_permissions(st.st_mode, args.octal_permissions)

            # Handle size display modes
            if getattr(args, 'total_size', False) and is_dir:
                raw_size = get_dir_total_size(path)
            else:
                raw_size = st.st_size

            if args.bytes:
                size_str = str(raw_size)
            else:
                size_str = format_size(raw_size, not args.metric_units)

            ts = pick_timestamp(st, time_field)
            time_str = format_timestamp(ts, args.time_style)
            if numeric:
                user_str, group_str = str(st.st_uid), str(st.st_gid)
            else:
                try:
                    user_str = pwd.getpwuid(st.st_uid).pw_name if pwd else str(st.st_uid)
                except (KeyError, AttributeError):
                    user_str = str(st.st_uid)
                try:
                    group_str = grp.getgrgid(st.st_gid).gr_name if grp else str(st.st_gid)
                except (KeyError, AttributeError):
                    group_str = str(st.st_gid)
            # --smart-group: show the group only when it differs from the owner
            if smart_group and group_str == user_str:
                group_str = ""
            icon = get_file_icon(path, is_dir) if args.icons else ""
            git_st = git_cache.get_status(path) if git_cache and args.git else None
            git_badge = f"{c('31;1', f'{git_st:<3}', use_color)}" if git_st else "   "

            repo_col = ""
            if repos_mode:
                summary = get_repo_summary(path, with_status=not getattr(args, "git_repos_no_status", False))
                cell = f"{summary or '-':<20}"
                repo_col = c('35', cell, use_color) + " "

            loc_badge = ""
            if args.loc:
                loc_res = classify_file_loc(path)
                if loc_res:
                    _, lc = loc_res
                    loc_badge = " " + format_loc_badge(lc, args.loc if isinstance(args.loc, str) else "both")

            shown_path = display_path(path, args)
            name_text = quote_name(shown_path, quotes) + (classify_indicator(path, st) if classify else "")
            path_display = make_hyperlink(path, 0, name_text, args.hyperlink, args.hyperlink_format)

            # Colors
            if getattr(args, 'color_scale', False):
                size_color = get_scale_color(raw_size, "size", use_color, scale_mode)
                time_color = get_scale_color(time.time() - ts, "age", use_color, scale_mode)
            else:
                size_color = "33"
                time_color = "36"

            meta_prefix = " ".join(meta_cols) + " " if meta_cols else ""
            row = meta_prefix
            if show_perms:
                row += c('90', f'{mode_str:<10}', use_color) + " "
            row += git_badge + " "
            if show_user:
                row += f"{user_str:<8} "
            row += f"{group_str:<8} "
            if show_size:
                row += c(size_color, f'{size_str:>9}', use_color) + " "
            if show_time:
                row += c(time_color, f'{time_str:<12}', use_color) + " "
            row += repo_col
            row += f"{icon + ' ' if icon else ''}{cc('path', path_display, use_color)}{loc_badge}"
            if getattr(args, "mounts", False):
                mount = get_mount_info(path)
                if mount:
                    row += c('90', f"  [{mount[0]} {mount[1]}]", use_color)
            print(row)

            if getattr(args, "extended", False):
                for xname, xsize in get_xattrs(path):
                    print(c('90', f"        {xname} ({xsize} bytes)", use_color))

            if path in content_hits:
                for sample in content_hits[path].get("samples", []):
                    lineno, text = sample[0], sample[1]
                    line_display = make_hyperlink(path, lineno, f"{lineno}:", args.hyperlink, args.hyperlink_format)
                    print(f"    {c('90', line_display, use_color)} {text}")
        except OSError:
            print(path)

# ---------------------------------------------------------------------------
# Grid Layout Rendering (from eza-0.23.5 src/output/grid.rs)
# ---------------------------------------------------------------------------
def grid_terminal_width(args):
    """--width overrides the detected terminal width (eza -w/--width)."""
    override = getattr(args, "width", None)
    if override:
        return max(1, int(override))
    return shutil.get_terminal_size((80, 24)).columns

def render_grid(paths, use_color, args, terminal_width=None):
    """Render paths in a multi-column grid.

    eza fills columns downwards by default and across rows with -x/--across
    (eza-0.23.5 src/output/grid.rs). pfind 3.0.0 only ever filled across; the
    down-column layout is now the default to match eza.
    """
    if not paths:
        return

    if terminal_width is None:
        terminal_width = grid_terminal_width(args)

    quotes = not getattr(args, "no_quotes", False)
    classify = getattr(args, "classify", False)

    cells = []
    for path in paths:
        icon = get_file_icon(path, os.path.isdir(path)) if args.icons else ""
        name = quote_name(os.path.basename(short_nix(path, getattr(args, "short_nix", False))), quotes)
        if classify:
            name += classify_indicator(path)
        is_dir = os.path.isdir(path)
        plain = f"{icon + ' ' if icon else ''}{name}"
        coloured = f"{icon + ' ' if icon else ''}{c('1;34', name, use_color) if is_dir else name}"
        cells.append((plain, coloured))

    col_width = max(len(p) for p, _ in cells) + 2
    num_cols = max(1, terminal_width // col_width)
    num_rows = -(-len(cells) // num_cols)  # ceiling division

    def pad(idx):
        plain, coloured = cells[idx]
        return coloured + " " * max(0, col_width - len(plain))

    if getattr(args, "across", False):
        # Row-major: entries run left to right.
        for r in range(num_rows):
            row = [pad(i) for i in range(r * num_cols, min((r + 1) * num_cols, len(cells)))]
            print("".join(row).rstrip())
    else:
        # Column-major (eza default): entries run top to bottom.
        for r in range(num_rows):
            row = []
            for col in range(num_cols):
                idx = col * num_rows + r
                if idx < len(cells):
                    row.append(pad(idx))
            if row:
                print("".join(row).rstrip())

# ---------------------------------------------------------------------------
# Oneline Rendering (from eza-0.23.5)
# ---------------------------------------------------------------------------
def render_oneline(paths, use_color, args):
    """Render one entry per line (simple list)."""
    classify = getattr(args, "classify", False)
    # --oneline is a list format that gets piped, so names are left unquoted
    # unless quoting was asked for explicitly elsewhere; --no-quotes is a no-op
    # here by design (see --no-quotes help text).
    for path in paths:
        icon = get_file_icon(path, os.path.isdir(path)) if args.icons else ""
        is_dir = os.path.isdir(path)
        shown = display_path(path, args) + (classify_indicator(path) if classify else "")

        if args.long:
            # Simple long format
            try:
                st = get_file_stat(path, getattr(args, "dereference", False))
                size_str = format_size(st.st_size, not args.metric_units)
                display_name = c("1;34", shown, use_color) if is_dir else c("1;32", shown, use_color)
                print(f"{icon + ' ' if icon else ''}{display_name:<60} {c('33', size_str, use_color)}")
            except OSError:
                print(shown)
        else:
            display_name = c("1;34", shown, use_color) if is_dir else shown
            if args.null:
                print(f"{icon + ' ' if icon else ''}{display_name}", end="\0")
            else:
                print(f"{icon + ' ' if icon else ''}{display_name}")

# ---------------------------------------------------------------------------
# Vimgrep Format Rendering (from ripgrep-15.2.0)
# ---------------------------------------------------------------------------
def render_vimgrep(content_hits, use_color, args):
    """Render in vimgrep format: file:line:col:match"""
    items = sorted(content_hits.items())
    if args.limit:
        items = items[:args.limit]
    for path, hit_data in items:
        lines_data = hit_data.get("lines_data", [])
        if not lines_data:
            # Fall back to samples
            for sample in hit_data.get("samples", []):
                if len(sample) == 3:  # Has column info
                    lineno, text, col = sample
                    print(f"{path}:{lineno}:{col}:{text}")
                else:
                    lineno, text = sample
                    col = 1  # Default column
                    print(f"{path}:{lineno}:{col}:{text}")
        else:
            for line_info in lines_data:
                if len(line_info) == 4:  # Has column info
                    lineno, text, is_match, col = line_info
                else:
                    lineno, text, is_match = line_info
                    col = 1
                
                if is_match:  # Only output match lines, not context
                    print(f"{path}:{lineno}:{col}:{text}")

# ---------------------------------------------------------------------------
# Regex Replacement Preview (from ripgrep-15.2.0)
# ---------------------------------------------------------------------------
def apply_replacement_preview(line_text, replacement_pattern, pattern=None, is_regex=False):
    """Apply replacement pattern to occurrences of pattern in line_text.
    Supports $1, $2, ${name} capture groups when is_regex=True."""
    if not replacement_pattern:
        return line_text
    if pattern:
        try:
            if is_regex:
                rg_repl = re.sub(r'\$(\d+)', r'\\\1', replacement_pattern)
                rg_repl = re.sub(r'\$\{(\w+)\}', r'\\g<\1>', rg_repl)
                return re.sub(pattern, rg_repl, line_text)
            else:
                return line_text.replace(pattern, replacement_pattern)
        except Exception:
            return line_text
    return replacement_pattern

# ---------------------------------------------------------------------------
# Standard Ranked Rendering Engine
# ---------------------------------------------------------------------------
def cap_lines_data(lines_data, args):
    """Bound a file's context output to --max MATCH lines, keeping the context
    around each one.

    `--max` is documented as "content samples per file" and has always capped
    the no-context `samples` list, but the context path rendered every entry in
    `lines_data`. One kernel file with 36 hits therefore emitted 36 blocks and
    a single search returned 35 KB. Tool output is re-sent on every later turn,
    so that is a recurring bill, and the cap has to apply where the volume
    actually is. --passthru opts out by definition: it asked for whole files.
    """
    cap = getattr(args, "max", 0) or 0
    if cap <= 0 or getattr(args, "passthru", False):
        return lines_data, 0
    kept, matches_seen = [], 0
    for entry in lines_data:
        is_match = entry[2]
        if is_match:
            matches_seen += 1
            if matches_seen > cap:
                break
        kept.append(entry)
    total = sum(1 for e in lines_data if e[2])
    return kept, max(0, total - cap) if total > cap else 0

def prep_line_text(text, args):
    """--trim strips leading whitespace; --max-columns truncates long lines,
    suppressing them entirely unless --max-columns-preview is given
    (ripgrep-15.2.0 crates/core/flags/defs.rs MaxColumns)."""
    if getattr(args, "trim", False):
        text = text.lstrip()
    limit = getattr(args, "max_columns", 0) or 0
    if limit and len(text) > limit:
        if getattr(args, "max_columns_preview", False):
            return text[:limit] + " [… omitted end of long line]"
        return f"[Omitted long line with {len(text)} chars]"
    return text

def line_prefix(path, lineno, hit_data, args, in_heading):
    """Optional per-line prefixes: -H/--with-filename forces the path even in
    heading mode, -b/--byte-offset prepends the match's absolute byte offset."""
    parts = []
    if getattr(args, "with_filename", False) and in_heading and not getattr(args, "no_filename", False):
        parts.append(path)
    if getattr(args, "byte_offset", False):
        off = (hit_data.get("offsets") or {}).get(lineno)
        if off is not None:
            parts.append(str(off))
    return ":".join(parts) + ":" if parts else ""

def render(fused, name_scores, content_hits, git_cache, args, use_color, query_pattern=None):
    if not fused:
        print("no matches.", file=sys.stderr)
        return 0

    # Extract paths
    paths = [p for p, _, _ in fused]
    
    # Apply file/directory filtering. --show-symlinks keeps symlinks visible
    # even when -D/--only-files would otherwise filter them out (eza's
    # --show-symlinks / --no-symlinks pair).
    keep_links = getattr(args, "show_symlinks", False)
    if args.only_dirs:
        paths = [p for p in paths if os.path.isdir(p) or (keep_links and os.path.islink(p))]
    elif args.only_files:
        paths = [p for p in paths if os.path.isfile(p) or (keep_links and os.path.islink(p))]

    # Apply symlink filtering
    if args.no_symlinks:
        paths = [p for p in paths if not os.path.islink(p)]
    
    # Apply files-with-matches / files-without-match filtering
    if args.files_with_matches:
        paths = [p for p in paths if p in content_hits and content_hits[p].get("count", 0) > 0]
        for p in paths:
            if args.null:
                print(p, end="\0")
            else:
                print(p)
        return 0
    elif args.files_without_match:
        paths = [p for p in paths if p not in content_hits or content_hits[p].get("count", 0) == 0]
        for p in paths:
            if args.null:
                print(p, end="\0")
            else:
                print(p)
        return 0
    
    # Handle count modes
    if args.count_lines or args.count_matches:
        for path in paths:
            if path in content_hits:
                if args.count_matches:
                    count = content_hits[path].get("count", 0)
                else:  # count_lines
                    count = len(content_hits[path].get("samples", []))
                print(f"{count}:{path}")
            elif args.include_zero:
                print(f"0:{path}")
        return 0
    
    # Handle quiet mode (exit code only)
    if args.quiet:
        return 0 if paths else 1
    
    # Rebuild fused with filtered paths
    path_to_data = {p: (s, l) for p, s, l in fused}
    fused = [(p, path_to_data[p][0], path_to_data[p][1]) for p in paths if p in path_to_data]
    
    if not fused:
        print("no matches.", file=sys.stderr)
        return 0
    
    # Handle vimgrep format
    if args.vimgrep:
        render_vimgrep(content_hits, use_color, args)
        return 0
    
    # Handle grid layout
    if args.grid:
        render_grid(paths, use_color, args)
        return 0
    
    # Handle oneline layout
    if args.oneline:
        render_oneline(paths, use_color, args)
        return 0

    # Handle tree view
    if args.tree:
        render_tree_view(paths, content_hits, git_cache, args, use_color)
        return 0

    # Handle long table
    if args.long:
        render_long_table(paths, content_hits, git_cache, args, use_color)
        return 0

    # Standard ranked output
    shown = 0
    for path, score, labels in fused:
        if args.limit and shown >= args.limit:
            break
        shown += 1
        why = []
        if "name" in labels:
            why.append(c("36", "name", use_color))
        if "content" in labels:
            n = content_hits.get(path, {}).get("count", 0)
            why.append(c("33", f"{n} hit{'s' if n != 1 else ''}", use_color))
        if "semantic" in labels:
            why.append(c("35", "semantic", use_color))
        if "git" in labels:
            why.append(c("31", "dirty", use_color))

        tag = ",".join(why)
        icon = get_file_icon(path) if args.icons else ""
        git_st = git_cache.get_status(path) if git_cache and args.git else None
        git_badge = f" {c('31;1', f'[{git_st}]', use_color)}" if git_st else ""

        loc_badge = ""
        if args.loc:
            loc_res = classify_file_loc(path)
            if loc_res:
                _, lc = loc_res
                badge = format_loc_badge(lc, args.loc if isinstance(args.loc, str) else "both")
                loc_badge = f" {c('90', badge, use_color)}"

        shown_name = display_path(path, args) + (classify_indicator(path) if getattr(args, "classify", False) else "")
        path_display = make_hyperlink(path, 0, shown_name, args.hyperlink, args.hyperlink_format)
        header = f"{c('90', f'{score:6.4f}', use_color)}  [{tag}]  {icon + ' ' if icon else ''}{cc('path', path_display, use_color)}{git_badge}{loc_badge}"

        if args.files_only:
            if args.null:
                print(path, end="\0")
            else:
                print(path)
            continue

        # Format path with custom path_separator if specified
        out_path = path.replace("/", args.path_separator).replace("\\", args.path_separator) if getattr(args, 'path_separator', None) else path
        match_sep = getattr(args, 'field_match_separator', ':') or ':'
        ctx_sep = getattr(args, 'field_context_separator', '-') or '-'
        ctx_divider = "" if getattr(args, 'no_context_separator', False) else (getattr(args, 'context_separator', '--') or '--')

        # Handle heading vs no-heading modes
        if args.no_heading:
            # Prefix each line with filename
            hit_data = content_hits.get(path, {})
            if (args.context > 0 or args.after_context > 0 or args.before_context > 0
                    or args.passthru):
                lines_data = hit_data.get("lines_data", [])
                for line_info in lines_data:
                    if len(line_info) == 4:  # Has column info
                        lineno, text, is_match, col = line_info
                    else:
                        lineno, text, is_match = line_info
                        col = 0
                    
                    sep = match_sep if is_match else ctx_sep
                    # --no-filename drops the path *and* the separator that
                    # would otherwise be left dangling at the start of the line.
                    head = "" if getattr(args, "no_filename", False) else f"{out_path}{sep}"
                    if args.column and col > 0 and not args.no_line_number:
                        lineno_disp = f"{head}{lineno}{sep}{col}{sep}"
                    elif not args.no_line_number:
                        lineno_disp = f"{head}{lineno}{sep}"
                    else:
                        lineno_disp = head

                    if args.replace and is_match:
                        # Apply replacement preview
                        text = apply_replacement_preview(text, args.replace, query_pattern, args.regex)
                    off = line_prefix(path, lineno, hit_data, args, in_heading=False)
                    print(f"{off}{lineno_disp} {prep_line_text(text, args)}")
            else:
                for sample in hit_data.get("samples", []):
                    if len(sample) == 3:  # Has column info
                        lineno, text, col = sample
                    else:
                        lineno, text = sample
                        col = 0
                    
                    head = "" if getattr(args, "no_filename", False) else f"{out_path}{match_sep}"
                    if args.column and col > 0 and not args.no_line_number:
                        lineno_disp = f"{head}{lineno}{match_sep}{col}{match_sep}"
                    elif not args.no_line_number:
                        lineno_disp = f"{head}{lineno}{match_sep}"
                    else:
                        lineno_disp = head

                    if args.replace:
                        text = apply_replacement_preview(text, args.replace, query_pattern, args.regex)
                    off = line_prefix(path, lineno, hit_data, args, in_heading=False)
                    print(f"{off}{lineno_disp} {prep_line_text(text, args)}")
        else:
            # Default: heading mode
            print(header)
            if not args.names:
                hit_data = content_hits.get(path, {})
                # If context windows exist, print formatted blocks
                if (args.context > 0 or args.after_context > 0 or args.before_context > 0
                        or args.passthru):
                    lines_data, omitted = cap_lines_data(hit_data.get("lines_data", []), args)
                    last_line = None
                    for line_info in lines_data:
                        if len(line_info) == 4:  # Has column info
                            lineno, text, is_match, col = line_info
                        else:
                            lineno, text, is_match = line_info
                            col = 0
                        
                        if last_line is not None and lineno > last_line + 1 and ctx_divider:
                            print(f"    {c('90', ctx_divider, use_color)}")
                        last_line = lineno
                        
                        sep = match_sep if is_match else ctx_sep
                        if args.column and col > 0:
                            lineno_disp = make_hyperlink(path, lineno, f"{lineno}{sep}{col}{sep}", args.hyperlink, args.hyperlink_format) if not args.no_line_number else ""
                        else:
                            lineno_disp = make_hyperlink(path, lineno, f"{lineno}{sep}", args.hyperlink, args.hyperlink_format) if not args.no_line_number else ""
                        
                        if args.replace and is_match:
                            text = apply_replacement_preview(text, args.replace, query_pattern, args.regex)
                        off = line_prefix(path, lineno, hit_data, args, in_heading=True)
                        text = prep_line_text(text, args)
                        if is_match:
                            print(f"    {off}{cc('line', lineno_disp, use_color)} {text}")
                        else:
                            print(f"    {off}{c('90', lineno_disp, use_color)} {text}")
                    if omitted:
                        # Say what was dropped. A silently capped result reads
                        # as the whole answer.
                        print(f"    {c('90', f'… {omitted} more match(es) in this file (raise --max)', use_color)}")
                else:
                    for sample in hit_data.get("samples", []):
                        if len(sample) == 3:  # Has column info
                            lineno, text, col = sample
                            lineno_disp = make_hyperlink(path, lineno, f"{lineno}{match_sep}{col}{match_sep}", args.hyperlink, args.hyperlink_format) if args.column and not args.no_line_number else make_hyperlink(path, lineno, f"{lineno}{match_sep}", args.hyperlink, args.hyperlink_format) if not args.no_line_number else ""
                        else:
                            lineno, text = sample
                            lineno_disp = make_hyperlink(path, lineno, f"{lineno}{match_sep}", args.hyperlink, args.hyperlink_format) if not args.no_line_number else ""
                        
                        if args.replace:
                            text = apply_replacement_preview(text, args.replace, query_pattern, args.regex)
                        off = line_prefix(path, lineno, hit_data, args, in_heading=True)
                        print(f"    {off}{c('90', lineno_disp, use_color)} {prep_line_text(text, args)}")

    if args.count:
        print(f"\n--- {len(fused)} file(s) matched"
              + (f", showing {shown}" if args.limit and len(fused) > shown else "")
              + " ---", file=sys.stderr)
    return 0

# ---------------------------------------------------------------------------
# Exact & Loose Snippet Search
# ---------------------------------------------------------------------------
def build_loose_regex(snippet):
    parts = [re.escape(tok) for tok in snippet.split()]
    return r"\s+".join(p for p in parts if p)

# ---------------------------------------------------------------------------
# File Sorting
# ---------------------------------------------------------------------------
# eza-0.23.5 src/fs/filter.rs SortField vocabulary. Capitalised variants sort
# case-sensitively (uppercase first); a leading dot ignores a leading dot in the
# name. `age`/`old` are oldest-first, `newest` is newest-first.
SORT_FIELDS = [
    "name", "Name", ".name", ".Name", "size", "ext", "Ext", "type", "inode",
    "date", "time", "modified", "changed", "accessed", "created",
    "age", "old", "newest", "path", "none",
]

def sort_files(files, sort_by, reverse=False, dirs_first=False, dirs_last=False):
    """Sort files by specified field with optional directory grouping."""
    if not sort_by or sort_by == "none":
        files_list = list(files)
    else:
        # age/old = oldest first, newest = newest first: both are mtime with a
        # baked-in direction, applied on top of any explicit --reverse.
        invert = sort_by == "newest"
        field = sort_by

        def get_sort_key(path):
            base = os.path.basename(path)
            try:
                st = os.lstat(path)
            except OSError:
                st = None

            def t(attr, default=0.0):
                return getattr(st, attr, default) if st else default

            if field in ("modified", "date", "time", "age", "old", "newest"):
                return t("st_mtime")
            if field == "accessed":
                return t("st_atime")
            if field == "changed":
                return t("st_ctime")
            if field == "created":
                return getattr(st, "st_birthtime", t("st_ctime")) if st else 0.0
            if field == "size":
                return t("st_size")
            if field == "inode":
                return t("st_ino")
            if field == "type":
                if not st:
                    return (9, base.lower())
                mode = st.st_mode
                rank = (0 if stat.S_ISDIR(mode) else
                        1 if stat.S_ISLNK(mode) else
                        2 if stat.S_ISREG(mode) else 3)
                return (rank, base.lower())
            if field == "ext":
                return (os.path.splitext(base)[1].lower(), base.lower())
            if field == "Ext":
                return (os.path.splitext(base)[1], base)
            if field == "Name":
                return base
            if field == ".name":
                return base.lstrip(".").lower()
            if field == ".Name":
                return base.lstrip(".")
            if field == "path":
                return path
            return base.lower()  # "name"

        files_list = sorted(files, key=get_sort_key, reverse=(reverse != invert))
    
    # Apply directory grouping
    if dirs_first:
        dirs = [f for f in files_list if os.path.isdir(f)]
        files_only = [f for f in files_list if not os.path.isdir(f)]
        return dirs + files_only
    elif dirs_last:
        dirs = [f for f in files_list if os.path.isdir(f)]
        files_only = [f for f in files_list if not os.path.isdir(f)]
        return files_only + dirs
    
    return files_list

# ---------------------------------------------------------------------------
# Language Type Listing
# ---------------------------------------------------------------------------
def print_type_list():
    """Print all language type definitions and exit."""
    print("Language types supported:")
    print()
    for lang, exts in sorted(LANGUAGE_TYPES.items()):
        print(f"  {lang:20s} {', '.join(exts)}")
    print()
    print(f"Total: {len(LANGUAGE_TYPES)} language types")
    print("Usage: pfind 'pattern' -t python -t rust")

# ---------------------------------------------------------------------------
# CLI Argument Parser Definition
# ---------------------------------------------------------------------------
def build_parser():
    p = argparse.ArgumentParser(
        prog="pfind",
        description=f"Unified hybrid ranked search & exploration powerhouse — tuned for {MACHINE}.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="presets: --brain --work --src --all | engines: ripgrep-15.2.0 + eza-0.23.5 + pfind RRF"
    )
    p.add_argument("query", nargs="?", default=None, help="what to find (literal substring, regex, or snippet)")
    p.add_argument("paths", nargs="*", help="roots to search (default: cwd, or a preset)")

    # presets
    p.add_argument("--brain", action="store_true", help="search the configured 'brain' preset root (+ Chroma semantic seam if configured)")
    p.add_argument("--work", action="store_true", help="search the configured 'work' preset root")
    p.add_argument("--src", action="store_true", help="search the configured 'src' preset root")
    p.add_argument("--all", action="store_true", help="search all configured presets in one shot (see ~/.config/pfind/presets.json)")

    # search modes
    p.add_argument("-r", "--regex", action="store_true", help="treat query as a regex")
    p.add_argument("-F", "--fixed-strings", action="store_true", help="treat query as a literal string (default)")
    p.add_argument("-i", "--ignore-case", action="store_true", help="force case-insensitive")
    p.add_argument("-s", "--case-sensitive", action="store_true", help="force strict case-sensitive")
    p.add_argument("-S", "--smart-case", action="store_true", help="smart case sensitivity (default)")
    p.add_argument("-w", "--word-regexp", action="store_true", help="match whole words only")
    p.add_argument("-x", "--exact", action="store_true", help="exact multi-line snippet: literal match, spans newlines")
    p.add_argument("--loose", action="store_true", help="snippet match tolerant of whitespace/indent drift")
    p.add_argument("--fuzzy", action="store_true", help="typo/partial-tolerant name matching")
    p.add_argument("-v", "--invert-match", action="store_true", help="select non-matching lines")
    p.add_argument("--pcre2", action="store_true", help="enable PCRE2 regex engine")
    p.add_argument("--engine", choices=["default", "pcre2", "auto"], default="default",
                   help="regex engine selector (default|pcre2|auto)")
    p.add_argument("-U", "--multiline", action="store_true", help="allow matches to span line boundaries")
    p.add_argument("--multiline-dotall", action="store_true", help="with --multiline, let . match newlines")
    p.add_argument("--line-regexp", action="store_true", help="match only whole lines")
    p.add_argument("-E", "--encoding", type=str, default=None, help="force text encoding (utf-16, latin1, ...)")
    p.add_argument("--no-unicode", action="store_true", help="disable Unicode-aware matching (ASCII only)")
    p.add_argument("--crlf", action="store_true", help="treat CRLF as the line terminator")
    p.add_argument("--null-data", action="store_true", help="use NUL as the line terminator")
    p.add_argument("--stop-on-nonmatch", action="store_true", help="stop searching a file after a non-match past a match")

    # context & sinks
    p.add_argument("-C", "--context", type=int, default=0, help="show NUM lines before and after match")
    p.add_argument("-A", "--after-context", type=int, default=0, help="show NUM lines after match")
    p.add_argument("-B", "--before-context", type=int, default=0, help="show NUM lines before match")
    p.add_argument("--context-separator", type=str, default="--", help="separator between context blocks (default: --)")
    p.add_argument("--no-context-separator", action="store_true", help="suppress separator between context blocks")
    p.add_argument("--field-context-separator", type=str, default="-", help="separator for context line numbers (default: -)")
    p.add_argument("--field-match-separator", type=str, default=":", help="separator for match line numbers (default: :)")
    p.add_argument("--path-separator", type=str, default=None, help="override path separator in output")
    p.add_argument("-o", "--only-matching", action="store_true", help="show only matching parts")
    p.add_argument("--replace", type=str, default=None, help="show regex replacement preview ($1, $name, ${1})")
    p.add_argument("-m", "--max-count", type=int, default=0, help="max matches per file")
    p.add_argument("-c", "--count-lines", action="store_true", help="count matching lines per file")
    p.add_argument("--count-matches", action="store_true", help="count individual matches (not lines)")
    p.add_argument("--include-zero", action="store_true", help="with --count, show files with 0 matches too")
    p.add_argument("-M", "--max-columns", type=int, default=0, help="omit lines longer than NUM characters")
    p.add_argument("--max-columns-preview", action="store_true", help="with --max-columns, show a truncated preview instead")
    p.add_argument("-b", "--byte-offset", action="store_true", help="print the absolute byte offset of each line")
    p.add_argument("-H", "--with-filename", action="store_true", help="print the file path on every output line")
    p.add_argument("--no-filename", action="store_true", help="never print the file path on output lines")
    p.add_argument("--trim", action="store_true", help="strip leading whitespace from output lines")
    p.add_argument("--passthru", "--passthrough", action="store_true", dest="passthru",
                   help="print every line of matching files, not just matches")
    p.add_argument("--colors", action="append", dest="color_specs", metavar="SPEC", default=[],
                   help="per-component colour, e.g. 'match:fg:red' or 'path:fg:0x00AAFF' (repeatable)")
    p.add_argument("--hostname-bin", type=str, default=None, metavar="CMD",
                   help="command whose output supplies the hostname in OSC 8 hyperlinks")

    # scope & types
    p.add_argument("-t", "--type", action="append", dest="types", help="filter by language type (rust, py, c, etc.)")
    p.add_argument("--type-not", action="append", dest="types_not", help="exclude language type")
    p.add_argument("--type-add", action="append", dest="type_adds", metavar="SPEC", help="add custom language type e.g. 'pdf:*.pdf'")
    p.add_argument("--type-clear", action="append", dest="type_clears", metavar="TYPE", help="clear language type globs (or 'all')")
    p.add_argument("--type-list", action="store_true", help="list all language type definitions and exit")
    p.add_argument("--pre", type=str, default=None, help="pipe candidate files through external preprocessor")
    p.add_argument("--pre-glob", type=str, default=None, help="only apply --pre to matching files")
    p.add_argument("--ext", nargs="+", metavar="EXT", help="only these extensions (.py .sh ...)")
    p.add_argument("--exclude", nargs="+", metavar="GLOB", default=[], help="extra excludes")
    p.add_argument("-g", "--glob", action="append", dest="globs", default=[], help="include/exclude glob (repeatable)")
    p.add_argument("--iglob", action="append", dest="iglobs", default=[], metavar="GLOB",
                   help="case-insensitive include/exclude glob (repeatable)")
    p.add_argument("--glob-case-insensitive", action="store_true", help="treat all -g globs as case-insensitive")
    p.add_argument("--ignore-file", action="append", dest="ignore_files", default=[], metavar="PATH",
                   help="load additional ignore rules from PATH (repeatable)")
    p.add_argument("--ignore-file-case-insensitive", action="store_true", help="match --ignore-file rules case-insensitively")
    p.add_argument("-e", "--regexp", action="append", dest="patterns", default=[], help="additional pattern (repeatable, OR logic)")
    p.add_argument("-f", "--pattern-file", type=str, help="read patterns from file (one per line)")
    p.add_argument("-z", "--search-zip", action="store_true", help="search inside compressed archives (.gz, .zip, etc.)")
    p.add_argument("-a", "--text", action="store_true", help="treat binary files as text (no NUL filtering)")
    p.add_argument("--binary", action="store_true", help="search binary files but warn on NUL bytes")
    p.add_argument("--hidden", action="store_true", help="include hidden files")
    p.add_argument("--no-ignore", action="store_true", help="ignore .gitignore and .ignore rules")
    p.add_argument("--no-ignore-dot", action="store_true", help="don't respect .ignore / .rgignore")
    p.add_argument("--no-ignore-global", action="store_true", help="don't respect ~/.config/git/ignore")
    p.add_argument("--no-ignore-parent", action="store_true", help="don't ascend for ignore files")
    p.add_argument("--no-ignore-vcs", action="store_true", help="ignore .gitignore but keep .ignore")
    p.add_argument("--no-ignore-exclude", action="store_true", help="don't respect .git/info/exclude")
    p.add_argument("--no-ignore-messages", action="store_true", help="suppress errors about malformed ignore files")
    p.add_argument("--no-require-git", action="store_true", help="respect git ignore rules outside a git repository")
    p.add_argument("--git-ignore", action="store_true", help="honour .gitignore when listing files")
    p.add_argument("-u", "--unrestricted", action="count", default=0, help="-u skip ignores; -uu +hidden; -uuu +binary")
    p.add_argument("-d", "--max-depth", type=int, default=None, help="max directory descent depth")
    p.add_argument("--max-filesize", type=str, help="skip files larger than NUM (bytes, K, M, G)")
    p.add_argument("--one-file-system", action="store_true", help="don't cross filesystem boundaries")
    p.add_argument("--mmap", action="store_true", help="force memory-mapped I/O where possible")
    p.add_argument("--no-mmap", action="store_true", help="never use memory-mapped I/O")
    p.add_argument("--block-buffered", action="store_true", help="force block buffering on output")
    p.add_argument("--line-buffered", action="store_true", help="force line buffering on output")
    p.add_argument("--dfa-size-limit", type=str, default=None, metavar="NUM", help="cap the regex DFA cache size")
    p.add_argument("--regex-size-limit", type=str, default=None, metavar="NUM", help="cap the compiled regex size")
    p.add_argument("--no-config", action="store_true", help="ignore RIPGREP_CONFIG_PATH")

    # presentation & eza views
    p.add_argument("-l", "--long", action="store_true", help="show detailed file metadata table")
    p.add_argument("-T", "--tree", action="store_true", help="render hierarchical directory tree")
    p.add_argument("--level", type=int, default=None, help="max tree depth (for --tree)")
    p.add_argument("-R", "--recurse", action="store_true", help="flat recursive listing (no tree connectors)")
    p.add_argument("-G", "--grid", action="store_true", help="multi-column grid layout")
    p.add_argument("-1", "--oneline", action="store_true", help="one entry per line")
    p.add_argument("--across", action="store_true", help="fill the grid across rows rather than down columns")
    p.add_argument("--width", type=int, default=None, metavar="COLS", help="override the detected terminal width")
    p.add_argument("--loc", nargs="?", const="both", choices=["lines", "percent", "both"], default=None,
                   help="comment-aware lines of code (=lines|percent|both, default both)")
    p.add_argument("--code", nargs="?", const="both", choices=["lines", "percent", "both"], default=None,
                   help="aggregated code/comment/blank summary table (=lines|percent|both)")
    p.add_argument("--git", action="store_true", default=True, help="display Git status flags (default: on)")
    p.add_argument("--no-git", action="store_true", help="suppress all git columns")
    p.add_argument("--git-repos", action="store_true", help="show branch and dirty/clean state for repository directories")
    p.add_argument("--git-repos-no-status", action="store_true", help="show repository branch only (skips the status scan)")
    p.add_argument("--git-dirty", action="store_true", help="filter/boost modified Git files")
    p.add_argument("--dirty-boost", action="store_true", help="boost uncommitted Git files in RRF ranking")
    p.add_argument("--recency-boost", action="store_true", help="boost recently modified files in RRF ranking")
    p.add_argument("--color-scale", action="store_true", help="highlight size and recency with adaptive color scales")
    p.add_argument("--color-scale-mode", choices=["fixed", "gradient"], default="fixed", help="color scale mode (fixed|gradient)")
    p.add_argument("--total-size", action="store_true", help="calculate directory size as sum of contents")
    p.add_argument("--time", choices=["modified", "accessed", "changed", "created"], default="modified",
                   help="timestamp field shown in --long")
    p.add_argument("--changed", action="store_true", help="show the changed (ctime) timestamp; same as --time changed")
    p.add_argument("--accessed", action="store_true", help="show the accessed timestamp; same as --time accessed")
    p.add_argument("--modified", action="store_true", help="show the modified timestamp (default)")
    p.add_argument("--created", action="store_true", help="show the created timestamp; same as --time created")
    p.add_argument("--time-style", type=str, default="relative", metavar="STYLE",
                   help="timestamp format: default|iso|long-iso|full-iso|relative|+FORMAT (strftime)")
    p.add_argument("--octal-permissions", action="store_true", help="show numeric octal permissions")
    p.add_argument("--blocks", action="store_true", help="show allocated filesystem blocks")
    p.add_argument("--inode", action="store_true", help="show inode number")
    p.add_argument("--links", action="store_true", help="show hard link count")
    p.add_argument("--metric-units", action="store_true", help="use 1000-based metric units instead of 1024 binary")
    p.add_argument("--binary-units", action="store_true", help="use 1024-based binary units (KiB, MiB) - default")
    p.add_argument("--bytes", action="store_true", help="show raw byte count (no unit prefix)")
    p.add_argument("--header", action="store_true", help="show column header row")
    p.add_argument("--no-permissions", action="store_true", help="suppress the permissions column")
    p.add_argument("--no-filesize", action="store_true", help="suppress the file size column")
    p.add_argument("--no-user", action="store_true", help="suppress the user column")
    p.add_argument("--no-time", action="store_true", help="suppress the timestamp column")
    # eza spells this -g, but -g is ripgrep's --glob and pfind keeps ripgrep's
    # meaning for the short flag; --group is long-form only here.
    p.add_argument("--group", action="store_true", help="show the group column (shown by default in --long)")
    p.add_argument("--smart-group", action="store_true", help="show the group only when it differs from the owner")
    p.add_argument("--numeric", action="store_true", help="show user and group as numeric UID/GID")
    p.add_argument("--extended", action="store_true", help="list extended attributes (xattrs) and their sizes")
    p.add_argument("--flags", action="store_true", help="show file flags (BSD st_flags; ext attributes via lsattr on Linux)")
    p.add_argument("--mounts", action="store_true", help="show device and filesystem type for mount points")
    p.add_argument("--dereference", action="store_true", help="show the symlink target's metadata, not the link's")
    p.add_argument("--classify", action="store_true", help="append a type indicator (/ * @ | =) to names")
    p.add_argument("--absolute", nargs="?", const="on", choices=["on", "off", "follow"], default="off",
                   help="print absolute paths (on|off|follow; follow resolves symlinks)")
    p.add_argument("--no-quotes", action="store_true", help="don't quote names containing spaces or shell characters")
    p.add_argument("--short-nix", action="store_true", help="abbreviate Nix store hashes in paths")
    p.add_argument("--stdin", action="store_true", help="read the list of paths from stdin, one per line")
    p.add_argument("--treat-dirs-as-files", action="store_true", help="list directories as entries without descending")
    p.add_argument("--icons", action="store_true", help="display Nerd Font file icons")
    p.add_argument("--hyperlink", action="store_true", help="emit OSC 8 clickable terminal hyperlinks")
    p.add_argument("--hyperlink-format", type=str, default="file", choices=["file", "vscode", "grep+"], 
                   help="OSC 8 hyperlink format (file, vscode, grep+)")
    
    # filtering & sorting (eza)
    p.add_argument("--dirs-first", action="store_true", help="group directories before files")
    p.add_argument("--dirs-last", action="store_true", help="group directories after files")
    p.add_argument("-D", "--only-dirs", action="store_true", help="show only directories")
    p.add_argument("--only-files", action="store_true", help="show only files")
    p.add_argument("--show-symlinks", action="store_true", help="include symlinks with --only-dirs/--only-files")
    p.add_argument("--no-symlinks", action="store_true", help="exclude symlinks from listing")
    p.add_argument("--follow-symlinks", action="store_true", help="follow symlinks into directories")
    p.add_argument("--almost-all", action="store_true", help="include dotfiles, exclude . and ..")
    p.add_argument("-I", "--ignore-glob", type=str, help="pipe-separated globs to hide")
    p.add_argument("--reverse", action="store_true", help="reverse current sort order")

    # tuning & output
    p.add_argument("--names", action="store_true", help="filenames only")
    p.add_argument("--content", action="store_true", help="file contents only")
    p.add_argument("--files", action="store_true", help="list files that would be searched (no search)")
    p.add_argument("--files-with-matches", action="store_true", 
                   help="print only paths with matches")
    p.add_argument("--files-without-match", action="store_true", help="print only paths without matches")
    p.add_argument("--files-only", action="store_true", help="print paths only")
    p.add_argument("-q", "--quiet", action="store_true", help="suppress output; use exit code only")
    p.add_argument("--no-messages", action="store_true", help="suppress file open/read errors")
    p.add_argument("--count", action="store_true", help="print match count summary")
    p.add_argument("--stats", action="store_true", help="print search performance and match statistics")
    p.add_argument("--column", action="store_true", help="show 1-based column number of first match")
    p.add_argument("-n", "--line-number", action="store_true", default=True, help="show line numbers (default)")
    p.add_argument("-N", "--no-line-number", action="store_true", help="suppress line numbers")
    p.add_argument("--heading", action="store_true", help="group under file header")
    p.add_argument("--no-heading", action="store_true", help="prefix each line with filename")
    p.add_argument("--vimgrep", action="store_true", help="file:line:col:match format (vim :cgetfile)")
    p.add_argument("-p", "--pretty", action="store_true", help="force --color=always --heading --line-number")
    p.add_argument("-0", "--null", action="store_true", help="NUL-terminate file paths")
    p.add_argument("--sort", type=str, choices=SORT_FIELDS, metavar="FIELD",
                   help="sort by: " + " | ".join(SORT_FIELDS))
    p.add_argument("--sortr", type=str, choices=SORT_FIELDS, metavar="FIELD",
                   help="reverse sort by the same field vocabulary as --sort")
    p.add_argument("--color", type=str, choices=["never", "auto", "always", "ansi"], default="auto",
                   help="color output (never|auto|always|ansi)")
    p.add_argument("--workers", type=int, default=LOGICAL_CPUS, help=f"threads (default {LOGICAL_CPUS})")
    p.add_argument("--timeout", type=int, default=600, help="timeout seconds (default 600)")
    p.add_argument("--rrf-k", type=int, default=RRF_K_DEFAULT, help=f"RRF k (default {RRF_K_DEFAULT})")
    p.add_argument("--max", type=int, default=5, help="content samples per file (default 5)")
    p.add_argument("--limit", type=int, default=40, help="max files shown (default 40; 0 = all)")
    p.add_argument("--collection", default=BRAIN_DEFAULT_COLLECTION, help="brain collection for semantic seam")
    p.add_argument("--no-color", action="store_true", help="disable ANSI color")

    return p

# ---------------------------------------------------------------------------
# Main Execution Entry Point
# ---------------------------------------------------------------------------
def main():
    start_time = time.time()
    args = build_parser().parse_args()

    # Dynamic language type manipulation
    if getattr(args, 'type_clears', None):
        for tc in args.type_clears:
            tc_lower = tc.lower()
            if tc_lower == "all":
                LANGUAGE_TYPES.clear()
            elif tc_lower in LANGUAGE_TYPES:
                del LANGUAGE_TYPES[tc_lower]
    if getattr(args, 'type_adds', None):
        for ta in args.type_adds:
            if ":" in ta:
                t_name, globs_str = ta.split(":", 1)
                exts = [g.strip() for g in globs_str.split(",") if g.strip()]
                exts_norm = [e if e.startswith(".") else ("." + e if not e.startswith("*") else e.lstrip("*")) for e in exts]
                LANGUAGE_TYPES[t_name.lower().strip()] = exts_norm

    # Early exits
    if args.type_list:
        print_type_list()
        return 0

    # --colors / --hostname-bin affect every later render, so resolve them first
    parse_color_specs(args.color_specs)
    if args.hostname_bin:
        resolve_hostname_bin(args.hostname_bin)

    # eza's standalone timestamp-field flags are aliases for --time FIELD;
    # last one on the command line does not win here, most specific does.
    if args.changed:
        args.time = "changed"
    elif args.accessed:
        args.time = "accessed"
    elif args.created:
        args.time = "created"
    elif args.modified:
        args.time = "modified"

    # --engine is the general form of --pcre2; either one enables PCRE2.
    if args.engine == "pcre2":
        args.pcre2 = True
    if args.pcre2 and args.engine == "default":
        args.engine = "pcre2"

    # --binary-units is the default; --metric-units is the only real switch.
    if args.binary_units and args.metric_units:
        print("pfind: --binary-units and --metric-units conflict; using binary (1024)", file=sys.stderr)
        args.metric_units = False

    if args.mmap and args.no_mmap:
        print("pfind: --mmap and --no-mmap conflict; letting ripgrep choose", file=sys.stderr)
        args.mmap = args.no_mmap = False

    # --stdin: the path list comes from stdin rather than the command line.
    if args.stdin:
        stdin_paths = [line.rstrip("\n") for line in sys.stdin if line.strip()]
        if not stdin_paths:
            print("pfind: --stdin given but no paths on stdin", file=sys.stderr)
            return 1
        args.paths = list(args.paths) + stdin_paths

    if not HAVE_RG:
        # Flags that only exist as ripgrep passthrough have no pure-Python
        # equivalent. Say so rather than appearing to honour them.
        unsupported = [n for n, v in (
            ("--pre", args.pre), ("--pre-glob", args.pre_glob), ("--pcre2", args.pcre2),
            ("--mmap", args.mmap), ("--no-mmap", args.no_mmap),
            ("--dfa-size-limit", args.dfa_size_limit), ("--regex-size-limit", args.regex_size_limit),
            ("--null-data", args.null_data), ("--no-unicode", args.no_unicode),
            ("--block-buffered", args.block_buffered), ("--line-buffered", args.line_buffered),
            ("--no-config", args.no_config), ("--ignore-file", args.ignore_files),
        ) if v]
        if unsupported and not args.no_messages:
            print(f"pfind: ripgrep not installed; ignoring {', '.join(unsupported)} "
                  f"(no pure-Python equivalent)", file=sys.stderr)

    # Handle color mode
    if args.color == "never" or args.no_color:
        use_color = False
    elif args.color == "always" or args.color == "ansi":
        use_color = True
    else:  # auto
        use_color = sys.stdout.isatty() and not args.no_color
    
    # Handle --pretty flag
    if args.pretty:
        use_color = True
        args.heading = True
        args.line_number = True
    
    # Handle --no-git override
    if args.no_git:
        args.git = False
    
    # Handle -u/--unrestricted levels
    if args.unrestricted >= 1:
        args.no_ignore = True
    if args.unrestricted >= 2:
        args.hidden = True
    if args.unrestricted >= 3:
        args.text = True

    # --git-repos-no-status implies the repo column, minus the status scan
    if args.git_repos_no_status:
        args.git_repos = True

    presets = [name for name in ("brain", "work", "src") if getattr(args, name)]
    if args.all:
        presets = ["brain", "work", "src"]

    # Standalone --code summary mode: all positional args are paths
    if args.code:
        path_list = list(args.paths)
        if args.query:
            path_list.insert(0, args.query)
        roots = resolve_roots(path_list, presets)
        type_globs = resolve_type_globs(args.types, args.types_not)
        all_files = (rg_list_files(roots, args.ext, args.exclude, type_globs, args.hidden, args.no_ignore, args.workers, args.max_depth, globs=args.globs, opts=args)
                     if HAVE_RG else py_fallback_files(roots, set(args.ext or []), args.exclude, type_globs, args.max_depth, globs=args.globs, opts=args))
        for r in roots:
            if os.path.isfile(r) and str(r) not in all_files:
                all_files.append(str(r))
        if args.git_ignore and HAVE_RG:
            pass  # rg already applied .gitignore during enumeration
        elif args.git_ignore:
            all_files = git_ignored_filter(all_files)
        render_code_summary_table(all_files, use_color, args.code)
        return 0
    
    # --files mode: list files that would be searched (no search)
    if args.files:
        path_list = list(args.paths)
        if args.query:
            path_list.insert(0, args.query)
        roots = resolve_roots(path_list, presets)
        type_globs = resolve_type_globs(args.types, args.types_not)
        all_files = (rg_list_files(roots, args.ext, args.exclude, type_globs, args.hidden, args.no_ignore, args.workers, args.max_depth, globs=args.globs, opts=args)
                     if HAVE_RG else py_fallback_files(roots, set(args.ext or []), args.exclude, type_globs, args.max_depth, globs=args.globs, opts=args))
        for r in roots:
            if os.path.isfile(r) and str(r) not in all_files:
                all_files.append(str(r))
        # --limit applies here as everywhere else. This branch printed every
        # path unconditionally, which on a kernel-sized tree is a 14 MB dump;
        # a limit that only binds some modes is not a limit. The cut is
        # announced — silence and truncation must never look alike.
        shown = all_files[:args.limit] if args.limit else all_files
        for f in shown:
            print(f)
        if len(all_files) > len(shown):
            print(f"… and {len(all_files) - len(shown)} more file(s) not shown (raise --limit)")
        return 0

    # Exploratory Mode (zero query or explicit directory target with tree/long flags)
    is_exploratory = (not args.query or args.query == ".")
    # A layout flag plus a single existing path means "list that path", not
    # "search for a file named like that path". --grid/--oneline/--recurse are
    # included here so the grid options (--across, --width) are reachable on a
    # named directory, not only on the implicit cwd.
    if not is_exploratory and (args.tree or args.long or args.grid or args.oneline
                               or args.recurse or args.treat_dirs_as_files) and not args.paths:
        if os.path.exists(args.query):
            args.paths = [args.query]
            args.query = None
            is_exploratory = True

    if is_exploratory:
        roots = resolve_roots(args.paths, presets)
        type_globs = resolve_type_globs(args.types, args.types_not)
        git_cache = GitStatusCache(roots) if args.git else None
        if args.treat_dirs_as_files:
            # eza --treat-dirs-as-files: the named directories are the entries;
            # nothing is descended into.
            all_files = [str(r) for r in roots]
            limit = args.limit if args.limit else 100
            render_long_table(all_files[:limit], {}, git_cache, args, use_color)
            return 0
        if args.only_dirs:
            all_files = py_fallback_files(roots, args.ext, args.exclude, type_globs, args.max_depth, include_dirs=True, include_hidden=args.hidden or args.almost_all, globs=args.globs, opts=args)
            all_files = [f for f in all_files if os.path.isdir(f)]
        else:
            all_files = (rg_list_files(roots, args.ext, args.exclude, type_globs, args.hidden, args.no_ignore, args.workers, args.max_depth, globs=args.globs, opts=args)
                         if HAVE_RG else py_fallback_files(roots, set(args.ext or []), args.exclude, type_globs, args.max_depth, include_hidden=args.hidden or args.almost_all, globs=args.globs, opts=args))
            for r in roots:
                if os.path.isfile(r) and str(r) not in all_files:
                    all_files.append(str(r))
        
        # Apply filtering
        if args.only_files:
            all_files = [f for f in all_files
                         if os.path.isfile(f) or (args.show_symlinks and os.path.islink(f))]

        if args.no_symlinks:
            all_files = [f for f in all_files if not os.path.islink(f)]

        if args.git_ignore and not HAVE_RG:
            all_files = git_ignored_filter(all_files)
        
        if args.almost_all:
            # Include dotfiles but not . and ..
            # This is already handled by file enumeration, just noting it here
            pass
        
        if args.ignore_glob:
            # Apply custom ignore patterns
            ignore_patterns = args.ignore_glob.split('|')
            filtered = []
            for f in all_files:
                basename = os.path.basename(f)
                if not any(re.match(pat.replace('*', '.*'), basename) for pat in ignore_patterns):
                    filtered.append(f)
            all_files = filtered
        
        if args.git_dirty and git_cache:
            all_files = [f for f in all_files if git_cache.get_status(f)]
        
        # Apply sorting
        sort_field = args.sortr if args.sortr else args.sort
        reverse = bool(args.sortr) or args.reverse
        all_files = sort_files(all_files, sort_field, reverse, args.dirs_first, args.dirs_last)
        
        # Apply limit
        limit = args.limit if args.limit else (200 if args.tree else 100)
        all_files = all_files[:limit]
        
        # Handle output modes
        if args.recurse:
            # Flat recursive listing (no tree connectors)
            for f in all_files:
                icon = get_file_icon(f, os.path.isdir(f)) if args.icons else ""
                is_dir = os.path.isdir(f)
                display_name = c("1;34", f, use_color) if is_dir else c("1;32", f, use_color)
                if args.null:
                    print(f"{icon + ' ' if icon else ''}{display_name}", end="\0")
                else:
                    print(f"{icon + ' ' if icon else ''}{display_name}")
        elif args.grid:
            render_grid(all_files, use_color, args)
        elif args.oneline:
            render_oneline(all_files, use_color, args)
        elif args.tree:
            render_tree_view(all_files, {}, git_cache, args, use_color)
        elif args.long:
            render_long_table(all_files, {}, git_cache, args, use_color)
        else:
            render_long_table(all_files, {}, git_cache, args, use_color)
        return 0

    roots = resolve_roots(args.paths, presets)
    type_globs = resolve_type_globs(args.types, args.types_not)
    git_cache = GitStatusCache(roots) if args.git else None
    
    # Handle multiple patterns (-e/--regexp, -f/--pattern-file)
    patterns = []
    if args.query:
        patterns.append(args.query)
    if args.patterns:
        patterns.extend(args.patterns)
    if args.pattern_file:
        try:
            with open(args.pattern_file, 'r') as f:
                patterns.extend([line.strip() for line in f if line.strip()])
        except OSError as e:
            print(f"pfind: error reading pattern file: {e}", file=sys.stderr)
            return 1
    
    # If multiple patterns, combine with OR (|)
    if len(patterns) > 1:
        query = '|'.join(f'({re.escape(p) if not args.regex else p})' for p in patterns)
        args.regex = True  # Force regex mode for multiple patterns
    elif patterns:
        query = patterns[0]
    else:
        print("pfind: no query specified", file=sys.stderr)
        return 1

    # Search Mode
    snippet_mode = args.exact or args.loose or ("\n" in query)
    do_names = (not args.content) and not snippet_mode
    do_content = not args.names or snippet_mode

    if args.loose:
        pattern, use_regex, use_fixed, multiline = build_loose_regex(query), True, False, True
    elif snippet_mode:
        pattern, use_regex, use_fixed, multiline = query, False, True, True
    else:
        pattern = query
        use_regex = args.regex
        use_fixed = args.fixed_strings or (not args.regex)
        # -U/--multiline is the explicit form of what -x/--loose enable implicitly
        multiline = args.multiline or args.multiline_dotall

    # 1. Name search
    name_ranked, name_scores = [], {}
    if do_names:
        files = (rg_list_files(roots, args.ext, args.exclude, type_globs, args.hidden, args.no_ignore, args.workers, args.max_depth, globs=args.globs, opts=args)
                 if HAVE_RG else py_fallback_files(roots, set(args.ext or []), args.exclude, type_globs, args.max_depth, globs=args.globs, opts=args))
        for r in roots:
            if os.path.isfile(r) and str(r) not in files:
                files.append(str(r))
        scored = match_names(query, files, use_regex, args.ignore_case, args.fuzzy)
        name_ranked = [fp for fp, _ in scored]
        name_scores = dict(scored)

    # 2. Content search
    content_hits = {}
    if do_content:
        ctx_b = args.context or args.before_context
        ctx_a = args.context or args.after_context
        if HAVE_RG:
            content_hits = rg_content(
                pattern, roots, use_regex, args.ignore_case, args.case_sensitive,
                args.word_regexp, args.ext, args.exclude, type_globs, args.hidden,
                args.no_ignore, args.workers, args.max, fixed=use_fixed,
                multiline=multiline, search_zip=args.search_zip,
                context_before=ctx_b, context_after=ctx_a, max_depth=args.max_depth,
                pcre2=args.pcre2, invert_match=args.invert_match, timeout=args.timeout,
                extract_column=args.column, pre=args.pre, pre_glob=args.pre_glob,
                globs=args.globs, opts=args, only_matching=args.only_matching,
                smart_case=args.smart_case
            )
        else:
            files = py_fallback_files(roots, set(args.ext or []), args.exclude, type_globs, args.max_depth, globs=args.globs, opts=args)
            # os.walk yields nothing for a root that is itself a file, so a
            # named file would silently return "no matches" without this.
            for r in roots:
                if os.path.isfile(r) and str(r) not in files:
                    files.append(str(r))
            content_hits = py_fallback_content(
                pattern, files, use_regex, args.ignore_case, args.case_sensitive,
                args.word_regexp, args.max, search_zip=args.search_zip,
                context_before=ctx_b, context_after=ctx_a, invert_match=args.invert_match,
                opts=args, only_matching=args.only_matching
            )

    # Content rank by distinct-term coverage first, then total volume
    content_ranked = [p for p, _ in sorted(
        content_hits.items(),
        key=lambda kv: (-len(kv[1].get("terms", ())), -kv[1]["count"], kv[0])
    )]

    # 3. Semantic seam (opt-in --brain)
    semantic_ranked, brain_recall = [], []
    if args.brain:
        for hit in semantic_brain(args.query, args.collection, top_k=max(args.max, 5)):
            if hit["path"]:
                if hit["path"] not in semantic_ranked:
                    semantic_ranked.append(hit["path"])
            else:
                brain_recall.append(hit)

    # 4. Optional Git Dirty & Recency signals
    git_ranked = []
    if (args.git_dirty or args.dirty_boost) and git_cache:
        all_candidates = set(name_ranked) | set(content_ranked)
        git_ranked = [p for p in all_candidates if git_cache.get_status(p)]

    recency_ranked = []
    if args.recency_boost:
        all_candidates = set(name_ranked) | set(content_ranked)
        recency_ranked = sorted(
            all_candidates,
            key=lambda p: -os.path.getmtime(p) if os.path.exists(p) else 0
        )

    # 5. Multi-Signal RRF Fusion
    rankings = {}
    if name_ranked:
        rankings["name"] = name_ranked
    if content_ranked:
        rankings["content"] = content_ranked
    if semantic_ranked:
        rankings["semantic"] = semantic_ranked
    if git_ranked:
        rankings["git"] = git_ranked
    if recency_ranked:
        rankings["recency"] = recency_ranked

    if not rankings and not brain_recall:
        if not args.quiet and not args.no_messages:
            print("no matches.", file=sys.stderr)
        return 1

    fused = rrf_fuse(rankings, args.rrf_k) if rankings else []
    
    # Apply sorting if requested (overrides RRF ranking)
    if args.sort or args.sortr:
        sort_field = args.sortr if args.sortr else args.sort
        reverse = bool(args.sortr) or args.reverse
        paths = [p for p, _, _ in fused]
        sorted_paths = sort_files(paths, sort_field, reverse, args.dirs_first, args.dirs_last)
        # Rebuild fused with sorted order but preserve scores and labels
        path_to_data = {p: (s, l) for p, s, l in fused}
        fused = [(p, path_to_data[p][0], path_to_data[p][1]) for p in sorted_paths if p in path_to_data]
    elif args.dirs_first or args.dirs_last:
        # Apply directory grouping even without explicit sorting
        paths = [p for p, _, _ in fused]
        sorted_paths = sort_files(paths, None, False, args.dirs_first, args.dirs_last)
        path_to_data = {p: (s, l) for p, s, l in fused}
        fused = [(p, path_to_data[p][0], path_to_data[p][1]) for p in sorted_paths if p in path_to_data]
    
    render(fused, name_scores, content_hits, git_cache, args, use_color, query_pattern=pattern)

    # Brain Recall
    if brain_recall and not args.files_only:
        print(f"\n{c('35', '🧠 brain recall (semantic — memory chunks, not files):', use_color)}")
        for hit in brain_recall:
            print(f"    {c('90', '·', use_color)} {hit['snippet']}")

    # Search Stats
    if args.stats:
        elapsed = time.time() - start_time
        total_hits = sum(d.get("count", 0) for d in content_hits.values())
        print(f"\n{c('1;37', 'Search Statistics:', use_color)}", file=sys.stderr)
        print(f"  Duration:          {elapsed:.3f}s", file=sys.stderr)
        print(f"  Files with hits:   {len(content_hits)}", file=sys.stderr)
        print(f"  Total match count: {total_hits}", file=sys.stderr)
        print(f"  Engine:            {'ripgrep 15.2.0 (SIMD/mmap)' if HAVE_RG else 'pure-Python (fallback)'}", file=sys.stderr)

    return 0

if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
    except Exception as e:  # noqa: BLE001 — the catch exists to fix the exit code
        # grep convention: 0 = match, 1 = no match, 2 = error. Python's default
        # handler exits 1 on any unhandled exception, which made a crash look
        # exactly like "no matches" to every caller checking the exit code.
        print(f"pfind: error: {type(e).__name__}: {e}", file=sys.stderr)
        sys.exit(2)
