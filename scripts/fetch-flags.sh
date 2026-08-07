#!/usr/bin/env bash
# Copyright 2026 Bruno Venceslau. All rights reserved.
# Use of this source code is governed by a BSD-2-Clause
# license that can be found in the LICENSE file.
#
# fetch-flags.sh <makefile> <drift-workflow>
#
# Asserts the two places that download the vendored OpenAPI spec still carry
# the supply-chain flags they were given, and still carry no -L.
#
# Those flags were reasoned about once, at length, and are enforced by
# nothing: a future edit could restore -L or drop --remove-on-error and every
# gate would stay green. The comments explaining them are not a control. This
# is.
#
# It deliberately does NOT police every curl in the repo. go-latest-check and
# check-version.sh both pass -L legitimately — they parse a string into a
# variable rather than vendoring bytes to disk, so a redirect costs them
# nothing. Forbidding -L globally would either break them or teach the next
# maintainer that this gate is noise to be worked around.
#
# It also asserts update-spec does not compute a digest. The content pin is a
# tripwire only while a human updates contract.SpecSHA256 by hand; a recipe
# that regenerated it would satisfy the gate on every re-vendor and assert
# nothing. That decision currently lives in prose, which is exactly the class
# of thing this script exists to convert into a check.
set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "usage: fetch-flags.sh <makefile> <drift-workflow>" >&2
	exit 2
fi

makefile=$1
workflow=$2

for f in "$makefile" "$workflow"; do
	if [ ! -r "$f" ]; then
		echo "error: $f is not readable" >&2
		exit 2
	fi
done

fail=0
bad() {
	echo "error: $1" >&2
	fail=1
}

# The spec-fetch invocations: the ones naming the OpenAPI endpoint. Both
# files must have exactly one, or the shape of this gate no longer matches
# the thing it guards.
spec_url='api\.ynab\.com/papi/open_api_spec\.yaml'

# fetches extracts the shell commands that fetch the spec, one per line.
#
# It does three things, and each one closes a way the naive version could be
# fooled. It drops comments, because a commented-out canonical fetch left
# for reference would otherwise satisfy every flag check while the live
# recipe ran with -L -k. It joins continuations, because the flags and the
# URL sit on different physical lines in both files. And it splits on ; and
# && so that two commands sharing one logical line stay two commands —
# otherwise a compliant probe fetch could vouch for an unsafe one beside it.
#
# Splitting also stops the whole recipe from being treated as the fetch: the
# update-spec recipe is one continued line end to end, so without this a
# `cp -L` three commands later reads as the curl passing -L, and a
# --max-time anywhere in the recipe satisfies the check for the curl.
#
# awk rather than sed: joining and splitting both want a newline in the
# output, and `\n` in a sed replacement is a GNU extension BSD sed rejects.
fetches() {
	awk -v url="$spec_url" '
		/^[[:space:]]*#/ { next }                  # a whole-line comment
		{ buf = buf $0 }
		/\\$/ { sub(/\\$/, "", buf); next }        # continuation: keep going
		{
			n = split(buf, parts, /;|&&/)
			for (i = 1; i <= n; i++) {
				frag = parts[i]
				sub(/[[:space:]]#.*$/, "", frag)   # a trailing comment
				if (frag ~ /curl/ && frag ~ url) print frag
			}
			buf = ""
		}
	' "$1"
}

for f in "$makefile" "$workflow"; do
	n=$(fetches "$f" | grep -c . || true)
	if [ "$n" -ne 1 ]; then
		bad "$f has $n spec-fetch curl commands, expected exactly 1 — this gate no longer matches what it guards"
		continue
	fi

	line=$(fetches "$f")

	for flag in "--proto '=https'" '--tlsv1.2' '-fsS' '--max-time' '--remove-on-error'; do
		case $line in
		*"$flag"*) ;;
		*) bad "$f: the spec fetch no longer passes $flag" ;;
		esac
	done

	# -L would let whatever host a Location names supply the bytes. Match the
	# short form only as a separate word or inside a bundle, so --location and
	# a bare -L are both caught while, say, --tlsv1.2 is not.
	case $line in
	*--location*) bad "$f: the spec fetch passes --location; a redirect must not choose the bytes" ;;
	esac
	if printf '%s\n' "$line" | grep -Eq '(^|[[:space:]])-[a-zA-Z]*L'; then
		bad "$f: the spec fetch passes -L; a redirect must not choose the bytes"
	fi
done

# The content pin stays a human decision. Only the recipe's own command
# lines count: a sed range ending at the next line starting with a letter
# would sweep in the following comment block, so a comment DOCUMENTING this
# very rule ("never regenerate SpecSHA256 with sha256sum") would trip the
# gate and accuse the maintainer of what it says not to do.
recipe=$(awk '/^update-spec:/ { r = 1; next } r && !/^\t/ { exit } r && !/^[[:space:]]*#/' "$makefile")
case $recipe in
*sha256* | *shasum* | *SpecSHA256*) bad "$makefile: update-spec computes a digest; contract.SpecSHA256 must stay a deliberate human edit, or the content pin asserts nothing" ;;
esac

if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "ok: the spec-fetch flags and the content-pin boundary are intact"
