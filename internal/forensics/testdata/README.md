# Forensics fixtures

Small files the tests in `internal/forensics` and `internal/llm/tools/forensics_test.go` read. They are checked in so the tests need no compiler. The `.gitattributes` here marks them binary.

| File | What it is | Built with |
|---|---|---|
| `fx.c` | the source of the three executables | - |
| `fx.elf` | ELF64 x86_64 executable, NEEDED libc.so.6, RUNPATH `$ORIGIN/lib`, .symtab kept, no debug info | zig 0.16.0 + llvm-objcopy (MozillaBuild clang) |
| `fx.pe` | PE32+ x86_64 console EXE (named `.pe` so the repository's `*.exe` ignore rule does not apply) | zig 0.16.0 |
| `fx-fat.macho` | fat Mach-O: x86_64 slice with LC_VERSION_MIN_MACOSX 10.13, arm64 slice with LC_BUILD_VERSION 11.0 (ad-hoc signed by the linker) | zig 0.16.0 + llvm-lipo |
| `omni.ja` | Mozilla optimized jar layout (4-byte preload length, central directory first, end record twice) | `GORILLA_FORENSICS_FIXTURES=1 go test -run TestWriteOmniJaFixture ./internal/forensics/` |
| `fx.tar.bz2` | ustar with a folder, two files and a hard-link entry, owner 0/0, mtime 0 | GNU tar + bzip2 (Git for Windows) |
| `single.txt.bz2` | one bzip2-compressed file | bzip2 |
| `mutations.yaml` | the rules `fieldkit mutate` breaks on purpose | - |

Each executable carries the ASCII string `GORILLA_FIXTURE_ASCII` and the UTF-16LE string `GORILLA_WIDE_16`.

Commands (Git Bash, offline; nothing downloaded):

    zig cc -target x86_64-linux-gnu -O1 -g0 -fno-ident -Wl,-rpath,'$ORIGIN/lib' -Wl,--build-id=none -o fx-dbg.elf fx.c
    llvm-objcopy --strip-debug --remove-section=.comment fx-dbg.elf fx.elf
    zig cc -target x86_64-windows-gnu -Os -fno-ident -s -o fx.pe fx.c
    zig cc -target x86_64-macos.10.13 -O1 -g0 -fno-ident -o fx-x64.macho fx.c
    zig cc -target aarch64-macos.11.0 -O1 -g0 -fno-ident -o fx-arm64.macho fx.c
    llvm-lipo -create fx-x64.macho fx-arm64.macho -output fx-fat.macho
    tar --owner=0 --group=0 --numeric-owner --mtime=@0 --format=ustar --sort=name -cf fx.tar -C src docs docs/readme.txt notes.txt
    bzip2 -9 fx.tar

`--strip-debug` matters: zig's glibc start files carry debug info with the build machine's folder names, which would otherwise end up in a public repository.

## Exports, version info and signer names (2026-10-22)

| File | What it is | Built with |
|---|---|---|
| `fxlib.c` | the source of the three libraries: two exported functions, one exported data object, one static function | - |
| `fxlib.def` | PE export list: adds a forwarder (`gorilla_forwarded=KERNEL32.GetTickCount`) and an ordinal-only export (`@9 NONAME`) | - |
| `fxlib.rc` | PE `VERSIONINFO` resource | - |
| `fxlib.pedll` | PE32+ x86_64 DLL with exports and a version resource | zig 0.16.0 (`zig rc`, `zig cc`) |
| `fxlib.elfso` | ELF64 x86_64 shared library | zig 0.16.0 |
| `fxlib.machodylib` | thin x86_64 Mach-O dylib, LC_VERSION_MIN_MACOSX 10.13, export trie | zig 0.16.0 |

The odd extensions keep them clear of the repository's `*.dll`, `*.so` and `*.dylib` ignore rules. Checked with `strings` before committing: no folder names of the build machine.

    zig rc -x fxlib.rc fxlib.res
    zig cc -target x86_64-windows-gnu -shared -Os -fno-ident -s -o fxlib.pedll fxlib.c fxlib.def fxlib.res
    zig cc -target x86_64-linux-gnu -shared -fPIC -O1 -g0 -fno-ident -Wl,--build-id=none -o fxlib.elfso fxlib.c
    zig cc -target x86_64-macos.10.13 -shared -O1 -g0 -fno-ident -o fxlib.machodylib fxlib.c

No signed fixture can be made offline (a real signature needs a certificate from someone else). The signer tests in `binary_extra_test.go` build a minimal PKCS#7 SignedData with two throwaway certificates instead, append it to a copy of `fx.pe` as a `WIN_CERTIFICATE`, and hand-build a Mach-O SuperBlob around it. `fxlib.res` is an intermediate and is not kept.
