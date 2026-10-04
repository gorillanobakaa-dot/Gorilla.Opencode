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
    "objc": "objc",        # upstream has its own doc since the 2026-10-04 refresh
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

# Extensions the analyser registry has NO language for, but a rule doc exists
# for. Without this table those docs are dead weight: rules_for_languages() only
# sees languages the registry can classify, so a Swift or Solidity file got no
# guidance and no sign any was available. Mirrors upstream's system_rules.json.
EXTENSION_TO_DOC = {
    ".kt": "kotlin", ".kts": "kotlin",
    ".php": "php", ".phtml": "php",
    ".swift": "swift",
    ".zig": "zig",
    ".sol": "solidity", ".vy": "vyper",
    ".rego": "rego",
    ".proto": "protobuf", ".thrift": "thrift", ".capnp": "capnp",
    ".graphql": "graphql", ".gql": "graphql",
    ".prisma": "prisma",
    ".jl": "julia", ".r": "r",
    ".tf": "terraform", ".hcl": "terraform", ".tfvars": "terraform",
    ".bicep": "bicep", ".nix": "nix",
    ".hs": "haskell", ".lhs": "haskell",
    ".nim": "nim", ".nims": "nim", ".nimble": "nim",
    ".elm": "elm",
    ".jsonnet": "jsonnet", ".libsonnet": "jsonnet",
    ".ml": "ocaml", ".mli": "ocaml", ".re": "ocaml", ".rei": "ocaml",
    ".fs": "fsharp", ".fsi": "fsharp", ".fsx": "fsharp",
    ".v": "verilog", ".sv": "verilog", ".vh": "verilog",
    ".vhd": "vhdl", ".vhdl": "vhdl",
    ".ets": "arkts", ".astro": "astro",
    ".hbs": "handlebars_mustache", ".mustache": "handlebars_mustache",
    ".jinja2": "jinja", ".j2": "jinja", ".pug": "pug",
    ".yaml": "yaml", ".yml": "yaml",
    ".json": "json", ".json5": "json",
    ".properties": "properties", ".po": "po", ".pot": "pot",
}

# ".m" is both Objective-C and MATLAB. The registry calls it objc because that
# is what its analysers can run on; for GUIDANCE the first line decides. These
# are upstream's signals (internal/config/rules/sniffer.go): a MATLAB file
# cannot begin with "/" and its comments start with "%", so a C-style comment
# opener is itself a reliable Objective-C sign.
OBJC_FIRST_LINE = ("#import", "#include", "#pragma", "#if", "#define",
                   "@import", "@interface", "@implementation", "@class",
                   "@protocol", "//", "/*")

DEFAULT_DOC = "default"

# A project may keep its own rule docs in this directory at its root, named
# exactly like the vendored ones (c.md, python.md, default.md, ...). Layering,
# highest first -- the idea is upstream's custom > project > global > system:
#
#   <project>/.code-review-rules/<doc>.md     the project's own
#   rule_docs/<doc>.md                        vendored
#
# The project's text is ADDED after the vendored text, because the usual need is
# "also check our house rule", not "forget everything else". A project file
# whose first line is exactly  <!-- replace -->  replaces the vendored doc.
PROJECT_RULES_DIRNAME = ".code-review-rules"
REPLACE_MARKER = "<!-- replace -->"

_cache: Dict[str, str] = {}
_project_dir: str = ""
_sources: Dict[str, str] = {}


def set_project(target_dir: str) -> None:
    """Point the resolver at a project so its own rule docs are layered in.
    Clears the cache: the same doc name now resolves differently."""
    global _project_dir
    _project_dir = target_dir or ""
    _cache.clear()
    _sources.clear()


def _read_file(path: str) -> str:
    try:
        # utf-8 stated outright: the platform default on Windows is cp1252, which
        # turns every non-ASCII character in a rule doc into mojibake.
        with open(path, encoding="utf-8", errors="replace") as f:
            return f.read().strip()
    except OSError:
        return ""


def _read(doc: str) -> str:
    if doc in _cache:
        return _cache[doc]
    system = _read_file(os.path.join(RULES_DIR, f"{doc}.md"))
    project = ""
    if _project_dir:
        project = _read_file(os.path.join(_project_dir, PROJECT_RULES_DIRNAME, f"{doc}.md"))

    if project.startswith(REPLACE_MARKER):
        text, source = project[len(REPLACE_MARKER):].strip(), "project"
    elif project and system:
        text = system + "\n\n#### Project rules (from " + PROJECT_RULES_DIRNAME + ")\n" + project
        source = "system+project"
    elif project:
        text, source = project, "project"
    else:
        text, source = system, "system"

    _cache[doc] = text
    if text:
        _sources[doc] = source
    return text


def sources() -> Dict[str, str]:
    """{doc: "system" | "project" | "system+project"} for every doc read so far,
    so a reader can tell which rules were the project's own."""
    return dict(_sources)


def doc_for_file(path: str) -> str:
    """The rule doc name for one file, or "" if none applies. Filename first
    (pom.xml), then the extensions the registry cannot classify, then ".m"."""
    base = os.path.basename(path).lower()
    if base in FILENAME_TO_DOC:
        return FILENAME_TO_DOC[base]
    ext = os.path.splitext(base)[1]
    # Three docs upstream keys on a path PATTERN rather than a name: CI
    # workflows, other .github configuration, and MyBatis mapper/DAO XML.
    parts = path.replace("\\", "/").lower().split("/")
    if ext in (".yml", ".yaml") and ".github" in parts:
        return "github_workflows" if "workflows" in parts else "github_config"
    if ext == ".xml" and ("mapper" in base or "dao" in base):
        return "mapper_dao_xml"
    if ext == ".m":
        return "objc" if _looks_like_objc(path) else "matlab"
    return EXTENSION_TO_DOC.get(ext, "")


def _looks_like_objc(path: str) -> bool:
    """First non-blank line decides, as upstream does: an Objective-C signal
    means Objective-C, anything else means MATLAB. A file that cannot be read
    is Objective-C, which is what the registry already assumes for ".m"."""
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            for line in f:
                line = line.strip()
                if line:
                    return line.startswith(OBJC_FIRST_LINE)
    except OSError:
        return True
    return False


def rules_for_files(files) -> dict:
    """{doc: guidance} for files matched by NAME or EXTENSION rather than by a
    registry language: build files, config formats, and every language the
    registry has no analyser for. Complements rules_for_languages()."""
    out = {}
    for f in files or []:
        doc = doc_for_file(f)
        if doc and doc not in out:
            text = _read(doc)
            if text:
                out[doc] = text
    return dict(sorted(out.items()))


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
    # A language mapped to None has no VENDORED doc; a project may still keep
    # one under the language's own name, so fall back to that rather than "".
    doc = LANGUAGE_TO_DOC.get(lang, lang) or lang
    return _read(doc)


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
