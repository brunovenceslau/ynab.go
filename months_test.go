// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func init() {
	registerReadCase(readCase{
		op:      "getPlanMonths",
		fixture: "months/list.json",
		model:   []ynab.MonthSummary{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			months, _, err := c.Plan("p-1").Months.List(t.Context())
			return months, err
		},
	})
	registerReadCase(readCase{
		op:      "getPlanMonths",
		variant: "null",
		fixture: "months/list_null.json",
		model:   []ynab.MonthSummary{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			months, _, err := c.Plan("p-1").Months.List(t.Context())
			return months, err
		},
	})
	registerReadCase(readCase{
		op:      "getPlanMonth",
		fixture: "months/get.json",
		model:   ynab.MonthDetail{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Months.Get(t.Context(), ynab.NewMonth(2026, time.July))
		},
	})
	registerReadCase(readCase{
		op:      "getPlanMonth",
		variant: "null",
		fixture: "months/get_null.json",
		model:   ynab.MonthDetail{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Months.Get(t.Context(), ynab.CurrentMonth())
		},
	})

	registerNullFixture([]ynab.MonthSummary{}, "months/list_null.json", "months")
	registerNullFixture(ynab.MonthDetail{}, "months/get_null.json", "month")

	registerIntegrationCase(integrationCase{
		name: "months reads",
		ops:  []string{"getPlanMonths", "getPlanMonth"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			months, sk, err := plan.Months.List(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			require.NotEmpty(t, months)
			for _, m := range months {
				require.False(t, m.Month.IsZero())
			}

			current, err := plan.Months.Get(t.Context(), ynab.CurrentMonth())
			require.NoError(t, err)
			require.False(t, current.Month.IsZero(), "the server resolves the current literal")
			require.NotEmpty(t, current.Categories)
		},
	})
}

func TestMonthsList(t *testing.T) {
	t.Parallel()

	t.Run("golden with delta option", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "months/list.json", 0)
		months, sk, err := client.Plan("p-1").Months.List(t.Context(), ynab.Since(2900))
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months", rec.URL.Path)
		require.Equal(t, "2900", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(3000), sk)
		require.Len(t, months, 2)

		july := months[0]
		require.Equal(t, ynab.NewMonth(2026, time.July), july.Month)
		require.Equal(t, ynab.Milliunits(5000000), july.Income)
		require.Equal(t, "kickoff", *july.Note)
		require.Equal(t, 24, *july.AgeOfMoney)
		require.Equal(t, "2026-07-01", july.SyncID())

		june := months[1]
		require.Nil(t, june.Note)
		require.Nil(t, june.AgeOfMoney)
	})
}

func TestMonthsGet(t *testing.T) {
	t.Parallel()

	t.Run("concrete month", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "months/get.json", 0)
		got, err := client.Plan("p-1").Months.Get(t.Context(), ynab.NewMonth(2026, time.July))
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/2026-07-01", rec.URL.Path)
		require.Equal(t, ynab.NewMonth(2026, time.July), got.Month)
		require.Len(t, got.Categories, 1)
		require.Equal(t, "Groceries", got.Categories[0].Name)
	})

	t.Run("current month hits months current", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "months/get.json", 0)
		_, err := client.Plan("p-1").Months.Get(t.Context(), ynab.CurrentMonth())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/current", rec.URL.Path)
	})

	t.Run("zero month is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "months/get.json", 0)
		_, err := client.Plan("p-1").Months.Get(t.Context(), ynab.Month{})

		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "month", argErr.Field)
		require.Empty(t, rec.Method, "no request must be sent")
	})
}
