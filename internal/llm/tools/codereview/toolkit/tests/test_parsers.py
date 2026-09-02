#!/usr/bin/env python3
"""Feed each parser real-world tool output and assert it extracts the right thing."""
import os
import sys
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import findings as f  # pylint: disable=import-error,wrong-import-position

CASES = []
def case(name, parser, text, expect):
    CASES.append((name, parser, text, expect))

# --- cppcheck --template=gcc
case("cppcheck", f.parse_gcc_style,
"""src/foo.c:42:5: warning: Uninitialized variable: buf [uninitvar]
src/foo.c:88:1: error: Memory leak: p [memleak]
""",
[("src/foo.c", 42, "warning", "uninitvar"), ("src/foo.c", 88, "error", "memleak")])

# --- shellcheck -f gcc
case("shellcheck", f.parse_gcc_style,
"""deploy.sh:12:6: warning: Quote this to prevent word splitting. [SC2046]
""",
[("deploy.sh", 12, "warning", "SC2046")])

# --- flake8 (code leads the message, no severity word)
# E501 is pycodestyle formatting -> style. F401 is pyflakes -> error. E902 is a
# real syntax/IO error despite the E. This ordering is the whole point.
case("flake8", f.parse_gcc_style,
"""app.py:1:80: E501 line too long (92 > 79 characters)
app.py:9:1: F401 'os' imported but unused
app.py:3:1: E902 TokenizeError: EOF in multi-line statement
app.py:5:1: C901 'run' is too complex (14)
""",
[("app.py", 1, "style", "E501"), ("app.py", 9, "error", "F401"),
 ("app.py", 3, "error", "E902"), ("app.py", 5, "warning", "C901")])

# --- mypy
case("mypy", f.parse_gcc_style,
"""lib/x.py:23: error: Incompatible return value type (got "int", expected "str")  [return-value]
""",
[("lib/x.py", 23, "error", "return-value")])

# --- clang-tidy
case("clang-tidy", f.parse_gcc_style,
"""/abs/src/a.cpp:17:9: warning: use nullptr [modernize-use-nullptr]
""",
[("/abs/src/a.cpp", 17, "warning", "modernize-use-nullptr")])

# --- golangci-lint
case("golangci-lint", f.parse_gcc_style,
"""main.go:31:2: ineffectual assignment to err (ineffassign)
""",
[("main.go", 31, "warning", "ineffassign")])

# --- pylint --output-format=parseable
case("pylint", f.parse_pylint,
"""app.py:12: [C0116(missing-function-docstring), do_thing] Missing function docstring
app.py:40: [E1101(no-member), Foo.go] Instance of 'Foo' has no 'bar' member
""",
[("app.py", 12, "style", "missing-function-docstring"),
 ("app.py", 40, "error", "no-member")])

# --- cpplint
case("cpplint", f.parse_cpplint,
"""src/w.cc:88:  Missing space before {  [whitespace/braces] [5]
src/w.cc:90:  Lines should be <= 80 characters long  [whitespace/line_length] [2]
""",
[("src/w.cc", 88, "warning", "whitespace/braces"),
 ("src/w.cc", 90, "style", "whitespace/line_length")])

# --- bandit -f txt (block format, severity + CWE on separate lines)
case("bandit", f.parse_bandit,
""">> Issue: [B602:subprocess_popen_with_shell_equals_true] subprocess call with shell=True identified.
   Severity: High   Confidence: High
   CWE: CWE-78 (https://cwe.mitre.org/data/definitions/78.html)
   Location: ./run.py:55:4
   More Info: https://bandit.readthedocs.io/
>> Issue: [B101:assert_used] Use of assert detected.
   Severity: Low   Confidence: High
   CWE: CWE-703 (https://cwe.mitre.org/data/definitions/703.html)
   Location: ./t.py:3:0
""",
# NB: paths normalise, "./run.py" -> "run.py" -- that's _rel() doing its job.
[("run.py", 55, "error", "subprocess_popen_with_shell_equals_true (CWE-78)"),
 ("t.py", 3, "info", "assert_used (CWE-703)")])

# --- gitleaks -v  (MUST NOT leak the secret into the finding)
# FIXTURE_SECRET is assembled at runtime rather than written literally: gitleaks
# scans this repo too, and a test fixture that trips it on every run is how you
# train yourself to ignore a secrets scanner. The parser sees the same string.
FIXTURE_SECRET = "AKIA" + "IOSFODNN7EXAMPLE"
case("gitleaks", f.parse_gitleaks,
f"""Finding:     api_key = "{FIXTURE_SECRET}"
Secret:      {FIXTURE_SECRET}
RuleID:      aws-access-token
Entropy:     3.234
File:        config/settings.py
Line:        14
Commit:      a1b2c3d4e5f6a7b8
""",
[("config/settings.py", 14, "error", "aws-access-token")])

# --- cargo clippy (severity precedes the location line)
case("clippy", f.parse_rust,
"""warning: unused variable: `x`
 --> src/main.rs:4:9
  |
4 |     let x = 5;
  |         ^
error[E0308]: mismatched types
 --> src/lib.rs:20:14
""",
[("src/main.rs", 4, "warning", ""), ("src/lib.rs", 20, "error", "E0308")])

# --- gosec
case("gosec", f.parse_gosec,
"""[/home/u/p/crypto.go:31] - G401 (CWE-326): Use of weak cryptographic primitive (Confidence: High, Severity: Medium)
""",
[("/home/u/p/crypto.go", 31, "warning", "G401 (CWE-326)")])

# --- semgrep --json
case("semgrep", f.parse_semgrep,
"""{"results":[{"check_id":"python.lang.security.audit.exec-detected",
"path":"h.py","start":{"line":7,"col":1},"end":{"line":7,"col":20},
"extra":{"severity":"ERROR","message":"exec() detected","metadata":{"cwe":["CWE-95: Eval Injection"]}}}],"errors":[]}""",
[("h.py", 7, "error", "python.lang.security.audit.exec-detected (CWE-95)")])

# --- black
case("black", f.parse_format_check,
"""would reformat /home/u/p/app.py
Oh no! 1 file would be reformatted.
""",
[("target.py", None, "style", "unformatted")])


def run():
    failures = 0
    for name, parser, text, expect in CASES:
        ctx = {"tool_id": name, "cwd": "", "target_path": "target.py",
               "severity_markers": ["error", "warning"]}
        got = f.finalize(parser(text, ctx))
        actual = [(x.file, x.line, x.severity, x.rule_id) for x in got]
        exp = sorted(expect, key=lambda t: (f.SEVERITY_RANK.get(t[2], 9), t[0], t[1] or 0))
        if actual != exp:
            failures += 1
            print(f"FAIL {name}\n  expected {exp}\n  actual   {actual}")
        else:
            print(f"ok   {name:<14} {len(got)} finding(s)")

    # gitleaks must never carry the secret anywhere in the record
    ctx = {"tool_id": "gitleaks", "cwd": "", "target_path": "", "severity_markers": []}
    g = f.parse_gitleaks(CASES[9][2], ctx)
    blob = " ".join(x.message + x.raw + x.snippet for x in g)
    if FIXTURE_SECRET in blob:
        failures += 1
        print("FAIL gitleaks LEAKED THE SECRET into the finding record")
    else:
        print("ok   gitleaks       secret withheld from record")

    # corroboration: same file:line from two different tools groups; same tool twice does not
    fs = [f.Finding(tool="cppcheck", file="a.c", line=10, message="leak"),
          f.Finding(tool="clang-tidy", file="a.c", line=10, message="leak here"),
          f.Finding(tool="cppcheck", file="b.c", line=5, message="x"),
          f.Finding(tool="cppcheck", file="b.c", line=5, message="y")]
    groups = f.corroborated(f.finalize(fs))
    if len(groups) == 1 and groups[0][0].file == "a.c":
        print("ok   corroborated   1 group (a.c:10), same-tool dupes correctly ignored")
    else:
        failures += 1
        print(f"FAIL corroborated  got {[(g[0].file, sorted({x.tool for x in g})) for g in groups]}")

    print(f"\n{len(CASES)+2} checks, {failures} failure(s)")
    return failures


if __name__ == "__main__":
    sys.exit(1 if run() else 0)
