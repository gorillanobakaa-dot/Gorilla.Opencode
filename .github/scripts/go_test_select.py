"""Which Go packages CI tests (mirrors Fieldkit's .github/ci_tests.py, R7).

    python go_test_select.py BASE            the packages touched by BASE...HEAD plus their dependents
    python go_test_select.py full            the whole suite (push to main, tags, a manual run)
    options: --tags T   build tags passed to `go list`;  --github-output  also write mode=/packages= to $GITHUB_OUTPUT

Prints one line, "mode=all", "mode=none" or "mode=some", then (some) one package per line.
Fail safe: go.mod, go.sum, go.work, a workflow, the CI scripts, a .go file in no known package (a deleted
package, a generated file), or a diff git cannot read selects everything. Anything else outside a package
(documentation, release notes, packaging) selects nothing. A package is touched when a changed file is in its
directory tree (its own, not a nested package's) or is one of its embedded files; dependents are found through
Imports, TestImports and XTestImports (the reverse closure).
"""
import json
import os
import subprocess
import sys

ALL_ROOT = {"go.mod", "go.sum", "go.work", "go.work.sum"}
ALL_PREFIX = (".github/workflows/", ".github/scripts/", ".github/actions/")


def run(cmd):
    r = subprocess.run(cmd, capture_output=True, text=True, encoding="utf-8")
    if r.returncode != 0:
        raise RuntimeError(f"{' '.join(cmd)}: {r.stderr.strip()}")
    return r.stdout


def changed_files(base):
    out = run(["git", "diff", "--name-only", f"{base}...HEAD"])
    return [p.strip() for p in out.splitlines() if p.strip()]


def list_packages(tags=""):
    cmd = ["go", "list", "-e", "-json"]
    if tags:
        cmd += ["-tags", tags]
    dec, out, pkgs, i = json.JSONDecoder(), run(cmd + ["./..."]), [], 0
    while i < len(out):
        while i < len(out) and out[i].isspace():
            i += 1
        if i >= len(out):
            break
        obj, i = dec.raw_decode(out, i)
        pkgs.append(obj)
    return pkgs


def select(files, pkgs, root):
    """-> (mode, packages) with mode in all|none|some."""
    root = os.path.normcase(os.path.abspath(root))

    def rel(p):
        return os.path.relpath(os.path.normcase(os.path.abspath(p)), root).replace("\\", "/")

    dirs = {rel(p["Dir"]): p["ImportPath"] for p in pkgs}
    embeds = {}
    for p in pkgs:
        for k in ("EmbedFiles", "TestEmbedFiles", "XTestEmbedFiles"):
            for f in p.get(k) or []:
                embeds.setdefault(rel(os.path.join(p["Dir"], f)), set()).add(p["ImportPath"])
    touched = set()
    for f in files:
        f = f.replace("\\", "/")
        if f in ALL_ROOT or f.startswith(ALL_PREFIX):
            return "all", []
        touched |= embeds.get(f, set())
        d, hit = os.path.dirname(f), None
        while d:                                    # the root package takes only its own .go files
            if d in dirs:
                hit = dirs[d]
                break
            d = os.path.dirname(d)
        if not d and "." in dirs and f.endswith(".go") and "/" not in f:
            hit = dirs["."]
        if hit:
            touched.add(hit)
        elif f.endswith(".go") and f not in embeds:
            return "all", []
    if not touched:
        return "none", []
    users = {}
    for p in pkgs:
        for k in ("Imports", "TestImports", "XTestImports"):
            for imp in p.get(k) or []:
                users.setdefault(imp, set()).add(p["ImportPath"])
    seen, todo = set(touched), list(touched)
    while todo:
        for u in users.get(todo.pop(), ()):
            if u not in seen:
                seen.add(u)
                todo.append(u)
    return "some", sorted(seen)


def main(argv):
    args = [a for a in argv if not a.startswith("--")]
    tags = ""
    if "--tags" in argv:
        tags = argv[argv.index("--tags") + 1]
        args = [a for a in args if a != tags]
    base = args[0] if args else "full"
    try:
        if base == "full" or set(base) == {"0"}:
            mode, pkgs = "all", []
        else:
            mode, pkgs = select(changed_files(base), list_packages(tags), os.getcwd())
            if mode == "some":
                mod = run(["go", "list", "-m"]).strip()
                pkgs = [p if p != mod else "." for p in pkgs]
                pkgs = ["./" + p[len(mod) + 1:] if p.startswith(mod + "/") else p for p in pkgs]
    except (RuntimeError, OSError, ValueError) as e:
        print(f"go-test-select: {e}: the whole suite", file=sys.stderr)
        mode, pkgs = "all", []
    print("mode=" + mode)
    for p in pkgs:
        print(p)
    out = os.environ.get("GITHUB_OUTPUT")
    if "--github-output" in argv and out:
        with open(out, "a", encoding="utf-8") as fh:
            fh.write(f"mode={mode}\npackages={' '.join(pkgs)}\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
