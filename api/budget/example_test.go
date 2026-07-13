// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package budget_test

import (
	"fmt"
	"reflect"

	"pkg.venceslau.dev/ynab/api"

	"pkg.venceslau.dev/ynab"
)

func ExampleService_GetBudget() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	b, _ := c.Budget().GetBudget("<valid_budget_id>", nil)
	fmt.Println(reflect.TypeOf(b))

	// Output: *budget.Snapshot
}

func ExampleService_GetLastUsedBudget() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	b, _ := c.Budget().GetLastUsedBudget(nil)
	fmt.Println(reflect.TypeOf(b))

	// Output: *budget.Snapshot
}

func ExampleService_GetBudget_filtered() {
	c := ynab.NewClient("<valid_ynab_access_token>")

	f := api.Filter{LastKnowledgeOfServer: 10}
	b, _ := c.Budget().GetBudget("<valid_budget_id>", &f)
	fmt.Println(reflect.TypeOf(b))

	// Output: *budget.Snapshot
}

func ExampleService_GetBudgets() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	budgets, _ := c.Budget().GetBudgets()
	fmt.Println(reflect.TypeOf(budgets))

	// Output: []*budget.Summary
}

func ExampleService_GetBudgetSettings() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s, _ := c.Budget().GetBudgetSettings("<valid_budget_id>")
	fmt.Println(reflect.TypeOf(s))

	// Output: *budget.Settings
}
