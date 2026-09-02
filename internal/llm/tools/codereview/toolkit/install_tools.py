#!/usr/bin/env python3
"""
install_tools.py
-----------------
Sets up a complete local code-review/static-analysis toolchain.

What it does, in order, for every tool in tools_registry.py:
  1. Check if it's already installed (and where, and what version).
  2. If it's installed in more than one place on PATH (e.g. an apt copy AND a
     pip copy), warn about which one will actually run and why.
  3. If it's missing, install it using the best available method for THIS
     machine: apt first (system-integrated, fewest surprises), then a
     language-native method (pipx / npm / cargo / go), then a tool-specific
     installer script, then -- if nothing else works -- print exactly what
     you'd need to do by hand.
  4. Print a rolling status line per tool, then a final summary table.

Usage:
    python3 install_tools.py                 # check + install everything
    python3 install_tools.py --check-only    # just report, install nothing
    python3 install_tools.py --yes           # don't ask before each install
    python3 install_tools.py --skip-heavy    # skip slow ones (cargo-audit, semgrep, gitleaks)
    python3 install_tools.py --only cpp,python,go   # restrict to these languages

Safe to re-run any time -- everything is idempotent (checks before acting).
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from datetime import datetime

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from tools_registry import TOOLS, Tool, InstallSpec  # noqa: E402

HOME = os.path.expanduser("~")
LOCAL_BIN = os.path.join(HOME, ".local", "bin")
GO_BIN_GUESS = os.path.join(HOME, "go", "bin")

# ---------------------------------------------------------------------------
# tiny console helpers (no external deps -- plain ANSI, degrades gracefully)
# ---------------------------------------------------------------------------

USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None
def _c(code, s):
    return f"\033[{code}m{s}\033[0m" if USE_COLOR else s
def green(s): return _c("32", s)
def yellow(s): return _c("33", s)
def red(s): return _c("31", s)
def dim(s): return _c("2", s)
def bold(s): return _c("1", s)

def status_line(tag, msg):
    ts = datetime.now().strftime("%H:%M:%S")
    print(f"[{ts}] {tag:<10} {msg}")


# ---------------------------------------------------------------------------
# process helpers
# ---------------------------------------------------------------------------

def run(cmd, timeout=None, input_text=None, cwd=None):
    """Never raises. Returns (returncode, stdout, stderr)."""
    try:
        # check=False is deliberate and explicit: try_install() reads the return
        # code to decide whether to fall through to the next install method, so
        # a failed apt/pip/npm attempt must return normally rather than raise.
        p = subprocess.run(
            cmd, capture_output=True, text=True, timeout=timeout,
            input=input_text, cwd=cwd, check=False,
        )
        return p.returncode, p.stdout, p.stderr
    except FileNotFoundError:
        return 127, "", "command not found"
    except subprocess.TimeoutExpired:
        return 124, "", "timed out"
    except Exception as e:  # pragma: no cover
        return 1, "", str(e)


def as_root(cmd):
    if os.geteuid() == 0:
        return cmd
    if shutil.which("sudo"):
        return ["sudo"] + cmd
    return cmd  # best effort; will likely fail with a permissions error, which is informative enough


def find_all_in_path(binary_name):
    """Every place on PATH that has an executable with this name -- to catch
    version-conflicting duplicate installs (e.g. apt's flake8 AND a pipx one)."""
    found = []
    for d in os.environ.get("PATH", "").split(os.pathsep):
        candidate = os.path.join(d, binary_name)
        if os.path.isfile(candidate) and os.access(candidate, os.X_OK):
            found.append(candidate)
    return found


# ---------------------------------------------------------------------------
# package manager detection
# ---------------------------------------------------------------------------

def detect_pkg_manager():
    for mgr in ("apt-get", "dnf", "pacman", "apk", "zypper", "brew"):
        if shutil.which(mgr):
            return mgr
    return None

PKG_MANAGER = detect_pkg_manager()
_APT_UPDATED = False

def apt_update_once():
    global _APT_UPDATED
    if _APT_UPDATED or PKG_MANAGER != "apt-get":
        return
    status_line("SETUP", "Running `apt-get update` once for this session...")
    rc, out, err = run(as_root(["apt-get", "update", "-qq"]), timeout=180)
    if rc != 0:
        print(yellow(f"    apt-get update had issues (continuing anyway): {err.strip()[:200]}"))
    _APT_UPDATED = True


def apt_candidate_version(pkg):
    rc, out, _ = run(["apt-cache", "policy", pkg], timeout=20)
    if rc != 0:
        return None
    m = re.search(r"Candidate:\s*(\S+)", out)
    return m.group(1) if m and m.group(1) != "(none)" else None


def apt_install(pkgs, assume_yes):
    apt_update_once()
    cmd = as_root(["apt-get", "install"] + (["-y"] if assume_yes else []) + pkgs)
    status_line("APT", f"installing: {' '.join(pkgs)}")
    rc, out, err = run(cmd, timeout=900)
    return rc == 0, (out + err)


# ---------------------------------------------------------------------------
# language-native install helpers
# ---------------------------------------------------------------------------

def ensure_pipx(assume_yes):
    if shutil.which("pipx"):
        return True
    if PKG_MANAGER == "apt-get":
        ok, _ = apt_install(["pipx"], assume_yes)
        if ok:
            run(["pipx", "ensurepath"], timeout=30)
            return True
    # fallback: pip --user (PEP 668 requires --break-system-packages on Debian/Ubuntu)
    rc, _, err = run([sys.executable, "-m", "pip", "install", "--user",
                       "--break-system-packages", "pipx"], timeout=120)
    if rc == 0:
        run([sys.executable, "-m", "pipx", "ensurepath"], timeout=30)
        return True
    print(yellow(f"    could not bootstrap pipx: {err.strip()[:200]}"))
    return False


def pipx_install(pkg, assume_yes):
    if not ensure_pipx(assume_yes):
        return False, "pipx unavailable"
    status_line("PIPX", f"installing: {pkg}")
    rc, out, err = run(["pipx", "install", pkg], timeout=300)
    return rc == 0, (out + err)


def npm_ready():
    return shutil.which("npm") is not None

def npm_prefix_writable() -> bool:
    """True if `npm install -g` can write without root.

    Version-manager installs (nvm/fnm/volta/asdf) keep the prefix under $HOME,
    where sudo is not merely unnecessary but actively harmful. Falls back to
    assuming root is needed if we can't determine the prefix, since that's the
    system-npm case.
    """
    rc, out, _ = run(["npm", "config", "get", "prefix"], timeout=30)
    prefix = (out or "").strip()
    if rc != 0 or not prefix or prefix == "undefined":
        return False
    for candidate in (os.path.join(prefix, "lib"), prefix):
        if os.path.isdir(candidate):
            return os.access(candidate, os.W_OK)
    return False


def npm_install_global(pkg_spec, assume_yes):
    if not npm_ready():
        if PKG_MANAGER == "apt-get":
            ok, msg = apt_install(["npm", "nodejs"], assume_yes)
            if not ok:
                return False, "npm/nodejs unavailable and apt install failed: " + msg
        else:
            return False, "npm not found and no known way to install it automatically here"
    status_line("NPM", f"installing globally: {pkg_spec}")
    # Only escalate if the global prefix actually needs root. A version manager
    # (nvm, fnm, volta, asdf) puts the prefix under $HOME, and then `sudo npm`
    # fails twice over: root's PATH has no nvm shim, so npm isn't even found,
    # and had it been found it would install somewhere this user can't run.
    # This is exactly how eslint/prettier/stylelint silently failed to install
    # on a machine where `npm install -g` works perfectly by hand.
    cmd = ["npm", "install", "-g"] + pkg_spec.split()
    if not npm_prefix_writable():
        cmd = as_root(cmd)
    rc, out, err = run(cmd, timeout=300)
    if rc != 0 and not npm_prefix_writable():
        return False, (out + err + "\n[hint: npm's global prefix needs root here; "
                       "if sudo is unavailable, run `npm config set prefix ~/.npm-global` "
                       "and add ~/.npm-global/bin to PATH]")
    return rc == 0, (out + err)


def rustup_ready():
    return shutil.which("rustup") is not None

def ensure_rustup(assume_yes):
    if rustup_ready():
        return True
    if PKG_MANAGER == "apt-get":
        ok, _ = apt_install(["rustup"], assume_yes)
        if ok and shutil.which("rustup"):
            run(["rustup", "default", "stable"], timeout=300)
            return True
    print(yellow("    rustup not found and not apt-installable here. Install it yourself with:"))
    print(yellow("      curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"))
    return False

def install_rustup_component_clippy(assume_yes):
    if not ensure_rustup(assume_yes):
        return False, "rustup unavailable"
    rc, out, err = run(["rustup", "component", "add", "clippy"], timeout=180)
    return rc == 0, (out + err)

def install_rustup_component_rustfmt(assume_yes):
    if not ensure_rustup(assume_yes):
        return False, "rustup unavailable"
    rc, out, err = run(["rustup", "component", "add", "rustfmt"], timeout=180)
    return rc == 0, (out + err)


def install_golangci_lint(assume_yes):
    """Uses the official install.sh, targeting ~/.local/bin directly so this
    works even before a Go toolchain is on PATH (the project's own docs
    recommend install.sh over `go install` for reproducibility anyway)."""
    os.makedirs(LOCAL_BIN, exist_ok=True)
    script_url = "https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh"
    status_line("SCRIPT", "downloading golangci-lint install.sh ...")
    rc, out, err = run(["curl", "-sSfL", script_url], timeout=60)
    if rc != 0 or not out:
        return False, f"could not download install.sh: {err.strip()[:200]}"
    status_line("SCRIPT", f"installing golangci-lint into {LOCAL_BIN} ...")
    rc, out2, err2 = run(["sh", "-s", "--", "-b", LOCAL_BIN], input_text=out, timeout=180)
    if rc == 0 and not os.path.dirname(LOCAL_BIN) in os.environ.get("PATH", ""):
        print(yellow(f"    installed to {LOCAL_BIN} -- add this to PATH: export PATH=\"$PATH:{LOCAL_BIN}\""))
    return rc == 0, (out2 + err2)


def install_gitleaks_binary(assume_yes):
    """Tries `go install` first (if Go is present), otherwise downloads the
    matching release tarball straight from GitHub releases."""
    if shutil.which("go"):
        status_line("GO", "go install gitleaks ...")
        rc, out, err = run(["go", "install", "github.com/gitleaks/gitleaks/v8@latest"], timeout=300)
        if rc == 0:
            return True, out + err
    # fallback: grab a release binary
    import platform
    machine = platform.machine().lower()
    arch = "x64" if machine in ("x86_64", "amd64") else ("arm64" if "aarch64" in machine or "arm64" in machine else machine)
    os.makedirs(LOCAL_BIN, exist_ok=True)
    status_line("SCRIPT", "looking up latest gitleaks release ...")
    rc, out, err = run(["curl", "-sSfL", "https://api.github.com/repos/gitleaks/gitleaks/releases/latest"], timeout=30)
    if rc != 0:
        return False, "could not query GitHub releases API: " + err
    m = re.search(r'"tag_name":\s*"v([\d.]+)"', out)
    if not m:
        return False, "could not parse latest gitleaks version"
    ver = m.group(1)
    asset = f"gitleaks_{ver}_linux_{arch}.tar.gz"
    url = f"https://github.com/gitleaks/gitleaks/releases/download/v{ver}/{asset}"
    tmp_tar = "/tmp/gitleaks.tar.gz"
    rc, _, err = run(["curl", "-sSfL", "-o", tmp_tar, url], timeout=120)
    if rc != 0:
        return False, f"download failed for {url}: {err}"
    rc, _, err = run(["tar", "-xzf", tmp_tar, "-C", LOCAL_BIN, "gitleaks"], timeout=30)
    if rc == 0:
        os.chmod(os.path.join(LOCAL_BIN, "gitleaks"), 0o755)
        return True, f"installed to {LOCAL_BIN}/gitleaks"
    return False, f"could not extract gitleaks tarball: {err}"


SCRIPT_INSTALLERS = {
    "install_rustup_component_clippy": install_rustup_component_clippy,
    "install_rustup_component_rustfmt": install_rustup_component_rustfmt,
    "install_golangci_lint": install_golangci_lint,
    "install_gitleaks_binary": install_gitleaks_binary,
}


# ---------------------------------------------------------------------------
# per-tool orchestration
# ---------------------------------------------------------------------------

INSTALL_ROUTE_ATTRS = ("apt", "dnf", "pacman", "pip", "npm", "cargo_component",
                       "cargo_install", "go_install", "installer_script")


def is_builtin(tool: Tool) -> bool:
    """True for checks implemented inside this toolkit rather than fetched.

    Both conditions matter: no binary to probe for AND no way to obtain one. A
    tool missing only a check_cmd might still be installable, and one with only
    an install route is plainly external.
    """
    if tool.check_cmd:
        return False
    return not any(getattr(tool.install, a, None) for a in INSTALL_ROUTE_ATTRS)


def get_installed_version(tool: Tool):
    if not tool.check_cmd:
        return None, None
    binary = tool.check_cmd[0]
    path = shutil.which(binary)
    if not path:
        return None, None
    rc, out, err = run(tool.check_cmd, timeout=15)
    text = out + err
    if rc not in (0, 1):  # some tools (cargo clippy w/o component) exit non-zero cleanly
        return path, None
    m = re.search(tool.version_regex, text)
    return path, (m.group(1) if m else (text.strip().splitlines()[0] if text.strip() else "unknown"))


def try_install(tool: Tool, args):
    spec: InstallSpec = tool.install
    attempts = []

    if spec.apt and PKG_MANAGER == "apt-get":
        cand = apt_candidate_version(spec.apt)
        if cand:
            ok, msg = apt_install([spec.apt], args.yes)
            attempts.append(("apt", ok, msg))
            if ok:
                return "apt", cand, attempts

    if spec.installer_script:
        fn = SCRIPT_INSTALLERS.get(spec.installer_script)
        if fn:
            ok, msg = fn(args.yes)
            attempts.append((spec.installer_script, ok, msg))
            if ok:
                return spec.installer_script, None, attempts

    if spec.pip:
        if spec.prefer_pipx:
            ok, msg = pipx_install(spec.pip, args.yes)
            attempts.append(("pipx", ok, msg))
            if ok:
                return "pipx", None, attempts
        else:
            rc, out, err = run([sys.executable, "-m", "pip", "install", "--user",
                                 "--break-system-packages", spec.pip], timeout=180)
            attempts.append(("pip", rc == 0, out + err))
            if rc == 0:
                return "pip", None, attempts

    if spec.npm:
        ok, msg = npm_install_global(spec.npm, args.yes)
        attempts.append(("npm", ok, msg))
        if ok:
            return "npm", None, attempts

    if spec.cargo_component:
        ok, msg = (install_rustup_component_clippy(args.yes) if spec.cargo_component == "clippy"
                   else install_rustup_component_rustfmt(args.yes))
        attempts.append(("rustup component", ok, msg))
        if ok:
            return "rustup component", None, attempts

    if spec.cargo_install:
        if ensure_rustup(args.yes):
            rc, out, err = run(["cargo", "install", spec.cargo_install], timeout=600)
            attempts.append(("cargo install", rc == 0, out + err))
            if rc == 0:
                return "cargo install", None, attempts

    if spec.go_install:
        if shutil.which("go"):
            rc, out, err = run(["go", "install", f"{spec.go_install}@latest"], timeout=300)
            attempts.append(("go install", rc == 0, out + err))
            if rc == 0:
                return "go install", None, attempts

    return None, None, attempts


HEAVY_TOOL_IDS = {"cargo-audit", "semgrep-fast", "semgrep-deep", "gitleaks-worktree",
                   "gitleaks-history", "golangci-lint", "cargo-clippy", "cargo-fmt-check"}


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--check-only", action="store_true", help="report status only, install nothing")
    ap.add_argument("--yes", action="store_true", help="don't prompt before installing")
    ap.add_argument("--skip-heavy", action="store_true", help="skip slow/network-heavy installs")
    ap.add_argument("--only", type=str, default=None,
                     help="comma-separated language filter, e.g. cpp,python,go")
    args = ap.parse_args()

    lang_filter = set(x.strip() for x in args.only.split(",")) if args.only else None

    print(bold(f"\nCode Review Toolkit -- setup  (package manager: {PKG_MANAGER or 'none detected'})\n"))

    seen_ids = set()
    report = []
    for tool in TOOLS:
        if tool.id in seen_ids:
            continue
        seen_ids.add(tool.id)
        if tool.scope == "checklist":
            continue  # nothing to install
        if lang_filter and not (set(tool.languages) & lang_filter) and "*" not in tool.languages:
            continue
        if args.skip_heavy and tool.id in HEAVY_TOOL_IDS:
            status_line("SKIP", f"{tool.label} (--skip-heavy)")
            report.append({"id": tool.id, "status": "skipped"})
            continue

        path, version = get_installed_version(tool)
        if path:
            dupes = find_all_in_path(os.path.basename(path))
            if len(dupes) > 1:
                status_line("CONFLICT", f"{tool.label}: multiple copies on PATH -> using {path}")
                for d in dupes:
                    print(dim(f"             also found: {d}"))
            status_line("OK", f"{tool.label:<28} {version or ''}  ({path})")
            report.append({"id": tool.id, "status": "already_installed", "path": path, "version": version})
            continue

        # Tools that ship inside this toolkit (the heuristics.py heuristics)
        # have no binary to look for and no package to fetch. Without this they
        # fall through to try_install(), which finds no method and reports
        # FAILED -- alarming, and wrong: they were never missing.
        if is_builtin(tool):
            status_line("BUILTIN", f"{tool.label:<28} (ships with this toolkit -- nothing to install)")
            report.append({"id": tool.id, "status": "builtin"})
            continue

        if tool.scope == "manual" and not tool.check_cmd:
            report.append({"id": tool.id, "status": "n/a"})
            continue

        if args.check_only:
            status_line("MISSING", f"{tool.label} (not installed -- would attempt install)")
            report.append({"id": tool.id, "status": "missing"})
            continue

        method, version_installed, attempts = try_install(tool, args)
        if method:
            status_line("INSTALLED", f"{tool.label} via {method}")
            report.append({"id": tool.id, "status": "installed", "method": method})
        else:
            status_line("FAILED", f"{tool.label} -- no install method succeeded")
            if tool.install.manual_notes:
                print(yellow(f"             manual install: {tool.install.manual_notes}"))
            for method_name, ok, msg in attempts:
                if not ok:
                    print(dim(f"             tried {method_name}: {msg.strip()[:160]}"))
            report.append({"id": tool.id, "status": "failed", "attempts": [a[0] for a in attempts]})

    out_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".setup")
    os.makedirs(out_dir, exist_ok=True)
    report_path = os.path.join(out_dir, "install_report.json")
    with open(report_path, "w") as f:
        json.dump({"generated": datetime.now().isoformat(), "results": report}, f, indent=2)

    ok_count = sum(1 for r in report if r["status"] in ("already_installed", "installed"))
    print(bold(f"\nDone. {ok_count}/{len(report)} tools ready. Full report: {report_path}\n"))

    if shutil.which("ollama") is None:
        print(dim("Note: no local LLM runtime (ollama) detected on PATH. That's optional -- "
                   "code_review.py works fine with --llm-endpoint pointing at any OpenAI- or "
                   "Anthropic-compatible HTTP endpoint, local or remote, or with no LLM at all."))


if __name__ == "__main__":
    main()
