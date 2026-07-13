// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package user_test

import (
	"fmt"
	"reflect"

	"pkg.venceslau.dev/ynab"
)

func ExampleService_GetUser() {
	c := ynab.NewClient("<valid_ynab_access_token>")
	user, _ := c.User().GetUser()
	fmt.Println(reflect.TypeOf(user))

	// Output: *user.User
}
