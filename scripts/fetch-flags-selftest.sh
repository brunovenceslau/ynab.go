#!/usr/bin/env bash
# Copyright 2026 Bruno Venceslau. All rights reserved.
# Use of this source code is governed by a BSD-2-Clause
# license that can be found in the LICENSE file.
#
# The network-free proof that fetch-flags.sh is not vacuous. Twin of
# apidiff-selftest, check-version-selftest and spec-shape-selftest.
#
# A grep gate's characteristic failure is passing on everything: one wrong
# pattern and it asserts nothing while reporting ok. So every flag it claims
# to require is removed in turn and must be caught, and the -L it must NOT
# police elsewhere is exercised too.
#
# Fixtures are hand-written rather than derived from the real Makefile: the
# gate's whole job is to notice the real files changing, so deriving the
# fixtures from them would couple the proof to the thing under test.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
gate="$here/fetch-flags.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail=0
note() { echo "FAIL: $1"; fail=1; }

url=https://api.ynab.com/papi/open_api_spec.yaml
flags="--proto '=https' --tlsv1.2 --max-time 60 -fsS --remove-on-error"

# mkmake <path> <curl-flags> [extra-recipe-line]
mkmake() {
	{
		printf 'update-spec:\n'
		printf '\tcurl %s \\\n' "$2"
		printf '\t\t%s -o openapi.yaml\n' "$url"
		[ "$#" -ge 3 ] && printf '\t%s\n' "$3"
		printf '\tgit diff --stat openapi.yaml\n'
		printf '\n'
		printf 'go-latest-check:\n'
		# A legitimate -L elsewhere: parses a string, vendors nothing.
		printf "\t@curl --proto '=https' --tlsv1.2 -fsSL 'https://go.dev/dl/?mode=json'\n"
	} > "$1"
}

mkflow() {
	{
		printf 'name: spec-drift\n'
		printf 'jobs:\n  diff:\n    steps:\n      - run: |\n'
		printf '          curl %s \\\n' "$2"
		printf '            %s -o /tmp/live_spec.yaml\n' "$url"
	} > "$1"
}

# accepts <name>
accepts() {
	if "$gate" "$tmp/Makefile" "$tmp/drift.yaml" >/dev/null 2>&1; then
		echo "ok: $1"
	else
		note "$1 — the gate rejected a compliant pair"
	fi
}

# rejects <name> <expected-exit>
rejects() {
	local rc=0
	"$gate" "$tmp/Makefile" "$tmp/drift.yaml" >/dev/null 2>&1 || rc=$?
	if [ "$rc" -eq "$2" ]; then
		echo "ok (fail): $1"
	else
		note "$1 — expected exit $2, got $rc"
	fi
}

mkmake "$tmp/Makefile" "$flags"
mkflow "$tmp/drift.yaml" "$flags"
accepts "a compliant pair passes, and a legitimate -L elsewhere does not trip it"

# Each required flag, removed in turn, from each file.
for missing in "--proto '=https'" '--tlsv1.2' '--max-time 60' '-fsS' '--remove-on-error'; do
	stripped=${flags/$missing/}
	mkmake "$tmp/Makefile" "$stripped"
	mkflow "$tmp/drift.yaml" "$flags"
	rejects "Makefile dropping ${missing} is caught" 1

	mkmake "$tmp/Makefile" "$flags"
	mkflow "$tmp/drift.yaml" "$stripped"
	rejects "drift.yaml dropping ${missing} is caught" 1
done

# -L, in both spellings and both files.
mkmake "$tmp/Makefile" "$flags -L"
mkflow "$tmp/drift.yaml" "$flags"
rejects "Makefile regaining -L is caught" 1

mkmake "$tmp/Makefile" "${flags/-fsS/-fsSL}"
mkflow "$tmp/drift.yaml" "$flags"
rejects "Makefile regaining L inside a bundle is caught" 1

mkmake "$tmp/Makefile" "$flags"
mkflow "$tmp/drift.yaml" "$flags --location"
rejects "drift.yaml regaining --location is caught" 1

# The content pin must stay a human edit.
mkmake "$tmp/Makefile" "$flags" 'sha256sum openapi.yaml > openapi.yaml.sha256'
mkflow "$tmp/drift.yaml" "$flags"
rejects "update-spec computing a digest is caught" 1

# The gate must notice when it no longer matches what it guards.
mkmake "$tmp/Makefile" "$flags"
mkflow "$tmp/drift.yaml" "$flags"
printf 'update-spec:\n\tcurl %s -o openapi.yaml\n' "$flags" > "$tmp/Makefile"
rejects "a spec fetch the gate cannot find is caught" 1

{
	printf 'update-spec:\n'
	printf '\tcurl %s \\\n\t\t%s -o openapi.yaml\n' "$flags" "$url"
	printf '\tcurl %s \\\n\t\t%s -o copy.yaml\n' "$flags" "$url"
} > "$tmp/Makefile"
mkflow "$tmp/drift.yaml" "$flags"
rejects "two spec fetches in one file is caught" 1

# The three ways a naive line-matching gate is fooled. Each of these passed
# an earlier version of fetch-flags.sh while the live fetch ran unsafely.
mkflow "$tmp/drift.yaml" "$flags"
{
	printf 'SPEC_URL := %s\n' "$url"
	printf '# Canonical fetch, for reference:\n'
	printf '#   curl %s %s -o openapi.yaml\n' "$flags" "$url"
	# The dollar comes from a variable so the fixture carries no quoted
	# $( ) for the linter to object to. What matters to the test is only
	# that the recipe line carries no URL for the gate to anchor on.
	printf 'update-spec:\n\tcurl -L -k -o openapi.yaml %s(SPEC_URL)\n' '$'
} > "$tmp/Makefile"
rejects "a commented-out canonical fetch does not vouch for the live one" 1

{
	printf 'update-spec:\n'
	printf '\tcurl -o openapi.yaml %s \\\n' "$url"
	printf '\t\t# flags: %s\n' "$flags"
} > "$tmp/Makefile"
rejects "flags demoted to a trailing comment do not count" 1

mkmake "$tmp/Makefile" "$flags"
{
	printf 'name: spec-drift\njobs:\n  diff:\n    steps:\n      - run: |\n'
	printf '          curl %s \\\n' "$flags"
	printf '            %s -o /tmp/probe.yaml; \\\n' "$url"
	printf "            curl --insecure --proto '=http' -o /tmp/live_spec.yaml %s\n" "$url"
} > "$tmp/drift.yaml"
rejects "a compliant probe does not vouch for an unsafe fetch beside it" 1

# And the two false positives the narrowing removed: the gate must not fire
# on a -L belonging to another command, nor on a comment documenting its own
# rule.
mkflow "$tmp/drift.yaml" "$flags"
mkmake "$tmp/Makefile" "$flags" 'cp -L /tmp/staged openapi.yaml'
accepts "a -L on another command in the recipe is not the fetch's"

mkmake "$tmp/Makefile" "$flags"
printf '\n# NOTE: never regenerate contract.SpecSHA256 with sha256sum.\n' >> "$tmp/Makefile"
accepts "a comment documenting the digest rule does not trip it"

# Fail closed on its own inputs.
mkmake "$tmp/Makefile" "$flags"
rm -f "$tmp/drift.yaml"
rejects "an unreadable input fails closed" 2

mkflow "$tmp/drift.yaml" "$flags"
for argc in 0 1 3; do
	rc=0
	case $argc in
	0) "$gate" >/dev/null 2>&1 || rc=$? ;;
	1) "$gate" "$tmp/Makefile" >/dev/null 2>&1 || rc=$? ;;
	3) "$gate" "$tmp/Makefile" "$tmp/drift.yaml" extra >/dev/null 2>&1 || rc=$? ;;
	esac
	if [ "$rc" -eq 2 ]; then
		echo "ok (fail): a ${argc}-argument call fails closed"
	else
		note "a ${argc}-argument call must exit 2, got $rc"
	fi
done

if [ "$fail" -ne 0 ]; then
	echo "fetch-flags gate self-test FAILED"
	exit 1
fi
echo "ok: fetch-flags gate classifies every case correctly"
