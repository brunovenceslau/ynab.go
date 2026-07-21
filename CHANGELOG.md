# Changelog

All notable changes to this module are documented here, in
[Keep a Changelog](https://keepachangelog.com/) style. Versions follow
[semantic versioning](https://semver.org/).

## [Unreleased]

## [1.6.1] - 2026-07-21

### Added

- `examples/` directory with runnable programs, most of which call the live API —
  quickstart, delta-sync (with a persisted cursor), split transactions,
  and mock-based testing. Complements the offline godoc examples on
  pkg.go.dev, which stay `httptest`-backed so they run and verify in CI.

## [1.6.0] - 2026-07-21

Version numbering note: this is the first release of the rewrite, and
it starts at 1.6.0 rather than 1.0.0. Nineteen older versions are
permanently cached by the Go module proxy under this path: this
module's own pre-rewrite tag v0.1.0 (the old code, go.mod already
declaring this path — currently the proxy's `@latest`), plus the
predecessor's v1.0.0-v1.5.0. Retract requires the retracting version
to be the highest, so 1.6.0 sits above the range and retracts it (see
`retract` in go.mod): `go get pkg.venceslau.dev/ynab` now resolves to
1.6.0, and none of the old versions can be selected by `@latest` or a
version range. Explicitly pinning a retracted version still downloads
it with a warning — that is Go's designed behavior, and the reason
retract exists rather than deletion.

The greenfield rewrite: a new, frozen public surface covering all 44
operations of the YNAB API v1 (OpenAPI 1.86.0).

### Added

- Domain-first surface: `client.Plan(id).Categories.Assign(...)` — one
  package, services grouped under an I/O-free plan handle.
- Exact money: `Milliunits` (int64 thousandths) with `SplitEven`, parsing,
  and `CurrencyFormat` rendering; floats never carry money.
- `Optional[T]` tri-state for writes (omitted / null / value) on Go's
  `omitzero` — deliberately set zero values always reach the wire.
- First-class delta sync: `ServerKnowledge` cursors, `Since`, `SyncState`,
  `Plan.Delta`, and `MergeByID` with tombstone handling.
- Sentinel error taxonomy with class + sub-code matching, pre-flight
  `*ArgumentError` for spec-stated invariants, and built-in write-safe
  retries.
- Zero runtime dependencies; `testify` and `kin-openapi` test-only.

### Upgrading from the archived v1.x line

This module replaces the archived v1.x releases. The import path is now
`pkg.venceslau.dev/ynab` and the API is a clean break — the old
per-resource packages, pointer-heavy models, and `budgets` naming are
gone. Start from the
[package documentation](https://pkg.go.dev/pkg.venceslau.dev/ynab) and its
examples; the concepts map one-to-one (budget → plan), but no source
compatibility is provided or implied.

Existing consumers of the predecessor keep working: its released
versions stay downloadable from the module proxy under their original
paths — `go.bmvs.io/ynab` for v1.0.0-v1.3.0, then
`github.com/brunomvsouza/ynab.go` for v1.1.4-v1.5.0 — and the cache
survives any repository rename. The `archive/v*` tags preserve that
history for humans. (Under `pkg.venceslau.dev/ynab` the old versions
are retracted: `@latest` and version ranges never pick them, though an
explicit pin still resolves with a warning.)

[Unreleased]: https://github.com/brunovenceslau/ynab.go/compare/v1.6.1...HEAD
[1.6.1]: https://github.com/brunovenceslau/ynab.go/releases/tag/v1.6.1
[1.6.0]: https://github.com/brunovenceslau/ynab.go/releases/tag/v1.6.0
