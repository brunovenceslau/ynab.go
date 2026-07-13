// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package api_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkg.venceslau.dev/ynab/api"
)

func TestDate_UnmarshalJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wrapper := struct {
			Name string
			Date *api.Date
		}{}

		err := json.Unmarshal([]byte(`{"Name": "YNAB", "Date": "2009-01-29"}`), &wrapper)
		assert.NoError(t, err)
		assert.Equal(t, "2009-01-29 00:00:00 +0000 UTC", wrapper.Date.String())
	})

	t.Run("json string without Date field", func(t *testing.T) {
		wrapper := struct {
			Name string
			Date *api.Date
		}{}

		err := json.Unmarshal([]byte(`{"Name": "YNAB"}`), &wrapper)
		assert.NoError(t, err)
		assert.Nil(t, wrapper.Date)
	})

	t.Run("json string with null Date field", func(t *testing.T) {
		wrapper := struct {
			Name string
			Date *api.Date
		}{}

		err := json.Unmarshal([]byte(`{"Name": "YNAB", "Date": null}`), &wrapper)
		assert.NoError(t, err)
		assert.Nil(t, wrapper.Date)
	})
}

func TestDate_MarshalJSON(t *testing.T) {
	date, err := api.DateFromString("2020-01-20")
	assert.NoError(t, err)

	wrapper := struct {
		Date api.Date
	}{
		date,
	}

	buf, err := json.Marshal(&wrapper)
	assert.NoError(t, err)
	assert.Equal(t, `{"Date":"2020-01-20"}`, string(buf))
}

func TestDateFromString(t *testing.T) {
	table := []struct {
		InputDate          string
		OutputDateToString string
		OutputError        bool
	}{
		{"2018-02-01", "2018-02-01 00:00:00 +0000 UTC", false},
		{"2018-13-01", "0001-01-01 00:00:00 +0000 UTC", true},
	}

	for _, test := range table {
		date, err := api.DateFromString(test.InputDate)
		assert.Equal(t, test.OutputError, err != nil)
		assert.Equal(t, test.OutputDateToString, date.String())
	}
}

func TestDateFormat(t *testing.T) {
	apiDate1, err := api.DateFromString("2018-02-01")
	assert.NoError(t, err)

	apiDate2, err := api.DateFromString("2018-12-01")
	assert.NoError(t, err)

	table := []struct {
		InputDate           api.Date
		OutputFormattedDate string
	}{
		{apiDate1, "2018-02-01"},
		{apiDate2, "2018-12-01"},
		{api.Date{}, "0001-01-01"},
	}

	for _, test := range table {
		formattedDate := api.DateFormat(test.InputDate)
		assert.Equal(t, test.OutputFormattedDate, formattedDate)
	}
}
