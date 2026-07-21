// Command quickstart lists the accounts of your last-used plan.
//
//	YNAB_TOKEN=... go run ./examples/quickstart
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"pkg.venceslau.dev/ynab"
)

func main() {
	client := ynab.New(os.Getenv("YNAB_TOKEN"))
	plan := client.Plan(ynab.PlanIDLastUsed) // resolves server-side; no id to look up

	accounts, _, err := plan.Accounts.List(context.Background())
	if err != nil {
		slog.Error("listing accounts", "err", err)
		os.Exit(1)
	}
	for _, a := range accounts {
		// BalanceFormatted is the server's rendering; Balance is the exact
		// int64 you do math on.
		fmt.Printf("%-28s %14s\n", a.Name, a.BalanceFormatted)
	}
}
