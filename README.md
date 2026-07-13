# YNAB API Go Library

[![Go Report Card](https://goreportcard.com/badge/pkg.venceslau.dev/ynab)](https://goreportcard.com/report/pkg.venceslau.dev/ynab) [![Go Reference](https://pkg.go.dev/badge/pkg.venceslau.dev/ynab.svg)](https://pkg.go.dev/pkg.venceslau.dev/ynab)

This is an UNOFFICIAL Go client for the YNAB API. It covers 100% of the resources made available by the [YNAB API](https://api.youneedabudget.com).

## Installation

```
go get pkg.venceslau.dev/ynab
```

## Usage

To use this client you must [obtain an access token](https://api.youneedabudget.com/#authentication-overview) from your [My Account](https://app.youneedabudget.com/settings) page of the YNAB web app.

```go
package main

import (
	"fmt"

	"pkg.venceslau.dev/ynab"
)

const accessToken = "bf0cbb14b4330-not-real-3de12e66a389eaafe2"

func main() {
	c := ynab.NewClient(accessToken)
	budgets, err := c.Budget().GetBudgets()
	if err != nil {
		panic(err)
	}

	for _, budget := range budgets {
		fmt.Println(budget.Name)
		// ...
	}
}
```

See the [reference documentation](https://pkg.go.dev/pkg.venceslau.dev/ynab) to see all the available methods with example usage.

## Import path & versioning

This module is imported as `pkg.venceslau.dev/ynab` (a vanity path that redirects
`go get` to the `github.com/brunovenceslau/ynab.go` repository).

It was previously published as `github.com/brunomvsouza/ynab.go`. With the move to
the vanity path, versioning restarted at `v0.1.0` — the same code as the old
`v1.5.0`, with an unchanged public API. The `0.x` series lets the API evolve
before being promoted to a stable `v1.0.0`. The old `v1.0.0`–`v1.5.0` tags are
kept for historical reference only and are not installable under the new import
path. See [CHANGELOG.md](CHANGELOG.md) for the full history.

## Development

- Make sure you have Go 1.19 or later installed
- Run tests with `go test -race ./...`

## License

BSD-2-Clause
