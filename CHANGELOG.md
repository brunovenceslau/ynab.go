# Changelog

All notable changes to this module are documented here, in
[Keep a Changelog](https://keepachangelog.com/) style. Versions follow
[semantic versioning](https://semver.org/).

## [Unreleased]

## [1.6.0] - 2026-07-18

Version numbering note: this is the first release of the rewrite, and
it starts at 1.6.0 rather than 1.0.0. The archived predecessor's tags
(v0.1.0-v1.5.0) were cached by the Go module proxy under this module
path before their rename to `archive/*`; those cache entries are
permanent, and Go's retract mechanism requires the retracting version
to be the highest. 1.6.0 sits above them and retracts the range — see
`retract` in go.mod — so `go get pkg.venceslau.dev/ynab` resolves
cleanly.

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
- Zero runtime dependencies; `github.com/stretchr/testify` test-only.

### Upgrading from the archived v1.x line

This module replaces the archived v1.x releases. The import path is now
`pkg.venceslau.dev/ynab` and the API is a clean break — the old
per-resource packages, pointer-heavy models, and `budgets` naming are
gone. Start from the
[package documentation](https://pkg.go.dev/pkg.venceslau.dev/ynab) and its
examples; the concepts map one-to-one (budget → plan), but no source
compatibility is provided or implied.

Existing consumers keep working untouched: every released version
(v0.1.0–v1.5.0) is cached permanently by the Go module proxy under its
original path — `go get github.com/brunomvsouza/ynab.go@v1.5.0` — and
that cache survives any repository rename. The `archive/v*` tags in this
repository preserve the same history for humans; they cannot be fetched
as `pkg.venceslau.dev/ynab` because their `go.mod` declares the old
module path.

[Unreleased]: https://github.com/brunovenceslau/ynab.go/compare/v1.6.0...HEAD
[1.6.0]: https://github.com/brunovenceslau/ynab.go/releases/tag/v1.6.0
