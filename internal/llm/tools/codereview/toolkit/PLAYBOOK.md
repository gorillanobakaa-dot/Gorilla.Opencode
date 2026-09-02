# PLAYBOOK -- how this whole thing actually works, step by step

This document assumes nothing. It is written so that a small, low-reasoning
model (or a person doing this by hand in a terminal) can follow it literally:
run this exact command, save the output to this exact file, then look for
these exact words in it. `code_review.py` already automates almost all of
this -- read this file if you want to understand *what* it's doing under the
hood, run a piece of it by hand, or hand these exact instructions to a
separate small/local model as its own step-by-step task list.

---

## 0. One-time setup

Step 1 -- open a terminal on your Debian 13 machine.

Step 2 -- go into the toolkit folder:
```bash
cd /path/to/code_review_toolkit
```

Step 3 -- run the installer. It checks what you already have (with real
version numbers) before installing anything, and never reinstalls something
that's already there:
```bash
python3 install_tools.py
```

Step 4 -- read the last few lines it prints. It will tell you how many of
the ~38 tools are ready and where the full report was saved
(`.setup/install_report.json`). Anything it couldn't install itself, it
prints the manual command for.

Step 5 (optional) -- if you want to skip the slow ones for now (semgrep,
cargo-audit, gitleaks, golangci-lint, cargo clippy/fmt):
```bash
python3 install_tools.py --skip-heavy
```

You can re-run `install_tools.py` any time -- it's safe, it just checks and
fills in gaps.

---

## 1. The three stages, in plain terms

- **Stage 0 (recon):** just counts lines of code per language and scans for
  obvious secrets in the files as they sit right now. Takes seconds. Always
  runs first so you have a lay of the land.
- **Stage 1 (fast):** linters and formatters -- pylint, flake8, eslint,
  stylelint, clang-format, cargo fmt/clippy, go vet, golangci-lint,
  shellcheck, semgrep's narrow "p/ci" ruleset. Seconds to low minutes per
  file.
- **Stage 2 (standard):** static analysis and security-focused tools --
  clang-tidy (if you gave it a compile database), mypy, bandit, gosec,
  staticcheck, cargo-audit.
- **Stage 3 (deep, conditional):** ONLY runs automatically if stage 1/2
  output contained something that looks security-shaped (the words
  "overflow", "injection", "hardcoded", "CWE-", "secret", etc. -- see
  `ESCALATION_KEYWORDS` in `tools_registry.py`). When triggered, it re-runs
  the relevant tools in their strictest mode (`bandit -iii -lll`, full
  `semgrep p/security-audit`, `cppcheck --enable=all --inconclusive`, a full
  git-history secrets scan) but ONLY on the file(s) that triggered it --
  not your whole tree. Pass `--deep` to force this on everything regardless.

Every single tool's raw, complete output is saved to its own file in the
results folder (`<target>/.code_review/<timestamp>/<tool>__<file>.txt`).
Nothing is ever thrown away or summarized-then-discarded -- the summary
report always points back at the exact log it came from.

---

## 2. Quick-start recipes for your four projects

### A. Your Go project (Gorilla.Opencode)
```bash
cd /path/to/code_review_toolkit
python3 code_review.py ~/src/gorilla-opencode
```
That's it. `go.mod` is detected automatically -> profile `go` -> golangci-lint,
go vet, staticcheck, gosec, plus semgrep/gitleaks/shellcheck for anything
non-Go in the repo (scripts, Dockerfiles, etc.).

To review just what you changed since your last commit:
```bash
python3 code_review.py ~/src/gorilla-opencode --diff HEAD~1
```

### B. Chroma (vector DB, mostly Python + some Rust bindings)
```bash
python3 code_review.py ~/src/chroma --diff origin/main
```
Detected as profile `python` (or `rust` if `Cargo.toml` is at the root) ->
pylint/flake8/mypy/bandit/black/isort run on the Python side, cargo
clippy/fmt/audit run automatically too if there's a Cargo.toml anywhere
langs are detected.

### C. Firefox (mozilla-central checkout, building from source)
Firefox ships its own linting/static-analysis entry points that already
know the tree's conventions -- always prefer these over generic tools for
JS/C++ style:

```bash
# fast lint pass (wraps eslint, flake8, clang-format, etc. -- tree-aware)
cd ~/src/firefox
./mach lint path/to/the/file/you/patched.cpp
# or, for everything you've changed but not yet pushed:
./mach lint --outgoing
```

```bash
# clang-tidy with Firefox-specific checks (auto-downloads its own clang-tidy
# on first run into .mozbuild -- expect a pause the first time)
./mach static-analysis check --checks="-*, google-readability-braces-around-statements" --fix path/to/file.cpp
# or just the defaults on the whole file:
./mach static-analysis check path/to/file.cpp
```

Then, separately, run the generic pipeline for anything mach doesn't cover
(Rust components, Python build scripts, shell scripts, secrets):
```bash
python3 code_review.py ~/src/firefox --diff HEAD~1
```
(`code_review.py` detects the `firefox` profile automatically from
`mach` + `python/mozbuild` and skips eslint/prettier/cpplint itself, since
`./mach lint` already covers that ground better.)

### D. Your custom Linux kernel (compiling from source)
The kernel has its own canonical tools -- run these directly, in this order:

**Step 1 -- checkpatch (always run this first, on every patch):**
```bash
cd ~/src/linux
scripts/checkpatch.pl --strict --codespell -f drivers/your/patched_file.c
# or, if you have an actual patch file:
scripts/checkpatch.pl --strict --codespell your_change.patch
```

**Step 2 -- sparse (semantic checker, catches locking/address-space bugs):**
```bash
make C=2 drivers/your/subsystem/
```

**Step 3 -- clang-tidy, if you've built with clang at least once:**
```bash
make CC=clang clang-tidy
# requires CONFIG_CC_IS_CLANG=y in your .config -- build once with CC=clang first if not
```

**Step 4 -- Coccinelle semantic patches (slower, run when you have time):**
```bash
make coccicheck MODE=report
```

Then run the generic pipeline for anything outside the kernel's own tooling
(Python scripts under `scripts/` and `tools/`, shell scripts, secrets):
```bash
python3 code_review.py ~/src/linux --profile linux-kernel --diff HEAD~1
```

---

## 3. Manual-tier tools, spelled out literally

These need project-specific build state the script can't safely fabricate,
so it never runs them for you -- it just prints the exact command. Here's
what that looks like for the two most common ones:

### valgrind (memory errors in a compiled binary)
Step 1 -- build your program normally, e.g. `gcc -g -O0 -o myprog myprog.c`.
Step 2 -- run:
```bash
valgrind --leak-check=full --show-leak-kinds=all --track-origins=yes \
  --log-file=valgrind_out.txt ./myprog [your program's normal arguments]
```
Step 3 -- open `valgrind_out.txt`. Look for "Invalid read", "Invalid write",
"definitely lost", or "Conditional jump depends on uninitialised value".

### scan-build (clang static analyzer over a real build)
Step 1 -- run your normal build command prefixed with `scan-build`:
```bash
scan-build -o scan-build-results make -j$(nproc)
```
Step 2 -- when it finishes, scan-build prints a path to an HTML report
under `scan-build-results/`. Open `index.html` in a browser.

---

## 4. Pointing this at an LLM (any vendor, or none)

`code_review.py` never calls any cloud API unless you pass `--llm-endpoint`.
Three ways to use it:

**A local model via Ollama:**
```bash
ollama pull llama3        # one-time
ollama serve               # if not already running
python3 code_review.py ~/src/myproj \
  --llm-endpoint http://localhost:11434/v1/chat/completions \
  --llm-model llama3 --llm-api-style openai
```

**A local model via llama.cpp's server (or LM Studio, vLLM, etc. -- anything
that speaks the OpenAI chat-completions schema):**
```bash
python3 code_review.py ~/src/myproj \
  --llm-endpoint http://localhost:8080/v1/chat/completions \
  --llm-model whatever-your-server-calls-it --llm-api-style openai
```

**Any cloud provider, via an API key you supply yourself in an env var of
your choosing (the script never hardcodes which provider or reads a key you
didn't name):**
```bash
export MY_KEY=sk-...
python3 code_review.py ~/src/myproj \
  --llm-endpoint https://api.example-provider.com/v1/chat/completions \
  --llm-model their-model-name --llm-api-style openai --llm-api-key-env MY_KEY
```

**No LLM at all** -- just omit `--llm-endpoint`. You still get the full
report, every raw log, and the manual-step instructions.

---

## 5. Troubleshooting

- **"externally-managed-environment" pip error:** Debian 13 blocks bare
  `pip install` on the system Python (PEP 668). `install_tools.py` already
  routes around this with `pipx` automatically -- you shouldn't hit this
  unless you run pip yourself directly.
- **A tool installed via `pipx`/`go install`/`cargo install` isn't found:**
  make sure `~/.local/bin` (pipx), `$(go env GOPATH)/bin` (go install), and
  `~/.cargo/bin` (cargo install) are on your `PATH`. Add to your shell rc:
  ```bash
  export PATH="$PATH:$HOME/.local/bin:$HOME/go/bin:$HOME/.cargo/bin"
  ```
- **`CONFLICT` warnings from install_tools.py:** it found the same tool name
  in more than one PATH directory (e.g. an apt copy and a pipx copy). It
  tells you which one will actually run. On a merged-`/usr` system, seeing
  both `/usr/bin/x` and `/bin/x` is just a symlink and harmless.
- **A tool shows `ERROR` (not `CLEAN`, not `ISSUES`) in the rolling output:**
  it exited with a failure code and we couldn't parse any findings from its
  output -- do NOT treat this as "nothing wrong". Open its log file. The
  most common cause is a blocked network call (semgrep/cargo-audit need to
  reach their rule/advisory registries).
- **Firefox/kernel builds are enormous** -- pointing `code_review.py` at the
  repo root without `--diff` will, by default, cap at `--max-files 2000`
  (most-recently-modified first) rather than trying to lint every file in a
  700,000-file tree in one run. Use `--diff <ref>` for real patch review.
