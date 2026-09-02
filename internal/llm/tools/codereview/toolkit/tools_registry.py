#!/usr/bin/env python3
"""
tools_registry.py
------------------
Single source of truth for every code-review / static-analysis / security tool
this toolkit knows about: how to check if it's installed, how to install it,
how to run it, and what it's good for.

Nothing in here executes anything by itself. install_tools.py reads this to
set your machine up; code_review.py reads this to build and run jobs.

Design notes (read this if you're extending the registry):

  scope="auto-file"     -> the orchestrator runs this tool once per matching
                            file, in parallel, output redirected to its own
                            log file. Safe to fire-and-forget.
  scope="auto-project"   -> run once for the whole target directory (tools
                            that want a package/workspace, e.g. `go vet ./...`,
                            `cargo clippy`, `golangci-lint run`, `semgrep`).
  scope="manual"          -> the tool needs project-specific build state
                            (a compile_commands.json, a configured kernel
                            .config, a bootstrapped Firefox mach environment,
                            a compiled binary for valgrind, etc.) that this
                            script cannot safely fabricate on your behalf.
                            code_review.py will NOT execute these. Instead it
                            prints (and saves) the exact, literal command(s)
                            you or an LLM should run by hand, plus what to do
                            with the output. This is intentional, not laziness:
                            auto-launching a full Firefox or kernel rebuild
                            from a generic "point me at a folder" script would
                            be slow, environment-fragile, and surprising.
  scope="checklist"       -> no CLI tool exists (e.g. FreeMarker). Emits a
                            manual review checklist into the report instead.

  stage=0  recon          -> cheap, informational, always runs first.
  stage=1  fast            -> linters/formatters, seconds per file.
  stage=2  standard        -> static analysis / security, slower.
  stage=3  deep             -> only auto-triggered if stage 1/2 output hits an
                            ESCALATION_KEYWORDS hint, or the user passes --deep.
"""

from dataclasses import dataclass, field
from typing import Callable, Dict, List, Optional
import os
import sys

import findings as _f

_TOOLKIT_DIR = os.path.dirname(os.path.abspath(__file__))

# ---------------------------------------------------------------------------
# Extension -> language map
# ---------------------------------------------------------------------------

EXTENSION_LANGUAGE: Dict[str, str] = {
    ".c": "c", ".h": "c",
    ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".hpp": "cpp", ".hh": "cpp", ".hxx": "cpp",
    ".m": "objc", ".mm": "objcpp",
    ".py": "python", ".pyi": "python",
    ".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
    ".ts": "typescript", ".tsx": "typescript",
    ".css": "css", ".scss": "css", ".less": "css",
    ".go": "go",
    ".rs": "rust",
    ".sh": "shell", ".bash": "shell",
    ".ftl": "freemarker",
    ".java": "java",
    ".mk": "make",
}
MAKEFILE_NAMES = {"Makefile", "makefile", "GNUmakefile"}

# Directories we never want to walk into (build output, vendored, VCS internals)
DEFAULT_IGNORE_DIRS = {
    ".git", ".hg", ".svn", "node_modules", "target", "dist", "build",
    "__pycache__", ".mozbuild", "obj-*", ".cargo", "vendor", ".venv", "venv",
    ".tox", "third_party", "objdir", ".ccache",
}

# Keywords that, if seen in a stage 1/2 log, escalate that file to stage 3.
ESCALATION_KEYWORDS = [
    "CWE-", "CVE-", "overflow", "use-after-free", "double-free", "uaf",
    "injection", "unsanitized", "hardcoded", "hard-coded", "secret",
    "race condition", "deserializ", "SSTI", "command injection",
    "path traversal", "format string", "null pointer dereference",
    "buffer overrun", "SEC_", "security",
]


@dataclass
class InstallSpec:
    # Rough installed footprint in MB, for the disk-space estimate `--doctor`
    # shows before anyone starts a download. Deliberately approximate and
    # deliberately rounded UP: the number exists so a person with 200MB free
    # finds out now rather than when apt fills the disk halfway through. It is
    # never used to decide anything automatically.
    size_mb: int = 15
    apt: Optional[str] = None            # Debian/Ubuntu package name
    dnf: Optional[str] = None            # Fedora/RHEL package name (best-effort)
    pacman: Optional[str] = None         # Arch package name (best-effort)
    pip: Optional[str] = None            # PyPI package name
    prefer_pipx: bool = False            # install `pip` package via pipx (isolated venv)
    npm: Optional[str] = None            # npm package name (installed with -g)
    cargo_component: Optional[str] = None  # `rustup component add <this>`
    cargo_install: Optional[str] = None  # `cargo install <this>`
    go_install: Optional[str] = None     # `go install <this>@latest`
    installer_script: Optional[str] = None  # name of a function in install_tools.py's SCRIPT_INSTALLERS
    manual_notes: str = ""               # shown if every automatic method is unavailable


@dataclass
class Tool:
    id: str
    label: str
    languages: List[str]
    category: str            # lint | format | static-analysis | security | secrets | recon | checklist
    stage: int
    scope: str                # auto-file | auto-project | manual | checklist
    check_cmd: List[str]
    install: InstallSpec = field(default_factory=InstallSpec)
    version_regex: str = r"(\d+\.\d+(\.\d+)?)"
    build_cmd: Optional[Callable[[str, dict], Optional[List[str]]]] = None
    manual_cmd: Optional[Callable[[dict], str]] = None
    checklist: Optional[List[str]] = None
    # Turns this tool's raw log text into normalised Finding records. Assigned
    # from PARSER_BY_TOOL_ID at the bottom of this file, not inline, so the
    # mapping is auditable in one place. Tools without one fall back to the
    # marker heuristic, which is what every tool used before findings.py.
    parse_output: Optional[Callable[[str, dict], list]] = None
    severity_markers: List[str] = field(default_factory=lambda: ["error", "warning"])
    profiles: List[str] = field(default_factory=lambda: ["generic"])
    notes: str = ""
    included_in: Optional[str] = None   # informational: "this tool's checks overlap with <id>"


# ---------------------------------------------------------------------------
# build_cmd helpers (kept tiny and literal on purpose -- these ARE the exact
# commands that get run; nothing hidden, nothing templated into obscurity)
# ---------------------------------------------------------------------------

def _cppcheck_cmd(path, ctx):
    return ["cppcheck", "--enable=warning,style,performance,portability",
            "--inline-suppr", "--suppress=missingIncludeSystem", "--template=gcc", path]

def _cppcheck_deep_cmd(path, ctx):
    return ["cppcheck", "--enable=all", "--inconclusive", "--inline-suppr",
            "--suppress=missingIncludeSystem", "--template=gcc", path]

def _clang_format_check_cmd(path, ctx):
    return ["clang-format", "--dry-run", "--Werror", path]

def _clang_tidy_cmd(path, ctx):
    if not ctx.get("compile_commands_dir"):
        return None
    return ["clang-tidy", "-p", ctx["compile_commands_dir"], path]

def _cpplint_cmd(path, ctx):
    return ["cpplint", path]

def _pylint_cmd(path, ctx):
    return ["pylint", "--output-format=parseable", path]

def _flake8_cmd(path, ctx):
    return ["flake8", path]

def _mypy_cmd(path, ctx):
    return ["mypy", "--ignore-missing-imports", "--show-error-codes", path]

def _bandit_cmd(path, ctx):
    return ["bandit", "-q", "-f", "txt", path]

def _bandit_deep_cmd(path, ctx):
    return ["bandit", "-iii", "-lll", "-f", "txt", path]

def _black_check_cmd(path, ctx):
    return ["black", "--check", "--diff", path]

def _isort_check_cmd(path, ctx):
    return ["isort", "--check-only", "--diff", path]

def _vulture_cmd(path, ctx):
    return ["vulture", path]

def _eslint_cmd(path, ctx):
    cfg = ctx.get("eslint_fallback_config")
    if ctx.get("has_own_eslint_config") or cfg is None:
        return ["eslint", "--no-color", path]
    return ["eslint", "--no-color", "--no-eslintrc", "-c", cfg, path]

def _prettier_check_cmd(path, ctx):
    return ["prettier", "--check", path]

def _stylelint_cmd(path, ctx):
    cfg = ctx.get("stylelint_fallback_config")
    if ctx.get("has_own_stylelint_config") or cfg is None:
        return ["stylelint", path]
    return ["stylelint", "--config", cfg, path]

def _shellcheck_cmd(path, ctx):
    return ["shellcheck", "-f", "gcc", path]

def _cargo_clippy_cmd(path, ctx):
    return ["cargo", "clippy", "--all-targets", "--", "-W", "clippy::all"]

def _cargo_fmt_check_cmd(path, ctx):
    return ["cargo", "fmt", "--check"]

def _cargo_audit_cmd(path, ctx):
    return ["cargo", "audit"]

def _golangci_lint_cmd(path, ctx):
    return ["golangci-lint", "run", "./..."]

def _go_vet_cmd(path, ctx):
    return ["go", "vet", "./..."]

def _staticcheck_cmd(path, ctx):
    return ["staticcheck", "./..."]

def _gosec_cmd(path, ctx):
    return ["gosec", "-quiet", "./..."]

def _semgrep_fast_cmd(path, ctx):
    # --json: semgrep's pretty output is a colour-aligned tree whose shape moves
    # between releases, so it's the one tool we ask for machine output. The log
    # still holds semgrep's complete, unedited response. See findings.py notes.
    return ["semgrep", "scan", "--quiet", "--json", "--config", "p/ci", path]

def _semgrep_deep_cmd(path, ctx):
    packs = ["p/security-audit", "p/owasp-top-ten"]
    extra = ctx.get("semgrep_language_packs") or []
    cmd = ["semgrep", "scan", "--quiet", "--json"]
    for p in packs + extra:
        cmd += ["--config", p]
    cmd.append(path)
    return cmd

def _gitleaks_cmd(path, ctx):
    return ["gitleaks", "detect", "--source", path, "--no-git", "-v"]

def _gitleaks_git_history_cmd(path, ctx):
    return ["gitleaks", "detect", "--source", path, "-v", "--log-opts=--all"]

def _cloc_cmd(path, ctx):
    return ["cloc", "--quiet", path]


# ---- in-house heuristics (heuristics.py) --------------------------------
# These shell out to our own script exactly like any external binary would, so
# they inherit the orchestrator's parallelism, timeouts and per-job logging for
# free rather than needing a second execution path inside the process.
#
# Naming: these are named for what they CHECK (brace-parity, kernel-locks), not
# for who wrote them. A prefix repeated across every sibling tool destroys shell
# and grep ergonomics and adds nothing a reader needs.

def _heuristic(check, path, ctx):
    return [sys.executable, os.path.join(_TOOLKIT_DIR, "heuristics.py"), check, path]

def _brace_parity_cmd(path, ctx):
    return _heuristic("brace-parity", path, ctx)

def _template_lookup_cmd(path, ctx):
    return _heuristic("template-lookup", path, ctx)

def _unified_build_cmd(path, ctx):
    # Unified-build pollution is only a hazard in a tree that actually does
    # unified builds. `profiles` is informational for auto-run tools (only the
    # manual tier consults it), so gating has to happen here -- returning None
    # is the registry's existing "don't schedule this job" signal.
    if ctx.get("profile") != "firefox":
        return None
    return _heuristic("unified-build", path, ctx)

def _kernel_locks_cmd(path, ctx):
    if ctx.get("profile") != "linux-kernel":
        return None
    return _heuristic("kernel-locks", path, ctx)


def _mach_lint_cmd(ctx):
    return "cd {target} && ./mach lint {sub}".format(
        target=ctx["target"], sub=ctx.get("rel_path", "")
    )

def _mach_static_analysis_cmd(ctx):
    return (
        "cd {target} && ./mach static-analysis check {sub}\n"
        "# First run auto-downloads the clang-tidy toolchain into .mozbuild -- expect a pause."
    ).format(target=ctx["target"], sub=ctx.get("rel_path", "") or ".")

def _kernel_checkpatch_cmd(ctx):
    if ctx.get("is_patch"):
        return "cd {target} && scripts/checkpatch.pl --strict --codespell {patch}".format(
            target=ctx["target"], patch=ctx.get("patch_path", "your.patch")
        )
    return "cd {target} && scripts/checkpatch.pl --strict --codespell -f {sub}".format(
        target=ctx["target"], sub=ctx.get("rel_path", "path/to/file.c")
    )

def _kernel_clang_tidy_cmd(ctx):
    return (
        "cd {target} && make CC=clang clang-tidy\n"
        "# Requires CONFIG_CC_IS_CLANG=y in your .config (build once with CC=clang first).\n"
        "# Internally runs scripts/clang-tools/gen_compile_commands.py then run-clang-tools.py."
    ).format(target=ctx["target"])

def _kernel_clang_analyzer_cmd(ctx):
    return "cd {target} && make CC=clang clang-analyzer".format(target=ctx["target"])

def _kernel_sparse_cmd(ctx):
    return "cd {target} && make C=2 {sub}".format(
        target=ctx["target"], sub=ctx.get("kernel_subdir", "")
    )

def _kernel_coccicheck_cmd(ctx):
    return "cd {target} && make coccicheck MODE=report".format(target=ctx["target"])

def _valgrind_cmd(ctx):
    return (
        "valgrind --leak-check=full --show-leak-kinds=all --track-origins=yes "
        "--log-file={outfile} {binary} {args}"
    ).format(outfile=ctx.get("outfile", "valgrind_out.txt"),
              binary=ctx.get("binary", "./your_compiled_binary"),
              args=ctx.get("args", ""))

def _scan_build_cmd(ctx):
    return (
        "scan-build -o {outdir} {build_cmd}\n"
        "# {build_cmd} is whatever you'd normally type to build this project "
        "(e.g. `make -j$(nproc)`), just prefixed by scan-build."
    ).format(outdir=ctx.get("outdir", "scan-build-results"), build_cmd=ctx.get("build_cmd", "make -j$(nproc)"))


# ---------------------------------------------------------------------------
# The registry
# ---------------------------------------------------------------------------

TOOLS: List[Tool] = [

    # ---- recon (stage 0, always runs, whole target) -----------------------
    Tool(
        id="cloc", label="cloc (line/language census)", languages=["*"],
        category="recon", stage=0, scope="auto-project",
        check_cmd=["cloc", "--version"],
        install=InstallSpec(size_mb=6, apt="cloc"),
        build_cmd=_cloc_cmd,
        notes="Not a reviewer -- just gives you a lay of the land before anything else runs.",
    ),
    Tool(
        id="gitleaks-worktree", label="gitleaks (secrets in current files)", languages=["*"],
        category="secrets", stage=1, scope="auto-project",
        check_cmd=["gitleaks", "version"],
        install=InstallSpec(size_mb=30, go_install="github.com/gitleaks/gitleaks/v8",
                             installer_script="install_gitleaks_binary"),
        build_cmd=_gitleaks_cmd,
        severity_markers=["leak", "secret", "finding"],
        notes="Scans the files as they sit on disk right now for committed-looking secrets.",
    ),

    # ---- C / C++ ------------------------------------------------------------
    Tool(
        id="cppcheck", label="cppcheck", languages=["c", "cpp"],
        category="static-analysis", stage=1, scope="auto-file",
        check_cmd=["cppcheck", "--version"],
        install=InstallSpec(size_mb=40, apt="cppcheck"),
        build_cmd=_cppcheck_cmd,
        severity_markers=["error", "warning", "style", "performance", "portability"],
        profiles=["generic", "linux-kernel", "firefox"],
    ),
    Tool(
        id="cppcheck-deep", label="cppcheck (--enable=all, inconclusive)", languages=["c", "cpp"],
        category="static-analysis", stage=3, scope="auto-file",
        check_cmd=["cppcheck", "--version"],
        install=InstallSpec(size_mb=40, apt="cppcheck"),
        build_cmd=_cppcheck_deep_cmd,
        severity_markers=["error", "warning"],
        included_in="cppcheck",
    ),
    Tool(
        id="clang-format-check", label="clang-format --dry-run", languages=["c", "cpp"],
        category="format", stage=1, scope="auto-file",
        check_cmd=["clang-format", "--version"],
        install=InstallSpec(size_mb=70, apt="clang-format"),
        build_cmd=_clang_format_check_cmd,
        severity_markers=["warning", "error"],
        profiles=["generic"],  # NOT firefox/kernel -- they have their own formatting entry points
    ),
    Tool(
        id="cpplint", label="cpplint (Google C++ style)", languages=["c", "cpp"],
        category="lint", stage=1, scope="auto-file",
        check_cmd=["cpplint", "--version"],
        install=InstallSpec(size_mb=4, pip="cpplint", prefer_pipx=True),
        build_cmd=_cpplint_cmd,
        severity_markers=["error"],
    ),
    Tool(
        id="clang-tidy", label="clang-tidy (needs compile_commands.json)", languages=["c", "cpp"],
        category="static-analysis", stage=2, scope="auto-file",
        check_cmd=["clang-tidy", "--version"],
        install=InstallSpec(size_mb=180, apt="clang-tidy"),
        build_cmd=_clang_tidy_cmd,
        severity_markers=["warning:", "error:"],
        notes="Only runs if you point --compile-commands-dir at a directory containing "
              "compile_commands.json. See PLAYBOOK.md for how to generate one for "
              "Firefox and the kernel specifically (each has its own tree-native path -- "
              "see the 'firefox' and 'linux-kernel' manual tools below instead).",
    ),
    Tool(
        id="valgrind", label="valgrind memcheck (on a compiled binary)", languages=["c", "cpp"],
        category="static-analysis", stage=3, scope="manual",
        check_cmd=["valgrind", "--version"],
        install=InstallSpec(size_mb=70, apt="valgrind"),
        manual_cmd=_valgrind_cmd,
        notes="Needs an already-compiled, runnable binary plus real invocation args -- "
              "there's no safe generic way to guess those, so this is always manual.",
    ),
    Tool(
        id="scan-build", label="clang static analyzer (scan-build)", languages=["c", "cpp"],
        category="static-analysis", stage=3, scope="manual",
        check_cmd=["scan-build", "--help"],
        install=InstallSpec(size_mb=180, apt="clang-tools"),
        manual_cmd=_scan_build_cmd,
        notes="Wraps your normal build command and needs a full rebuild -- run it manually "
              "when you have time to spare (can take as long as the underlying build).",
    ),

    # ---- Python ---------------------------------------------------------------
    Tool(
        id="pylint", label="pylint", languages=["python"],
        category="lint", stage=1, scope="auto-file",
        check_cmd=["pylint", "--version"],
        install=InstallSpec(size_mb=30, apt="pylint", pip="pylint", prefer_pipx=True),
        build_cmd=_pylint_cmd,
        severity_markers=["error", "warning", "convention", "refactor"],
    ),
    Tool(
        id="flake8", label="flake8", languages=["python"],
        category="lint", stage=1, scope="auto-file",
        check_cmd=["flake8", "--version"],
        install=InstallSpec(size_mb=12, apt="flake8", pip="flake8", prefer_pipx=True),
        build_cmd=_flake8_cmd,
        severity_markers=["E", "W", "F"],
    ),
    Tool(
        id="mypy", label="mypy (type checking)", languages=["python"],
        category="static-analysis", stage=1, scope="auto-file",
        check_cmd=["mypy", "--version"],
        install=InstallSpec(size_mb=60, apt="mypy", pip="mypy", prefer_pipx=True),
        build_cmd=_mypy_cmd,
        severity_markers=["error:"],
    ),
    Tool(
        id="bandit", label="bandit (Python security)", languages=["python"],
        category="security", stage=2, scope="auto-file",
        check_cmd=["bandit", "--version"],
        install=InstallSpec(size_mb=20, apt="bandit", pip="bandit", prefer_pipx=True),
        build_cmd=_bandit_cmd,
        severity_markers=["Issue:", "Severity:"],
    ),
    Tool(
        id="bandit-deep", label="bandit (all confidence/severity levels)", languages=["python"],
        category="security", stage=3, scope="auto-file",
        check_cmd=["bandit", "--version"],
        install=InstallSpec(size_mb=20, apt="bandit", pip="bandit", prefer_pipx=True),
        build_cmd=_bandit_deep_cmd,
        severity_markers=["Issue:"],
        included_in="bandit",
    ),
    Tool(
        id="black-check", label="black --check", languages=["python"],
        category="format", stage=1, scope="auto-file",
        check_cmd=["black", "--version"],
        install=InstallSpec(size_mb=25, apt="black", pip="black", prefer_pipx=True),
        build_cmd=_black_check_cmd,
        severity_markers=["would reformat"],
    ),
    Tool(
        id="isort-check", label="isort --check-only", languages=["python"],
        category="format", stage=1, scope="auto-file",
        check_cmd=["isort", "--version"],
        install=InstallSpec(size_mb=6, apt="isort", pip="isort", prefer_pipx=True),
        build_cmd=_isort_check_cmd,
        severity_markers=["ERROR"],
    ),
    Tool(
        id="vulture", label="vulture (dead code)", languages=["python"],
        category="lint", stage=3, scope="auto-file",
        check_cmd=["vulture", "--version"],
        install=InstallSpec(size_mb=4, pip="vulture", prefer_pipx=True),
        build_cmd=_vulture_cmd,
        severity_markers=["unused"],
    ),

    # ---- JavaScript / TypeScript / CSS ----------------------------------------
    Tool(
        id="eslint", label="eslint", languages=["javascript", "typescript"],
        category="lint", stage=1, scope="auto-file",
        check_cmd=["eslint", "--version"],
        install=InstallSpec(size_mb=45, npm="eslint"),
        build_cmd=_eslint_cmd,
        severity_markers=["error", "warning"],
        profiles=["generic"],  # firefox tree: use ./mach lint instead (its own config)
    ),
    Tool(
        id="prettier-check", label="prettier --check", languages=["javascript", "typescript", "css"],
        category="format", stage=1, scope="auto-file",
        check_cmd=["prettier", "--version"],
        install=InstallSpec(size_mb=12, npm="prettier"),
        build_cmd=_prettier_check_cmd,
        severity_markers=["[warn]"],
        profiles=["generic"],
    ),
    Tool(
        id="stylelint", label="stylelint", languages=["css"],
        category="lint", stage=1, scope="auto-file",
        check_cmd=["stylelint", "--version"],
        install=InstallSpec(size_mb=40, npm="stylelint stylelint-config-standard"),
        build_cmd=_stylelint_cmd,
        severity_markers=["error", "warning"],
    ),

    # ---- Rust -------------------------------------------------------------------
    Tool(
        id="cargo-clippy", label="cargo clippy", languages=["rust"],
        category="lint", stage=1, scope="auto-project",
        check_cmd=["cargo", "clippy", "--version"],
        install=InstallSpec(size_mb=120, installer_script="install_rustup_component_clippy",
                             manual_notes="rustup component add clippy"),
        build_cmd=_cargo_clippy_cmd,
        severity_markers=["warning:", "error:"],
    ),
    Tool(
        id="cargo-fmt-check", label="cargo fmt --check", languages=["rust"],
        category="format", stage=1, scope="auto-project",
        check_cmd=["cargo", "fmt", "--version"],
        install=InstallSpec(size_mb=40, installer_script="install_rustup_component_rustfmt",
                             manual_notes="rustup component add rustfmt"),
        build_cmd=_cargo_fmt_check_cmd,
        severity_markers=["Diff in"],
    ),
    Tool(
        id="cargo-audit", label="cargo audit (dependency CVEs)", languages=["rust"],
        category="security", stage=2, scope="auto-project",
        check_cmd=["cargo", "audit", "--version"],
        install=InstallSpec(size_mb=90, cargo_install="cargo-audit"),
        build_cmd=_cargo_audit_cmd,
        severity_markers=["Vulnerability", "ID:"],
        notes="Needs network access to fetch the RustSec advisory database.",
    ),

    # ---- Go ------------------------------------------------------------------------
    Tool(
        id="golangci-lint", label="golangci-lint (meta-linter)", languages=["go"],
        category="lint", stage=1, scope="auto-project",
        check_cmd=["golangci-lint", "--version"],
        install=InstallSpec(size_mb=180, installer_script="install_golangci_lint"),
        build_cmd=_golangci_lint_cmd,
        severity_markers=["warning", "error"],
        notes="Bundles govet, staticcheck, errcheck, gosec, revive, gofmt, goimports and more.",
    ),
    Tool(
        id="go-vet", label="go vet", languages=["go"],
        category="lint", stage=1, scope="auto-project",
        check_cmd=["go", "version"],
        install=InstallSpec(size_mb=0, apt="golang-go",
                             manual_notes="Or grab a newer release straight from https://go.dev/dl/ "
                                           "if you want a more current version than Debian ships."),
        build_cmd=_go_vet_cmd,
        severity_markers=["vet:"],
        included_in="golangci-lint",
    ),
    Tool(
        id="staticcheck", label="staticcheck (standalone)", languages=["go"],
        category="static-analysis", stage=2, scope="auto-project",
        check_cmd=["staticcheck", "-version"],
        install=InstallSpec(size_mb=90, go_install="honnef.co/go/tools/cmd/staticcheck"),
        build_cmd=_staticcheck_cmd,
        severity_markers=["SA", "ST", "error"],
        included_in="golangci-lint",
    ),
    Tool(
        id="gosec", label="gosec (Go security)", languages=["go"],
        category="security", stage=2, scope="auto-project",
        check_cmd=["gosec", "--version"],
        install=InstallSpec(size_mb=70, go_install="github.com/securego/gosec/v2/cmd/gosec"),
        build_cmd=_gosec_cmd,
        severity_markers=["Severity:", "CWE"],
        included_in="golangci-lint",
    ),

    # ---- Multi-language ---------------------------------------------------------------
    Tool(
        id="semgrep-fast", label="semgrep (p/ci ruleset)", languages=["python", "javascript", "typescript", "go", "c", "cpp"],
        category="static-analysis", stage=1, scope="auto-project",
        check_cmd=["semgrep", "--version"],
        install=InstallSpec(size_mb=180, pip="semgrep", prefer_pipx=True),
        build_cmd=_semgrep_fast_cmd,
        severity_markers=["ERROR", "WARNING"],
        notes="No login/API key needed -- pulls the public 'p/ci' ruleset from the registry. "
              "Runs once across the whole target (semgrep understands multiple languages in "
              "a single pass, so per-file invocations would just be slower and redundant).",
    ),
    Tool(
        id="semgrep-deep", label="semgrep (security-audit + owasp-top-ten)", languages=["python", "javascript", "typescript", "go", "c", "cpp"],
        category="security", stage=3, scope="auto-project",
        check_cmd=["semgrep", "--version"],
        install=InstallSpec(size_mb=180, pip="semgrep", prefer_pipx=True),
        build_cmd=_semgrep_deep_cmd,
        severity_markers=["ERROR", "WARNING"],
        included_in="semgrep-fast",
    ),
    Tool(
        id="shellcheck", label="shellcheck", languages=["shell"],
        category="lint", stage=1, scope="auto-file",
        check_cmd=["shellcheck", "--version"],
        install=InstallSpec(size_mb=25, apt="shellcheck"),
        build_cmd=_shellcheck_cmd,
        severity_markers=["error", "warning", "note"],
    ),
    Tool(
        id="gitleaks-history", label="gitleaks (full git history)", languages=["*"],
        category="secrets", stage=3, scope="auto-project",
        check_cmd=["gitleaks", "version"],
        install=InstallSpec(size_mb=30, go_install="github.com/gitleaks/gitleaks/v8",
                             installer_script="install_gitleaks_binary"),
        build_cmd=_gitleaks_git_history_cmd,
        severity_markers=["leak", "secret"],
        included_in="gitleaks-worktree",
        notes="Only meaningful inside a git repo; walks the *entire* commit history, so it's slow.",
    ),

    # ---- Firefox tree-native (only surfaced when a mozilla-central checkout is detected) --
    Tool(
        id="firefox-mach-lint", label="./mach lint (Firefox's own bundled linters)",
        languages=["*"], category="lint", stage=1, scope="manual",
        check_cmd=[], install=InstallSpec(),
        manual_cmd=_mach_lint_cmd,
        profiles=["firefox"],
        notes="Wraps eslint/flake8/clang-format/etc. already configured correctly for this "
              "tree. Always prefer this over generic eslint/prettier/clang-format on a "
              "mozilla-central checkout.",
    ),
    Tool(
        id="firefox-mach-static-analysis", label="./mach static-analysis check (clang-tidy, Firefox-tuned)",
        languages=["c", "cpp"], category="static-analysis", stage=2, scope="manual",
        check_cmd=[], install=InstallSpec(),
        manual_cmd=_mach_static_analysis_cmd,
        profiles=["firefox"],
        notes="Downloads its own pinned clang-tidy + Firefox-specific checks on first run.",
    ),

    # ---- Linux kernel tree-native (only surfaced when a kernel checkout is detected) -----
    Tool(
        id="kernel-checkpatch", label="scripts/checkpatch.pl (the canonical kernel style/patch checker)",
        languages=["c"], category="lint", stage=1, scope="manual",
        check_cmd=["perl", "--version"],
        install=InstallSpec(apt="perl"),
        manual_cmd=_kernel_checkpatch_cmd,
        profiles=["linux-kernel"],
        notes="Run this first, always -- it's what upstream kernel maintainers actually run "
              "against your patch before anything else is considered.",
    ),
    Tool(
        id="kernel-clang-tidy", label="make CC=clang clang-tidy (kernel-tree clang-tidy target)",
        languages=["c"], category="static-analysis", stage=2, scope="manual",
        check_cmd=["clang", "--version"],
        install=InstallSpec(apt="clang"),
        manual_cmd=_kernel_clang_tidy_cmd,
        profiles=["linux-kernel"],
    ),
    Tool(
        id="kernel-clang-analyzer", label="make CC=clang clang-analyzer",
        languages=["c"], category="static-analysis", stage=3, scope="manual",
        check_cmd=["clang", "--version"],
        install=InstallSpec(apt="clang"),
        manual_cmd=_kernel_clang_analyzer_cmd,
        profiles=["linux-kernel"],
    ),
    Tool(
        id="kernel-sparse", label="make C=2 (sparse semantic checker)",
        languages=["c"], category="static-analysis", stage=2, scope="manual",
        check_cmd=["sparse", "--version"],
        install=InstallSpec(apt="sparse"),
        manual_cmd=_kernel_sparse_cmd,
        profiles=["linux-kernel"],
        notes="Catches address-space/endianness/locking-context mistakes clang-tidy won't.",
    ),
    Tool(
        id="kernel-coccicheck", label="make coccicheck (Coccinelle semantic patches)",
        languages=["c"], category="static-analysis", stage=3, scope="manual",
        check_cmd=["spatch", "--version"],
        install=InstallSpec(apt="coccinelle"),
        manual_cmd=_kernel_coccicheck_cmd,
        profiles=["linux-kernel"],
    ),

    # ---- in-house heuristics ----------------------------------------------------------
    # Rebuilt from the multi-agent-code-auditor skill, whose orchestrator went
    # missing (~/Downloads/Multi-Agent.Code.Auditor/auditor.py no longer exists).
    # Nothing off-the-shelf in this registry performs these checks: they encode
    # knowledge about particular codebases, not about a language.
    Tool(
        id="brace-parity", label="brace/paren/bracket parity (comment- and string-aware)",
        languages=["c", "cpp", "objc", "objcpp"],
        category="static-analysis", stage=1, scope="auto-file",
        check_cmd=[],  # our own script; always present alongside this file
        install=InstallSpec(),
        build_cmd=_brace_parity_cmd,
        profiles=["generic", "firefox", "linux-kernel"],
        notes="Cheapest possible catch for the worst possible outcome: a patch that "
              "cannot compile. Follows one #if branch rather than all of them, so "
              "conditional compilation doesn't produce phantom imbalances.",
    ),
    Tool(
        id="template-lookup", label="dependent-name lookup without `typename`",
        languages=["cpp", "objcpp"],
        category="static-analysis", stage=2, scope="auto-file",
        check_cmd=[], install=InstallSpec(),
        build_cmd=_template_lookup_cmd,
        profiles=["generic", "firefox"],
        notes="Lexical heuristic, reported at info severity on purpose -- real C++ "
              "name lookup needs a frontend, so treat hits as 'go look', not 'this "
              "is broken'.",
    ),
    Tool(
        id="unified-build", label="Firefox unified-build pollution (macros, file-scope using)",
        languages=["cpp", "objcpp"],
        category="static-analysis", stage=2, scope="auto-file",
        check_cmd=[], install=InstallSpec(),
        build_cmd=_unified_build_cmd,
        profiles=["firefox"],
        notes="Only scheduled under --profile firefox: these patterns are harmless "
              "in a normal build and dangerous only when files are concatenated "
              "into one translation unit.",
    ),
    Tool(
        id="kernel-locks", label="sleeping while holding a spinlock (GFP_KERNEL et al)",
        languages=["c"],
        category="static-analysis", stage=2, scope="auto-file",
        check_cmd=[], install=InstallSpec(),
        build_cmd=_kernel_locks_cmd,
        profiles=["linux-kernel"],
        notes="Only scheduled under --profile linux-kernel. Same-function scan: a "
              "sleep inside a called function is invisible to it, so a clean result "
              "is not proof of atomic-context safety.",
    ),

    # ---- FreeMarker: no CLI tool exists -----------------------------------------------
    Tool(
        id="ftl-checklist", label="FreeMarker manual review checklist", languages=["freemarker"],
        category="checklist", stage=1, scope="checklist",
        check_cmd=[], install=InstallSpec(),
        checklist=[
            "Confirm the Configuration's output_format / auto_escaping_policy enables "
            "autoescaping for HTML contexts -- if it's disabled, every ${...} is a "
            "potential injection point.",
            "Grep for `?api`, `?new()`, and `freemarker.template.utility.Execute` / "
            "`ObjectConstructor` usage -- these are the classic FreeMarker "
            "template-injection (SSTI) building blocks and should not appear near "
            "user-controlled input.",
            "Check every ${...} that renders into HTML is passed through `?html` (or "
            "relies on autoescaping) rather than being trusted raw.",
            "Verify user input never reaches a template *name* or gets `?interpret`-ed "
            "(interpreting user strings as FreeMarker source is close to arbitrary code "
            "execution).",
            "Look for directive misuse (`<#assign>` overwriting shared/global variables, "
            "`<#import>`/`<#include>` paths built from request parameters).",
        ],
        notes="Also worth pointing semgrep's generic 'p/security-audit' pack at a "
              "directory containing .ftl files -- its generic pattern engine can catch "
              "some raw ${...} interpolation even without an FTL-specific ruleset.",
    ),
]

TOOLS_BY_ID: Dict[str, Tool] = {t.id: t for t in TOOLS}


# ---------------------------------------------------------------------------
# Output parsers -- which function turns each tool's log into Findings.
#
# Kept as one table rather than an argument on 39 Tool entries, so you can see
# the whole coverage picture at a glance and spot what's still on the fallback.
#
# Anything NOT listed here uses findings.parse_marker_fallback, i.e. exactly
# the heuristic every tool used before findings.py existed. Adding a parser is
# therefore always an improvement and never a regression -- which is why this
# migration can be done a tool at a time.
# ---------------------------------------------------------------------------

PARSER_BY_TOOL_ID: Dict[str, Callable[[str, dict], list]] = {
    # gcc-style `path:line:col: severity: message` -- the majority
    "cppcheck": _f.parse_gcc_style,
    "cppcheck-deep": _f.parse_gcc_style,
    "clang-tidy": _f.parse_gcc_style,
    "flake8": _f.parse_gcc_style,
    "mypy": _f.parse_gcc_style,
    "shellcheck": _f.parse_gcc_style,
    "go-vet": _f.parse_gcc_style,
    "staticcheck": _f.parse_gcc_style,
    "golangci-lint": _f.parse_gcc_style,
    "eslint": _f.parse_gcc_style,
    "stylelint": _f.parse_gcc_style,
    "vulture": _f.parse_gcc_style,

    # bespoke formats
    "pylint": _f.parse_pylint,
    "cpplint": _f.parse_cpplint,
    "bandit": _f.parse_bandit,
    "bandit-deep": _f.parse_bandit,
    "gitleaks-worktree": _f.parse_gitleaks,
    "gitleaks-history": _f.parse_gitleaks,
    "cargo-clippy": _f.parse_rust,
    "cargo-audit": _f.parse_rust,
    "gosec": _f.parse_gosec,
    "semgrep-fast": _f.parse_semgrep,
    "semgrep-deep": _f.parse_semgrep,

    # our own checks already speak the Finding shape
    "brace-parity": _f.parse_heuristics,
    "template-lookup": _f.parse_heuristics,
    "unified-build": _f.parse_heuristics,
    "kernel-locks": _f.parse_heuristics,

    # formatters: one finding per unformatted file, not one per line
    "black-check": _f.parse_format_check,
    "isort-check": _f.parse_format_check,
    "prettier-check": _f.parse_format_check,
    "clang-format-check": _f.parse_format_check,
    "cargo-fmt-check": _f.parse_format_check,

    # cloc is recon -- it reports a census, not problems. Explicitly no findings,
    # so it can never inflate a count or look like it "found" something.
    "cloc": lambda text, ctx: [],
}

for _t in TOOLS:
    _t.parse_output = PARSER_BY_TOOL_ID.get(_t.id)


def unparsed_tool_ids() -> List[str]:
    """Auto-run tools still relying on the marker fallback. Handy as a to-do
    list when extending parser coverage; also surfaced by --audience agent so a
    consumer knows which findings carry a trustworthy line number."""
    return sorted(t.id for t in TOOLS
                  if t.scope in ("auto-file", "auto-project") and t.parse_output is None)


def tools_for_language(lang: str) -> List[Tool]:
    return [t for t in TOOLS if lang in t.languages]


def tools_for_stage(stage: int) -> List[Tool]:
    return [t for t in TOOLS if t.stage == stage]


def all_apt_packages() -> List[str]:
    seen, out = set(), []
    for t in TOOLS:
        pkg = t.install.apt
        if pkg and pkg not in seen:
            seen.add(pkg)
            out.append(pkg)
    return out
