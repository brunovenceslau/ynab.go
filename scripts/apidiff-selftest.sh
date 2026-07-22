#!/usr/bin/env bash
# apidiff-selftest.sh — the network-free proof that apidiff.sh classifies
# gorelease output correctly. It is the twin of check-version-selftest: a
# handful of positive cases and "expected failures failed" negatives.
#
# The gate takes the gorelease command as argv, so we inject a *fake* gorelease
# that prints canned output and returns a chosen exit code — the whole
# classifier runs offline, with no real gorelease and no crafted API break.
# A throwaway git repo carrying a v1.6.0 tag stands in for the module family,
# so the run is hermetic against the real checkout's tags.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
gate="$here/apidiff.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fake="$tmp/fake-gorelease.sh"
cat >"$fake" <<'STUB'
#!/usr/bin/env bash
# Ignores its args (the gate appends -base=...); prints a fixture, exits a code.
cat "$FAKE_OUT"
exit "${FAKE_RC:-1}"
STUB
chmod +x "$fake"

repo="$tmp/repo"
mkdir "$repo"
git -C "$repo" init -q
git -C "$repo" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
# -c tag.*=false: a host may globally force signed/annotated tags; this
# throwaway base tag needs neither.
git -C "$repo" -c tag.gpgSign=false -c tag.forceSignAnnotated=false tag v1.6.0

fixture() { printf '%s\n' "$2" >"$tmp/$1"; }

# Exit code of the real gate when fed fixture $1 with fake exit code $2.
run() (
	cd "$repo"
	FAKE_OUT="$tmp/$1" FAKE_RC="$2" "$gate" "$fake" >/dev/null 2>&1
	echo $?
)

fail=0
pass_case() { # fixture rc label
	got=$(run "$1" "$2")
	if [ "$got" != 0 ]; then
		echo "FAIL: $3 — gate should PASS but exited $got"
		fail=1
	else
		echo "ok (pass): $3"
	fi
}
fail_case() { # fixture rc label
	got=$(run "$1" "$2")
	if [ "$got" = 0 ]; then
		echo "FAIL: $3 — gate should FAIL but passed a break"
		fail=1
	else
		echo "ok (fail): $3"
	fi
}

fixture only_version '# pkg.venceslau.dev/ynab
## incompatible changes
Version: value changed from "1.6.0" to "1.6.1"

# summary
Incompatible changes were detected.'

fixture version_plus_break '# pkg.venceslau.dev/ynab
## incompatible changes
Version: value changed from "1.6.0" to "1.6.1"
WithTimeout: changed from func(time.Duration) Option to func(time.Duration, int) Option

# summary'

fixture break_only '# pkg.venceslau.dev/ynab
## incompatible changes
Client.RawDo: removed

# summary'

fixture version_removed '# pkg.venceslau.dev/ynab
## incompatible changes
Version: removed

# summary'

fixture version_const_to_var '# pkg.venceslau.dev/ynab
## incompatible changes
Version: changed from const to var

# summary'

fixture version_like_name '# pkg.venceslau.dev/ynab
## incompatible changes
VersionInfo: removed

# summary'

fixture cross_package '# pkg.venceslau.dev/ynab
## incompatible changes
Version: value changed from "1.6.0" to "1.6.1"

# pkg.venceslau.dev/ynab/internal/transport
## incompatible changes
Core.Do: removed

# summary'

fixture clean '# pkg.venceslau.dev/ynab/examples/quickstart
## compatible changes
package added

# summary
Suggested version: v1.6.1'

fixture crash 'gorelease: internal error resolving base'

# Positives: the surface is compatible.
pass_case only_version 1 'only a Version value bump is exempt'
pass_case clean 0 'compatible-only diff (gorelease exit 0)'

# Negatives: a real break must fail the gate.
fail_case version_plus_break 1 'Version bump + a real break behind it'
fail_case break_only 1 'a break with no Version line'
fail_case version_removed 1 'Version removed is NOT a value bump'
fail_case version_const_to_var 1 'Version const->var is NOT a value bump'
fail_case version_like_name 1 'VersionInfo is not the Version symbol'
fail_case cross_package 1 'a break in a second package still fails'
fail_case crash 2 'gorelease crash fails closed'

# The gate must reject an empty invocation rather than silently pass.
got=$( (cd "$repo" && "$gate" >/dev/null 2>&1; echo $?) )
if [ "$got" != 2 ]; then
	echo "FAIL: missing gorelease command should exit 2, got $got"
	fail=1
else
	echo 'ok (fail): missing gorelease command exits 2'
fi

if [ "$fail" -ne 0 ]; then
	echo 'apidiff-selftest: FAILED'
	exit 1
fi
echo 'ok: apidiff gate classifies every case correctly'
