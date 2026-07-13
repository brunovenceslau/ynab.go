# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Import path history

| Stage      | Import path                        | Notes                                             |
| ---------- | ---------------------------------- | ------------------------------------------------- |
| Origin     | `github.com/brunomvsouza/ynab.go`  | Original upstream module. Tags `v1.0.0`–`v1.5.0`. |
| Fork home  | `github.com/brunovenceslau/ynab.go`| Repository this module is hosted from.            |
| **Now**    | `pkg.venceslau.dev/ynab`           | Vanity import path. Versioning restarts at `v0.1.0`. |

The module is imported as:

```go
import "pkg.venceslau.dev/ynab"
```

The vanity path `pkg.venceslau.dev/ynab` redirects `go get` to the
`github.com/brunovenceslau/ynab.go` repository via a `go-import` meta tag.

## Versioning reset

With the move to the `pkg.venceslau.dev/ynab` import path, versioning was
restarted from `v0.1.0`.

A `0.x` series signals that the public API may still evolve without the
compatibility guarantees that a `1.x` release implies. The API will be promoted
to `v1.0.0` once its surface is considered stable under the new import path.

`v0.1.0` is the same code that was previously released as `v1.5.0` under the old
`github.com/brunomvsouza/ynab.go` import path — only the module path changed, the
public API is unchanged.

## Archived releases (`v1.0.0`–`v1.5.0`)

The `v1.0.0` through `v1.5.0` tags and their GitHub Releases are kept as
historical record. They belong to the previous `github.com/brunomvsouza/ynab.go`
import path and their `go.mod` still declares that path, so they are **not valid
versions of `pkg.venceslau.dev/ynab`** and cannot be fetched under the new import
path. They are retained only to preserve the release notes and contributor
history; new versions start at `v0.1.0`.

Because `v1.5.0` is a higher semantic version than `v0.1.0`, GitHub may keep
showing it under the "Latest release" badge. The `v0.1.0` release is pinned as
the latest release manually; the `v1.x` tags remain available for reference only.

## [0.1.0]

Initial release under the `pkg.venceslau.dev/ynab` import path.

### Changed

- Module path changed to `pkg.venceslau.dev/ynab` (was
  `github.com/brunomvsouza/ynab.go`). Update your imports accordingly:
  ```go
  import "pkg.venceslau.dev/ynab"
  ```

Everything else is identical to the previous `v1.5.0` release under the old
import path, including the removal of the discontinued rate-limit functionality
and the `DeleteTransaction` API call.

[0.1.0]: https://github.com/brunovenceslau/ynab.go/releases/tag/v0.1.0
