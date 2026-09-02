#!/usr/bin/env python3
"""
patch_port.py -- forward-port, backport, rebase, refresh and port patch series.

WHAT THIS IS FOR
================

You maintain a set of patches against a moving base. The kernel goes 6.19 ->
7.0.5 -> 7.0.9; Firefox goes 154 -> 155. Your patches were written against the
old one and must be made to apply, still mean the same thing, and still build
against the new one.

That loop:

    old base
      -> existing patch series
      -> new base version
      -> rebase / cherry-pick
      -> conflicts
      -> resolve
      -> compile
      -> run tests
      -> review the diff
      -> new patch series

Every step of it is somewhere a person quietly gets it wrong, and the two that
cost the most are the two that look like success:

  * A patch that applies WITH FUZZ has moved. `patch` will happily place a hunk
    dozens of lines from where it was written for, in a function that merely
    looks similar. It reports success. The build may even pass. The change is in
    the wrong place.

  * A patch that is ALREADY UPSTREAM applies as an empty diff or fails as
    "reversed". Both look like a problem with your patch. Neither is.

So this tool refuses to say "ported" on the strength of an exit code.

THE METHOD, WHICH IS THE PART WORTH LEARNING
============================================

Written out because an agent -- or a person -- following these seven rules will
port patches better than one who knows more git commands.

1. KNOW WHICH DIRECTION YOU ARE GOING.
   Forward-porting carries a patch to a NEWER base: the surrounding code has
   moved on, and your change must be re-expressed against what is there now.
   Backporting carries it to an OLDER base: the code your patch depends on may
   not exist yet, and the honest answer is sometimes "this cannot be backported
   without also backporting X". They are not the same operation and they fail
   differently.

2. ESTABLISH THE BASE BEFORE YOU TOUCH THE PATCH.
   "It does not apply" is almost never about the patch. It is about applying it
   to a tree that is not what you think it is -- wrong tag, dirty worktree,
   patches already partly applied. Check the tree first, every time.

3. THREE-WAY BEATS FUZZ, ALWAYS.
   `git apply -3` and `git am -3` use blob context to merge, and when they fail
   they leave conflict markers naming both sides. `patch --fuzz` guesses by
   proximity and tells you nothing. Prefer the tool that fails informatively
   over the one that succeeds quietly.

4. A CONFLICT IS INFORMATION, NOT AN ERROR.
   It is the base telling you exactly which assumption your patch made that is
   no longer true. Read the conflict before resolving it; the resolution is
   usually obvious once you know what moved and why.

5. AN EMPTY RESULT MEANS SOMETHING, AND IT IS NOT SUCCESS.
   If a patch applies but changes nothing, it is already upstream. That is a
   fine outcome -- drop the patch -- but it must be reported as "already
   present", never as "applied".

6. A PORT IS NOT DONE WHEN IT APPLIES. IT IS DONE WHEN IT BUILDS AND PASSES.
   Applying is the easy half. This tool runs your build and test commands if you
   give them, and says plainly that it did not if you do not.

7. REGENERATE THE PATCH FROM THE RESULT.
   The ported patch is what the new tree actually contains, not your old file
   with the offsets nudged. Refresh it from the tree so the series you ship is
   the series you tested.

WINDOWS AND LINUX
=================

Runs on both. Git is the engine and is cross-platform; `patch(1)` is used only
as a fallback and its absence is reported rather than assumed. No shell is
spawned -- arguments are passed as lists, so paths with spaces need no quoting.
Line endings are handled explicitly, because a CRLF checkout will make every
hunk in a LF patch fail to apply and the error will blame your patch.

USAGE
=====

    python patch_port.py                      # interactive menu
    python patch_port.py --tree DIR --list    # inspect a series
    python patch_port.py --tree DIR --op forward-port --patch P --onto REF
    python patch_port.py --tree DIR --op backport    --patch P --onto REF
    python patch_port.py --tree DIR --op rebase      --series DIR --onto REF
    python patch_port.py --tree DIR --op refresh     --patch P
    python patch_port.py --tree DIR --op port-series --series DIR --onto REF

    --build "make -j8"      run after applying; a port is not done until it builds
    --test  "make test"     run after building
    --json                  machine-readable report instead of prose

Exit codes:
    0  the operation completed and everything it was asked to verify passed
    1  conflicts remain, or a build/test step failed -- the tree is left as-is
       for you to inspect
    2  bad usage, or the tree is not in a state where work can begin
    3  nothing to do (patch already applied upstream, or empty series)
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time

SCHEMA = "patch-port/1"

GIT_TIMEOUT = 600          # a rebase over a large tree is not fast
BUILD_TIMEOUT = 14400      # a kernel or Firefox build is measured in hours


# ─────────────────────────────────────────────────────────────────────────────
#  Running things
# ─────────────────────────────────────────────────────────────────────────────

def run(cmd, cwd=None, timeout=GIT_TIMEOUT, stdin_text=None):
    """Run a command and return (returncode, stdout, stderr).

    shell=False deliberately. Every path here can contain spaces -- Windows
    users keep source under C:\\Users\\name\\... routinely -- and a shell would
    re-split them. It also means nothing in a patch file can be interpreted as
    a shell metacharacter, which matters when the input is untrusted.
    """
    try:
        p = subprocess.run(
            cmd, cwd=cwd, input=stdin_text, capture_output=True, text=True,
            errors="replace", timeout=timeout, shell=False,
        )
        return p.returncode, p.stdout or "", p.stderr or ""
    except subprocess.TimeoutExpired:
        return 124, "", "timed out after %ds: %s" % (timeout, " ".join(cmd[:3]))
    except FileNotFoundError:
        return 127, "", "not found: %s" % cmd[0]
    except OSError as e:
        return 126, "", str(e)


def have(binary):
    """shutil.which honours PATHEXT, so this finds git.exe and patch.exe too."""
    return shutil.which(binary)


# ─────────────────────────────────────────────────────────────────────────────
#  RULE 2 -- establish the base before touching the patch
# ─────────────────────────────────────────────────────────────────────────────

class Tree:
    """A source tree, and the truth about what state it is in."""

    def __init__(self, path):
        self.path = os.path.abspath(path)
        self.git = have("git")
        self.is_repo = False
        self.head = ""
        self.branch = ""
        self.describe = ""
        self.dirty = []
        self.untracked = 0
        self.in_progress = ""      # rebase / am / merge / cherry-pick left half-done
        self.kind = "unknown"      # kernel / firefox / go / rust / generic
        self.problems = []
        self._probe()

    def _probe(self):
        if not os.path.isdir(self.path):
            self.problems.append("not a directory: %s" % self.path)
            return
        if not self.git:
            self.problems.append(
                "git is not installed. Three-way apply, series handling and "
                "regeneration all need it; without it this tool can only "
                "inspect patches.")
            return

        rc, out, _ = run([self.git, "rev-parse", "--is-inside-work-tree"], cwd=self.path)
        self.is_repo = (rc == 0 and out.strip() == "true")
        if not self.is_repo:
            self.problems.append(
                "%s is not a git worktree. Porting without git means fuzz "
                "matching and no three-way merge, which is how a hunk lands in "
                "the wrong function. Clone or `git init` the tree first." % self.path)
            return

        _, out, _ = run([self.git, "rev-parse", "HEAD"], cwd=self.path)
        self.head = out.strip()
        _, out, _ = run([self.git, "rev-parse", "--abbrev-ref", "HEAD"], cwd=self.path)
        self.branch = out.strip()
        _, out, _ = run([self.git, "describe", "--tags", "--always"], cwd=self.path)
        self.describe = out.strip()

        _, out, _ = run([self.git, "status", "--porcelain"], cwd=self.path)
        for line in out.splitlines():
            if line.startswith("??"):
                self.untracked += 1
            elif line.strip():
                self.dirty.append(line[3:].strip())

        gitdir = os.path.join(self.path, ".git")
        for marker, label in (("rebase-merge", "rebase"), ("rebase-apply", "rebase or am"),
                              ("MERGE_HEAD", "merge"), ("CHERRY_PICK_HEAD", "cherry-pick")):
            if os.path.exists(os.path.join(gitdir, marker)):
                self.in_progress = label
                self.problems.append(
                    "a %s is already in progress in this tree. Finish or abort it "
                    "before starting another operation -- stacking them is how "
                    "half-applied series happen." % label)

        self.kind = self._detect_kind()

    def _detect_kind(self):
        def has(*p):
            return os.path.exists(os.path.join(self.path, *p))
        if has("Kconfig") and has("MAINTAINERS") and has("scripts", "checkpatch.pl"):
            return "kernel"
        if has("mach") and has("browser") and has("toolkit"):
            return "firefox"
        if has("go.mod"):
            return "go"
        if has("Cargo.toml"):
            return "rust"
        return "generic"

    def ready(self):
        """Can work begin? Dirty is a warning; in-progress is a stop."""
        return self.is_repo and not self.in_progress

    def summary(self):
        lines = ["tree      : %s" % self.path,
                 "kind      : %s" % self.kind]
        if self.is_repo:
            lines.append("branch    : %s" % (self.branch or "(detached)"))
            lines.append("describe  : %s" % (self.describe or "(no tags)"))
            lines.append("head      : %s" % self.head[:12])
            lines.append("worktree  : %s" % (
                "clean" if not self.dirty else
                "%d modified file(s) -- %s" % (len(self.dirty), ", ".join(self.dirty[:3]))))
            if self.untracked:
                lines.append("untracked : %d file(s)" % self.untracked)
        for p in self.problems:
            lines.append("PROBLEM   : %s" % p)
        return "\n".join(lines)


# ─────────────────────────────────────────────────────────────────────────────
#  Patches
# ─────────────────────────────────────────────────────────────────────────────

FILE_RE = re.compile(r"^\+\+\+ b/(.+?)\s*$", re.M)
OLDFILE_RE = re.compile(r"^--- a/(.+?)\s*$", re.M)
HUNK_RE = re.compile(r"^@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@", re.M)
SUBJECT_RE = re.compile(r"^Subject: (?:\[[^\]]*\] )?(.+)$", re.M)
FROM_RE = re.compile(r"^From ([0-9a-f]{7,40}) ", re.M)


class Patch:
    """One patch file, and what can be known about it without applying it."""

    def __init__(self, path):
        self.path = os.path.abspath(path)
        self.name = os.path.basename(path)
        self.text = ""
        self.error = ""
        self.files = []
        self.hunks = 0
        self.subject = ""
        self.origin_commit = ""
        self.is_mailbox = False
        self.crlf = False
        self._read()

    def _read(self):
        try:
            with open(self.path, "rb") as fh:
                raw = fh.read()
        except OSError as e:
            self.error = str(e)
            return
        # RULE: line endings are checked, not assumed. A CRLF patch against an
        # LF tree fails every hunk, and git blames the patch rather than the
        # encoding -- an hour lost to something that is not a conflict at all.
        self.crlf = b"\r\n" in raw
        self.text = raw.decode("utf-8", errors="replace")
        self.files = FILE_RE.findall(self.text) or OLDFILE_RE.findall(self.text)
        self.hunks = len(HUNK_RE.findall(self.text))
        m = SUBJECT_RE.search(self.text)
        self.subject = m.group(1).strip() if m else ""
        m = FROM_RE.search(self.text)
        if m:
            self.origin_commit = m.group(1)
            self.is_mailbox = True

    def describe(self):
        bits = ["%-38s" % self.name[:38]]
        bits.append("%2d hunk%s" % (self.hunks, " " if self.hunks == 1 else "s"))
        bits.append("%2d file%s" % (len(self.files), " " if len(self.files) == 1 else "s"))
        if self.is_mailbox:
            bits.append("mailbox")
        if self.crlf:
            bits.append("CRLF")
        if self.subject:
            bits.append(self.subject[:44])
        return "  ".join(bits)


def load_series(path):
    """Load a directory of patches, or a quilt-style `series` file, in order.

    Order matters more than anything else in a series: patch 4 is written
    against the tree as patch 3 left it. Applying them alphabetically when the
    author numbered them differently produces conflicts that are entirely the
    porter's own doing.
    """
    path = os.path.abspath(path)
    if os.path.isfile(path):
        return [Patch(path)]

    series_file = os.path.join(path, "series")
    names = []
    if os.path.isfile(series_file):
        # quilt order is authoritative when present.
        with open(series_file, "r", encoding="utf-8", errors="replace") as fh:
            for line in fh:
                line = line.split("#", 1)[0].strip()
                if line:
                    names.append(line)
    else:
        names = sorted(n for n in os.listdir(path)
                       if n.endswith((".patch", ".diff")))
    return [Patch(os.path.join(path, n)) for n in names
            if os.path.isfile(os.path.join(path, n))]


# ─────────────────────────────────────────────────────────────────────────────
#  Applying -- RULE 3 and RULE 5
# ─────────────────────────────────────────────────────────────────────────────

class ApplyResult:
    """What actually happened, in terms that distinguish the look-alikes."""

    CLEAN = "applied-clean"
    THREE_WAY = "applied-three-way"     # merged using blob context; review it
    FUZZ = "applied-with-fuzz"           # RELOCATED by search; review hardest
    CONFLICT = "conflict"
    ALREADY = "already-present"         # RULE 5: upstream already has it
    REVERSED = "reversed"               # applies backwards: usually also upstream
    FAILED = "failed"

    def __init__(self, status, patch, detail="", conflicts=None, files=None):
        self.status = status
        self.patch = patch
        self.detail = detail
        self.conflicts = conflicts or []
        self.files = files or []

    @property
    def ok(self):
        return self.status in (self.CLEAN, self.THREE_WAY, self.FUZZ,
                               self.ALREADY)

    def headline(self):
        return {
            self.CLEAN: "applied cleanly",
            self.THREE_WAY: "applied by three-way merge -- READ THE RESULT, "
                            "context moved and the hunk was placed by merge, not by match",
            self.FUZZ: "applied WITH FUZZ -- the hunk was RELOCATED by searching "
                       "for its context, not matched at the recorded line. This is "
                       "the one outcome that can be silently WRONG: read the diff "
                       "before you trust it",
            self.CONFLICT: "CONFLICT -- %d file(s) need a decision" % len(self.conflicts),
            self.ALREADY: "already present in this tree; nothing to do -- drop this "
                          "patch from the series rather than forcing it",
            self.REVERSED: "applies in reverse, which almost always means the change "
                           "is already upstream in a different form",
            self.FAILED: "did not apply",
        }[self.status]


def check_applies(git, tree, patch, reverse=False):
    """Ask git whether a patch would apply, without touching the tree."""
    cmd = [git, "apply", "--check", "--verbose"]
    if reverse:
        cmd.append("--reverse")
    cmd.append(patch.path)
    rc, _, err = run(cmd, cwd=tree.path)
    return rc == 0, err


def apply_with_fuzz(tree, patch, max_fuzz=2):
    """Last rung of the ladder: let GNU patch look for the context.

    Returns an ApplyResult on success, or None if this could not be tried or
    did not work -- None means "carry on to CONFLICT/FAILED", so the caller's
    reporting is unchanged when fuzz is unavailable.

    This is deliberately last, and deliberately loud. Fuzz is the only
    strategy here that can put a hunk in the WRONG PLACE and still exit 0:
    if the same few lines of context appear twice in a file, patch takes the
    first match it finds. Three-way cannot do that, because it reasons about
    the actual blob. So fuzz runs only after three-way has failed, its fuzz
    factor is bounded rather than unlimited, and the offsets it used are
    surfaced in the result instead of being swallowed.

    The tree is restored before AND after a failed attempt, so a half-applied
    patch never leaks into the next one in a series.
    """
    exe = have("patch")
    if not exe:
        return None

    # Start from a clean tree: the failed three-way may have left partial work.
    run([tree.git, "checkout", "--", "."], cwd=tree.path)

    rc, out, err = run([exe, "-p1", "--fuzz=%d" % max_fuzz,
                        "--no-backup-if-mismatch", "-i", patch.path],
                       cwd=tree.path)
    if rc != 0:
        run([tree.git, "checkout", "--", "."], cwd=tree.path)
        return None

    moved = []
    for line in ((out or "") + "\n" + (err or "")).splitlines():
        t = line.strip()
        if t and ("offset" in t or "fuzz" in t.lower()):
            moved.append(t)

    detail = "git could not place these hunks; GNU patch found the context by searching"
    if moved:
        detail = detail + " -- " + "; ".join(moved)
    return ApplyResult(ApplyResult.FUZZ, patch, detail, files=patch.files)


def apply_patch(tree, patch, three_way=True):
    """Apply one patch, and classify the outcome honestly.

    The order of the checks is the method:
      - would it apply in reverse? then it is already upstream (RULE 5)
      - does it apply cleanly? best case
      - does three-way work? applied, but the context moved -- say so (RULE 3)
      - otherwise conflict, and name the files (RULE 4)
    """
    git = tree.git
    if patch.error:
        return ApplyResult(ApplyResult.FAILED, patch, patch.error)
    if not patch.hunks:
        return ApplyResult(ApplyResult.FAILED, patch,
                           "no hunks found -- is this a patch file?")

    # RULE 5, checked FIRST. A patch that is already upstream will otherwise
    # look like a conflict, and someone will resolve it by re-applying a change
    # the tree already has.
    rev_ok, _ = check_applies(git, tree, patch, reverse=True)
    if rev_ok:
        return ApplyResult(ApplyResult.ALREADY, patch,
                           "the tree already contains this change (it applies in reverse)",
                           files=patch.files)

    ok, err = check_applies(git, tree, patch)
    if ok:
        rc, _, err2 = run([git, "apply", patch.path], cwd=tree.path)
        if rc == 0:
            return ApplyResult(ApplyResult.CLEAN, patch, files=patch.files)
        return ApplyResult(ApplyResult.FAILED, patch, err2, files=patch.files)

    if three_way:
        rc, _, err3 = run([git, "apply", "--3way", patch.path], cwd=tree.path)
        conflicted = conflicted_files(tree)
        if rc == 0 and not conflicted:
            return ApplyResult(ApplyResult.THREE_WAY, patch,
                               "context had moved; git merged it using blob history",
                               files=patch.files)
        if conflicted:
            return ApplyResult(ApplyResult.CONFLICT, patch, err3,
                               conflicts=conflicted, files=patch.files)

        # RULE 3 is still intact here: three-way has ALREADY been tried and
        # could not merge. The usual reason is not a real conflict at all --
        # it is that this tree has never seen the patch's pre-image blob, so
        # there is no common ancestor to merge against. git then falls back to
        # a direct application, which demands the context sit at the recorded
        # line, and a forward-port is precisely the case where it does not:
        # upstream added lines above the hunk and everything shifted down.
        #
        # GNU patch will SEARCH for that context instead. Only now, having
        # exhausted every strategy that reasons about content, is that
        # allowed -- and every hunk it moves is reported with its offset.
        fz = apply_with_fuzz(tree, patch)
        if fz is not None:
            return fz

        return ApplyResult(ApplyResult.FAILED, patch, (err3 or err).strip(),
                           files=patch.files)

    return ApplyResult(ApplyResult.FAILED, patch, err.strip(), files=patch.files)


def conflicted_files(tree):
    rc, out, _ = run([tree.git, "diff", "--name-only", "--diff-filter=U"], cwd=tree.path)
    return [l.strip() for l in out.splitlines() if l.strip()] if rc == 0 else []


def conflict_report(tree, files, max_files=6, max_lines=40):
    """Show the conflict, because RULE 4 says it is information.

    A list of filenames tells a reader nothing. The markers tell them exactly
    which assumption the patch made that the new base no longer satisfies, and
    that is usually enough to resolve it without opening an editor at all.
    """
    out = []
    for path in files[:max_files]:
        full = os.path.join(tree.path, path)
        out.append("--- %s" % path)
        try:
            with open(full, "r", encoding="utf-8", errors="replace") as fh:
                lines = fh.read().splitlines()
        except OSError as e:
            out.append("    (cannot read: %s)" % e)
            continue
        shown, inside = 0, False
        for i, line in enumerate(lines, 1):
            if line.startswith("<<<<<<<"):
                inside = True
            if inside:
                out.append("  %5d | %s" % (i, line[:150]))
                shown += 1
            if line.startswith(">>>>>>>"):
                inside = False
                out.append("")
            if shown >= max_lines:
                out.append("    ... truncated; open the file to see the rest")
                break
    if len(files) > max_files:
        out.append("... and %d more conflicted file(s)" % (len(files) - max_files))
    return "\n".join(out)


# ─────────────────────────────────────────────────────────────────────────────
#  RULE 6 -- a port is not done until it builds
# ─────────────────────────────────────────────────────────────────────────────

def verify(tree, build_cmd, test_cmd, log):
    """Run the build and tests if given. Report plainly when not given.

    The silence is the point. A tool that says nothing about testing invites the
    reader to assume it tested. This says, in the report, that it did not.
    """
    results = {}
    for label, cmd in (("build", build_cmd), ("test", test_cmd)):
        if not cmd:
            results[label] = {
                "ran": False,
                "detail": "no %s command was given, so NOTHING was %s. "
                          "This port is unverified." % (label, "built" if label == "build" else "tested"),
            }
            continue
        parts = cmd if isinstance(cmd, list) else cmd.split()
        log("running %s: %s" % (label, " ".join(parts)))
        started = time.time()
        rc, out, err = run(parts, cwd=tree.path, timeout=BUILD_TIMEOUT)
        tail = (out + err).strip().splitlines()[-25:]
        results[label] = {
            "ran": True,
            "ok": rc == 0,
            "exit_code": rc,
            "seconds": round(time.time() - started, 1),
            "tail": tail,
        }
        log("  %s %s in %.0fs" % (label, "PASSED" if rc == 0 else "FAILED", time.time() - started))
    return results


# ─────────────────────────────────────────────────────────────────────────────
#  Operations
# ─────────────────────────────────────────────────────────────────────────────

def op_inspect(tree, patches, log):
    """Say what is here and what would happen, without changing anything.

    Always safe, and worth doing first. It answers "does this series still
    apply" in seconds, before an hour is spent on a build.
    """
    log(tree.summary())
    log("")
    if not patches:
        log("no patches found")
        return 3, {"patches": []}

    log("%d patch(es), in the order they will be applied:" % len(patches))
    rows = []
    for i, p in enumerate(patches, 1):
        log("  %2d. %s" % (i, p.describe()))
        would = "unknown"
        if tree.ready():
            rev_ok, _ = check_applies(tree.git, tree, p, reverse=True)
            if rev_ok:
                would = "already-present"
            else:
                ok, _ = check_applies(tree.git, tree, p)
                would = "clean" if ok else "needs-three-way-or-conflicts"
        rows.append({"name": p.name, "subject": p.subject, "hunks": p.hunks,
                     "files": p.files, "crlf": p.crlf, "would_apply": would})
        log("      -> %s" % would)
    crlf = [p.name for p in patches if p.crlf]
    if crlf:
        log("")
        log("NOTE: %d patch(es) use CRLF line endings: %s" % (len(crlf), ", ".join(crlf[:4])))
        log("      Against an LF tree every hunk will fail and git will blame the")
        log("      patch rather than the encoding. Convert before concluding the")
        log("      patch is broken.")
    return 0, {"patches": rows}


def op_apply_series(tree, patches, log, stop_on_conflict=True):
    """Apply patches in order, stopping where a human is needed.

    Stopping is deliberate. Continuing past a conflict means every later patch
    applies against a tree that is not what its author assumed, and the failures
    that follow are noise generated by this tool rather than information about
    the patches.
    """
    applied, results = [], []
    for i, p in enumerate(patches, 1):
        log("[%d/%d] %s" % (i, len(patches), p.name))
        r = apply_patch(tree, p)
        results.append(r)
        log("        %s" % r.headline())
        if r.status == ApplyResult.CONFLICT:
            log("")
            log(conflict_report(tree, r.conflicts))
            log("")
            log("  Resolve, then:  git add <files>  and re-run with --resume")
            log("  Or abandon it:  git checkout -- .   (loses the partial apply)")
            if stop_on_conflict:
                return 1, results, applied
        elif r.status in (ApplyResult.FAILED, ApplyResult.REVERSED):
            if stop_on_conflict:
                return 1, results, applied
        elif r.status != ApplyResult.ALREADY:
            applied.append(p)
    return 0, results, applied


def op_refresh(tree, patch, log):
    """RULE 7 -- regenerate the patch from what the tree actually contains.

    A refreshed patch is not the old file with its offsets nudged. It is a fresh
    diff of the tree, which is the only version that certainly matches what was
    built and tested.
    """
    if not tree.ready():
        return 2, {}
    rc, out, err = run([tree.git, "diff", "--"] + (patch.files or []), cwd=tree.path)
    if rc != 0:
        log("could not diff the tree: %s" % err.strip())
        return 1, {}
    if not out.strip():
        log("the tree contains no changes to these files, so there is nothing to")
        log("refresh. Either the patch was never applied, or it is already upstream.")
        return 3, {"written": ""}
    dest = patch.path + ".refreshed"
    with open(dest, "w", encoding="utf-8", newline="\n") as fh:
        fh.write(out)
    log("wrote %s (%d bytes, LF line endings)" % (dest, len(out)))
    log("Compare it with the original before replacing anything:")
    log("  git diff --no-index %s %s" % (patch.name, os.path.basename(dest)))
    return 0, {"written": dest, "bytes": len(out)}


def op_port(tree, patches, onto, direction, build, test, log):
    """Forward-port or backport onto a new base. RULE 1.

    The two directions differ in what failure MEANS, so the advice differs:
    forward-porting into a conflict usually means the code you touched was
    refactored; backporting into a conflict often means the change depends on
    something that does not exist yet in the older base, and no amount of
    conflict resolution will make that false.
    """
    if onto:
        log("checking out base: %s" % onto)
        rc, _, err = run([tree.git, "checkout", onto], cwd=tree.path)
        if rc != 0:
            log("could not check out %s: %s" % (onto, err.strip()))
            log("Establish the base first (RULE 2). Fetch the tag, or name one that exists:")
            log("  git tag --list | tail")
            return 2, {}
        tree._probe()
        log("now at: %s" % (tree.describe or tree.head[:12]))
        log("")

    code, results, applied = op_apply_series(tree, patches, log)

    if code != 0:
        log("")
        if direction == "backport":
            log("Backporting stopped. Before resolving, ask whether the patch depends")
            log("on something this older base does not have. If it does, the honest")
            log("outcome is 'not backportable without also taking X', not a forced")
            log("resolution that compiles but means something different.")
        else:
            log("Forward-porting stopped. The base has moved under this patch. Read")
            log("the conflict: it names the assumption that is no longer true.")
        return code, {"results": [r.status for r in results]}

    log("")
    log("all %d patch(es) applied" % len(applied))
    ver = verify(tree, build, test, log)
    ok = all(v.get("ok", True) for v in ver.values() if v.get("ran"))
    if not ok:
        log("")
        log("APPLIED BUT NOT VERIFIED. The tree is left as it is so you can look.")
        return 1, {"verify": ver}
    return 0, {"verify": ver, "applied": [p.name for p in applied]}


# ─────────────────────────────────────────────────────────────────────────────
#  Interactive
# ─────────────────────────────────────────────────────────────────────────────

MENU = [
    ("inspect", "Inspect only -- what is here, and would it still apply?",
     "Changes nothing. Always safe, and the right first move: it answers\n"
     "        'does this series still apply' in seconds rather than after a build."),
    ("forward-port", "Forward-port a patch or series onto a NEWER base",
     "Your patch was written against an older tree. The code around it has\n"
     "        moved on and the change must be re-expressed against what is there now."),
    ("backport", "Backport a patch or series onto an OLDER base",
     "The reverse. Watch for changes that depend on code the older base does\n"
     "        not have yet -- sometimes the honest answer is 'not without also taking X'."),
    ("rebase", "Rebase an applied series onto a new base",
     "For work already committed in this tree, rather than loose .patch files."),
    ("refresh", "Refresh a patch from the current tree",
     "Regenerates the patch from what the tree actually contains, so what you\n"
     "        ship is what you tested."),
    ("port-series", "Port a whole series, stopping at the first conflict",
     "Applies in order and stops where a person is needed. Continuing past a\n"
     "        conflict only manufactures more of them."),
]


def ask(prompt, default=""):
    try:
        got = input(prompt).strip()
    except (EOFError, KeyboardInterrupt):
        print("")
        sys.exit(130)
    return got or default


def interactive():
    print("")
    print("=" * 72)
    print(" PATCH PORTING")
    print("=" * 72)
    print("")
    print("This carries patches between base versions -- a kernel moving 6.19 to")
    print("7.0.9, a Firefox moving 154 to 155 -- and refuses to call a port done")
    print("just because a command exited zero.")
    print("")

    tree_path = ask("Source tree [.]: ", ".")
    tree = Tree(tree_path)
    print("")
    print(tree.summary())
    print("")
    if not tree.is_repo:
        print("Cannot continue without a git worktree. See PROBLEM above.")
        return 2
    if tree.in_progress:
        print("Cannot continue: a %s is already in progress." % tree.in_progress)
        print("Finish it (git rebase --continue) or abandon it (git rebase --abort).")
        return 2
    if tree.dirty:
        print("WARNING: the worktree has %d modified file(s)." % len(tree.dirty))
        print("Applying on top of uncommitted work makes it impossible to tell")
        print("afterwards which changes came from the patch.")
        if ask("Continue anyway? [y/N]: ", "n").lower() not in ("y", "yes"):
            return 2
        print("")

    print("WHAT DO YOU WANT TO DO?")
    print("-" * 72)
    for i, (_, title, why) in enumerate(MENU, 1):
        print("  %d. %s" % (i, title))
        print("        %s" % why)
        print("")
    choice = ask("Choose 1-%d [1]: " % len(MENU), "1")
    try:
        op = MENU[int(choice) - 1][0]
    except (ValueError, IndexError):
        print("not a choice: %s" % choice)
        return 2
    print("")

    patches = []
    if op != "rebase":
        src = ask("Patch file or series directory: ")
        if not src or not os.path.exists(src):
            print("not found: %s" % src)
            return 2
        patches = load_series(src)
        if not patches:
            print("no patches found in %s" % src)
            return 3
        print("found %d patch(es)" % len(patches))
        print("")

    onto = ""
    if op in ("forward-port", "backport", "rebase", "port-series"):
        onto = ask("Base to port ONTO (tag, branch or commit; blank = current HEAD): ")
        print("")

    build = test = ""
    if op != "inspect":
        print("A port is not finished when it applies -- it is finished when it")
        print("builds and passes. Leave these blank and the report will say the")
        print("port is unverified, which is honest but not much use.")
        if tree.kind == "kernel":
            print("  suggestion:  make -j$(nproc)   /   make modules")
        elif tree.kind == "firefox":
            print("  suggestion:  ./mach build      /   ./mach test")
        build = ask("Build command [none]: ")
        test = ask("Test command [none]: ")
        print("")

    return dispatch(op, tree, patches, onto, build, test, print)


# ─────────────────────────────────────────────────────────────────────────────
#  Dispatch and main
# ─────────────────────────────────────────────────────────────────────────────

def dispatch(op, tree, patches, onto, build, test, log):
    # RULE 2, enforced rather than merely advised.
    #
    # The interactive path asked about this and the scripted path did not, so a
    # half-finished rebase was refused when a person was watching and ignored
    # when one was not -- which is precisely backwards. An agent running this
    # unattended is the caller most likely to stack a second operation on top of
    # an abandoned one, and least able to recognise the wreckage afterwards.
    #
    # inspect is exempt: it changes nothing, and being able to look at a broken
    # tree is how anyone works out what to do about it.
    if op != "inspect" and not tree.ready():
        for problem in tree.problems:
            log("REFUSING: %s" % problem)
        if not tree.problems:
            log("REFUSING: the tree is not in a state where work can begin.")
        log("")
        log("Nothing was changed. Resolve the above, then run this again.")
        return 2

    if op == "inspect":
        code, _ = op_inspect(tree, patches, log)
        return code
    if op == "refresh":
        if not patches:
            log("refresh needs a patch to regenerate")
            return 2
        code, _ = op_refresh(tree, patches[0], log)
        return code
    if op == "rebase":
        if not onto:
            log("rebase needs --onto")
            return 2
        log("rebasing %s onto %s" % (tree.branch or "HEAD", onto))
        rc, out, err = run([tree.git, "rebase", onto], cwd=tree.path)
        log((out + err).strip()[:4000])
        if rc != 0:
            log("")
            log("The rebase stopped. This is information, not failure (RULE 4):")
            log("  git status              which files disagree")
            log("  git diff                the conflict itself")
            log("  git rebase --continue   after resolving and `git add`")
            log("  git rebase --abort      to put the tree back as it was")
            return 1
        ver = verify(tree, build, test, log)
        return 0 if all(v.get("ok", True) for v in ver.values() if v.get("ran")) else 1
    if op in ("forward-port", "backport", "port-series"):
        direction = "backport" if op == "backport" else "forward"
        code, _ = op_port(tree, patches, onto, direction, build, test, log)
        return code
    log("unknown operation: %s" % op)
    return 2


def main(argv):
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--tree", default="")
    ap.add_argument("--op", default="",
                    choices=["", "inspect", "forward-port", "backport", "rebase",
                             "refresh", "port-series"])
    ap.add_argument("--patch", default="")
    ap.add_argument("--series", default="")
    ap.add_argument("--onto", default="")
    ap.add_argument("--build", default="")
    ap.add_argument("--test", default="")
    ap.add_argument("--list", action="store_true", help="same as --op inspect")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args(argv)

    if not args.tree and not args.op and not args.list:
        return interactive()

    tree = Tree(args.tree or ".")
    op = "inspect" if args.list else (args.op or "inspect")
    src = args.patch or args.series
    patches = load_series(src) if src else []

    if args.json:
        captured = []
        code = dispatch(op, tree, patches, args.onto, args.build, args.test,
                        lambda m: captured.append(str(m)))
        json.dump({
            "schema": SCHEMA,
            "operation": op,
            "tree": tree.path,
            "tree_kind": tree.kind,
            "platform": "windows" if os.name == "nt" else "posix",
            "base": tree.describe or tree.head[:12],
            "worktree_clean": not tree.dirty,
            "problems": tree.problems,
            "patches": [{"name": p.name, "subject": p.subject, "hunks": p.hunks,
                         "files": p.files, "crlf": p.crlf} for p in patches],
            "exit_code": code,
            "verified": bool(args.build or args.test),
            "caveat": (
                "An operation that exits 0 means the patches applied and any build "
                "or test command given passed. If no build command was given, "
                "NOTHING was built and this port is unverified. A patch reported as "
                "applied-three-way was placed by merge rather than by exact match "
                "and must be read before it is trusted. A patch reported as "
                "applied-with-fuzz is weaker still: its hunks were RELOCATED by "
                "searching for surrounding context, so if that context appears more "
                "than once in the file the change can land in the wrong place and "
                "still report success. Read every fuzzed hunk against the original."
            ),
            "log": captured,
        }, sys.stdout, indent=None)
        sys.stdout.write("\n")
        return code

    return dispatch(op, tree, patches, args.onto, args.build, args.test, print)


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except KeyboardInterrupt:
        sys.exit(130)
