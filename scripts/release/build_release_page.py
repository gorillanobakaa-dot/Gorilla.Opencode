#!/usr/bin/env python3
"""The "What is built in" table of a Gorilla OpenCode release page, generated, never typed.

    python scripts/release/build_release_page.py helpers            print the block
    python scripts/release/build_release_page.py helpers --write ReleaseNotes/GITHUB-RELEASE-NOTES-vX.md
    python scripts/release/build_release_page.py helpers --check  ReleaseNotes/GITHUB-RELEASE-NOTES-vX.md

Inputs (the only sources, so the page cannot disagree with the program):
  internal/toolbox/SHA256SUMS      which helper is carried on which system (windows-/linux-/darwin- prefixes)
  third_party/PINS.md              the version of each helper
  internal/arsenal/manifest.json   Tesseract, ImageMagick, PowerToys and the rest: not carried, with the
                                   exact install command per system, and a plain "not available" where there is none

The block sits between <!-- helpers:begin --> and <!-- helpers:end -->. --write replaces it (or appends it to the
page when the page has none); --check exits 1 when the page's block differs from a fresh one.
"""
import argparse
import json
import re
import sys
from pathlib import Path

BEGIN, END = "<!-- helpers:begin -->", "<!-- helpers:end -->"
SYSTEMS = [("Windows", "windows-"), ("Linux", "linux-"), ("macOS", "darwin-")]
# helper -> the text its first cell in PINS.md contains
PIN_KEY = {"pfind": "pfind", "ast-grep": "ast-grep", "coreutils": "coreutils", "difft": "difftastic",
           "hyperfine": "hyperfine", "ninja": "ninja", "sd": "sd", "cwebp": "cwebp", "avifenc": "avifenc",
           "oxipng": "oxipng", "7za": "7-zip", "ffmpeg": "ffmpeg", "ffprobe": "ffmpeg"}
# not carried: shown with their install command, from the arsenal manifest
OUTSIDE = ["tesseract", "imagemagick", "powertoys"]
MANAGERS = {"Windows": ["winget", "scoop"], "Linux": ["apt", "dnf", "pacman"], "macOS": ["brew"]}
PLATFORM = {"Windows": "windows", "Linux": "linux", "macOS": "macos"}
VERSION_HEADERS = ("tag", "version", "release")


def carried(root):
    """-> {helper: {system: True}} from SHA256SUMS."""
    out = {}
    for line in (root / "internal/toolbox/SHA256SUMS").read_text(encoding="utf-8").splitlines():
        m = re.match(r"[0-9a-fA-F]{64}\s+\*?(\S+)", line)
        if not m:
            continue
        rel = m.group(1)
        name = rel.rsplit("/", 1)[-1]
        for ext in (".gz", ".exe"):
            if name.endswith(ext):
                name = name[: -len(ext)]
        for system, prefix in SYSTEMS:
            if rel.startswith(prefix):
                out.setdefault(name, {})[system] = True
    return out


def versions(root):
    """-> {PINS first-cell text (lower): version} for every pins table that has a tag, version or release column."""
    found, header = {}, None
    for line in (root / "third_party/PINS.md").read_text(encoding="utf-8").splitlines():
        if not line.startswith("|"):
            header = None
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if header is None:
            header = [c.lower() for c in cells]
            continue
        if set("".join(cells)) <= set("-: "):
            continue
        col = next((i for i, h in enumerate(header) if h in VERSION_HEADERS), None)
        if col is not None and col < len(cells):
            ver = cells[col].replace("*", "").split(",")[0].split()[0] if cells[col].replace("*", "").strip() else ""
            found[cells[0].lower()] = ver
    return found


def version_of(helper, pins):
    key = PIN_KEY.get(helper, helper)
    for cell, ver in pins.items():
        first = re.split(r"[ (]", cell)[0] if key != "7-zip" else cell
        if cell == key or first == key or (len(key) > 3 and key in cell):
            return ver
    return "see third_party/PINS.md"


def manifest_entries(root):
    data = json.loads((root / "internal/arsenal/manifest.json").read_text(encoding="utf-8"))
    return {e["id"]: e for s in data["series"] for e in s["entries"]}


def install_cell(entry, system):
    plats = entry.get("platforms")
    if plats and PLATFORM[system] not in plats:
        return f"not available on {system}"
    for mgr in MANAGERS[system]:
        pk = entry.get("packages", {}).get(mgr)
        if pk:
            verb = {"winget": "winget install", "scoop": "scoop install", "apt": "sudo apt install",
                    "dnf": "sudo dnf install", "pacman": "sudo pacman -S", "brew": "brew install"}[mgr]
            return f"`{verb} {' '.join(pk)}`"
    return f"no package manager entry for {system}"


def build_block(root):
    root = Path(root)
    have, pins, man = carried(root), versions(root), manifest_entries(root)
    lines = [BEGIN, "", "### What is built in", "",
             "Carried inside the program and checked by hash before every run. Nothing to install. "
             "A dash means that system does not carry it. On macOS, ffmpeg and ffprobe are 8.0 and need macOS 11 "
             "(third_party/PINS.md).", "",
             "| Helper | Version | Windows | Linux | macOS |", "|---|---|---|---|---|"]
    for name in sorted(have):
        row = " | ".join("yes" if have[name].get(s) else "-" for s, _ in SYSTEMS)
        lines.append(f"| {name} | {version_of(name, pins)} | {row} |")
    lines += ["", "Not carried, but known: `/arsenal` finds them on your machine and prints the line to install them.", "",
              "| Program | Windows | Linux | macOS |", "|---|---|---|---|"]
    for pid in OUTSIDE:
        e = man.get(pid)
        if e is None:
            continue
        lines.append(f"| {e['title']} ({pid}) | " + " | ".join(install_cell(e, s) for s, _ in SYSTEMS) + " |")
    lines += ["", END]
    return "\n".join(lines) + "\n"


def splice(page, block):
    i, j = page.find(BEGIN), page.find(END)
    if 0 <= i < j:
        return page[:i] + block.rstrip("\n") + page[j + len(END):]
    return page.rstrip("\n") + "\n\n" + block


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("what", choices=["helpers"])
    ap.add_argument("--root", default=str(Path(__file__).resolve().parents[2]))
    g = ap.add_mutually_exclusive_group()
    g.add_argument("--write", metavar="PAGE")
    g.add_argument("--check", metavar="PAGE")
    a = ap.parse_args(argv)
    block = build_block(a.root)
    if a.write:
        p = Path(a.write)
        p.write_text(splice(p.read_text(encoding="utf-8"), block), encoding="utf-8", newline="\n")
        print(f"wrote the helpers block into {p}")
        return 0
    if a.check:
        page = Path(a.check).read_text(encoding="utf-8")
        i, j = page.find(BEGIN), page.find(END)
        ok = 0 <= i < j and page[i:j + len(END)] == block.rstrip("\n")
        print("helpers block is current" if ok else f"helpers block in {a.check} is missing or stale: rerun with --write")
        return 0 if ok else 1
    sys.stdout.write(block)
    return 0


if __name__ == "__main__":
    sys.exit(main())
