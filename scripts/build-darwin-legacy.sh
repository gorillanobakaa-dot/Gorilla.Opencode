#!/usr/bin/env bash
# Build Gorilla OpenCode for Intel Macs on macOS 10.13 High Sierra (and 10.14+).
# Usage: VERSION=0.1.147 scripts/build-darwin-legacy.sh [output-path]
#   MIRROR=1  fetch golang.org/x, google.golang.org, cloud.google.com,
#             go.uber.org, gopkg.in deps from their GitHub mirrors (for sandboxes that
#             block proxy.golang.org / golang.org). Never touches the repo's go.mod/go.sum.
# Idempotent: rebuilds the patched GOROOT copy every run; safe to re-run after failure.
set -euo pipefail
REPO="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO"
OUT="${1:-$REPO/dist/gorilla-opencode-mac-10.13}"
WORK="${TMPDIR:-/tmp}/gorilla-darwin-legacy"
GR="$WORK/goroot"
export GOTOOLCHAIN=local   # a toolchain switch would bypass the patched GOROOT

SRC_GOROOT="$(go env GOROOT)"
case "$(go env GOVERSION)" in go1.24.*) ;; *) echo "warn: patch written against go1.24.x, have $(go env GOVERSION); anchors are checked" >&2 ;; esac

rm -rf "$GR" && mkdir -p "$WORK" && cp -a "$SRC_GOROOT" "$GR" && chmod -R u+w "$GR/src/crypto/x509/internal/macos"
M="$GR/src/crypto/x509/internal/macos"
python3 -I - "$M/security.go" "$M/security.s" <<'PY'
import sys
g, s = sys.argv[1], sys.argv[2]
# UTF-8 and LF on every platform (2026-10-19): without encoding=, Python on
# Windows reads Go's sources as cp1252 and the "·" in the .s anchor below
# never matches; without newline='', it would write CRLF into the toolchain.
def read(p):
    with open(p, encoding="utf-8", newline="") as f:
        return f.read()
def write(p, text):
    with open(p, "w", encoding="utf-8", newline="") as f:
        f.write(text)
t = read(g)
a = '//go:cgo_import_dynamic x509_SecTrustEvaluateWithError SecTrustEvaluateWithError'
assert a in t, "anchor missing in security.go (Go stdlib changed; update patch)"
assert 'func SecTrustEvaluate(trustObj CFRef) (CFRef, error)' in t, "SecTrustEvaluate helper missing"
start = t.index(a)
endm = 'func x509_SecTrustEvaluateWithError_trampoline()\n'
end = t.index(endm, start) + len(endm)
new = '''// Patched for macOS 10.13: SecTrustEvaluateWithError needs 10.14+.
func SecTrustEvaluateWithError(trustObj CFRef) (int, error) {
	res, err := SecTrustEvaluate(trustObj)
	if err != nil {
		return int(err.(OSStatus).status), err
	}
	if r := uint32(res); r == 1 || r == 4 { // kSecTrustResultProceed, kSecTrustResultUnspecified
		return 0, nil
	}
	return ErrSecNotTrusted, errors.New("x509: certificate not trusted by macOS (SecTrustResultType " + strconv.Itoa(int(uint32(res))) + ")")
}
'''
write(g, t[:start] + new + t[end:])
a2 = 'TEXT ·x509_SecTrustEvaluateWithError_trampoline(SB),NOSPLIT,$0-0\n\tJMP x509_SecTrustEvaluateWithError(SB)\n'
u = read(s); assert a2 in u, "anchor missing in security.s"
write(s, u.replace(a2, ''))
PY

BUILD_ARGS=()
if [ "${MIRROR:-0}" = 1 ]; then
  cp "$REPO/go.mod" "$WORK/b.mod" && cp "$REPO/go.sum" "$WORK/b.sum"
  REPL=$(python3 -I - "$REPO/go.mod" <<'PY'
import re, sys
m = [('golang.org/x/','github.com/golang/'),('google.golang.org/grpc','github.com/grpc/grpc-go'),
 ('google.golang.org/protobuf','github.com/protocolbuffers/protobuf-go'),('google.golang.org/genai','github.com/googleapis/go-genai'),
 ('google.golang.org/genproto','github.com/googleapis/go-genproto'),('cloud.google.com/go','github.com/googleapis/google-cloud-go'),
 ('go.uber.org/multierr','github.com/uber-go/multierr'),
 ('gopkg.in/yaml.v3','github.com/go-yaml/yaml/v3')]
for p, v in re.findall(r'^\s*(\S+)\s+(v\S+)', open(sys.argv[1]).read(), re.M):
    for k, r in m:
        if p.startswith(k): print(f'-replace={p}={r}{p[len(k):]}@{v}'); break
PY
)
  # shellcheck disable=SC2086
  go mod edit -modfile="$WORK/b.mod" $REPL
  export GOPROXY=direct GOSUMDB=off GOFLAGS=-mod=mod
  BUILD_ARGS=(-modfile="$WORK/b.mod")
fi

mkdir -p "$(dirname "$OUT")"
GOROOT="$GR" CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
  "$GR/bin/go" build "${BUILD_ARGS[@]}" -trimpath -ldflags="-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v${VERSION:?set VERSION, e.g. VERSION=0.1.147}" -o "$OUT" .

# Verify: the 10.14+ symbol is gone and no macOS 12+ chained-fixups load command exists.
if grep -aq '_SecTrustEvaluateWithError' "$OUT"; then echo "FAIL: binary still imports SecTrustEvaluateWithError" >&2; exit 1; fi
python3 -I - "$OUT" <<'PY'
import struct, sys
d = open(sys.argv[1], 'rb').read(); n = struct.unpack_from('<I', d, 16)[0]; o = 32
for _ in range(n):
    c, sz = struct.unpack_from('<II', d, o)
    if c == 0x80000034: sys.exit("FAIL: LC_DYLD_CHAINED_FIXUPS present (needs macOS 12)")
    o += sz
PY
echo "OK: $OUT"
