#!/usr/bin/env sh
# Release gate: the tag being released must be plain vX.Y.Z semver and
# equal the Version constant baked into the User-Agent, and the vanity
# host must still serve the go-import meta tag that makes
# pkg.venceslau.dev/ynab resolvable. CHECK_VERSION_SKIP_NET=1 skips the
# vanity-host check so the pure half is testable offline (CI self-test).
# Usage: check-version.sh vX.Y.Z
set -eu

tag="${1:?usage: check-version.sh vX.Y.Z}"

if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
	echo "error: tag '$tag' is not a plain vX.Y.Z semver tag" >&2
	exit 1
fi

version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' client.go)
if [ -z "$version" ]; then
	echo "error: could not read the Version constant from client.go" >&2
	exit 1
fi

if [ "$tag" != "v$version" ]; then
	echo "error: tag '$tag' does not match Version constant '$version' (expected v$version)" >&2
	exit 1
fi
echo "ok: tag $tag matches Version $version"

if [ "${CHECK_VERSION_SKIP_NET:-}" = "1" ]; then
	echo "ok: vanity check skipped (CHECK_VERSION_SKIP_NET=1)"
	exit 0
fi

meta=$(curl --proto '=https' --tlsv1.2 --max-time 30 -fsSL 'https://pkg.venceslau.dev/ynab?go-get=1')
expected='pkg.venceslau.dev/ynab git https://github.com/brunovenceslau/ynab'
if ! printf '%s\n' "$meta" | grep -qF "$expected"; then
	echo "error: vanity host is not serving the go-import meta tag:" >&2
	echo "  expected to find: <meta name=\"go-import\" content=\"$expected\">" >&2
	exit 1
fi
echo "ok: vanity go-import meta tag present"
