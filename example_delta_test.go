package ynab_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"

	"pkg.venceslau.dev/ynab"
)

// ExamplePlan_Delta is the delta-sync loop: one full read, then
// incremental reads driven by a persisted SyncState, with MergeByID
// folding changes and tombstones into a local store.
func ExamplePlan_Delta() {
	// Stands in for api.ynab.com: a full export on the first read, then —
	// once a cursor is presented — only what changed after it, deletions
	// arriving as tombstones.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("last_knowledge_of_server") == "" {
			_, _ = w.Write([]byte(`{"data":{"plan":
				{"transactions":[{"id":"t-1"}]},"server_knowledge":8000}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"plan":
			{"transactions":[{"id":"t-1","deleted":true}]},"server_knowledge":8100}}`))
	}))
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
	// Stands in for api.ynab.com: the full read carries the category at
	// 500.000; the delta read answers only what changed — the same
	// category, reassigned to 600.000.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("last_knowledge_of_server") == "" {
			_, _ = w.Write([]byte(`{"data":{"category_groups":[{"categories":[
				{"id":"c-1","name":"Groceries","budgeted":500000}]}],"server_knowledge":2000}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"category_groups":[{"categories":[
			{"id":"c-1","name":"Groceries","budgeted":600000}]}],"server_knowledge":2001}}`))
	}))
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
