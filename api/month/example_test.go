// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package month_test

import (
	"fmt"
	"reflect"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/api"
)

//nolint:govet
func ExampleService_GetMonth() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	d, _ := api.DateFromString("2010-01-01")
	m, _ := c.Month().GetMonth("<valid_budget_id>", d)
	fmt.Println(reflect.TypeOf(m))

	// Output: *month.Month
}

//nolint:govet
func ExampleService_GetMonths() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	f := &api.Filter{LastKnowledgeOfServer: 10}
	months, _ := c.Month().GetMonths("<valid_budget_id>", f)
	fmt.Println(reflect.TypeOf(months))

	// Output: *month.SearchResultSnapshot
}
