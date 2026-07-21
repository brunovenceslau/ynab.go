#!/usr/bin/env bash
# apidiff.sh — the public-surface compatibility gate.
#
# gorelease (golang.org/x/exp; its engine is x/exp/apidiff) diffs our
# exported API against the latest release of THIS module's family (v1.6.0
# and up — v0.1.0 still exists upstream under the archived predecessor's
# module path, so an implicit base would produce a garbage cross-module
# diff). Before the first family release there is no base: the gate says so
# and passes.
#
# One change is deliberately exempt: a *value* bump of the exported `Version`
# constant. Its value changes on every single release by construction, and
# apidiff — correctly, by its own rules — treats a changed constant value as
# an incompatible change (a const's value is usable in constant expressions,
# so it is part of the API). But the version string is release metadata, not
# a compatibility-bearing part of the surface: a value bump of `Version` alone
# must never gate a release. Note the exemption is on the value-change message
# only — removing `Version`, or changing its kind (const→var/func) or type,
# keeps its own distinct apidiff message and still fails the gate, as does
# every other incompatible change. So a real break cannot ride the exemption.
#
# Usage: apidiff.sh <gorelease-command...>
#   e.g. CGO_ENABLED=0 apidiff.sh go run golang.org/x/exp/cmd/gorelease@vX
set -euo pipefail

if [ "$#" -eq 0 ]; then
	echo 'apidiff.sh: missing gorelease command' >&2
	exit 2
fi

# Prereleases (tags containing '-', e.g. v1.7.0-rc1) are never a compat base:
# sort -V orders them adjacent to the final tag and they are not shipped API.
# The `|| true` keeps the pipeline from tripping `set -e` when grep finds none.
base=$(git tag -l 'v1.[6-9]*' 'v1.[1-9][0-9]*' 'v[2-9]*' |
	{ grep -v -- '-' || true; } | sort -V | tail -1)
if [ -z "$base" ]; then
	echo 'apidiff: no released tag in this module family yet — activates after v1.6.0'
	exit 0
fi

# Run gorelease, capturing output and exit code without tripping `set -e`.
set +e
out=$("$@" -base="$base" 2>&1)
rc=$?
set -e
printf '%s\n' "$out"

if [ "$rc" -eq 0 ]; then
	exit 0
fi

# The gate failed. Collect every line that describes an incompatible change —
# the body lines under each package's "## incompatible changes" heading.
# (A heading or a new "# package" line ends the section; blank lines are
# dropped by `NF`.)
incompat=$(printf '%s\n' "$out" |
	awk '/^## incompatible changes/{f=1;next} /^#/{f=0} f && NF')

# Exactly one message is exempt: a value bump of the top-level Version const,
# `Version: value changed from ...`. The exemption relies on Version living in
# the root package — its message carries no "./pkg." path prefix; if it ever
# moves to a subpackage this stops matching and the gate reverts to failing on
# every release, loudly. Removal / kind-change / retype of Version keep their
# own messages and are NOT exempt.
#
# If there is at least one incompatible change and every one is that exempt
# value bump, the surface is compatible in every way that matters. Otherwise a
# real break slipped in — fail with gorelease's own exit code.
exempt='^Version: value changed from '
nonexempt=$(printf '%s\n' "$incompat" | grep -v "$exempt" || true)
if [ -n "$incompat" ] && [ -z "$nonexempt" ]; then
	echo
	echo 'apidiff: the only incompatible change is a Version value bump (release'
	echo '         metadata, exempt by policy) — the public surface is compatible:'
	printf '%s\n' "$incompat" | grep "$exempt" | sed 's/^/           /' || true
	exit 0
fi
exit "$rc"
