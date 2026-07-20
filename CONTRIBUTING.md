# Contributing

Thanks for helping make this the Go client YNAB deserves. Correctness is
enforced by machinery, not vigilance — the gates below are not optional,
and they are the same ones CI runs.

## Getting started

```sh
git clone https://github.com/brunovenceslau/ynab.go
cd ynab.go
make lint test contract
```

That's the whole setup: a Go 1.25+ toolchain and `make`. Linting runs
golangci-lint through `go run` at a pinned version — nothing to install,
nothing at `@latest`.

## The gate workflow

Before every commit:

```sh
make lint       # go vet + golangci-lint (pinned) — zero suppressions policy
make test       # go test -race -shuffle=on ./...
make contract   # the G1–G5 contract gates against the vendored spec
```

A change that fails any gate does not merge. The contract gates are the
project's thesis: the operation table diffs both ways against the vendored
`openapi.yaml`, write bodies are byte-exact with their emitted key sets,
struct tags are lint-checked over the wire models, every decode runs
header-stripped, and every nullable field needs a null-variant fixture. If
your change adds surface, the gates will tell you exactly which registrations
are missing.

## Ground rules

- **Wire truth over memory.** Every claim about API behavior must be
  confronted with the vendored `openapi.yaml` or https://api.ynab.com.
  Observed divergences go into `API_NOTES.md` at the moment of discovery —
  never ship a workaround without its ledger entry.
- **Zero runtime dependencies.** `github.com/stretchr/testify` is the only
  test dependency. Adding any other dependency — including test-only — is an
  ask-first change: open an issue before the PR.
- **Surface changes are spec-before-code.** The public API is deliberate and
  frozen per release. Propose surface changes in an issue first; PRs that
  grow the surface without prior agreement will be declined regardless of
  quality.
- **Tests ship with the change**, in the same PR: unit/endpoint tests through
  the public surface plus a live-integration case (`//go:build integration`)
  registering its operation ids. `-race` and `t.Parallel` discipline apply;
  the integration suite stays sequential.
- **User-visible changes get a CHANGELOG line** under `[Unreleased]`, in the
  same PR.
- **Do not run `make update-spec`** casually — it overwrites the pinned
  vendored spec. If you did: `git checkout -- openapi.yaml`. Re-vendoring is
  ask-first.

## Which tests does my change need?

Every kind of test here exists to kill a specific class of bug — pick by
what your change touches:

- **Unit tests** (plain `_test.go`): the behavior of what you wrote,
  through the public surface. Always.
- **Endpoint cases** (`registerEndpointCase`, gate G4/G5): any operation
  that decodes a response. They run every decode twice with optional
  headers stripped and prove every nullable field against a null-variant
  fixture — the class of bug they kill is "works on my fixture".
- **Write cases** (`registerWriteCase`, gate G2): any operation that
  sends a body. Byte-exact bodies including the emitted key set — the
  bug they kill is a stray or missing JSON key the server silently
  accepts today and rejects tomorrow.
- **Wire-model registration** (`registerWriteModel`/`registerReadModel`,
  gate G3): any new request/response struct. A reflection lint over
  tags and Optional/omitzero discipline.
- **Live-integration case** (`registerIntegrationCase`, tokenless
  completeness gate + `make integration`): any new operation. The only
  layer that catches the server disagreeing with the vendored spec.
- **Fuzz targets**: any hand-written parser. No-panic plus round-trip.
- **Examples**: any new user-facing concept — they are documentation
  that cannot rot, and `go test` runs them.

The gates will list exactly which registrations are missing if you
forget one.

## Triage promise

Issues are acknowledged within 14 days and resolved — an accept/decline
decision, and the fix when accepted — within 30 days of acknowledgement.
Small, well-gated PRs get the fastest reviews. This is a one-person
project; the numbers are deliberately modest so they can be kept.
