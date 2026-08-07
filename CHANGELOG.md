# Changelog

All notable changes to this module are documented here, in
[Keep a Changelog](https://keepachangelog.com/) style. Versions follow
[semantic versioning](https://semver.org/).

## [Unreleased]

### Changed

- Re-vendored `openapi.yaml` from the live YNAB spec. Upstream amended one
  description — `SaveTransactionWithOptionalFields.subtransactions`, the
  shared save-transaction schema — without bumping the spec version, which
  stays 1.86.0. No path, field, type, or enum moved, so the public surface is
  unchanged and no upgrade action is needed.
- Documented the two split-write rules the amended description states, on
  `TransactionSpec.Splits` and `TransactionUpdate`: the API rejects splits on
  tracking accounts and on transfers between on-budget accounts (a transfer
  *to* a tracking account may be split), and updating the subtransactions of
  an existing split transaction returns an error. Both are enforced by the
  server: the write payload does not carry account type, so the
  tracking-account rule cannot be checked before the request; the update rule
  is structurally unreachable — `TransactionUpdate` carries no split legs.
  Godoc only; no behavior change.

### Security

- Hardened the fetch that vendors `openapi.yaml`, in `make update-spec` and in
  the drift workflow that mirrors it. Dropped `-L`: the endpoint answers 200
  with zero redirects, and following one would let whatever host a `Location`
  header names supply the bytes — measured against a local server, `-fsSL`
  silently vendors a cross-host redirect's body. Added `--remove-on-error`, so
  a partial download is deleted rather than left on disk as truncated YAML
  (also measured), which matters because every operation and the version line
  precede `components:`, so a body cut there still scans as 44 valid ops and
  passes every offline gate. Added `--max-time` to match the workflow.
  No code, API, or behavior change; the vendored spec this fetch writes ships
  inside the module zip, so its provenance is part of the release supply chain.
- Pinned `openapi.yaml` by content: `contract.SpecSHA256`, asserted by a new
  contract test. The version pin could not do this — upstream amends the spec
  in place without moving `info.version`, exactly as it did earlier in this
  same `[Unreleased]` block. Measured before the pin: a hostile `servers:` url,
  a `maxLength` tightened from 500 to 50, an amended description, a body
  truncated to 53% of its bytes, and a single appended newline each passed
  every offline gate; all five now fail. Re-vendoring becomes two edits in
  one commit — the spec and the constant — which is the deliberate act the
  project already required but could not enforce. The assertion is test-only
  and the constant lives in `internal/`, so no public surface moved.
- Pinned the transcribed wire constants to the spec they came from. The five
  `maxLength` constants and the five string-enum sets were copied out of
  `openapi.yaml` by hand and, until now, nothing checked they still matched:
  the content pin notices that the spec moved, but a maintainer who reviews
  the diff and re-pins the digest can still leave a stale bound behind, and
  the client would go on rejecting payloads the server accepts. Two contract
  tests now diff both, in both directions — a changed value fails on
  comparison, and a bound or enum upstream *adds* fails on completeness,
  which is the case a value check can never see. Mutation-tested: a
  `maxLength` moved, an enum value added, one removed, a new unmapped bound,
  and either Go side edited — six for six. Test-only; no public surface.
- `make update-spec` now fetches to a temp file and only replaces the vendored
  spec once `scripts/spec-shape.sh` agrees the download carries the same
  operation count as the file it would overwrite. A shape check, not a
  validity one — the content pin is what covers the bytes. This closes the gap the
  fetch flags cannot reach: a redirect is not an error under `-f`, so
  `--remove-on-error` never fires on one, and curl wrote the 302's own body —
  attacker-chosen, not merely empty — at exit 0, destroying the vendored
  artifact while the recipe reported success. Three controls now, none
  subsuming another: curl's non-zero exit covers a truncated transfer, which
  keeps the operation count and so is invisible to the shape check; the shape
  check covers a body whose count differs; the content pin covers the bytes. A network-free
  self-test of 17 fixtures, wired into `local-ci` and CI, pins the parts a
  coarse suite would miss — a near-miss pair fixing *how* the count is
  computed, distinct exit codes for "not a spec" and "called wrong", a
  missing or operationless baseline, and a refusal to answer at all when the
  input cannot be scanned. Tooling only; no module byte changes.

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
