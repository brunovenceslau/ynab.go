# Contributing

Thanks for helping make this the Go client YNAB deserves. The bar here is
craftsmanship enforced by machinery — the gates below are not optional, and
they are the same ones CI runs.

## Getting started

```sh
git clone https://github.com/brunovenceslau/ynab
cd ynab
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
- **Do not run `make update-spec`** casually — it overwrites the pinned
  vendored spec. If you did: `git checkout -- openapi.yaml`. Re-vendoring is
  ask-first.

## Triage promise

Issues are acknowledged within 14 days and receive an accept/decline decision
within 60. Small, well-gated PRs get the fastest reviews.
