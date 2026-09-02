#!/usr/bin/env python3
"""
rules.py
--------
Review know-how, keyed by language.

`tools_registry.py` answers "what can I RUN against this file?". This answers
"what should someone LOOK FOR in it?" -- the part no linter in the registry
covers, because it's judgement rather than pattern matching.

The content lives in rule_docs/ as plain Markdown, vendored from OpenCodeReview
(Apache-2.0, see rule_docs/NOTICE.md). Keeping it as data rather than baking it
into a prompt string means:

  * a human can read and edit it without touching Python
  * the same text serves an agent, a local model, and a person
  * refreshing from upstream is a download, not a merge

Nothing here executes anything or calls out to the network unless you explicitly
run `python3 rules.py --refresh`.

Usage:
    import rules
    rules.for_language("c")              -> markdown str ("" if none)
    rules.rules_for_languages(["c","go"]) -> {"c": "...", "go": "...", "default": "..."}
    python3 rules.py --list
    python3 rules.py c
    python3 rules.py --refresh
"""

import os
import sys
from typing import Dict

RULES_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "rule_docs")

UPSTREAM_REPO = "alibaba/open-code-review"
UPSTREAM_PATH = "internal/config/rules/rule_docs"

# Our language ids (from tools_registry.EXTENSION_LANGUAGE) -> rule_docs filename.
# Kept explicit rather than guessed: upstream names files after ecosystems
# ("ts_js_tsx_jsx") while we name them after languages ("typescript"), and a
# silent miss here would mean an agent reviewing Rust with no Rust guidance and
# no indication anything was absent.
LANGUAGE_TO_DOC = {
    "c": "c",
    "cpp": "cpp",
    "objc": "c",           # near enough for memory/buffer guidance
    "objcpp": "cpp",
    "python": "python",
    "javascript": "ts_js_tsx_jsx",
    "typescript": "ts_js_tsx_jsx",
    "go": "go",
    "rust": "rust",
    "java": "java",
    "freemarker": "freemarker",
    "css": None,           # no upstream doc; falls back to default
    "shell": None,
    "make": None,
}

# Files matched by exact filename rather than by language, for the config/build
# formats where the FILE is the thing with rules, not its extension.
FILENAME_TO_DOC = {
    "pom.xml": "pom_xml",
    "package.json": "package_json",
    "cargo.toml": "cargo_toml",
    "composer.json": "composer_json",
    "build.gradle": "build_gradle",
    "build.gradle.kts": "build_gradle",
}

DEFAULT_DOC = "default"

_cache: Dict[str, str] = {}


def _read(doc: str) -> str:
    if doc in _cache:
        return _cache[doc]
    path = os.path.join(RULES_DIR, f"{doc}.md")
    try:
        with open(path, errors="replace") as f:
            text = f.read().strip()
    except OSError:
        text = ""
    _cache[doc] = text
    return text


def available() -> list:
    """Rule doc names present on disk."""
    try:
        return sorted(n[:-3] for n in os.listdir(RULES_DIR)
                      if n.endswith(".md") and n != "NOTICE.md")
    except OSError:
        return []


def for_language(lang: str) -> str:
    """Review guidance for one language id. Empty string if we have none --
    callers should treat that as "no guidance", never as "nothing to check"."""
    if not lang:
        return ""
    doc = LANGUAGE_TO_DOC.get(lang, lang)
    return _read(doc) if doc else ""


def for_filename(path: str) -> str:
    """Guidance keyed on a specific filename (pom.xml, package.json, ...)."""
    doc = FILENAME_TO_DOC.get(os.path.basename(path).lower())
    return _read(doc) if doc else ""


def default_rules() -> str:
    """The language-agnostic checklist: correctness, security, performance,
    maintainability, test coverage. Always worth including."""
    return _read(DEFAULT_DOC)


def rules_for_languages(langs) -> dict:
    """{language: guidance} for the languages actually in scope, plus 'default'.

    Languages we have no doc for are omitted rather than given an empty string,
    so a consumer can tell "no guidance exists" apart from "guidance is blank".
    """
    out = {}
    for lang in sorted(set(langs or [])):
        text = for_language(lang)
        if text:
            out[lang] = text
    d = default_rules()
    if d:
        out["default"] = d
    return out


def missing_docs(langs) -> list:
    """Languages in scope with no rule doc -- an honest gap list."""
    return sorted({l for l in (langs or []) if l and not for_language(l)})


# ---------------------------------------------------------------------------
# refresh from upstream
# ---------------------------------------------------------------------------

def refresh() -> int:
    """Re-download the rule docs from upstream. Explicit, never automatic --
    a review tool that silently mutates its own criteria mid-run would make
    two runs incomparable."""
    import json
    import urllib.request

    tree_url = f"https://api.github.com/repos/{UPSTREAM_REPO}/git/trees/main?recursive=1"
    print(f"Fetching file list from {UPSTREAM_REPO} ...")
    with urllib.request.urlopen(tree_url, timeout=60) as r:
        tree = json.load(r)
    paths = [e["path"] for e in tree.get("tree", [])
             if e.get("type") == "blob" and e["path"].startswith(UPSTREAM_PATH + "/")
             and e["path"].endswith(".md")]
    if not paths:
        print("No rule docs found upstream -- layout may have changed. Nothing written.")
        return 1

    os.makedirs(RULES_DIR, exist_ok=True)
    base = f"https://raw.githubusercontent.com/{UPSTREAM_REPO}/main/"
    ok = 0
    for p in paths:
        name = os.path.basename(p)
        if name == "NOTICE.md":       # never clobber our own attribution file
            continue
        try:
            with urllib.request.urlopen(base + p, timeout=60) as r:
                data = r.read()
            with open(os.path.join(RULES_DIR, name), "wb") as f:
                f.write(data)
            ok += 1
        except Exception as e:
            print(f"  FAILED {name}: {e}")
    print(f"Refreshed {ok}/{len(paths)} rule doc(s) in {RULES_DIR}")
    print("Remember: rule_docs/NOTICE.md records provenance -- update the date there.")
    return 0


def main() -> int:
    args = sys.argv[1:]
    if not args or args[0] in ("-h", "--help"):
        print(__doc__)
        return 0
    if args[0] == "--refresh":
        return refresh()
    if args[0] == "--list":
        docs = available()
        print(f"{len(docs)} rule doc(s) in {RULES_DIR}:\n  " + "\n  ".join(docs))
        print("\nLanguage mapping:")
        for lang, doc in sorted(LANGUAGE_TO_DOC.items()):
            status = "-> " + (doc or "(none, uses default)")
            print(f"  {lang:<12} {status}")
        return 0
    text = for_language(args[0]) or _read(args[0])
    if not text:
        print(f"No rule doc for '{args[0]}'. Try --list.")
        return 1
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
