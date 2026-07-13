// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package api_test

import (
	"fmt"

	"pkg.venceslau.dev/ynab/api"
)

func ExampleDateFromString() {
	date, _ := api.DateFromString("2020-01-20")
	fmt.Println(date)

	// Output: 2020-01-20 00:00:00 +0000 UTC
}
