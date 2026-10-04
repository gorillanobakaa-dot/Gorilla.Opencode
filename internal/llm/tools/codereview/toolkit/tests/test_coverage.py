#!/usr/bin/env python3
"""Coverage: a tool that was only SCHEDULED must never be reported as having run.

Regression note: tools_ran used to be every tool with a job, including jobs that
ended 127 (not installed). On a machine with no analysers the report said 18 ran
and 17 never ran, sharing names, above zero findings. These cases keep the four
job states a partition and keep a language "reviewed" only when something for it
actually completed.
"""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import code_review as cr  # noqa: E402  # pylint: disable=import-error,wrong-import-position


def res(tool, path, rc, issues=0):
    job = cr.Job(tool_id=tool, target_path=path, argv=[], log_path="", cwd="")
    return cr.JobResult(job=job, returncode=rc, duration=0.0, issue_count=issues)


def check(label, got, want):
    ok = got == want
    print(f"  {'ok  ' if ok else 'FAIL'} {label}" + ("" if ok else f": got {got!r}, want {want!r}"))
    return ok


def main() -> int:
    good = True

    # 1. The reported bug: nothing installed.
    files = ["a.py", "b.go"]
    rs = [res("bandit", "a.py", 127), res("flake8", "a.py", 127), res("gosec", "b.go", 127)]
    c = cr.coverage_of(files, rs)
    good &= check("nothing installed: no tool completed", c["tools_completed"], [])
    good &= check("nothing installed: terminal state", c["terminal_state"], "nothing-ran")
    good &= check("nothing installed: python unreviewed", c["languages"]["python"]["state"], "unreviewed")
    good &= check("nothing installed: go unreviewed", c["languages"]["go"]["state"], "unreviewed")

    # 2. Ran and missing are disjoint, and states partition the jobs.
    rs = [res("flake8", "a.py", 0), res("bandit", "a.py", 127), res("gosec", "b.go", 124),
          res("pylint", "a.py", 3, issues=0), res("mypy", "a.py", 1, issues=2)]
    c = cr.coverage_of(files, rs)
    good &= check("mixed: completed tools", c["tools_completed"], ["flake8", "mypy"])
    good &= check("mixed: never completed", c["tools_never_completed"], ["bandit", "gosec", "pylint"])
    good &= check("mixed: no overlap",
                  set(c["tools_completed"]) & set(c["tools_never_completed"]), set())
    good &= check("mixed: states sum to planned", sum(c["jobs"].values()), c["jobs_planned"])
    good &= check("mixed: jobs by state", c["jobs"],
                  {"completed": 2, "missing": 1, "errored": 1, "timed_out": 1})
    good &= check("mixed: python reviewed", c["languages"]["python"]["state"], "reviewed")
    good &= check("mixed: go unreviewed", c["languages"]["go"]["state"], "unreviewed")
    good &= check("mixed: terminal partial", c["terminal_state"], "partial")
    good &= check("mixed: unreviewed list", c["languages_unreviewed"], ["go"])

    # 3. A tool with one completed job and one timeout still counts as having run.
    rs = [res("flake8", "a.py", 0), res("flake8", "c.py", 124)]
    c = cr.coverage_of(["a.py", "c.py"], rs)
    good &= check("partial tool: counted as completed", c["tools_completed"], ["flake8"])
    good &= check("partial tool: timeout still counted", c["jobs"]["timed_out"], 1)

    # 4. A language nothing was scheduled for is not silently "reviewed".
    c = cr.coverage_of(["a.py", "x.rs"], [res("flake8", "a.py", 0)])
    good &= check("no analyser scheduled", c["languages"]["rust"]["state"], "no-analyser")
    good &= check("no analyser: partial", c["terminal_state"], "partial")

    # 5. Everything completed.
    c = cr.coverage_of(["a.py"], [res("flake8", "a.py", 0), res("bandit", "a.py", 1, issues=3)])
    good &= check("all completed: complete", c["terminal_state"], "complete")

    # 6. No jobs at all.
    c = cr.coverage_of(["a.py"], [])
    good &= check("no jobs: nothing-ran", c["terminal_state"], "nothing-ran")

    print("PASS" if good else "FAILED")
    return 0 if good else 1


if __name__ == "__main__":
    sys.exit(main())
