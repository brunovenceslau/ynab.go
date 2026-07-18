package ynab_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"pkg.venceslau.dev/ynab"
)

// Example is the flagship vignette: construct a client, take the plan
// handle, and read through it.
func Example() {
	// The httptest server stands in for api.ynab.com.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"accounts":[
			{"name":"Checking","balance_formatted":"$123.93"},
			{"name":"Savings"}],"server_knowledge":1473}}`))
	}))
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
	// The httptest server stands in for api.ynab.com.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"11111111-2222-3333-4444-555555555555"}}}`))
	}))
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

// ExampleClient_Plans lists the plans the token can reach, with each
// plan's accounts embedded via IncludeAccounts.
func ExampleClient_Plans() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"plans":[
			{"name":"Family Plan","accounts":[{"name":"Checking"},{"name":"Mortgage"}]}]}}`))
	}))
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	list, err := client.Plans(context.Background(), ynab.IncludeAccounts())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, p := range list.Plans {
		fmt.Printf("%s (%d accounts)\n", p.Name, len(p.Accounts))
	}

	// Output:
	// Family Plan (2 accounts)
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

// ExampleMilliunits does money math on exact integers — 1000 milliunits
// to the currency unit, never floats.
func ExampleMilliunits() {
	price := ynab.UnitsToMilliunits(150)
	tip := ynab.MustParseMilliunits("22.505")
	total := price.Add(tip)
	fmt.Println("total:", total)

	legs := total.SplitEven(3) // parts differ by at most 0.001 and sum exactly
	fmt.Println("split:", legs[0], legs[1], legs[2])

	// Output:
	// total: 172.505
	// split: 57.502 57.502 57.501
}

// ExampleDate is the calendar-day value type: strict YYYY-MM-DD wire
// form, with the zero value meaning "no date".
func ExampleDate() {
	d := ynab.NewDate(2026, time.July, 18)
	fmt.Println(d, "-> two weeks later:", d.AddDays(14))

	parsed, _ := ynab.ParseDate("2026-01-31")
	fmt.Println("plus a month:", parsed.AddMonths(1)) // normalizes like time.AddDate

	var none ynab.Date // renders empty, encodes JSON null
	fmt.Printf("zero: %q IsZero: %v\n", none, none.IsZero())

	// Output:
	// 2026-07-18 -> two weeks later: 2026-08-01
	// plus a month: 2026-03-03
	// zero: "" IsZero: true
}

// ExampleMonth is the budget-month value type, deliberately distinct
// from Date. Besides concrete months there are two sentinels: the
// server-resolved CurrentMonth and the zero "no month" value.
func ExampleMonth() {
	m := ynab.NewMonth(2026, time.November)
	fmt.Println(m, "-> next:", m.Next())
	fmt.Println("wraps the year:", m.AddMonths(3))

	fmt.Println("sentinel:", ynab.CurrentMonth())

	var none ynab.Month // renders empty, encodes JSON null
	fmt.Printf("zero: %q IsZero: %v\n", none, none.IsZero())

	// Output:
	// 2026-11-01 -> next: 2026-12-01
	// wraps the year: 2027-02-01
	// sentinel: current
	// zero: "" IsZero: true
}

// Example_errorHandling distinguishes retry-worthy failures from
// terminal ones through the sentinel taxonomy.
func Example_errorHandling() {
	status, body := http.StatusTooManyRequests, `{"error":{"id":"429","name":"too_many_requests"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
	plan := client.Plan(ynab.PlanIDLastUsed)

	_, _, err := plan.Accounts.List(context.Background())
	switch {
	case errors.Is(err, ynab.ErrRateLimited):
		fmt.Println("rate limited — retryable:", ynab.IsRetryable(err))
	case errors.Is(err, ynab.ErrNotFound):
		fmt.Println("gone")
	}

	status, body = http.StatusNotFound, `{"error":{"id":"404.2","name":"resource_not_found"}}`
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

// ExampleIsRetryable classifies failures for custom retry loops — pair
// it with WithRetryDisabled when orchestrating your own attempts.
func ExampleIsRetryable() {
	err := fmt.Errorf("list accounts: %w", ynab.ErrServiceUnavailable)
	fmt.Println(ynab.IsRetryable(err))              // 503: retrying may succeed
	fmt.Println(ynab.IsRetryable(ynab.ErrNotFound)) // terminal 4xx answer
	fmt.Println(ynab.IsRetryable(context.Canceled)) // the caller's own decision

	// Output:
	// true
	// false
	// false
}

// ExampleAccountsService_List reads the plan's accounts.
func ExampleAccountsService_List() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"accounts":[
			{"name":"Checking","balance":123930},
			{"name":"Mortgage","balance":-395032000}]}}`))
	}))
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"month":{"month":"2026-07-01","to_be_budgeted":500000}}}`))
	}))
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

// ExampleCategoriesService_Assign budgets money to a category for a
// month — the write behind "assign 150 to Groceries this month".
func ExampleCategoriesService_Assign() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"category":{"name":"Groceries","budgeted":150000}}}`))
	}))
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	category, _, err := plan.Categories.Assign(context.Background(), ynab.CurrentMonth(),
		"ca111111-1111-1111-1111-111111111111", ynab.UnitsToMilliunits(150))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%s now has %s assigned\n", category.Name, category.Budgeted)

	// Output:
	// Groceries now has 150.000 assigned
}

// ExamplePayeesService_Create adds a payee by name.
func ExamplePayeesService_Create() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"payee":{"name":"New Landlord"},"server_knowledge":4200}}`))
	}))
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"payee_locations":[{},{}]}}`))
	}))
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"money_movements":[{},{}],"server_knowledge":5000}}`))
	}))
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"transactions":[
			{"date":"2026-07-10","amount":-294230,"account_name":"Checking"}]}}`))
	}))
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"transaction":{"id":"tr555555-5555-5555-5555-555555555555"}}}`))
	}))
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

// ExampleTransactionsService_UpdateBatch patches many transactions in
// one request. Every update field is tri-state: Set sends a value,
// SetNull clears, and unset leaves the server's value alone.
func ExampleTransactionsService_UpdateBatch() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"transaction_ids":["t-1","t-2"],"server_knowledge":6100}}`))
	}))
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)

	patches := []ynab.TransactionPatch{
		ynab.PatchByID("t-1", ynab.TransactionUpdate{
			Memo:      ynab.Set("reviewed"),           // send a value
			FlagColor: ynab.SetNull[ynab.FlagColor](), // clear the flag
		}), // every other field stays untouched
		ynab.PatchByID("t-2", ynab.TransactionUpdate{Approved: ynab.Set(true)}),
	}
	result, err := plan.Transactions.UpdateBatch(context.Background(), patches)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("updated %d transactions at knowledge %d\n",
		len(result.TransactionIDs), result.ServerKnowledge)

	// Output:
	// updated 2 transactions at knowledge 6100
}

// ExampleScheduledTransactionsService_List reads scheduled transactions;
// an empty plan yields an empty slice, not an error.
func ExampleScheduledTransactionsService_List() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"scheduled_transactions":[
			{"frequency":"monthly","date_next":"2026-08-01"},
			{"frequency":"monthly","date_next":"2026-08-01"},
			{"frequency":"never","date_next":"2026-08-01"}]}}`))
	}))
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"plan":
			{"name":"Family Plan","accounts":[{}],"categories":[{}]},"server_knowledge":8000}}`))
	}))
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
