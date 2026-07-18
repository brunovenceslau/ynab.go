# ynab

[![Go Reference](https://pkg.go.dev/badge/pkg.venceslau.dev/ynab.svg)](https://pkg.go.dev/pkg.venceslau.dev/ynab)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8)](go.mod)
[![CI](https://img.shields.io/github/actions/workflow/status/brunovenceslau/ynab/ci.yaml?branch=main&label=ci)](.github/workflows/ci.yaml)
[![License](https://img.shields.io/badge/license-BSD--2--Clause-blue)](LICENSE)

The definitive Go client for the [YNAB API v1](https://api.ynab.com): all 44
operations behind a domain-first surface, exact money arithmetic, first-class
delta sync, and **zero runtime dependencies**.

```go
client := ynab.New(os.Getenv("YNAB_TOKEN"))
plan := client.Plan(ynab.PlanIDLastUsed)

accounts, knowledge, err := plan.Accounts.List(ctx)
```

## Install

```sh
go get pkg.venceslau.dev/ynab
```

Requires Go 1.25+.

## The plan handle

Everything plan-scoped hangs off the handle `client.Plan(id)` returns — built
without I/O, bound to its id for good:

```go
plan := client.Plan(ynab.PlanIDLastUsed)

groups, knowledge, err := plan.Categories.List(ctx)
category, knowledge, err := plan.Categories.Assign(ctx, ynab.CurrentMonth(), categoryID, ynab.UnitsToMilliunits(150))
detail, knowledge, err := plan.Export(ctx) // the whole plan, one request
```

Money is `Milliunits` (exact int64 thousandths — never floats), months are a
dedicated `Month` type that accepts the server-resolved `ynab.CurrentMonth()`,
and write payloads use the `Optional` tri-state: omitted, `SetNull` (clear), or
`Set` — zero values included, so `Approved: ynab.Set(false)` reaches the wire.

## Delta sync

Delta-capable reads return a `ServerKnowledge` cursor; hand it back to receive
only what changed, tombstones included:

```go
st := &ynab.SyncState{} // JSON-persistable; save it between runs

detail, err := plan.Delta(ctx, st) // full read first, increments after
store = ynab.MergeByID(store, detail.Transactions)
```

See the [package examples](https://pkg.go.dev/pkg.venceslau.dev/ynab#pkg-examples)
for the full delta loop and the flatten-before-merge categories caveat.

## Errors

API failures match a sentinel taxonomy through `errors.Is`, class- and
sub-code-wise at once:

```go
_, _, err := plan.Transactions.Create(ctx, spec)
switch {
case errors.Is(err, ynab.ErrConflict):    // duplicate import_id
case errors.Is(err, ynab.ErrRateLimited): // wait — a *ynab.Error carries RetryAfter
case errors.Is(err, ynab.ErrForbidden):   // any 403.*, including 403.1 subscription lapsed
}
```

Client-side pre-flight failures (a zero `Month`, a 501-character payee name,
split legs that do not sum) are `*ArgumentError` — the request is never sent.
Retries for 429/500/503 are built in, write-safe by default, and
`IsRetryable` exposes the same classification for custom loops.

## Testing your integration

Two dependency-free seams, both with runnable examples: point `WithBaseURL`
at an `httptest.Server`, or install a fake `http.RoundTripper` via
`WithHTTPClient`. You need nothing from this library's internals.

## Reliability

Correctness is enforced by machinery, not vigilance: the operation table is
diffed both ways against the vendored OpenAPI spec on every CI run, write
bodies are asserted byte-exact including their emitted key sets, every decode
runs twice with all optional response headers stripped, every nullable field
has a null-variant fixture, and a weekly job diffs the live spec for drift.
Observed-vs-documented API divergences live in [API_NOTES.md](API_NOTES.md).

## Support

Issues are acknowledged within **14 days** and receive an accept/decline
decision within **60 days**. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
gate workflow before opening a PR.

## License

[BSD 2-Clause](LICENSE). Not affiliated with YNAB — use at your own risk.
