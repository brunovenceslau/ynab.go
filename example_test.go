package ynab_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/ynabtest"
)

// Example is the flagship vignette: construct a client, take the plan
// handle, and read through it.
func Example() {
	srv := ynabtest.NewServer(nil) // stands in for api.ynab.com
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	accounts, knowledge, err := plan.Accounts.List(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%d accounts at knowledge %d\n", len(accounts), knowledge)
	fmt.Printf("first: %s (%s)\n", accounts[0].Name, accounts[0].BalanceFormatted)

	// Output:
	// 2 accounts at knowledge 1473
	// first: Checking ($123.93)
}

// ExampleNew constructs a client from a personal access token.
func ExampleNew() {
	srv := ynabtest.NewServer(nil) // stands in for api.ynab.com
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	user, err := client.User(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("authenticated as", user.ID)

	// Output:
	// authenticated as 11111111-2222-3333-4444-555555555555
}

// ExampleClient_Plan shows that the handle is free: no I/O happens until
// a service method runs.
func ExampleClient_Plan() {
	client := ynab.New("token")
	plan := client.Plan(ynab.PlanIDDefault) // no request was sent
	fmt.Println("bound to:", plan.ID())

	// Output:
	// bound to: default
}

// ExampleOptional teaches the tri-state trap: unset means "leave it",
// SetNull means "clear it", and Set always sends — zero values included.
func ExampleOptional() {
	show := func(label string, u ynab.CategoryUpdate) {
		raw, _ := json.Marshal(u)
		fmt.Printf("%-8s %s\n", label, raw)
	}

	show("unset", ynab.CategoryUpdate{})
	show("null", ynab.CategoryUpdate{GoalTarget: ynab.SetNull[ynab.Milliunits]()})
	show("value", ynab.CategoryUpdate{GoalTarget: ynab.Set(ynab.Milliunits(500000))})
	show("zero", ynab.CategoryUpdate{GoalTarget: ynab.Set(ynab.Milliunits(0))})

	// Output:
	// unset    {}
	// null     {"goal_target":null}
	// value    {"goal_target":500000}
	// zero     {"goal_target":0}
}

// Example_errorHandling distinguishes retry-worthy failures from
// terminal ones through the sentinel taxonomy.
func Example_errorHandling() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
	plan := client.Plan(ynab.PlanIDLastUsed)

	srv.FailWith(429, "429", "too_many_requests")
	_, _, err := plan.Accounts.List(context.Background())
	switch {
	case errors.Is(err, ynab.ErrRateLimited):
		fmt.Println("rate limited — retryable:", ynab.IsRetryable(err))
	case errors.Is(err, ynab.ErrNotFound):
		fmt.Println("gone")
	}

	srv.FailWith(404, "404.2", "resource_not_found")
	_, _, err = plan.Accounts.List(context.Background())
	switch {
	case errors.Is(err, ynab.ErrRateLimited):
		fmt.Println("rate limited")
	case errors.Is(err, ynab.ErrNotFound):
		fmt.Println("gone — retryable:", ynab.IsRetryable(err))
	}

	// Output:
	// rate limited — retryable: true
	// gone — retryable: false
}

// ExampleAccountsService_List reads the plan's accounts.
func ExampleAccountsService_List() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	accounts, _, err := plan.Accounts.List(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, a := range accounts {
		fmt.Printf("%s: %s\n", a.Name, a.Balance)
	}

	// Output:
	// Checking: 123.930
	// Mortgage: -395032.000
}

// ExampleMonthsService_Get reads the server-resolved current month.
func ExampleMonthsService_Get() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	month, err := plan.Months.Get(context.Background(), ynab.CurrentMonth())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%s: to be budgeted %s\n", month.Month, month.ToBeBudgeted)

	// Output:
	// 2026-07-01: to be budgeted 500.000
}

// ExamplePayeesService_Create adds a payee by name.
func ExamplePayeesService_Create() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	payee, knowledge, err := plan.Payees.Create(context.Background(), "New Landlord")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("created %s at knowledge %d\n", payee.Name, knowledge)

	// Output:
	// created New Landlord at knowledge 4200
}

// ExamplePayeeLocationsService_List reads payee locations (no delta on
// these endpoints).
func ExamplePayeeLocationsService_List() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	locations, err := plan.PayeeLocations.List(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(locations), "locations")

	// Output:
	// 2 locations
}

// ExampleMoneyMovementsService_List reads money movements — the endpoints
// that return a cursor but accept none.
func ExampleMoneyMovementsService_List() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	movements, knowledge, err := plan.MoneyMovements.List(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%d movements at knowledge %d\n", len(movements), knowledge)

	// Output:
	// 2 movements at knowledge 5000
}

// ExampleTransactionsService_List filters server-side: the list-since-
// date vignette.
func ExampleTransactionsService_List() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	filter := ynab.TransactionFilter{
		SinceDate: ynab.NewDate(2026, 7, 1),
		Type:      ynab.TransactionTypeUnapproved,
	}
	transactions, _, err := plan.Transactions.List(context.Background(), filter)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, tx := range transactions {
		fmt.Printf("%s %s %s\n", tx.Date, tx.Amount, tx.AccountName)
	}

	// Output:
	// 2026-07-10 -294.230 Checking
}

// ExampleTransactionsService_Create builds a split transaction whose
// legs always sum exactly, courtesy of SplitEven.
func ExampleTransactionsService_Create() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	total := ynab.MustParseMilliunits("-100.000")
	legs := total.SplitEven(3) // sums exactly to total, by construction

	spec := ynab.TransactionSpec{
		AccountID:  "ac111111-1111-1111-1111-111111111111",
		Date:       ynab.NewDate(2026, 7, 10),
		Amount:     total,
		PayeeName:  ynab.Set("Grocer & Co"),
		CategoryID: ynab.SetNull[string](), // splits carry the categories
		Approved:   ynab.Set(false),        // Set(false) reaches the wire
		Splits: []ynab.SubtransactionSpec{
			{Amount: legs[0], CategoryID: ynab.Set("ca111111-1111-1111-1111-111111111111")},
			{Amount: legs[1], CategoryID: ynab.Set("ca222222-2222-2222-2222-222222222222")},
			{Amount: legs[2], Memo: ynab.Set("the remainder leg")},
		},
	}
	created, _, err := plan.Transactions.Create(context.Background(), spec)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("legs %s + %s + %s = %s\n", legs[0], legs[1], legs[2], total)
	fmt.Println("created", created.ID)

	// Output:
	// legs -33.334 + -33.333 + -33.333 = -100.000
	// created tr555555-5555-5555-5555-555555555555
}

// ExampleScheduledTransactionsService_List reads scheduled transactions;
// an empty plan yields an empty slice, not an error.
func ExampleScheduledTransactionsService_List() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	scheduled, _, err := plan.Scheduled.List(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, s := range scheduled {
		fmt.Printf("%s next on %s\n", s.Frequency, s.DateNext)
	}

	// Output:
	// monthly next on 2026-08-01
	// monthly next on 2026-08-01
	// never next on 2026-08-01
}

// ExamplePlan_Export pulls the whole plan in one request.
func ExamplePlan_Export() {
	srv := ynabtest.NewServer(nil)
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	detail, knowledge, err := plan.Export(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%s: %d accounts, %d categories, knowledge %d\n",
		detail.Name, len(detail.Accounts), len(detail.Categories), knowledge)

	// Output:
	// Family Plan: 1 accounts, 1 categories, knowledge 8000
}
