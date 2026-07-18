# ynab

[![Go Reference](https://pkg.go.dev/badge/pkg.venceslau.dev/ynab.svg)](https://pkg.go.dev/pkg.venceslau.dev/ynab)
[![CI](https://img.shields.io/github/actions/workflow/status/brunovenceslau/ynab.go/ci.yaml?branch=main&label=ci)](https://github.com/brunovenceslau/ynab.go/actions/workflows/ci.yaml)
[![License](https://img.shields.io/badge/license-BSD--2--Clause-blue)](LICENSE)

A complete Go client for the [YNAB API v1](https://api.ynab.com): all 44
operations behind a domain-first surface, exact money arithmetic, first-class
delta sync, and **zero runtime dependencies**. Not an OAuth flow, not a
persistence layer, not a budgeting engine — a wire-faithful client.

> YNAB's API says *budget*; this library says **plan**, following YNAB's own
> product vocabulary. `client.Plan(id)` maps to `/budgets/{budget_id}` on the
> wire — the mapping is one-to-one.

## Install

```sh
go get pkg.venceslau.dev/ynab
```

Requires Go 1.25+ (the minimum in [go.mod](go.mod)). The public API follows
[SemVer](https://semver.org): the v1 surface is frozen, and every change is
recorded in the [CHANGELOG](CHANGELOG.md).

Create a [Personal Access Token](https://api.ynab.com/#personal-access-tokens)
at app.ynab.com → Settings → Developer Settings and export it as `YNAB_TOKEN`.

## Quick start

A complete program — no IDs to look up, `PlanIDLastUsed` resolves server-side:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"pkg.venceslau.dev/ynab"
)

func main() {
	client := ynab.New(os.Getenv("YNAB_TOKEN"))
	plan := client.Plan(ynab.PlanIDLastUsed)

	accounts, _, err := plan.Accounts.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range accounts {
		fmt.Printf("%s  %s\n", a.Name, a.BalanceFormatted)
	}
	// Checking  $1,282.23   (your accounts will differ)
}
```

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

YNAB's quota is about 200 requests per hour — delta sync is how a polling
integration lives inside it. Delta-capable reads return a `ServerKnowledge`
cursor; hand it back to receive only what changed, tombstones included:

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

## Upgrading from the archived v1.x line

The archived `github.com/brunovenceslau/ynab.go` releases are replaced by this
module at `pkg.venceslau.dev/ynab`. It is a clean break — budget → plan, one
package, no source compatibility; the old tags remain available. See the
[CHANGELOG](CHANGELOG.md) for the migration summary.

## Support

Issues are acknowledged within **14 days** and receive an accept/decline
decision within **60 days**. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
gate workflow before opening a PR.

## License

[BSD 2-Clause](LICENSE). Not affiliated with YNAB — use at your own risk.
