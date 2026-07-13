// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package month_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/jarcoal/httpmock.v1"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/api"
	"pkg.venceslau.dev/ynab/api/month"
)

func TestService_GetMonths(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/months"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "months": [
      {
        "month": "2017-10-01",
        "note": null,
        "to_be_budgeted": 0,
        "age_of_money": 14,
        "income": 3077330,
        "budgeted": 3271990,
        "activity": -3128590
      }
		],
		"server_knowledge": 10
	}
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	f := &api.Filter{LastKnowledgeOfServer: 10}
	snapshot, err := client.Month().GetMonths("aa248caa-eed7-4575-a990-717386438d2c", f)
	assert.NoError(t, err)
	assert.IsType(t, &month.SearchResultSnapshot{}, snapshot)

	m := snapshot.Months[0]

	var (
		expectedAgeOfMoney      int64 = 14
		expectedToBeBudgeted    int64
		expectedIncome          int64  = 3077330
		expectedBudgeted        int64  = 3271990
		expectedActivity        int64  = -3128590
		expectedServerKnowledge uint64 = 10
	)
	assert.Equal(t, expectedServerKnowledge, snapshot.ServerKnowledge)
	assert.Equal(t, "2017-10-01 00:00:00 +0000 UTC", m.Month.String())
	assert.Equal(t, &expectedToBeBudgeted, m.ToBeBudgeted)
	assert.Equal(t, &expectedAgeOfMoney, m.AgeOfMoney)
	assert.Equal(t, &expectedIncome, m.Income)
	assert.Equal(t, &expectedBudgeted, m.Budgeted)
	assert.Equal(t, &expectedActivity, m.Activity)
	assert.Nil(t, m.Note)
}

func TestService_GetMonth(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/months/2017-10-01"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "month": {
			"month": "2017-10-01",
			"note": null,
			"to_be_budgeted": 0,
			"age_of_money": 14,
			"income": 3077330,
			"budgeted": 3271990,
			"activity": -3128590
		}
	}
}
		`)
			return res, nil
		},
	)

	date, err := api.DateFromString("2017-10-01")
	assert.NoError(t, err)

	client := ynab.NewClient("")
	m, err := client.Month().GetMonth("aa248caa-eed7-4575-a990-717386438d2c", date)
	assert.NoError(t, err)
	assert.IsType(t, &month.Month{}, m)

	var (
		expectedAgeOfMoney   int64 = 14
		expectedToBeBudgeted int64
		expectedIncome       int64 = 3077330
		expectedBudgeted     int64 = 3271990
		expectedActivity     int64 = -3128590
	)
	assert.Equal(t, "2017-10-01 00:00:00 +0000 UTC", m.Month.String())
	assert.Equal(t, &expectedToBeBudgeted, m.ToBeBudgeted)
	assert.Equal(t, &expectedAgeOfMoney, m.AgeOfMoney)
	assert.Equal(t, &expectedIncome, m.Income)
	assert.Equal(t, &expectedBudgeted, m.Budgeted)
	assert.Equal(t, &expectedActivity, m.Activity)
	assert.Nil(t, m.Note)
}
