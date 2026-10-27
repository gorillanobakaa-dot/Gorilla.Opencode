#!/usr/bin/env bash
# Run `go test` and, when it fails, turn the failures into GitHub workflow
# annotations. Annotations are readable without a GitHub login
# (GET /repos/{owner}/{repo}/check-runs/{job id}/annotations), while the step
# log needs one, so this is how an unattended session reads a red CI run.
#
# GitHub keeps at most 10 annotations of each level per step, so:
#   ::error   one per failing (leaf) test, with its first message line
#   ::warning one per failing package (build failures included)
#   ::notice  one per panic line (a panic hides the test's own message)
#
# Usage: go-test-annotate.sh [go test args...]   (default: ./... -count=1)
set -o pipefail

out=test.out
if [ "$#" -eq 0 ]; then set -- ./... -count=1; fi

go test "$@" 2>&1 | tee "$out"
status=$?
if [ "$status" -eq 0 ]; then exit 0; fi

awk '
function esc(s) { gsub(/%/, "%25", s); gsub(/\r/, "", s); return s }
function esct(s) { s = esc(s); gsub(/:/, "%3A", s); gsub(/,/, "%2C", s); return s }
function trim(s) { sub(/^[ \t]+/, "", s); sub(/[ \t]+$/, "", s); return s }
function flush_pending(msg) {
	if (pending != "") {
		tests[++ntests] = pending SUBSEP msg
		pkgtests++
		pending = ""
	}
}
/^[ \t]*--- FAIL: / {
	# A parent whose next line is a subtest FAIL is dropped: the leaf says more.
	pending = ""
	name = $0
	sub(/^[ \t]*--- FAIL: /, "", name)
	sub(/ \([0-9.]+s\)[ \t\r]*$/, "", name)
	pending = name
	next
}
/^panic: / {
	panics[++npanics] = trim($0)
	flush_pending(trim($0))
	next
}
/^(FAIL|ok)[ \t]+[^ \t]+/ {
	flush_pending("(no message)")
	if ($1 == "FAIL") {
		pkg = $2
		rest = $0
		sub(/^FAIL[ \t]+[^ \t]+[ \t]*/, "", rest)
		pkgs[++npkgs] = pkg SUBSEP pkgtests SUBSEP trim(rest)
		for (i = pkgstart + 1; i <= ntests; i++) tpkg[i] = pkg
	}
	pkgstart = ntests
	pkgtests = 0
	next
}
{
	if (pending != "") {
		line = trim($0)
		if (line != "" && line != "FAIL") flush_pending(line)
	}
}
END {
	flush_pending("(no message)")
	for (i = 1; i <= ntests; i++) {
		split(tests[i], t, SUBSEP)
		short = tpkg[i]; sub(/.*\//, "", short)
		printf "::error title=%s::%s\n", esct(short "." t[1]), esc(t[2])
		names[tpkg[i]] = names[tpkg[i]] (names[tpkg[i]] == "" ? "" : " ") t[1]
	}
	# The package warning names every failing test: past the 10th ::error the
	# names were lost (run 40488750905: shell had 3 failures, 1 was shown).
	for (i = 1; i <= npkgs; i++) {
		split(pkgs[i], p, SUBSEP)
		printf "::warning title=FAIL %s::%s failing test(s) %s: %s\n", esct(p[1]), p[2], esc(p[3]), esc(names[p[1]])
	}
	for (i = 1; i <= npanics; i++) printf "::notice title=panic::%s\n", esc(panics[i])
	if (ntests + npkgs + npanics == 0) print "::error title=go test::exit status non-zero, no FAIL line parsed"
}
' "$out"

exit "$status"
