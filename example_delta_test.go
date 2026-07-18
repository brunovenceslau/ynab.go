package ynab_test

import (
	"context"
	"fmt"
	"slices"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/ynabtest"
)

// ExamplePlan_Delta is the delta-sync loop: one full read, then
// incremental reads driven by a persisted SyncState, with MergeByID
// folding changes and tombstones into a local store.
func ExamplePlan_Delta() {
	srv := ynabtest.NewServer(nil) // stands in for api.ynab.com
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)
	ctx := context.Background()

	// First run: st is zero, so Delta performs the full read. Persist st
	// (it is plain JSON) and hand it back next run.
	st := &ynab.SyncState{}
	full, err := plan.Delta(ctx, st)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	store := ynab.MergeByID(nil, full.Transactions)
	fmt.Printf("full read: %d transactions, knowledge %d\n", len(store), st.Plan)

	// Later runs return only what changed — including tombstones, which
	// MergeByID deletes from the store.
	delta, err := plan.Delta(ctx, st)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	store = ynab.MergeByID(store, delta.Transactions)
	fmt.Printf("delta: %d changes, knowledge %d\n", len(delta.Transactions), st.Plan)
	fmt.Printf("store now holds %d transactions\n", len(store))

	// Output:
	// full read: 1 transactions, knowledge 8000
	// delta: 1 changes, knowledge 8100
	// store now holds 0 transactions
}

// ExampleCategoriesService_List_delta shows the one delta stream whose
// changes arrive nested: getCategories nests changed categories inside
// their groups, so flatten every group's Categories before merging by id
// — merging groups wholesale would drop unchanged categories.
func ExampleCategoriesService_List_delta() {
	srv := ynabtest.NewServer(nil) // stands in for api.ynab.com
	defer srv.Close()

	client := ynab.New("token", ynab.WithBaseURL(srv.URL))
	plan := client.Plan(ynab.PlanIDLastUsed)
	ctx := context.Background()

	flatten := func(groups []ynab.CategoryGroup) []ynab.Category {
		var all []ynab.Category
		for _, g := range groups {
			all = append(all, g.Categories...)
		}
		return all
	}

	groups, knowledge, err := plan.Categories.List(ctx)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	store := ynab.MergeByID(nil, flatten(groups))

	changed, _, err := plan.Categories.List(ctx, ynab.Since(knowledge))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	store = ynab.MergeByID(store, flatten(changed))

	var names []string
	for _, c := range store {
		names = append(names, fmt.Sprintf("%s assigned %s", c.Name, c.Budgeted))
	}
	slices.Sort(names)
	for _, line := range names {
		fmt.Println(line)
	}

	// Output:
	// Groceries assigned 600.000
}
