// Command split-transaction creates a split transaction whose legs sum
// exactly to the parent, using SplitEven to divide the amount without
// losing a milliunit to rounding.
//
// This WRITES a real transaction — point it at a scratch plan, never a
// budget you rely on.
//
//	YNAB_TOKEN=... YNAB_PLAN_ID=... YNAB_ACCOUNT_ID=... go run ./examples/split-transaction
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"pkg.venceslau.dev/ynab"
)

func main() {
	planID := os.Getenv("YNAB_PLAN_ID")
	accountID := os.Getenv("YNAB_ACCOUNT_ID")
	if planID == "" || accountID == "" {
		// The target is explicit on purpose: this writes to a real budget,
		// so you name the scratch plan rather than defaulting to last-used.
		slog.Error("set YNAB_PLAN_ID (a scratch plan!) and YNAB_ACCOUNT_ID — this writes a real transaction")
		os.Exit(1)
	}
	plan := ynab.New(os.Getenv("YNAB_TOKEN")).Plan(ynab.PlanID(planID))

	// Split -100.00 three ways: SplitEven distributes the odd remainder so
	// the legs sum to the parent exactly (-33.34, -33.33, -33.33).
	total := ynab.UnitsToMilliunits(-100)
	legs := total.SplitEven(3)

	spec := ynab.TransactionSpec{
		AccountID:  accountID,
		Date:       ynab.Today(),
		Amount:     total,
		CategoryID: ynab.SetNull[string](), // a split parent carries no category
		PayeeName:  ynab.Set("Example Store"),
	}
	for i, leg := range legs {
		spec.Splits = append(spec.Splits, ynab.SubtransactionSpec{
			Amount: leg,
			Memo:   ynab.Set(fmt.Sprintf("leg %d", i+1)),
		})
	}

	tx, _, err := plan.Transactions.Create(context.Background(), spec)
	if err != nil {
		slog.Error("creating split", "err", err)
		os.Exit(1)
	}
	fmt.Printf("created split %s (%s) with %d legs:\n", tx.ID, tx.AmountFormatted, len(tx.Subtransactions))
	for _, leg := range tx.Subtransactions {
		fmt.Printf("  %s\n", leg.AmountFormatted)
	}
}
