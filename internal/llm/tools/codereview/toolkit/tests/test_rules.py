#!/usr/bin/env python3
"""Rule docs: every vendored doc must be reachable, and a project's own rules
must be layered in without silently discarding the vendored ones.

Regression note: after the 2026-10-04 refresh twenty docs existed on disk that
nothing could ever return, because rules were looked up only by the languages
the analyser registry can classify. A Swift file got no guidance and no sign
that any was available.
"""
import os
import sys
import tempfile

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import rules  # noqa: E402  # pylint: disable=import-error,wrong-import-position


def check(label, got, want):
    ok = got == want
    print(f"  {'ok  ' if ok else 'FAIL'} {label}" + ("" if ok else f": got {got!r}, want {want!r}"))
    return ok


def main() -> int:
    good = True
    rules.set_project("")

    # 1. No doc on disk is unreachable.
    reachable = {d for d in rules.LANGUAGE_TO_DOC.values() if d}
    reachable |= set(rules.FILENAME_TO_DOC.values())
    reachable |= set(rules.EXTENSION_TO_DOC.values())
    reachable |= {"objc", "matlab", rules.DEFAULT_DOC}
    pattern_docs = {"mapper_dao_xml", "github_workflows", "github_config"}
    orphans = sorted(set(rules.available()) - reachable - pattern_docs)
    good &= check("no vendored doc is unreachable", orphans, [])

    # 2. Every mapping points at a doc that exists.
    dangling = sorted(d for d in reachable if d not in rules.available())
    good &= check("no mapping points at a missing doc", dangling, [])

    # 3. Files the registry cannot classify still get guidance.
    got = rules.rules_for_files(["a/b/Token.sol", "x/main.swift", "pom.xml", "README"])
    good &= check("by extension and by filename", sorted(got), ["pom_xml", "solidity", "swift"])
    good &= check("CI workflow by path", rules.doc_for_file(".github/workflows/ci.yml"), "github_workflows")
    good &= check("other .github config by path", rules.doc_for_file(r"repo\.github\dependabot.yml"), "github_config")
    good &= check("plain yaml elsewhere", rules.doc_for_file("deploy/values.yaml"), "yaml")
    good &= check("mapper xml by name", rules.doc_for_file("src/UserMapper.xml"), "mapper_dao_xml")

    with tempfile.TemporaryDirectory() as tmp:
        # 4. ".m" is decided by its first line.
        objc = os.path.join(tmp, "View.m")
        matlab = os.path.join(tmp, "solve.m")
        with open(objc, "w", encoding="utf-8") as f:
            f.write("\n// Copyright\n#import <UIKit/UIKit.h>\n")
        with open(matlab, "w", encoding="utf-8") as f:
            f.write("% solve the system\nx = A \\ b;\n")
        good &= check(".m with a C comment is Objective-C", rules.doc_for_file(objc), "objc")
        good &= check(".m with a % comment is MATLAB", rules.doc_for_file(matlab), "matlab")

        # 5. A project's own rule is ADDED to the vendored one.
        os.makedirs(os.path.join(tmp, rules.PROJECT_RULES_DIRNAME))
        with open(os.path.join(tmp, rules.PROJECT_RULES_DIRNAME, "c.md"), "w", encoding="utf-8") as f:
            f.write("- never call house_banned()\n")
        system_c = rules.for_language("c")
        rules.set_project(tmp)
        layered = rules.for_language("c")
        good &= check("project rule present", "house_banned" in layered, True)
        good &= check("vendored rule kept", system_c in layered, True)
        good &= check("source recorded", rules.sources().get("c"), "system+project")

        # 6. The replace marker replaces, and says so.
        with open(os.path.join(tmp, rules.PROJECT_RULES_DIRNAME, "go.md"), "w", encoding="utf-8") as f:
            f.write(rules.REPLACE_MARKER + "\n- only this\n")
        rules.set_project(tmp)
        good &= check("replace marker replaces", rules.for_language("go"), "- only this")
        good &= check("replace source recorded", rules.sources().get("go"), "project")

        # 7. A project doc for a language with no vendored doc still works.
        with open(os.path.join(tmp, rules.PROJECT_RULES_DIRNAME, "shell.md"), "w", encoding="utf-8") as f:
            f.write("- quote every expansion\n")
        rules.set_project(tmp)
        good &= check("project-only doc for a language with no vendored doc", rules.for_language("shell"), "- quote every expansion")

    rules.set_project("")
    good &= check("leaving the project restores the vendored doc", rules.for_language("c"), system_c)

    print("PASS" if good else "FAILED")
    return 0 if good else 1


if __name__ == "__main__":
    sys.exit(main())
