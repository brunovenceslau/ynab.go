// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package ynab_test

import (
	"fmt"
	"reflect"

	"pkg.venceslau.dev/ynab"
)

func ExampleNewClient() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	c.User().GetUser() //nolint:errcheck
}

func ExampleClientServicer_User() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.User()
	fmt.Println(reflect.TypeOf(s))

	// Output: *user.Service
}

func ExampleClientServicer_Account() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.Account()
	fmt.Println(reflect.TypeOf(s))

	// Output: *account.Service
}

func ExampleClientServicer_Budget() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.Budget()
	fmt.Println(reflect.TypeOf(s))

	// Output: *budget.Service
}

func ExampleClientServicer_Category() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.Category()
	fmt.Println(reflect.TypeOf(s))

	// Output: *category.Service
}

func ExampleClientServicer_Month() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.Month()
	fmt.Println(reflect.TypeOf(s))

	// Output: *month.Service
}

func ExampleClientServicer_Payee() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.Payee()
	fmt.Println(reflect.TypeOf(s))

	// Output: *payee.Service
}

func ExampleClientServicer_Transaction() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	s := c.Transaction()
	fmt.Println(reflect.TypeOf(s))

	// Output: *transaction.Service
}
