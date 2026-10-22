# Looking inside programs and archives — dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers phase 1 of the binary and forensics toolkit (2026-10-21, after v0.1.149): two new tools the AI can use, `binary_inspect` and `archive`. Not released yet; a local commit only.

---

# Track one: plain language

## What happened

On Linux the AI could look inside a program file or a zip file by running small programs that Linux always has. Windows and the old MacBook Air (macOS 10.13) do not have those programs, so there the AI was half blind: asked "will this program run on my Mac?" or "what is inside this Firefox file?", it could only guess.

Two tools are now built into Gorilla OpenCode itself, so they work the same on every computer, with nothing to install:

- **binary_inspect** looks inside a program or library (Windows `.exe`/`.dll`, Linux, Mac) without running it: what processor it is for, which other files it needs, whether it is signed, which parts look packed or encrypted, readable text inside it, and for Mac programs the oldest macOS they say they need.
- **archive** lists what is inside a zip, jar, Firefox `omni.ja`, tar, `.tar.gz`, `.tar.bz2`, `.gz` or `.bz2` file, searches it, and takes ONE file out of it.

## Honest state of play

Tried on real files on this laptop, not only on test files:

- Firefox 157's `omni.ja` (71 MB, 5,839 files): listed and searched in 5 thousandths of a second.
- Firefox's `xul.dll` (180 MB): all readable text found and searched in under 2 seconds.
- The Mac package of Gorilla OpenCode 0.1.149-dev on the USB stick: opened, the program taken out, and inspected.

That last test found something worth knowing: the Mac program says inside it "needs macOS 11.0", yet it runs on the Air, which has macOS 10.13. The reason: that note is written in a format macOS 10.13 is too old to read, so 10.13 ignores it. The tool therefore says **UNPROVEN** in that case, not "no", and says that only a real run on the Air settles it.

Not tried: the tools inside a conversation with a real AI model. They were tested directly, the way the program calls them.

## Worst case if something is wrong

`binary_inspect` only reads. The worst it can do is report something wrong, for example a wrong "OK" for the Mac check. The Mac check reads only the program's own label; a program can also fail because a library it needs is missing on 10.13, which the label does not show.

`archive` writes only when asked to take a file out, and then:
- only into your project folder (by default a folder called `extracted`),
- only after it asks you, like every tool that writes,
- never over a file that is already there,
- never for names that try to climb out of the folder (like `../../Windows/...`), never for links, never for files over 256 MB.

## What changed for you

**"Will this run on my Mac?"**
- Before: on Windows or the Mac, the AI had no tool for this and guessed.
- After: `binary_inspect` answers from the program's own label, slice by slice, with OK, NO, UNPROVEN or UNKNOWN.

**"What is inside omni.ja?"**
- Before: depended on a zip program being installed; Windows has none that the AI can use reliably.
- After: `archive` lists, searches (`*.js`, or any word) and takes one file out.

**Cost of every message**
- Before: tool descriptions sent with every message cost about 10,204 tokens.
- After: about 10,562 (+358, about 3% more of what a small local model reads per message). Both tools can be switched off in `/context`. With "Deferred tool loading" switched on, they are not sent at all until the AI searches for them.

## What you can do now

- Ask "what does this .exe need to run?" or "is this Mac program for Intel?" and point at the file.
- Ask "find the preferences file in omni.ja and show it": the AI lists, finds, takes it out (it asks you first), and reads it.
- Switch either tool off in `/context` if you never need it; on a slow connection the "low bandwidth" choice switches both off.

## What is still missing

- The `.xz`, `.zst`, `.7z` and `.rar` formats: the tool says it cannot open them and that an external tool is needed.
- Comparing two builds ("what changed"), reading GTK theme resources, video checks: later phases.
- Checking that a signature is genuine: the tool only says whether there is one.

## How to check that the new parts work on your computer

1. In a conversation, ask: "use binary_inspect on" followed by the full path of any `.exe`.
2. You should see the processor type, the list of DLLs it uses, and a table of sections.
3. Ask: "use archive to list" followed by the path of any `.zip` file.

## Should you be concerned?

No new connection was added. Nothing is downloaded or installed, no other program is started, and nothing is sent anywhere except what the AI already sees. Text found inside a program is marked as "untrusted", because whoever made the program could have written instructions aimed at the AI.

## Glossary

- **Entropy**: how random the bytes in a part of a file look, from 0 to 8. Above about 7.2 usually means compressed, packed or encrypted.
- **Fat (universal) Mac program**: one file holding versions for several processors, for example Intel and Apple Silicon.
- **omni.ja**: the zip file in which Firefox keeps most of its own scripts and settings.

---

# Track two: developer

## Problem Being Solved

The harness depended on binutils, `file`, `strings` and `unzip` for executable and archive inspection. The Windows laptop has some of them only through scoop gcc, none can read Mach-O, and a fresh macOS 10.13 has none of them on PATH (PLAN.md, "2026-10-21: binary and forensics toolkit", inventory table). The model had no deterministic way to answer "which arch / which deps / which minimum macOS" or to open an `omni.ja`.

## Approach Taken

New package `internal/forensics`, standard library only: `debug/elf`, `debug/pe`, `debug/macho` (incl. `NewFatFile`), `archive/zip`, `archive/tar`, `compress/gzip`, `compress/bzip2`, `crypto/sha256`. No cgo, no new module, no process, no network. Two thin tools in `internal/llm/tools/forensics.go`. Every list is capped (sections 40, libraries 40, symbols/imports 150, strings 150 × 160 chars, archive listing 200, find 100) and every cap prints a line saying so and how to narrow.

## Before

No tool. `bash` + whatever binaries the machine had.

## After

- `binary_inspect {path, part=summary|sections|imports|symbols|strings, filter, min_len}`.
  - ELF: class, endianness, `e_type`, machine, OS/ABI, entry, `PT_INTERP`, `DT_NEEDED`, `DT_RPATH`/`DT_RUNPATH`, `.symtab`/dynamic symbol counts, sections with flags and entropy.
  - PE: PE32/PE32+, EXE/DLL, machine, subsystem, entry RVA, image base, link timestamp, imports grouped by DLL (from `ImportedSymbols`; `ImportedLibraries` returns nothing in debug/pe), delay-load directory presence, Authenticode directory presence (not verified), CLR header presence.
  - Mach-O thin and fat: CPU, filetype, PIE/two-level/executable-stack flags, `LC_VERSION_MIN_MACOSX` (0x24) or `LC_BUILD_VERSION` (0x32, platform 1) decoded from raw load commands, `LC_CODE_SIGNATURE` presence, dylibs, `LC_RPATH`, sections; per slice a 10.13.6 verdict.
  - strings: one pass, ASCII runs plus UTF-16LE runs of ASCII characters in both byte phases, scan capped at 256 MiB, returned through `NewUntrustedTextResponse`.
- `archive {path, action=list|find|extract, pattern, entry, dest}`. Format by magic bytes, never by extension. Gzip/bzip2 streams are peeked for `ustar` at offset 257 and walked as tar. Decompression is capped at 2 GiB per walk (`capReader`) and the cap is reported. Extract: one regular file, `O_CREATE|O_EXCL`, ≤ 256 MiB (declared and actual), SHA-256 of what was written.

## Known Alternatives

- Shelling out to binutils/llvm-objdump/7z: not present on the target machines, and a process per call.
- `github.com/saferwall/pe`, `lief` bindings: richer PE parsing (exports, resources, Authenticode verification) at the cost of a dependency or cgo.
- YARA/capa: cgo or external binaries; phase 3 as pinned cards.

## Files Changed

- `internal/forensics/` (new): `binary.go`, `strings.go`, `archive.go`, `binary_test.go`, `archive_test.go`, `testdata/` (fixtures, `README.md` with build commands, `mutations.yaml`, `.gitattributes`).
- `internal/llm/tools/forensics.go`, `forensics_test.go` (new).
- `internal/llm/tools/toolsearch.go`: both tools deferrable.
- `internal/llm/agent/tools.go`: registered in `coderToolSet`; `loadoutIDForTool`, `deferredToolNames`.
- `internal/llm/agent/calibrate.go`: both schemas measured.
- `internal/config/loadout.go`: rows `tool.binary_inspect`, `tool.archive` (default ON), both in `lowBandwidthOff`.
- `internal/llm/agent/schema_cost_test.go`, `docs/WHAT-A-CURIOUS-USER-COSTS.md`: 10,204 → 10,562.

## Decisions Made

- **Default ON, deferrable.** +358 tokens per turn (binary_inspect 180, archive 178, measured by `TestDefaultToolSchemaCostIsCounted`). The tools exist for machines that have nothing else; off by default they would be found only by someone who already knows. With `tool_search` on they cost one catalogue line each. Switched off by the low-bandwidth preset.
- **No custom reader for Mozilla's optimized jar.** mozpack `JarWriter(optimize=True)` writes `[u32 preload][central directory][end record, cdir_offset=4][local entries][end record]`. Go's `archive/zip.readDirectoryEnd` (verified in the go1.24.13 source) computes a non-zero base offset from the trailing end record, then, finding a valid directory header at `cdir_offset` itself, resets it to 0. `TestOmniJaFixtureIsTheOptimizedLayout` asserts the layout and the native open; `TestLargeOptimizedJarStillOpens` repeats it at 200 KiB, past the ~64 KiB end-record search window. Firefox 157's own `omni.ja` is NOT in this layout (local header first, central directory at the end; checked on the 2026-10-10 build) and opens as a plain zip.
- **The 10.13 verdict depends on the load command.** `LC_VERSION_MIN_MACOSX` above 10.13.6 → NO. `LC_BUILD_VERSION` above 10.13.6 → UNPROVEN, because 10.13 predates that command: Gorilla OpenCode 0.1.149-dev for the Air (`D:\MACBOOK`, built with the patched go1.24) carries `LC_BUILD_VERSION minos 11.0 sdk 11.0` and runs on the Air. 10.13.6 is the comparison point (High Sierra's last release).
- **Extraction refusals fail closed** on the entry NAME before any permission prompt: empty, absolute, `..`, `.`/empty segment, `:` (drive letter, NTFS stream), control characters, trailing dot/space, Windows device names; backslashes are separators. Then, below the destination, any existing link or junction (`ModeSymlink|ModeIrregular`) is refused, as is an existing target. The destination must resolve (symlinks followed) inside the working directory or a configured root (`ResolveWriteTarget`); folders are created only after the grant.
- **Strings are fenced as untrusted**; names from binaries and archives pass through `clean` (control, zero-width and bidi characters removed, length capped).

## Tried and Abandoned

- `zig cc -Wl,--strip-debug`: lld dropped `.symtab` too; the ELF fixture is stripped with `llvm-objcopy --strip-debug` instead, which keeps it. Without stripping, zig's glibc start files put the build machine's paths (user name included) into `.debug_*`.
- `-s` on the PE link did not shrink it (56,832 bytes either way).

## ⚠ Claimed But Not Verified

- That 10.13 ignores `LC_BUILD_VERSION` in general. The evidence is one binary (0.1.149-dev) that runs there; hence UNPROVEN, not OK.
- Behavior inside a live model conversation (Gemma 4 E2B or any other): not run. The token cost is measured; whether Gemma picks the right `part`/`action` is not.
- The patched legacy-darwin go1.24 toolchain is not on this laptop. Checked instead: `GOOS=darwin GOARCH=amd64` build of the whole program with stock go1.24.13 (`~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.24.13.windows-amd64`) and `go test ./internal/forensics` on go1.24.13.

## Addendum 2026-10-22: full=true, three users

The owner: Gorilla OpenCode is used "locally/manually by a person, big model such as yourself and in cases of extreme emergencies, like a last hail mary by a local SLM". The caps above were sized only for the small model. Both tools now take `full=true`: every list cap is lifted and `strings` reads the whole file; a result over the 400 KB response limit is kept whole in a file by the existing spill (`clampToolContent`). Every capped line now ends "or full=true". Cost: +24 tokens per tool (204 + 202 measured). Tests: `TestFullLiftsEveryCap`, `TestFullListsEveryEntry`, `TestFullReachesBothTools`; four more mutations in `testdata/mutations.yaml`.

## Open Items

- Exports (PE export directory, ELF defined dynamic symbols as a list), PE resources/version info, Authenticode and Mach-O signature verification.
- xz/zstd (no stdlib decoder; card per PLAN.md).
- Phase 1b: GResource/GVDB, build-to-build diff on top of this package.

## How to verify this work is correct

    go test ./internal/forensics/ ./internal/llm/tools/ ./internal/llm/agent/ ./internal/config/
    fieldkit mutate --plan internal/forensics/testdata/mutations.yaml --cwd . -- go test ./internal/forensics/ ./internal/llm/tools/ -run "^Test(ELF|PE|MachO|Legacy|Strings|Entropy|ListCap|Refuses|Clean|Omni|Large|Tar|Single|Stream|Unsupported|Safe|Traversal|Extract|BinaryInspect|ArchiveTool)"
    go test ./internal/llm/agent/ -run TestDefaultToolSchemaCost -v

## Technical Debt

- The junction test (`TestExtractRefusesLinkedFolderInDestination`) runs `cmd /c mklink /J` on Windows when `os.Symlink` is refused; it skips if neither works.
- `writeEntry`'s lexical `within` check is unreachable after `SafeEntryPath`; kept as a second wall, not covered by a mutation.

## Claim Sources

- Mozilla jar layout: `python/mozbuild/mozpack/mozjar.py`, `JarWriter.finish` (Firefox source tree), and the `unzip -l` warning on the fixture ("320 extra bytes at beginning"), which is what unzip says of an optimized jar.
- archive/zip base-offset rule: `src/archive/zip/reader.go`, `readDirectoryEnd`, go1.24.13.
- Load command numbers: `<mach-o/loader.h>` (`LC_VERSION_MIN_MACOSX 0x24`, `LC_BUILD_VERSION 0x32`, `LC_CODE_SIGNATURE 0x1d`, `PLATFORM_MACOS 1`).
- Timings and the 0.1.149-dev reading: run 2026-10-21 on this laptop (Firefox 157 build of 2026-10-10 under Fieldkit `state/build-harness/leakgate`).
