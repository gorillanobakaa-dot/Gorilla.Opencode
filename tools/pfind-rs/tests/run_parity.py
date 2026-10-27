#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Parity oracle for pfind.exe (tools/pfind-rs) against pfind.py 3.3.0.

    python -I tools/pfind-rs/tests/run_parity.py --both

Builds a fixture tree in a temporary folder it creates (and deletes again),
then runs every case against pfind.py (internal/llm/tools/pfind.py, with
ripgrep when it is on PATH) and against pfind.exe
(tools/pfind-rs/target/release/pfind[.exe], or --exe PATH). Each case carries
the assertions of the Go test it was ported from (internal/llm/tools/
find_test.go, find_names_test.go) or of the pfind.py changelog entry it
covers; each implementation must pass them on its own, and with --both the two
outputs must also match (stdout, exit code, and stderr unless the case says
stderr differs by design).

Exit 0 only if every selected implementation passes every case and (with
--both) every output matches. Options: --py / --exe-only to run one side,
-k TEXT to run only cases whose name contains TEXT, -v to print diffs in full.

Output normalisation, and why:
  * pfind.py runs under Python's text-mode stdout, which on Windows writes CRLF;
    pfind.exe writes LF (as ripgrep does). One CR before each LF is removed from
    pfind.py's output only, so a CR that is part of a file's content survives.
  * Listings that pfind.py took from `rg --files` come back in the order rg's
    threads finished, which changes from run to run. Those cases compare the
    lines as a sorted list ("unordered"); pfind.exe prints them sorted.
  * The Go tests use a file named after an agent rules file for the "whatever
    its capitals" case; this fixture uses CHANGES.md / changes.md instead (same
    behavior under test: a name is found whatever its capitals).
"""
import argparse
import difflib
import gzip
import os
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
PFIND_PY = REPO / "internal" / "llm" / "tools" / "pfind.py"
EXE_NAME = "pfind.exe" if os.name == "nt" else "pfind"
DEFAULT_EXE = REPO / "tools" / "pfind-rs" / "target" / "release" / EXE_NAME

# find.go's own flags (internal/llm/tools/find.go): every agent search goes out like this.
FINDGO = ["--no-color", "--count", "--limit", "15", "--max", "3", "--max-columns", "500", "--max-columns-preview"]
OLD = time.mktime((2020, 1, 1, 12, 0, 0, 0, 0, -1))

TREE = {
    "project/CHANGES.md": "# changes\n",
    "project/src/main.go": "package main\n\nfunc main() {\n\tprintln(\"hello needle\")\n}\n",
    "project/src/util.go": "package main\n\n// needle helper\nfunc helper() int { return 42 }\n",
    "project/src/config.go": "package main\n\nvar config = map[string]int{}\n",
    "project/docs/guide.md": "# guide\n\nThe needle is here.\nAnother line.\n",
    "project/docs/README.md": "needle here\n",
    "project/a/CHANGES.md": "needle here\n",
    "project/a/README.md": "needle here\n",
    "project/a/changes.txt": "needle here\n",
    "project/b/notes.md": "needle here\n",
    "project/.github/workflows/ci.yml": "jobs:\n  build:\n    needle: true\n",
    "project/.hidden/secret.txt": "needle in a hidden folder\n",
    "project/node_modules/pkg/index.js": "needle in node_modules\n",
    "project/lib/regex.py": "x = foo(bar)[0] + 1\n# a comment\ndef f():\n    '''doc'''\n    return 'needle'\n",
    "project/lib/long.txt": "needle " + "x" * 900 + "\n",
    "project/lib/dash.txt": "the option -v is here\n",
    "project/lib/Case.txt": "Needle with a capital\nneedle lower\nNEEDLE upper\n",
    "project/lib/words.txt": "needles and needle and needled\nhaystack only\nneedle\nneedle\nneedle\nneedle\nneedle\n",
    "project/lib/snippet_target.c": "int main(void)\n{\n    return 0;\n}\n",
    "project/lib/emoji.txt": "needle \U0001F600 smile\n",
}


def build_fixture(root: Path):
    for rel, body in TREE.items():
        p = root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes(body.encode("utf-8"))
    # Windows line endings for the -x snippet case
    (root / "project/lib/crlf.c").write_bytes(b"int main(void)\r\n{\r\n    return 0;\r\n}\r\n")
    (root / "project/data").mkdir()
    with gzip.open(root / "project/data/archive.txt.gz", "wb") as f:
        f.write(b"compressed needle inside\n")
    import io, tarfile, zipfile
    with zipfile.ZipFile(root / "project/data/bundle.zip", "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("one.txt", "first member\n")
        z.writestr("two.txt", "zipped needle member\n")
    with tarfile.open(root / "project/data/bundle.tar.gz", "w:gz") as t:
        body = b"tarred needle member\n"
        info = tarfile.TarInfo("inner/three.txt")
        info.size = len(body)
        t.addfile(info, io.BytesIO(body))
    # a tree too large to draw: 30 folders x 10 files
    for i in range(30):
        d = root / "project" / "big" / f"dir{i:02d}" / "sub"
        d.mkdir(parents=True, exist_ok=True)
        for j in range(10):
            (d / f"f{j}.txt").write_bytes(b"x\n")
    (root / "project" / ".gitignore").write_bytes(b"ignored_dir/\n*.log\n")
    (root / "project" / "ignored_dir").mkdir()
    (root / "project" / "ignored_dir" / "x.txt").write_bytes(b"needle ignored\n")
    (root / "project" / "debug.log").write_bytes(b"needle log\n")
    (root / "snippet.txt").write_bytes(b"int main(void)\n{\n    return 0;\n}\n")
    (root / "patterns.txt").write_bytes(b"haystack\nhelper\n")
    # mutation fixtures (tests/mutations-pfind-rs.yaml), outside project/ so the other cases are unchanged
    extra = root / "extra"
    extra.mkdir()
    (extra / "numbered.txt").write_bytes(b"needle one\nneedle two\nneedle three\nneedle four\n")
    (extra / "blob.txt").write_bytes(b"binary needle\x00\x01\x02\n")  # not .bin: that is in the default skip list
    with gzip.open(extra / "bomb.txt.gz", "wb") as f:
        f.write(b"\n" * (3 * 1024 * 1024) + b"bomb needle\n")
    # every file old and fixed, so relative times read the same on both sides
    for p in root.rglob("*"):
        os.utime(p, (OLD, OLD))
    # recency fixture: distinct times
    rec = root / "recent"
    rec.mkdir()
    for i, name in enumerate(["old.txt", "mid.txt", "new.txt"]):
        f = rec / name
        f.write_bytes(b"recent\n")
        os.utime(f, (OLD + i * 86400 * 400, OLD + i * 86400 * 400))
    # git repository for --git-dirty
    repo = root / "repo"
    repo.mkdir()
    (repo / "clean.txt").write_bytes(b"clean\n")
    (repo / "dirty.txt").write_bytes(b"before\n")
    git = shutil.which("git")
    if git:
        hooks = "NUL" if os.name == "nt" else "/dev/null"
        g = [git, "-C", str(repo), "-c", "user.name=t", "-c", "user.email=t@example.invalid",
             "-c", "commit.gpgsign=false", "-c", f"core.hooksPath={hooks}"]
        subprocess.run(g + ["init", "-q"], check=True)
        subprocess.run(g + ["add", "."], check=True)
        subprocess.run(g + ["commit", "-q", "-m", "init"], check=True)
        (repo / "dirty.txt").write_bytes(b"after\n")
        (repo / "untracked.txt").write_bytes(b"new\n")


class Case:
    def __init__(self, name, args, rc=0, has=(), lacks=(), mode="exact", stdin=None, cwd="project",
                 stderr_same=True, env=None, has_err=(), why=""):
        self.name, self.args, self.rc = name, args, rc
        self.has, self.lacks, self.mode = list(has), list(lacks), mode
        self.stdin, self.cwd, self.stderr_same, self.env = stdin, cwd, stderr_same, env or {}
        self.has_err, self.why = list(has_err), why


def cases(root: Path):
    P = str(root / "project")
    L = str(root / "project" / "lib")
    R = str(root / "recent")
    G = str(root / "repo")
    X = str(root / "extra")
    f = lambda rel: str(root / "project" / rel)  # noqa: E731
    return [
        # --- ported from find_test.go ---
        Case("returns matching lines, not just paths", FINDGO + ["-C", "2", "--", "needle", P],
             has=["main.go", "hello needle", "guide.md"], why="TestFindReturnsMatchingLinesNotJustPaths"),
        Case("files-only omits lines", FINDGO + ["--files-only", "--", "needle", P],
             has=["main.go"], lacks=["hello needle"], why="TestFindFilesOnlyOmitsLines"),
        Case("literal query needs no escaping", FINDGO + ["-C", "2", "--", "foo(bar)[0]", P],
             has=["regex.py", "foo(bar)[0]"], why="TestFindLiteralQueryNeedsNoEscaping"),
        Case("type filter narrows", FINDGO + ["--type", "go", "-C", "2", "--", "needle", P],
             has=["main.go"], lacks=["guide.md"], why="TestFindTypeFilterNarrows"),
        Case("lists a directory without a query", FINDGO + ["--files", "--max-depth", "2", "--limit", "0", "--", P],
             has=["CHANGES.md"], mode="unordered", why="TestFindListsDirectoryWithoutQuery"),
        Case("no matches is exit 1, not an error", FINDGO + ["-C", "2", "--", "zzqqxx_nothing", P],
             rc=1, has_err=["no matches."], why="TestFindNoMatchesIsNotAnError"),
        Case("slash glob matches like the examples", FINDGO + ["--iglob", "**/src/*.go", "--files", "--", P],
             has=["main.go", "util.go"], lacks=["guide.md"], mode="unordered", why="TestFindSlashGlobMatchesLikeTheExamples"),
        Case("says when the file cap cut", ["--no-color", "--count", "--limit", "2", "--max", "3", "-C", "2", "--", "needle", P],
             has_err=["file(s) matched, showing 2"], why="TestFindSaysWhenTheFileCapCut"),
        Case("long lines truncated per line", ["--no-color", "--count", "--limit", "15", "--max", "3", "--max-columns", "50",
                                              "--max-columns-preview", "-C", "2", "--", "needle", L],
             has=["omitted end of long line"], lacks=["x" * 60], why="TestFindLongLinesAreTruncatedPerLine"),
        Case("query starting with a dash is not a flag", FINDGO + ["-C", "2", "--", "-v", P],
             has=["dash.txt"], why="TestFindQueryStartingWithDashIsNotAFlag"),
        Case("fuzzy finds a misspelled name", FINDGO + ["--fuzzy", "--files-only", "--", "confgi", P],
             has=["config.go"], why="TestFindFuzzyFindsMisspelledName"),
        Case("tree view shows structure", ["--no-color", "--count", "--limit", "40", "--tree", "--", f("src")],
             has=["main.go", "└──"], why="TestFindTreeViewShowsStructure"),
        Case("long view shows metadata", ["--no-color", "--count", "--limit", "20", "--long", "--sort", "name", "--", f("src")],
             has=["main.go", " B "], why="TestFindLongViewShowsMetadata"),
        Case("code view summarises languages", FINDGO + ["--code", "--max-filesize", "2M", "--", P],
             has=["Go", "Python", "TOTAL"], mode="unordered", why="TestFindCodeViewSummarisesLanguages"),
        Case("recent listing sorts newest first", FINDGO + ["--sortr", "modified", "--oneline", "--", R],
             has=["new.txt"], why="TestFindRecentListingSortsNewestFirst"),
        Case("modified-only filters to dirty files", FINDGO + ["--git-dirty", "--oneline", "--", G],
             has=["dirty.txt", "untracked.txt"], lacks=["clean.txt"], mode="unordered", why="TestFindModifiedOnlyFiltersToDirtyFiles"),
        Case("explicitly asking for a hidden path finds it",
             FINDGO + ["--iglob", "**/.github/**", "--hidden", "--no-ignore-vcs", "--files", "--", P],
             has=["ci.yml"], mode="unordered", why="TestExplicitlyAskingForAHiddenPathFindsIt"),
        Case("an ordinary search still skips hidden files", FINDGO + ["-C", "2", "--", "needle", P],
             lacks=["secret.txt", "ci.yml", "node_modules"], why="TestAnOrdinarySearchStillSkipsHiddenFiles"),
        Case(".gitignore outside a repository needs --no-require-git",
             FINDGO + ["--no-require-git", "-C", "2", "--", "needle", P],
             has=["guide.md"], lacks=["ignored_dir", "debug.log"]),
        Case("the type list matches the engine", ["--type-list"], has=["python", "Total:"], why="TestTheTypeListMatchesTheEngine"),
        Case("folder patterns: src/**", FINDGO + ["--iglob", "**/src/**", "--files", "--", P],
             has=["main.go", "util.go"], lacks=["guide.md"], mode="unordered", why="TestFolderPatternsWorkWithAndWithoutRipgrep"),
        Case("folder patterns: *.md", FINDGO + ["--iglob", "*.md", "--files", "--", P],
             has=["guide.md"], lacks=["main.go"], mode="unordered", why="TestFolderPatternsWorkWithAndWithoutRipgrep"),
        # --- ported from find_names_test.go ---
        Case("a name is found whatever its capitals", FINDGO + ["--iglob", "**/changes.md", "--files", "--", P],
             has=["CHANGES.md"], lacks=["main.go"], mode="unordered", why="TestANameIsFoundWhateverItsCapitals"),
        Case("a name pattern and a type mean both",
             FINDGO + ["--iglob", "**/changes.md", "--type", "md", "-C", "2", "--", "needle", P],
             has=["CHANGES.md"], lacks=["README.md", "notes.md", "changes.txt"], why="TestANamePatternAndATypeMeanBothNotEither"),
        Case("a tree too large to draw shows folders", ["--no-color", "--count", "--limit", "40", "--tree", "--", P],
             has=["FOLDERS ONLY", "This is NOT the whole tree"], why="TestATreeTooLargeToDrawShowsFoldersAndSaysSo"),
        Case("a small tree is still drawn in full", ["--no-color", "--count", "--limit", "40", "--tree", "--", L],
             has=["regex.py"], lacks=["FOLDERS ONLY"], why="TestASmallTreeIsStillDrawnInFull"),
        Case("a cut list says it is incomplete", ["--no-color", "--count", "--limit", "3", "--files", "--", P],
             has=["this list is INCOMPLETE"], mode="count", why="TestACutListDoesNotNameASwitchTheModelCannotUse"),
        # --- pfind.py 3.3.0 changelog and flags ---
        Case("json ranked results", ["--json", "--limit", "5", "--", "needle", P], has=['"engine": "ripgrep"', '"results"']),
        Case("json with no matches", ["--json", "--", "zzqqxx_nothing", P], rc=1, has=['"matched": 0']),
        Case("exact snippet matches CRLF files (query file)", ["--no-color", "-x", "--query-file", str(root / "snippet.txt"), L],
             has=["crlf.c", "snippet_target.c"]),
        Case("snippet from stdin", ["--no-color", "-", L, "-x"], stdin=b"int main(void)\n{\n", has=["crlf.c"]),
        Case("loose snippet", ["--no-color", "--loose", "--", "return    0;", L], has=["snippet_target.c"]),
        Case("smart case: lowercase ignores case", ["--no-color", "--content", "-C", "1", "--", "needle", f("lib/Case.txt")],
             has=["Needle with a capital", "NEEDLE upper"]),
        Case("smart case: a capital is exact", ["--no-color", "--content", "-C", "1", "--", "Needle", f("lib/Case.txt")],
             has=["Needle with a capital"]),
        Case("-s wins over -i", ["--no-color", "--content", "-i", "-s", "--", "needle", f("lib/Case.txt")],
             has=["needle lower"], lacks=["NEEDLE upper"]),
        Case("-i makes a capital pattern ignore case", ["--no-color", "--content", "-i", "--", "NEEDLE", f("lib/Case.txt")],
             has=["needle lower", "Needle with a capital"]),
        Case("--max stops at the cap", ["--no-color", "--content", "--max", "2", "-C", "0", "--", "needle", X + "/numbered.txt"],
             has=["needle two"], lacks=["needle three"]),
        Case("a binary file is skipped by default", ["--no-color", "--content", "--", "binary needle", X], rc=1,
             lacks=["binary needle"]),
        Case("the decompression budget stops a bomb", ["--no-color", "-z", "--content", "--", "bomb needle", X],
             env={"PFIND_NO_RG": "1", "PFIND_MAX_DECOMPRESSED_MB": "1"}, rc=1, lacks=["bomb needle"],
             has_err=["decompression budget"], stderr_same=False,
             why="the budget belongs to pfind.py's built-in engine (ripgrep's -z has none)"),
        Case("word regexp", ["--no-color", "--content", "-w", "-C", "1", "--", "needle", f("lib/words.txt")]),
        Case("invert match", ["--no-color", "--content", "-v", "--", "needle", f("lib/words.txt")], has=["haystack only"]),
        Case("only matching", ["--no-color", "--content", "-o", "-r", "--", "need[a-z]+", f("lib/words.txt")], has=["needles"]),
        Case("count matches", ["--no-color", "--count-matches", "--", "needle", L], mode="unordered"),
        Case("count lines", ["--no-color", "-c", "--", "needle", L], mode="unordered"),
        Case("files with matches", ["--no-color", "--files-with-matches", "--", "needle", L]),
        Case("files without match", ["--no-color", "--files-without-match", "--", "needle", L]),
        Case("vimgrep with column", ["--no-color", "--vimgrep", "--column", "--", "needle", L]),
        Case("no-heading with context", ["--no-color", "--no-heading", "-C", "1", "--", "helper", P]),
        Case("replace preview", ["--no-color", "--replace", "NEEDLE", "--", "needle", f("docs/guide.md")], has=["NEEDLE"]),
        Case("regex replace with group", ["--no-color", "-r", "--replace", "<$1>", "--", "(need)le", f("docs/guide.md")], has=["<need>"]),
        Case("max-count", ["--no-color", "--content", "-m", "2", "-C", "0", "--", "needle", f("lib/words.txt")]),
        Case("context capped by --max", ["--no-color", "--content", "--max", "2", "-C", "1", "--", "needle", f("lib/words.txt")],
             has=["more match(es) in this file"]),
        Case("passthru", ["--no-color", "--content", "--passthru", "--", "helper", f("src/util.go")], has=["package main"]),
        Case("byte offset", ["--no-color", "--content", "-b", "-C", "1", "--", "needle", f("docs/guide.md")]),
        Case("trim and with-filename", ["--no-color", "--content", "--trim", "-H", "-C", "1", "--", "needle", f("lib/regex.py")]),
        Case("several patterns (-e)", ["--no-color", "--content", "-e", "haystack", "-e", "helper", "--", P]),
        Case("pattern file (-f)", ["--no-color", "--content", "-f", str(root / "patterns.txt"), "--", P], has=["util.go"]),
        Case("search inside .gz (-z)", ["--no-color", "-z", "--content", "--", "compressed needle", P], has=["archive.txt.gz"],
             stderr_same=False, why="ripgrep's -z needs a gzip program on PATH; pfind.exe decompresses in-process"),
        Case("search inside .zip and .tar.gz members (-z)", ["--no-color", "-z", "--content", "-C", "1", "--", "needle member", f("data")],
             has=["bundle.zip", "zipped needle member", "bundle.tar.gz", "tarred needle member"],
             env={"PFIND_NO_RG": "1"}, stderr_same=False,
             why="ripgrep's -z does not open .zip or tar members, so the oracle here is pfind.py's built-in engine"),
        Case("emoji content prints", ["--no-color", "--content", "--", "smile", L], has=["\U0001F600"]),
        Case("names only", ["--no-color", "--names", "--", "readme", P], has=["README.md"]),
        Case("ext filter with and without dot", ["--no-color", "--ext", "go", ".md", "--", "needle", P],
             has=["main.go", "guide.md"], lacks=["regex.py"]),
        Case("exclude glob", ["--no-color", "--exclude", "docs", "--", "needle", P], lacks=["guide.md"]),
        Case("type-not", ["--no-color", "--type-not", "md", "--", "needle", P], lacks=["guide.md"]),
        Case("no-ignore finds ignored", ["--no-color", "--no-ignore", "--", "needle", P], has=["ignored_dir"]),
        Case("unrestricted twice finds hidden", ["--no-color", "-uu", "--", "needle", P], has=["secret.txt"]),
        Case("max-depth", ["--no-color", "--files", "-d", "1", "--", P], mode="unordered"),
        Case("grid layout", ["--no-color", "--grid", "--width", "60", "--sort", "name", "--", L]),
        Case("grid across", ["--no-color", "--grid", "--across", "--width", "60", "--sort", "name", "--", L]),
        Case("long table: octal, header, bytes", ["--no-color", "--long", "--header", "--octal-permissions", "--bytes",
                                                  "--sort", "name", "--", L]),
        Case("long table: iso time, classify", ["--no-color", "--long", "--time-style", "long-iso", "--classify",
                                                "--sort", "Name", "--", f("src")]),
        Case("loc badge", ["--no-color", "--loc", "--", "needle", f("lib/regex.py")], has=["[code:"]),
        Case("oneline dirs-first", ["--no-color", "--oneline", "--dirs-first", "--sort", "name", "--", f("docs")]),
        Case("only-dirs listing", ["--no-color", "--oneline", "--only-dirs", "--sort", "path", "--", P]),
        Case("recurse listing", ["--no-color", "-R", "--sort", "path", "--", f("src")]),
        Case("sort by name in a search", ["--no-color", "--sort", "name", "--files-only", "--", "needle", P]),
        Case("recency boost", ["--no-color", "--recency-boost", "--files-only", "--", "recent", R]),
        Case("rrf-k changes scores", ["--no-color", "--rrf-k", "10", "--", "needle", P]),
        Case("hyperlinks (file format)", ["--color", "always", "--hyperlink", "--content", "--", "helper", f("src/util.go")]),
        Case("colors spec", ["--color", "always", "--colors", "path:fg:blue", "--colors", "line:style:underline",
                             "--content", "--", "helper", f("src/util.go")]),
        Case("presets from the environment", ["--no-color", "--work", "--files-only", "--", "needle"],
             env={"PFIND_WORK": P}, has=["guide.md"]),
        Case("unconfigured preset refuses with instructions", ["--no-color", "--src", "--files-only", "--", "zzqqxx_nothing"],
             rc=1, has_err=["preset --src is not configured"]),
        Case("quiet exit code", ["-q", "--", "needle", P]),
        Case("empty query file is refused", ["--query-file", str(root / "empty.txt"), P], rc=2, stderr_same=False,
             has_err=["a query is needed"], why="argparse prints its full usage line; pfind.exe prints a short one"),
    ]


def norm_py(b: bytes) -> str:
    s = b.decode("utf-8", "replace")
    return s.replace("\r\n", "\n") if os.name == "nt" else s


def run(cmd, case, root):
    env = dict(os.environ)
    for k in ("PFIND_BRAIN", "PFIND_WORK", "PFIND_SRC", "PFIND_CHROMA", "PFIND_BRAIN_DB", "PFIND_COLLECTION", "PFIND_NO_RG"):
        env.pop(k, None)
    env["XDG_CONFIG_HOME"] = str(root / "cfg")
    env["APPDATA"] = str(root / "cfg")
    env["COLUMNS"] = "80"
    env["PFIND_MACHINE"] = "parityhost"
    env.update(case.env)
    t = time.perf_counter()
    p = subprocess.run(cmd + case.args, input=case.stdin, capture_output=True, cwd=str(root / case.cwd), env=env, timeout=300)
    return p.returncode, p.stdout, p.stderr, time.perf_counter() - t


def check(name, rc, out, err, case):
    problems = []
    if rc != case.rc:
        problems.append(f"{name}: exit {rc}, expected {case.rc}")
    for s in case.has:
        if s not in out:
            problems.append(f"{name}: stdout lacks {s!r}")
    for s in case.lacks:
        if s in out:
            problems.append(f"{name}: stdout holds {s!r}")
    for s in case.has_err:
        if s not in err:
            problems.append(f"{name}: stderr lacks {s!r}")
    return problems


def comparable(text, mode):
    lines = text.split("\n")
    if mode == "unordered":
        return sorted(lines)
    if mode == "count":
        return [str(len(lines)), lines[-2] if len(lines) > 1 else ""]
    return lines


def long_path(p: Path) -> Path:
    if os.name != "nt":
        return p
    import ctypes
    buf = ctypes.create_unicode_buffer(1024)
    if ctypes.windll.kernel32.GetLongPathNameW(str(p), buf, 1024):
        return Path(buf.value)
    return p


def main():
    ap = argparse.ArgumentParser(description="pfind.py vs pfind.exe parity oracle")
    ap.add_argument("--both", action="store_true", help="run pfind.py and pfind.exe and compare (default)")
    ap.add_argument("--py", action="store_true", help="run pfind.py only")
    ap.add_argument("--exe-only", action="store_true", help="run pfind.exe only")
    ap.add_argument("--exe", default=str(DEFAULT_EXE), help="path to pfind.exe")
    ap.add_argument("-k", default="", help="only cases whose name contains this")
    ap.add_argument("-v", action="store_true", help="print full diffs")
    o = ap.parse_args()
    run_py = o.py or o.both or not o.exe_only
    run_exe = o.exe_only or o.both or not o.py
    if run_exe and not Path(o.exe).is_file():
        print(f"FAIL: {o.exe} not found; build it: cargo build --release --offline --locked (in tools/pfind-rs)")
        return 1
    root = long_path(Path(tempfile.mkdtemp(prefix="pfind-parity-")).resolve())
    failures, mismatches, ran = [], [], 0
    try:
        build_fixture(root)
        (root / "cfg").mkdir()
        (root / "empty.txt").write_bytes(b"")
        oracle = "pfind.py + ripgrep" if shutil.which("rg") else "pfind.py (built-in engine: rg not on PATH)"
        print(f"oracle: {oracle} | candidate: {o.exe}\nfixture: {root}")
        t_py = t_exe = 0.0
        for case in cases(root):
            if o.k and o.k not in case.name:
                continue
            ran += 1
            res = {}
            if run_py:
                rc, out, err, dt = run([sys.executable, "-I", str(PFIND_PY)], case, root)
                t_py += dt
                res["py"] = (rc, norm_py(out), norm_py(err))
            if run_exe:
                rc, out, err, dt = run([o.exe], case, root)
                t_exe += dt
                res["exe"] = (rc, out.decode("utf-8", "replace"), err.decode("utf-8", "replace"))
            probs = []
            for side, (rc, out, err) in res.items():
                probs += check(side, rc, out, err, case)
            diff = []
            if len(res) == 2:
                (prc, pout, perr), (erc, eout, eerr) = res["py"], res["exe"]
                if prc != erc:
                    diff.append(f"exit code: py {prc} vs exe {erc}")
                a, b = comparable(pout, case.mode), comparable(eout, case.mode)
                if a != b:
                    diff += ["stdout differs:"] + list(difflib.unified_diff(a, b, "pfind.py", "pfind.exe", lineterm="", n=1))
                if case.stderr_same and perr.strip() != eerr.strip():
                    diff += ["stderr differs:"] + list(difflib.unified_diff(perr.strip().split("\n"), eerr.strip().split("\n"),
                                                                            "pfind.py", "pfind.exe", lineterm="", n=1))
            status = "PASS" if not probs and not diff else "FAIL"
            print(f"[{status}] {case.name}" + (f"  ({case.why})" if case.why and status == "FAIL" else ""))
            for p in probs:
                print("    " + p)
            if diff:
                shown = diff if o.v else diff[:16]
                for d in shown:
                    print("    " + d)
                if len(diff) > len(shown):
                    print(f"    ... {len(diff) - len(shown)} more diff line(s); -v shows all")
            failures += probs
            if diff:
                mismatches.append(case.name)
        print(f"\n{ran} case(s); assertion failures: {len(failures)}; output mismatches: {len(mismatches)}")
        if run_py and run_exe:
            print(f"wall time: pfind.py {t_py:.1f} s, pfind.exe {t_exe:.1f} s")
    finally:
        shutil.rmtree(root, ignore_errors=True)
    ok = ran > 0 and not failures and not mismatches
    print("RESULT: " + ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
