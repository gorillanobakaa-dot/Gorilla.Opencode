#!/usr/bin/env python3
"""
baseline.py
-----------
Accepted findings: things a tool will keep reporting that this project has
already decided are fine.

Why this is needed
------------------
A real example, and the one that prompted this file. gorilla-opencode embeds
Google OAuth client secrets in `internal/auth/*_oauth.go`. gitleaks flags them
every run. But for *installed/desktop* OAuth apps Google explicitly does not
treat the client secret as confidential -- it ships inside the binary and cannot
be kept secret -- so every CLI doing Google/Gemini login embeds one. The finding
is correct as a pattern match and wrong as a verdict.

Without somewhere to record that decision, the choice is between a permanently
noisy report and turning the secrets scanner off. Both are bad: the first
teaches you to skim past gitleaks, and the second means the next *accidental*
key goes unnoticed. So decisions get written down instead.

How it works
------------
A `.code-review-baseline.json` in the reviewed project's root:

    {
      "accepted": [
        {
          "file": "internal/auth/gemini_oauth.go",
          "rule_id": "generic-api-key",
          "reason": "Google OAuth client secret for an installed/desktop app.
                     Google does not treat these as confidential; every CLI
                     doing Gemini login embeds one.",
          "accepted_by": "gorilla",
          "date": "2026-08-06"
        }
      ]
    }

Matching is `file` + `rule_id`, NOT line number. Line numbers move on every
edit above them, so a line-keyed baseline silently stops matching and the noise
returns. `file` may end with `/*` to accept a directory.

`reason` is REQUIRED. An entry without one is refused, because "we suppressed
this and nobody remembers why" is how a real vulnerability gets inherited.

`doc` is optional but preferred: a repo-relative path to the project's own
write-up. A project that has already reasoned this through usually has the
argument written down somewhere better than a JSON string -- gorilla-opencode
had `docs/CLIENT-SECRETS-EXPLAINED.md` explaining installed-app OAuth secrets
in detail, and a baseline that PARAPHRASES such a document immediately starts
drifting from it. Cite, don't restate. The path is checked for existence, and a
`doc` pointing at a file that has since been deleted is reported as a problem --
a citation to something that no longer exists is worse than no citation.

Nothing is deleted. Suppressed findings stay in report.json with
`suppressed=true` and the reason attached; they are excluded from the headline
counts and from corroboration. A number you chose to set aside, with a written
justification, is honest. A number that silently vanished is not.
"""

import json
import os
from typing import List, Optional, Tuple

BASELINE_FILENAME = ".code-review-baseline.json"


def path_for(target_dir: str) -> str:
    return os.path.join(target_dir, BASELINE_FILENAME)


def load(target_dir: str) -> Tuple[List[dict], List[str]]:
    """Returns (accepted_entries, problems). Never raises.

    A malformed baseline must not abort a review, but it must not silently
    suppress nothing either -- the problems list gets surfaced in the report.
    """
    p = path_for(target_dir)
    if not os.path.exists(p):
        return [], []
    try:
        with open(p, errors="replace") as f:
            data = json.load(f)
    except (OSError, ValueError) as e:
        return [], [f"{BASELINE_FILENAME} could not be read ({e}); nothing suppressed"]

    entries: List[dict] = []
    problems: List[str] = []
    raw = data.get("accepted") if isinstance(data, dict) else None
    if not isinstance(raw, list):
        return [], [f"{BASELINE_FILENAME} has no 'accepted' list; nothing suppressed"]

    # NB: not `e` -- `except ... as e` above binds and then DELETES that name at
    # the end of its block, so reusing it here reads as a use-after-delete to a
    # type checker and is genuinely confusing to a reader.
    for i, entry in enumerate(raw):
        if not isinstance(entry, dict):
            problems.append(f"entry {i} is not an object; ignored")
            continue
        if not entry.get("file") and not entry.get("rule_id"):
            problems.append(f"entry {i} matches everything (no file and no rule_id); ignored")
            continue
        doc = str(entry.get("doc", "")).strip()
        if doc and not os.path.exists(os.path.join(target_dir, doc)):
            problems.append(
                f"entry {i} cites doc '{doc}' which does not exist; the entry "
                f"still applies but its justification is now unverifiable")
        if not str(entry.get("reason", "")).strip():
            problems.append(
                f"entry {i} ({entry.get('file') or entry.get('rule_id')}) has no "
                f"'reason'; ignored -- an unexplained suppression is how a real "
                f"vulnerability gets inherited")
            continue
        entries.append(entry)
    return entries, problems


def _file_matches(pattern: str, path: str) -> bool:
    if not pattern:
        return True          # rule-only entry: applies project-wide
    if pattern.endswith("/*"):
        return path.startswith(pattern[:-1])
    return path == pattern


def match(finding, entries: List[dict]) -> Optional[dict]:
    """The first accepted entry covering this finding, or None."""
    for e in entries:
        if not _file_matches(e.get("file", ""), finding.file):
            continue
        rule = e.get("rule_id", "")
        # Substring rather than equality: rule ids often carry a suffix we
        # compose ourselves, e.g. "generic-api-key" vs "G401 (CWE-326)".
        if rule and rule not in (finding.rule_id or ""):
            continue
        return e
    return None


def apply(findings: List, entries: List[dict]) -> int:
    """Mark accepted findings. Returns how many were suppressed."""
    if not entries:
        return 0
    n = 0
    for f in findings:
        e = match(f, entries)
        if e:
            f.suppressed = True
            f.suppression_reason = str(e.get("reason", "")).strip()
            doc = str(e.get("doc", "")).strip()
            if doc:
                f.suppression_reason += f" See {doc} for the full reasoning."
            who, when = e.get("accepted_by", ""), e.get("date", "")
            if who or when:
                f.suppression_reason += f" [accepted by {who or 'unknown'}" \
                                        f"{', ' + when if when else ''}]"
            n += 1
    return n


def template(findings: List, limit: int = 20) -> str:
    """A starter baseline for the findings given, for copy-paste editing.

    Emits `reason: ""` on purpose -- an entry is refused until a human writes
    one, so this cannot be used to bulk-silence a report unread.
    """
    seen, out = set(), []
    for f in findings:
        key = (f.file, f.rule_id)
        if key in seen:
            continue
        seen.add(key)
        out.append({"file": f.file, "rule_id": f.rule_id, "reason": "",
                    "accepted_by": "", "date": ""})
        if len(out) >= limit:
            break
    return json.dumps({"accepted": out}, indent=2)
