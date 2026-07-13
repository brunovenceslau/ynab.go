// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package user_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/jarcoal/httpmock.v1"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/api/user"
)

func TestService_GetUser(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet, "https://api.youneedabudget.com/v1/user",
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "user": {
      "id": "aa248caa-eed7-4575-a990-717386438d2c"
    }
  }
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	u, err := client.User().GetUser()
	assert.NoError(t, err)

	expected := &user.User{
		ID: "aa248caa-eed7-4575-a990-717386438d2c",
	}
	assert.Equal(t, expected, u)

}
