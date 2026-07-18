# Changelog

All notable changes to this module are documented here, in
[Keep a Changelog](https://keepachangelog.com/) style. Versions follow
[semantic versioning](https://semver.org/).

## [Unreleased]

## [1.0.0] - 2026-07-18

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

This module replaces the archived `github.com/brunovenceslau/ynab.go`
releases. The import path is now `pkg.venceslau.dev/ynab` and the API is a
clean break — the old per-resource packages, pointer-heavy models, and
`budgets` naming are gone. Start from the
[package documentation](https://pkg.go.dev/pkg.venceslau.dev/ynab) and its
examples; the concepts map one-to-one (budget → plan), but no source
compatibility is provided or implied. The archived releases remain
available under their original tags for existing consumers.

[Unreleased]: https://github.com/brunovenceslau/ynab.go/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/brunovenceslau/ynab.go/releases/tag/v1.0.0
