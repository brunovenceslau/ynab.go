# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Versioning reset

This module now lives at the import path `pkg.venceslau.dev/ynab`.

With the move to the new import path, versioning was restarted from `v0.1.0`.
A `0.x` series signals that the public API may still evolve without the
compatibility guarantees that a `1.x` release implies. The API will be promoted
to `v1.0.0` once its surface is considered stable under the new import path.

`v0.1.0` is the same code that was previously released as `v1.5.0` under the old
`github.com/brunomvsouza/ynab.go` import path — only the module path changed, the
public API is unchanged. The old `v1.0.0`–`v1.5.0` tags belong to the previous
import path and are not valid versions of `pkg.venceslau.dev/ynab`.

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
