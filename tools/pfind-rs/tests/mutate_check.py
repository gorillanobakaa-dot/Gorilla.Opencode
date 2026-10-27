"""Test command for tests/mutations-pfind-rs.yaml: rebuild pfind.exe, then run the parity cases against it.

    fieldkit guard -- fieldkit mutate --plan tests/mutations-pfind-rs.yaml --cwd . -- python -I tools/pfind-rs/tests/mutate_check.py

Exit 0 only if the offline build succeeds and every parity case passes on pfind.exe.
"""
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
CRATE = HERE.parent


def main() -> int:
    build = subprocess.run(["cargo", "build", "--release", "--offline", "--locked", "--quiet"], cwd=CRATE)
    if build.returncode != 0:
        print("mutate_check: cargo build failed")
        return build.returncode or 1
    unit = subprocess.run(["cargo", "test", "--release", "--offline", "--locked", "--quiet"], cwd=CRATE)
    if unit.returncode != 0:
        print("mutate_check: cargo test failed")
        return unit.returncode
    # --both, not --exe-only: many cases hold no assertion of their own and are checked by matching pfind.py's output
    return subprocess.run([sys.executable, "-I", str(HERE / "run_parity.py"), "--both"]).returncode


if __name__ == "__main__":
    sys.exit(main())
