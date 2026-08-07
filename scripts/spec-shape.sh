#!/usr/bin/env bash
# Copyright 2026 Bruno Venceslau. All rights reserved.
# Use of this source code is governed by a BSD-2-Clause
# license that can be found in the LICENSE file.
#
# spec-shape.sh <vendored> <candidate>
#
# Exits 0 when <candidate> carries the same number of operations as
# <vendored>. Used by `make update-spec` between the fetch and the move, so a
# document that is not the spec never becomes the tracked artifact.
#
# This is a SHAPE check, not a validity check. A file containing nothing but
# the right number of `      operationId:` lines passes, as does the genuine
# spec with a hostile servers: url. What it buys is the one case the fetch
# flags cannot reach: a redirect is not an error under curl's -f, so the
# transfer "succeeds" and curl writes the 302's own body — attacker-chosen,
# not merely empty. Three controls, none subsuming another: curl's non-zero
# exit covers a truncated transfer, this covers a body whose operation count
# differs, and contract.SpecSHA256 is the only one that covers the bytes.
#
# The expected count comes from <vendored> rather than a literal, so THIS
# gate needs no edit when the operation set legitimately changes. (44 is
# still written out in the contract table and its tests — those are the
# edits such a change is supposed to force.)
set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "usage: spec-shape.sh <vendored> <candidate>" >&2
	exit 2
fi

vendored=$1
candidate=$2

for f in "$vendored" "$candidate"; do
	if [ ! -f "$f" ]; then
		echo "error: $f does not exist" >&2
		exit 2
	fi
	if [ ! -r "$f" ]; then
		echo "error: $f is not readable" >&2
		exit 2
	fi
done

# grep exits 1 on zero matches, which is a legitimate answer, and 2+ when it
# could not scan at all. A bare `|| true` would collapse both into the empty
# string — and since POSIX exempts an `if` condition from set -e, the two
# `[ "$x" -eq ... ]` tests below would then error, be read as false, and the
# script would fall off the end at exit 0. A supply-chain gate that accepts
# on internal error is worse than no gate, so anything but 0 or 1 is fatal.
count() {
	local n rc=0
	n=$(grep -c '^      operationId:' -- "$1") || rc=$?
	if [ "$rc" -gt 1 ]; then
		echo "error: cannot scan $1 (grep exited $rc)" >&2
		exit 2
	fi
	printf '%s\n' "${n:-0}"
}

want=$(count "$vendored")
got=$(count "$candidate")

if [ "$want" -eq 0 ]; then
	echo "error: the vendored spec $vendored declares no operations — refusing to" >&2
	echo "       compare against it, since every candidate would then pass" >&2
	exit 2
fi

if [ "$got" -ne "$want" ]; then
	echo "error: $candidate declares $got operations, $vendored declares $want — not vendoring." >&2
	echo "       A redirect body or a wrong document lands here; the candidate is left in" >&2
	echo "       place above so you can look at what actually arrived." >&2
	echo "       If upstream really changed the operation set, that is a contract-table" >&2
	echo "       change: re-fetch with the curl in the update-spec recipe, then update" >&2
	echo "       internal/contract/table.go and contract.SpecSHA256 in the same commit." >&2
	exit 1
fi
