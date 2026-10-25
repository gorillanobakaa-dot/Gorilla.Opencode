#!/usr/bin/env python3
# Version: 1.0.0 · updated 26-10-25
"""Build the .deb and .rpm on a machine without dpkg-deb or rpmbuild (Windows), with nfpm.

WHY (2026-10-25)
    v0.1.152 was released from a Windows laptop. scripts/build-deb.sh and build-rpm.sh need
    dpkg-deb, rpmbuild and a Linux binary they can run; and on Windows the exec bit of a staged
    file is not kept, so running those scripts there would make packages with wrong modes. The
    owner asked for the same packages as the Linux machine. This lays out the SAME files as
    build-deb.sh (paths, modes, control fields, postinst/postrm), with every mode written down,
    and packs them with nfpm (Go, no system tools). The .rpm holds the same files as the .deb,
    as build-rpm.sh makes it.

KEEP IN STEP
    The file list mirrors scripts/build-deb.sh. --compare <a .deb built on Linux> checks the
    layout against a real one: every path in it must be here too (the version-named release
    notes excepted), and the modes must match.

USAGE
    python scripts/build-packages-nfpm.py <version> <linux-binary> <out-dir> [--compare old.deb]
"""
import argparse
import fnmatch
import io
import os
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DOC = "/usr/share/doc/gorilla-opencode"

DOCS = ["docs/CONTROL-AND-COST.md", "docs/COMMANDS.md", "docs/SESSIONS-AND-STORAGE.md", "docs/CODE-REVIEW.md",
        "docs/FOOTPRINT.md", "docs/SATELLITE.md", "docs/SHELL-SAFETY.md", "docs/SECURITY-AUDIT-2026-08-18.md",
        "docs/OSINT-RESEARCH.md", "docs/OSINT-DOCTRINE.md", "docs/OSINT-SOURCE-CATALOG.md",
        "docs/FIND-TOOL-METRICS.md"]

POSTINST = """#!/bin/sh
set -e
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database /usr/share/applications || true
"""
PFIND = """#!/bin/sh
exec python3 /usr/share/gorilla-opencode/pfind.py "$@"
"""
DESCRIPTION = """Terminal AI coding agent (revived original OpenCode)
The original MIT-licensed Go OpenCode by Kujtim Hoxha, revived and
kept working with 2026 AI providers: NVIDIA NIM, Google Gemini 3,
local models via Ollama, and the originally supported providers.
No telemetry, no accounts; bring your own API keys.

Dual-track documentation (plain-language and developer) is installed
under /usr/share/doc/gorilla-opencode."""


def layout(binary: Path, gen: Path) -> list:
    """[(src, dst, mode)] - the files build-deb.sh installs, in its order."""
    f = [(binary, "/usr/bin/gorilla-opencode", 0o755)]
    for s in (128, 256, 512, 1024):
        f.append((ROOT / f"internal/assets/icons/gorilla-opencode-{s}.png",
                  f"/usr/share/icons/hicolor/{s}x{s}/apps/gorilla-opencode.png", 0o644))
    f.append((ROOT / "internal/assets/icons/gorilla-opencode.svg",
              "/usr/share/icons/hicolor/scalable/apps/gorilla-opencode.svg", 0o644))
    f.append((ROOT / "packaging/gorilla-opencode.desktop", "/usr/share/applications/gorilla-opencode.desktop", 0o644))
    f.append((ROOT / "packaging/setup-searxng.sh", "/usr/share/gorilla-opencode/setup-searxng.sh", 0o755))
    f.append((ROOT / "internal/llm/tools/pfind.py", "/usr/share/gorilla-opencode/pfind.py", 0o644))
    wrap = gen / "pfind"
    wrap.write_bytes(PFIND.encode())
    f.append((wrap, "/usr/bin/pfind", 0o755))
    f.append((ROOT / "README.md", f"{DOC}/README.md", 0o644))
    f.append((ROOT / "Changelogs/DOCUMENTATION.dual-track.md", f"{DOC}/DOCUMENTATION.dual-track.md", 0o644))
    f.append((ROOT / "PHILOSOPHY.md", f"{DOC}/PHILOSOPHY.md", 0o644))
    f.append((ROOT / "LICENSE", f"{DOC}/copyright", 0o644))
    for d in DOCS:
        f.append((ROOT / d, f"{DOC}/{Path(d).name}", 0o644))
    # Case-sensitive, as bash globs on Linux: Windows' glob would also take
    # LAYMAN-RELEASE-NOTES-v0.1.34.md, which the Linux .deb never carried.
    names = sorted(os.listdir(ROOT / "Changelogs"))
    for pattern in ("CHANGELOG*.md", "*release-notes*.md"):
        for n in names:
            if fnmatch.fnmatchcase(n, pattern):
                f.append((ROOT / "Changelogs" / n, f"{DOC}/{n}", 0o644))
    if (ROOT / "docs/SETTINGS.md").is_file():
        f.append((ROOT / "docs/SETTINGS.md", f"{DOC}/SETTINGS.md", 0o644))
    missing = [str(s) for s, _, _ in f if not Path(s).is_file()]
    if missing:
        sys.exit("missing source file(s): " + ", ".join(missing))
    return f


def maintainer() -> str:
    """The Maintainer line of scripts/build-deb.sh: one source, and no second copy of it here."""
    m = re.search(r"^Maintainer: (.+)$", (ROOT / "scripts/build-deb.sh").read_text(encoding="utf-8"), re.M)
    if not m:
        sys.exit("no Maintainer: line in scripts/build-deb.sh")
    return m.group(1).strip()


def nfpm_yaml(version: str, files: list, gen: Path, release: str = "") -> str:
    """release "" for the .deb (Version: 0.1.152, as dpkg-deb wrote it), "1" for the .rpm (Release: 1)."""
    (gen / "postinst").write_bytes(POSTINST.encode())
    q = lambda p: str(p).replace("\\", "/")
    lines = [
        "name: gorilla-opencode", "arch: amd64", "platform: linux", f'version: "{version}"'] + (
        [f'release: "{release}"'] if release else []) + [
        "section: devel", "priority: optional", f'maintainer: "{maintainer()}"',
        "homepage: https://github.com/gorillanobakaa-dot/Gorilla.Opencode", "license: MIT",
        "description: |"] + ["  " + l if l else "  ." for l in DESCRIPTION.splitlines()] + [
        "depends: [lynx, python3, ripgrep, 'xclip | xsel | wl-clipboard']",
        "overrides:", "  rpm:", "    depends: [python3, lynx, ripgrep]", "    recommends: [xclip]",
        # xz, as dpkg-deb and rpmbuild compress by default: gzip made the .deb 27 MB instead of ~20 MB,
        # a cost in download time on the slow links this program is built for.
        "deb:", "  compression: xz", "rpm:", "  compression: xz",
        "scripts:", f'  postinstall: "{q(gen / "postinst")}"', f'  postremove: "{q(gen / "postinst")}"',
        "contents:"]
    for src, dst, mode in files:
        lines += [f'  - src: "{q(src)}"', f'    dst: "{dst}"', "    file_info:", f"      mode: 0{mode:o}",
                  "      owner: root", "      group: root"]
    return "\n".join(lines) + "\n"


def ar_members(raw: bytes) -> list:
    """[(name, header bytes, body)] of an ar archive."""
    assert raw[:8] == b"!<arch>\n", "not an ar archive"
    out, pos = [], 8
    while pos < len(raw):
        head = raw[pos:pos + 60]
        size = int(head[48:58].strip())
        out.append((head[:16].strip().decode(), head, raw[pos + 60:pos + 60 + size]))
        pos += 60 + size + (size & 1)
    return out


def recompress_deb(deb: Path) -> None:
    """data.tar.xz again at xz preset 6, dpkg-deb's default. Measured 2026-10-25: nfpm's own xz
    left this .deb at 25.3 MB where preset 6 makes 22.2 MB of the same tar."""
    import lzma
    parts = []
    for name, head, body in ar_members(deb.read_bytes()):
        if name == "data.tar.xz":
            body = lzma.compress(lzma.decompress(body), format=lzma.FORMAT_XZ, check=lzma.CHECK_CRC64, preset=6)
            head = head[:48] + f"{len(body):<10}".encode() + head[58:]
        parts.append(head + body + (b"\n" if len(body) & 1 else b""))
    deb.write_bytes(b"!<arch>\n" + b"".join(parts))


def deb_members(deb: Path) -> dict:
    """{path: mode} of the data tarball inside a .deb (ar archive), files and links only."""
    raw = deb.read_bytes()
    assert raw[:8] == b"!<arch>\n", f"{deb}: not an ar archive"
    pos = 8
    while pos < len(raw):
        name, size = raw[pos:pos + 16].strip().decode(), int(raw[pos + 48:pos + 58].strip())
        body = raw[pos + 60:pos + 60 + size]
        pos += 60 + size + (size & 1)
        if name.startswith("data.tar"):
            mode = "r:xz" if name.endswith(".xz") else "r:gz" if name.endswith(".gz") else \
                   "r:zst" if name.endswith(".zst") else "r"
            if mode == "r:zst":
                import zstandard  # only for a zstd-compressed .deb
                body = zstandard.ZstdDecompressor().decompress(body, max_output_size=1 << 30)
                mode = "r"
            with tarfile.open(fileobj=io.BytesIO(body), mode=mode) as t:
                return {"/" + m.name.lstrip("./"): m.mode & 0o7777 for m in t.getmembers() if m.isfile() or m.issym()}
    raise SystemExit(f"{deb}: no data.tar member")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("version")
    ap.add_argument("binary")
    ap.add_argument("out")
    ap.add_argument("--compare", help="a .deb built on Linux by scripts/build-deb.sh")
    a = ap.parse_args()
    version, binary, out = a.version.lstrip("v"), Path(a.binary).resolve(), Path(a.out).resolve()
    if f"v{version}".encode() not in binary.read_bytes():
        sys.exit(f"{binary} does not carry the stamp v{version}")
    nfpm = shutil.which("nfpm") or sys.exit("nfpm not found (go install github.com/goreleaser/nfpm/v2/cmd/nfpm)")
    out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as tmp:
        gen = Path(tmp)
        files = layout(binary, gen)
        targets = {"deb": out / f"gorilla-opencode_{version}_amd64.deb",
                   "rpm": out / f"gorilla-opencode-{version}-1.x86_64.rpm"}
        for packager, target in targets.items():
            cfg = gen / f"nfpm-{packager}.yaml"
            cfg.write_text(nfpm_yaml(version, files, gen, "" if packager == "deb" else "1"), encoding="utf-8")
            r = subprocess.run([nfpm, "package", "-f", str(cfg), "-p", packager, "-t", str(target)],
                               capture_output=True, text=True)
            if r.returncode != 0 or not target.is_file():
                sys.exit(f"nfpm {packager} failed:\n{r.stdout}\n{r.stderr}")
            if packager == "deb":
                recompress_deb(target)
    want = {dst: mode for _, dst, mode in files}
    # The release's own notes must be in the package: twice (v0.1.39, v0.1.111) a glob silently
    # dropped exactly these while the older notes kept the folder looking full.
    own = [f"{DOC}/v{version}-release-notes.layman.md", f"{DOC}/v{version}-release-notes.developer.md"]
    if [p for p in own if p not in want]:
        sys.exit(f"this release's notes are not in the package: {[p for p in own if p not in want]}")
    got = deb_members(targets["deb"])
    if got != want:
        sys.exit(f"the .deb does not hold the planned files: missing {sorted(set(want) - set(got))}, "
                 f"extra {sorted(set(got) - set(want))}, modes {[(k, oct(got[k])) for k in want if k in got and got[k] != want[k]]}")
    import lzma
    data = next(b for n, _, b in ar_members(targets["deb"].read_bytes()) if n.startswith("data.tar"))
    p6 = len(lzma.compress(lzma.decompress(data), preset=6))
    if len(data) > p6 * 1.01:
        sys.exit(f"the .deb's data is {len(data):,} bytes where xz preset 6 makes {p6:,}: a heavier download")
    print(f"deb: {targets['deb'].name}  {len(got)} files, modes as planned, data {len(data):,} bytes (xz -6)")
    print(f"rpm: {targets['rpm'].name}  ({targets['rpm'].stat().st_size:,} bytes)")
    if a.compare:
        old = deb_members(Path(a.compare))
        # Everything the older package had must be here (release notes accumulate, so its notes
        # too); only this release's own notes may be new.
        notes = re.compile(r"/v[0-9.]+-release-notes")
        only_old = sorted(set(old) - set(got))
        only_new = sorted(p for p in set(got) - set(old) if not notes.search(p))
        modes = sorted((p, oct(old[p]), oct(got[p])) for p in set(old) & set(got) if old[p] != got[p])
        if only_old or modes:
            sys.exit(f"differs from {Path(a.compare).name}: missing here {only_old}; mode differences {modes}")
        print(f"same layout as {Path(a.compare).name}: {len(set(old) & set(got))} paths in common, same modes"
              + (f"; new here: {only_new}" if only_new else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
