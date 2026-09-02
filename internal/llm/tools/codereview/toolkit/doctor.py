#!/usr/bin/env python3
"""
doctor.py
---------
Answers "can this machine actually review anything, and what will it cost me?"
BEFORE any work starts.

Why this exists
---------------
This toolkit does not review code by itself. It orchestrates about thirty real
analysers -- cppcheck, clang-tidy, bandit, semgrep, gitleaks, clippy,
golangci-lint, gosec, shellcheck and friends -- and those have to be present on
the machine. On a box with none of them installed, a run inspects nothing.

That is the dangerous case, and it is why this file exists. A report full of
"MISSING" looks very like a report that found no problems, especially to
something reading quickly. "No findings" and "nothing ran" must never be
confused, so we say plainly which one you're looking at, and refuse to proceed
when the answer is "nothing ran".

What it reports
---------------
  * per language: which analysers are present, which are missing
  * how many need downloading, and roughly how much disk that needs
  * free space on the target filesystem, and whether that's enough
  * the exact command to fix it

Usage:
    python3 code_review.py <target> --doctor     # only the tools <target> needs
    python3 code_review.py --doctor              # same, for the current directory
    python3 doctor.py <target>                   # standalone
    python3 doctor.py --all                      # every language, ignore any target

Scoping to the target is the useful default: being told that eslint is missing
is noise when you are reviewing a kernel driver. Use --all before setting a
machine up for arbitrary work.
"""

import os
import shutil
import sys

from tools_registry import TOOLS, EXTENSION_LANGUAGE

# A safety margin over the sum of tool sizes: package managers need scratch
# space for downloads and unpacking, and finishing an install with 5MB spare is
# its own kind of broken.
HEADROOM_MB = 500


def _run_check(cmd, timeout=15):
    import subprocess
    try:
        # check=False: we are asking "does this binary exist", and the answer
        # is the return code (127 = no). An exception would hide it.
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout,
                           check=False)
        return p.returncode
    except FileNotFoundError:
        return 127
    except Exception:
        return 1


def tool_present(tool) -> bool:
    """True if the tool can run. Tools with no check_cmd are our own Python and
    are always available -- they ship in this directory."""
    if not tool.check_cmd:
        return True
    return _run_check(tool.check_cmd) != 127


PLATFORM = os.name          # "posix" on Linux/macOS, "nt" on Windows
PLATFORM_LABEL = {"nt": "Windows", "posix": "Linux/Unix"}.get(PLATFORM, PLATFORM)


def install_route(tool) -> str:
    """How this tool would be obtained ON THIS MACHINE, for the report.

    Platform-aware because the previous version was not, and an install route
    that cannot work here is worse than none: an agent reads "apt", runs it on
    Windows, gets "apt is not recognized", and has learned nothing about why.
    Fourteen of the forty-three tools were apt-only, so on Windows that was the
    common case rather than an edge one.
    """
    i = tool.install
    routes = i.routes_for("windows" if PLATFORM == "nt" else "posix")
    if routes:
        label, _ = routes[0]
        return label
    return "manual"


def unavailable_here(tool) -> bool:
    """True if this tool cannot exist on this platform at all.

    Distinct from "not installed", and the distinction is the whole point.
    "Missing" invites an agent to install the thing. "Not available on this
    platform" tells it to stop asking -- and, for the kernel analysers, that the
    work belongs on a different machine. Reporting the second as the first sends
    a reader hunting for a package that was never going to exist.
    """
    return not tool.runs_on(PLATFORM)


# Manual tools that only make sense inside a particular tree. Kept here rather
# than imported from code_review.py so doctor.py stays runnable standalone on a
# machine where a review has never been attempted.
TREE_SPECIFIC_MANUAL = {
    "firefox-mach-lint": "firefox",
    "firefox-mach-static-analysis": "firefox",
    "kernel-checkpatch": "linux-kernel",
    "kernel-clang-tidy": "linux-kernel",
    "kernel-clang-analyzer": "linux-kernel",
    "kernel-sparse": "linux-kernel",
    "kernel-coccicheck": "linux-kernel",
}


def detect_profile(target: str) -> str:
    """Light profile sniff, matching code_review.detect_profile's markers."""
    if not target or not os.path.isdir(target):
        return "generic"

    def has(*parts):
        return os.path.exists(os.path.join(target, *parts))

    if has("mach") and has("python", "mozbuild"):
        return "firefox"
    if has("Kconfig") and has("MAINTAINERS") and has("scripts", "checkpatch.pl"):
        return "linux-kernel"
    if has("go.mod"):
        return "go"
    if has("Cargo.toml"):
        return "rust"
    if has("pyproject.toml") or has("setup.py") or has("requirements.txt"):
        return "python"
    return "generic"


def _manual_applies(tool, profile: str) -> bool:
    needed = TREE_SPECIFIC_MANUAL.get(tool.id)
    return needed is None or needed == profile


def free_mb(path: str) -> int:
    try:
        return shutil.disk_usage(path).free // (1024 * 1024)
    except OSError:
        return -1


def languages_in(target: str) -> set:
    """Languages actually present under `target`, so we only talk about tools
    that matter here. Cheap walk, capped -- this is a pre-check, not a scan."""
    if not target or not os.path.exists(target):
        return set()
    if os.path.isfile(target):
        ext = os.path.splitext(target)[1].lower()
        return {EXTENSION_LANGUAGE.get(ext)} - {None}
    found, seen = set(), 0
    from tools_registry import DEFAULT_IGNORE_DIRS
    for root, dirs, files in os.walk(target):
        dirs[:] = [d for d in dirs if d not in DEFAULT_IGNORE_DIRS and not d.startswith(".")]
        for fn in files:
            lang = EXTENSION_LANGUAGE.get(os.path.splitext(fn)[1].lower())
            if lang:
                found.add(lang)
            seen += 1
            if seen > 20000:
                return found
    return found


def report(target: str = "", langs=None, stream=sys.stdout) -> dict:
    """Print the readiness report. Returns a summary dict for callers."""
    w = stream.write
    scoped = bool(langs)
    langs = set(langs or [])

    relevant = []
    for t in TOOLS:
        if t.scope == "checklist":
            continue
        if "*" in t.languages or not scoped or (set(t.languages) & langs):
            relevant.append(t)

    auto = [t for t in relevant if t.scope in ("auto-file", "auto-project")]
    # Manual tools tied to a specific tree layout are only worth mentioning in
    # that tree. Firefox's ./mach lint is declared languages=["*"], so without
    # this the readiness check for a Go repo advised the reader about mach --
    # exactly the irrelevant noise that scoping to the target is meant to remove.
    profile = detect_profile(target)
    manual = [t for t in relevant
              if t.scope == "manual" and _manual_applies(t, profile)]

    present = [t for t in auto if tool_present(t)]
    # THREE states, not two. See unavailable_here().
    unavailable = [t for t in auto if t not in present and unavailable_here(t)]
    missing = [t for t in auto if t not in present and t not in unavailable]
    # Our own checks need no install; counting them as "installed analysers"
    # would let a bare machine look equipped when it has no third-party tools.
    external_present = [t for t in present if t.check_cmd]

    need_mb = sum(t.install.size_mb for t in missing)
    disk_path = target if target and os.path.exists(target) else os.getcwd()
    have_mb = free_mb(disk_path)

    w("\n" + "=" * 72 + "\n")
    w(" CODE REVIEW READINESS CHECK\n")
    w("=" * 72 + "\n\n")

    w("This toolkit does not review code on its own. It drives real analysers\n")
    w("(cppcheck, clang-tidy, bandit, semgrep, gitleaks, clippy, golangci-lint,\n")
    w("gosec, shellcheck, ...). Those must be installed on this machine. If they\n")
    w("are not here, a review inspects NOTHING -- and an empty result is not the\n")
    w("same as clean code.\n\n")

    # Say what machine this is BEFORE anything is called missing. An agent
    # reading a list of absent tools will otherwise try to install them with
    # whatever package manager it last saw, and on the wrong OS every one of
    # those attempts fails in a way that looks like a broken toolkit.
    w(f"Platform: {PLATFORM_LABEL}\n")
    if scoped:
        w(f"Scope: {target}\n")
        w(f"Languages found: {', '.join(sorted(langs)) or '(none detected)'}\n\n")
    else:
        w("Scope: every language this toolkit knows about (no target given)\n\n")

    # ---- per language ----------------------------------------------------
    by_lang: dict = {}
    for t in auto:
        # A multi-language tool must only be credited to languages actually IN
        # SCOPE. Listing every language it *could* handle produced lines like
        # "c: 2 ready, 0 missing" for a pure-Python project -- which reads as
        # "C is covered here" when there is no C and the two tools are semgrep
        # and gitleaks.
        if "*" in t.languages:
            tool_langs = sorted(langs) if scoped else ["all languages"]
        elif scoped:
            tool_langs = sorted(set(t.languages) & langs)
        else:
            tool_langs = t.languages
        for lang in tool_langs:
            key = "all languages" if lang == "*" else lang
            by_lang.setdefault(key, {"present": [], "missing": [], "unavailable": []})
            if t in present:
                bucket = "present"
            elif t in unavailable:
                bucket = "unavailable"
            else:
                bucket = "missing"
            by_lang[key][bucket].append(t.id)

    w("PER LANGUAGE\n")
    w("-" * 72 + "\n")
    for lang in sorted(by_lang):
        d = by_lang[lang]
        n_ok, n_no = len(d["present"]), len(d["missing"])
        mark = "OK  " if n_ok and not n_no else ("PART" if n_ok else "NONE")
        w(f"  [{mark}] {lang:<16} {n_ok} ready, {n_no} missing\n")
        if d["missing"]:
            w(f"         missing: {', '.join(sorted(d['missing']))}\n")
        # Reported apart from "missing" on purpose. These cannot be installed
        # here at all, so telling someone to install them wastes their time
        # and, if the someone is an agent, produces a failed command it cannot
        # explain. For the kernel analysers the useful advice is not a package
        # name -- it is "do this on a Linux machine".
        if d["unavailable"]:
            w(f"         NOT AVAILABLE on {PLATFORM_LABEL}: "
              f"{', '.join(sorted(d['unavailable']))}\n")
    w("\n")

    if unavailable:
        w("NOT AVAILABLE ON THIS PLATFORM\n")
        w("-" * 72 + "\n")
        w(f"These {len(unavailable)} do not exist for {PLATFORM_LABEL}. That is not a\n")
        w("missing install -- there is nothing to install. Do not try.\n\n")
        for t in sorted(unavailable, key=lambda x: x.id):
            w(f"  {t.id:<24} {t.label}\n")
        if any(t.id.startswith("kernel-") for t in unavailable):
            w("\n")
            w("  The kernel analysers are Linux programs. A kernel review belongs\n")
            w("  on a Linux machine; nothing here substitutes for them.\n")
        w("\n")

    # ---- what it would cost ---------------------------------------------
    w("WHAT INSTALLING THE MISSING TOOLS COSTS\n")
    w("-" * 72 + "\n")
    if not missing:
        w("  Nothing to install. Every analyser for this scope is already here.\n\n")
    else:
        routes: dict = {}
        for t in missing:
            routes.setdefault(install_route(t), []).append(t)
        w(f"  Tools to download/install : {len(missing)}\n")
        w(f"  Approximate disk needed   : {need_mb} MB"
          f" (+{HEADROOM_MB} MB working room = {need_mb + HEADROOM_MB} MB)\n")
        w(f"  Free space on {disk_path[:34]:<34}: "
          f"{'unknown' if have_mb < 0 else str(have_mb) + ' MB'}\n")
        w("  Network                   : required (package registries)\n")
        w("  Time                      : minutes, mostly downloading\n\n")
        w("  By install route:\n")
        for route in sorted(routes):
            ts = routes[route]
            mb = sum(t.install.size_mb for t in ts)
            w(f"    {route:<12} {len(ts):>2} tool(s), ~{mb:>4} MB   "
              f"{', '.join(sorted(t.id for t in ts))[:44]}\n")
        w("\n")
        # Sizes are estimates and the estimate is the point -- we are trying to
        # stop someone starting a 2GB install with 300MB free, not to be exact.
        if have_mb >= 0 and have_mb < need_mb + HEADROOM_MB:
            w("  *** NOT ENOUGH DISK SPACE ***\n")
            w(f"  Need about {need_mb + HEADROOM_MB} MB, have {have_mb} MB. "
              f"Free up {need_mb + HEADROOM_MB - have_mb} MB first,\n")
            # --only filters by LANGUAGE, not by tool id. Naming the wrong unit
            # here would send someone straight into a no-op install.
            langs_hint = ",".join(sorted(langs)[:3]) if langs else "cpp,python"
            w("  or install just what one language needs:\n")
            w(f"      python3 install_tools.py --only {langs_hint}\n\n")

    # ---- verdict ---------------------------------------------------------
    w("VERDICT\n")
    w("-" * 72 + "\n")
    ready = bool(external_present)
    if not auto:
        w("  No analysers apply to this scope at all. Either the languages here\n")
        w("  aren't covered by the registry, or no source files were found.\n")
    elif not ready:
        w("  NOT READY. No third-party analyser is installed, so a review right\n")
        w("  now would inspect nothing and report nothing -- which reads exactly\n")
        w("  like a clean result and is the worst way for this to fail.\n\n")
        w("  Fix it:   python3 install_tools.py\n")
        w("  Then:     python3 code_review.py <target>\n")
    elif missing:
        w(f"  USABLE, PARTIAL. {len(external_present)} analyser(s) ready, "
          f"{len(missing)} missing.\n")
        w("  A review will run and will find real things, but the languages\n")
        w("  listed above as PART or NONE are only partly covered. Do not read\n")
        w("  a quiet result for those as an all-clear.\n\n")
        w("  Complete it:  python3 install_tools.py\n")
    else:
        w(f"  READY. All {len(external_present)} analyser(s) for this scope are installed.\n")

    if manual:
        w(f"\n  Note: {len(manual)} tool(s) always need a human -- they require build\n")
        w("  state this script won't fabricate (a compile_commands.json, a\n")
        w("  configured kernel .config, a bootstrapped mach environment, a\n")
        w("  compiled binary for valgrind). They're printed as exact commands in\n")
        w(f"  the report instead of being guessed at: {', '.join(sorted(t.id for t in manual))[:52]}\n")
        # A manual tool that cannot run here at all is worth saying separately.
        # "Needs a human" and "needs a different operating system" are different
        # problems, and only one of them can be solved at this keyboard.
        blocked = [t for t in manual if unavailable_here(t)]
        if blocked:
            w(f"  Of those, {len(blocked)} cannot run on {PLATFORM_LABEL} at all: "
              f"{', '.join(sorted(t.id for t in blocked))}\n")
            w("  Those belong on a Linux machine. Nothing here substitutes.\n")

    w("\n  Whatever the verdict: these are STATIC analysers. They do not find\n")
    w("  wrong logic, broken invariants or swallowed errors. A clean run means\n")
    w("  the tools found nothing, not that the code is correct.\n")
    w("=" * 72 + "\n\n")

    return {
        "ready": ready,
        "present": [t.id for t in external_present],
        "missing": [t.id for t in missing],
        "need_mb": need_mb,
        "free_mb": have_mb,
        "enough_disk": have_mb < 0 or have_mb >= need_mb + HEADROOM_MB,
    }


def main() -> int:
    args = [a for a in sys.argv[1:] if a != "--all"]
    show_all = "--all" in sys.argv[1:]
    target = args[0] if args else ("" if show_all else os.getcwd())
    langs = None if show_all else languages_in(target)
    summary = report(target, langs)
    return 0 if summary["ready"] else 3


if __name__ == "__main__":
    sys.exit(main())
