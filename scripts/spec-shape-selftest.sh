#!/usr/bin/env bash
# Copyright 2026 Bruno Venceslau. All rights reserved.
# Use of this source code is governed by a BSD-2-Clause
# license that can be found in the LICENSE file.
#
# The network-free proof that spec-shape.sh accepts the spec and refuses
# everything else `make update-spec` can plausibly download. Twin of
# apidiff-selftest and check-version-selftest: no curl, no live endpoint.
#
# The baseline is the real vendored spec, deliberately: the "unchanged spec
# is accepted" case wants the real artifact, and no `44` literal appears
# anywhere here, so a legitimate re-vendor causes zero churn. The cost is
# that fixtures DERIVED from it can silently degenerate when upstream moves
# the lines they edit — so each one asserts it actually differs as intended
# before it is used. A fixture that stopped discriminating would otherwise
# leave the suite green while proving nothing.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
shape="$here/spec-shape.sh"
vendored="$here/../openapi.yaml"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail=0

note() { echo "FAIL: $1"; fail=1; }

# ok <name> <candidate> — the check must accept.
ok() {
	if "$shape" "$vendored" "$2" >/dev/null 2>&1; then
		echo "ok: $1"
	else
		note "$1 — the check rejected a document it should accept"
	fi
}

# rejects <name> <candidate> <expected-exit> — the check must refuse, with
# that exact code. 1 means "not a spec"; 2 means "you called me wrong".
rejects() {
	local rc=0
	"$shape" "$vendored" "$2" >/dev/null 2>&1 || rc=$?
	if [ "$rc" -eq "$3" ]; then
		echo "ok (fail): $1"
	else
		note "$1 — expected exit $3, got $rc"
	fi
}

# Same fail-closed discipline spec-shape.sh's count() documents: a bare
# `|| true` would turn an unscannable fixture into a silent zero.
anchored() {
	local n rc=0
	n=$(grep -c '^      operationId:' -- "$1") || rc=$?
	if [ "$rc" -gt 1 ]; then
		note "cannot scan fixture $1 (grep exited $rc)"
		printf '%s\n' -1
		return
	fi
	printf '%s\n' "${n:-0}"
}
want=$(anchored "$vendored")

cp "$vendored" "$tmp/same.yaml"
ok "the live spec unchanged is vendored" "$tmp/same.yaml"

# A description-only upstream edit must still be vendorable: this gate
# guards the operation set, not the bytes. The content pin guards those.
sed 's/^      description: .*/      description: reworded upstream/' "$vendored" > "$tmp/reworded.yaml"
if cmp -s "$vendored" "$tmp/reworded.yaml"; then
	note "the reworded fixture is byte-identical to the baseline — the sed stopped matching"
elif [ "$(anchored "$tmp/reworded.yaml")" -ne "$want" ]; then
	note "the reworded fixture changed the operation count — it no longer isolates description edits"
else
	ok "a description-only upstream edit is vendored" "$tmp/reworded.yaml"
fi

printf '<html><head><title>302 Found</title></head><body>Moved</body></html>\n' > "$tmp/redirect.html"
rejects "a redirect body does not become the spec" "$tmp/redirect.html" 1

: > "$tmp/empty.yaml"
rejects "an empty body does not become the spec" "$tmp/empty.yaml" 1

printf 'error: not found\n' > "$tmp/error.txt"
rejects "an error page does not become the spec" "$tmp/error.txt" 1

awk '!d && /^      operationId:/ { print "      operationId: extraThing"; d = 1 } { print }' \
	"$vendored" > "$tmp/extra.yaml"
if [ "$(anchored "$tmp/extra.yaml")" -ne $((want + 1)) ]; then
	note "the added-operation fixture did not add one — it proves nothing"
else
	rejects "an added operation is not a silent refresh" "$tmp/extra.yaml" 1
fi

awk '!d && /^      operationId:/ { d = 1; next } { print }' \
	"$vendored" > "$tmp/missing.yaml"
if [ "$(anchored "$tmp/missing.yaml")" -ne $((want - 1)) ]; then
	note "the removed-operation fixture did not remove one — it proves nothing"
else
	rejects "a removed operation is not a silent refresh" "$tmp/missing.yaml" 1
fi

# The near-miss pair. Both cases score the same under an unanchored match
# and differently under the real one, so together they pin HOW the count is
# computed — not just that some count is compared. Upstream writes
# `operationId` into prose routinely, so an unanchored pattern is reachable.
awk '!d && /^      operationId:/ { print "      # operationId is mentioned here in prose"; d = 1; next } { print }' \
	"$vendored" > "$tmp/prose-short.yaml"
if [ "$(anchored "$tmp/prose-short.yaml")" -ne $((want - 1)) ]; then
	note "the prose near-miss fixture did not trade an operation for a mention"
else
	rejects "an operation traded for a prose mention is refused" "$tmp/prose-short.yaml" 1
fi

{
	printf '# operationId appears in a comment\n'
	printf 'x-note: operationId appears in a value\n'
	cat "$vendored"
} > "$tmp/prose-extra.yaml"
if [ "$(anchored "$tmp/prose-extra.yaml")" -ne "$want" ]; then
	note "the prose-padding fixture changed the anchored count — it proves nothing"
else
	ok "prose mentions do not pad the count" "$tmp/prose-extra.yaml"
fi

# A truncated body is ACCEPTED, and that is the documented division of
# labour: every operationId precedes components:, so a cut past them keeps
# the count. What covers a truncated transfer is curl's non-zero exit, which
# aborts the recipe before the move. Pinned as a characterization case so the
# claim cannot invert silently if upstream ever reorders the document.
head -c $(( $(wc -c < "$vendored") / 2 )) "$vendored" > "$tmp/truncated.yaml"
if [ "$(anchored "$tmp/truncated.yaml")" -ne "$want" ]; then
	note "a half-truncated spec no longer keeps the operation count — the Makefile comment and spec-shape.sh's header both claim it does, and they are now wrong"
else
	ok "a truncated body is accepted — curl's exit is what covers it" "$tmp/truncated.yaml"
fi

# Fail closed on its own inputs, so a broken call site cannot pass quietly.
rejects "a missing candidate fails closed" "$tmp/absent.yaml" 2

rc=0
"$shape" "$tmp/absent-baseline.yaml" "$tmp/same.yaml" >/dev/null 2>&1 || rc=$?
if [ "$rc" -eq 2 ]; then
	echo "ok (fail): a missing baseline fails closed"
else
	note "a missing baseline must exit 2, got $rc — this is the recovery path after a hijacked fetch already destroyed the vendored spec"
fi

: > "$tmp/nospec.yaml"
rc=0
"$shape" "$tmp/nospec.yaml" "$tmp/same.yaml" >/dev/null 2>&1 || rc=$?
if [ "$rc" -eq 2 ]; then
	echo "ok (fail): an operationless baseline is refused, not used"
else
	note "an operationless baseline must exit 2, got $rc — otherwise every candidate matches it"
fi

# A wrong call must not be mistaken for a verdict.
for argc in 0 1 3; do
	rc=0
	case $argc in
	0) "$shape" >/dev/null 2>&1 || rc=$? ;;
	1) "$shape" "$vendored" >/dev/null 2>&1 || rc=$? ;;
	3) "$shape" "$vendored" "$vendored" extra >/dev/null 2>&1 || rc=$? ;;
	esac
	if [ "$rc" -eq 2 ]; then
		echo "ok (fail): a ${argc}-argument call fails closed"
	else
		note "a ${argc}-argument call must exit 2, got $rc"
	fi
done

# The gate must refuse to answer when it cannot scan. A `grep -c ... || true`
# would collapse an unscannable input into the empty string, and since POSIX
# exempts an `if` condition from set -e the comparisons would then error, be
# read as false, and the script would fall off the end accepting everything.
minbin=$tmp/bin
mkdir -p "$minbin"
for c in sh bash cat printf; do
	if src=$(command -v "$c" 2>/dev/null); then ln -sf "$src" "$minbin/$c"; fi
done
rc=0
PATH="$minbin" "$shape" "$vendored" "$tmp/redirect.html" >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
	echo "ok (fail): an unscannable input is refused, not accepted (exit $rc)"
else
	note "the gate accepted a redirect body when grep was unavailable — it fails OPEN"
fi

if [ "$fail" -ne 0 ]; then
	echo "spec-shape gate self-test FAILED"
	exit 1
fi
echo "ok: spec-shape gate classifies every case correctly"
