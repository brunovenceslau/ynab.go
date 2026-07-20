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

		// The complete expected value: June differs from July only where
		// the fixture says so (nil note, nil age of money, month).
		june := goldenJulyMonthSummary()
		june.Month = ynab.NewMonth(2026, time.June)
		june.Note = nil
		june.AgeOfMoney = nil
		require.Equal(t, []ynab.MonthSummary{goldenJulyMonthSummary(), june}, months)
		require.Equal(t, "2026-07-01", months[0].SyncID())
	})

	t.Run("delta with tombstone", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "months/list_delta.json", 0)
		months, sk, err := client.Plan("p-1").Months.List(t.Context(), ynab.Since(3000))
		require.NoError(t, err)
		require.Equal(t, "3000", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(3100), sk)
		require.Len(t, months, 2)
		require.True(t, months[1].IsDeleted())

		// Tombstones delete, changes upsert through MergeByID.
		local := map[string]ynab.MonthSummary{"2026-06-01": {}}
		merged := ynab.MergeByID(local, months)
		require.Len(t, merged, 1)
		require.NotContains(t, merged, "2026-06-01")
		require.Equal(t, ynab.Milliunits(5200000), merged["2026-07-01"].Income)
	})
}

func TestMonthsExtremeNumerics(t *testing.T) {
	t.Parallel()

	runExtremeNumericsCase(t, ynab.MonthDetail{}, "months/extreme.json", "month")
}

func TestMonthsGet(t *testing.T) {
	t.Parallel()

	t.Run("concrete month", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "months/get.json", 0)
		got, err := client.Plan("p-1").Months.Get(t.Context(), ynab.NewMonth(2026, time.July))
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/2026-07-01", rec.URL.Path)
		require.Equal(t, &ynab.MonthDetail{
			MonthSummary: goldenJulyMonthSummary(),
			Categories:   []ynab.Category{goldenGroceriesCategory()},
		}, got)
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
		require.Equal(t, "Months.Get", argErr.Op)
		require.Equal(t, "month", argErr.Field)
		require.Empty(t, rec.Method, "no request must be sent")
	})
}
