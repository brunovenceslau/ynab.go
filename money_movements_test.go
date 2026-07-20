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
		op:      "getMoneyMovements",
		fixture: "money_movements/list.json",
		model:   []ynab.MoneyMovement{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			movements, _, err := c.Plan("p-1").MoneyMovements.List(t.Context())
			return movements, err
		},
	})
	registerReadCase(readCase{
		op:      "getMoneyMovements",
		variant: "null",
		fixture: "money_movements/list_null.json",
		model:   []ynab.MoneyMovement{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			movements, _, err := c.Plan("p-1").MoneyMovements.List(t.Context())
			return movements, err
		},
	})
	registerReadCase(readCase{
		op:      "getMoneyMovementsByMonth",
		fixture: "money_movements/by_month.json",
		model:   []ynab.MoneyMovement{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			movements, _, err := c.Plan("p-1").MoneyMovements.ListByMonth(t.Context(), ynab.NewMonth(2026, time.July))
			return movements, err
		},
	})
	registerReadCase(readCase{
		op:      "getMoneyMovementGroups",
		fixture: "money_movements/groups.json",
		model:   []ynab.MoneyMovementGroup{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			groups, _, err := c.Plan("p-1").MoneyMovements.ListGroups(t.Context())
			return groups, err
		},
	})
	registerReadCase(readCase{
		op:      "getMoneyMovementGroups",
		variant: "null",
		fixture: "money_movements/groups_null.json",
		model:   []ynab.MoneyMovementGroup{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			groups, _, err := c.Plan("p-1").MoneyMovements.ListGroups(t.Context())
			return groups, err
		},
	})
	registerReadCase(readCase{
		op:      "getMoneyMovementGroupsByMonth",
		fixture: "money_movements/groups_by_month.json",
		model:   []ynab.MoneyMovementGroup{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			groups, _, err := c.Plan("p-1").MoneyMovements.ListGroupsByMonth(t.Context(), ynab.CurrentMonth())
			return groups, err
		},
	})

	registerNullFixture([]ynab.MoneyMovement{}, "money_movements/list_null.json", "money_movements")
	registerNullFixture([]ynab.MoneyMovementGroup{}, "money_movements/groups_null.json", "money_movement_groups")

	registerIntegrationCase(integrationCase{
		name: "money movements reads",
		// getPlanMonth: the body resolves the server's current month once
		// so the month-scoped asserts cannot race the server's timezone.
		ops: []string{
			"getMoneyMovements", "getMoneyMovementsByMonth",
			"getMoneyMovementGroups", "getMoneyMovementGroupsByMonth", "getPlanMonth",
		},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			movements, sk, err := plan.MoneyMovements.List(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk), "server knowledge returned despite no cursor being accepted")
			for _, m := range movements {
				require.NotEmpty(t, m.ID)
			}

			// Month-scoped reads answer rows scoped to the SERVER-resolved
			// current month (vacuously true on an empty list).
			current, err := plan.Months.Get(t.Context(), ynab.CurrentMonth())
			require.NoError(t, err)

			byMonth, _, err := plan.MoneyMovements.ListByMonth(t.Context(), current.Month)
			require.NoError(t, err)
			for _, m := range byMonth {
				if m.Month != nil {
					require.Equal(t, current.Month, *m.Month, "rows must be scoped to the requested month")
				}
			}

			_, _, err = plan.MoneyMovements.ListGroups(t.Context())
			require.NoError(t, err)

			groupsByMonth, _, err := plan.MoneyMovements.ListGroupsByMonth(t.Context(), current.Month)
			require.NoError(t, err)
			for _, g := range groupsByMonth {
				require.Equal(t, current.Month, g.Month, "groups must be scoped to the requested month")
			}
		},
	})
}

func TestMoneyMovements(t *testing.T) {
	t.Parallel()

	t.Run("list returns SK but accepts no cursor", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "money_movements/list.json", 0)
		movements, sk, err := client.Plan("p-1").MoneyMovements.List(t.Context())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/money_movements", rec.URL.Path)
		require.Empty(t, rec.URL.RawQuery, "the method signature accepts no options — no cursor can leak")
		require.Equal(t, ynab.ServerKnowledge(5000), sk)
		require.Equal(t, []ynab.MoneyMovement{
			goldenTopUpMovement(),
			{
				ID:              "mm333333-3333-3333-3333-333333333333",
				Amount:          0,
				AmountFormatted: "$0.00",
				AmountCurrency:  0,
			},
		}, movements)
	})

	t.Run("month variants hit the month paths", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "money_movements/by_month.json", 0)
		movements, sk, err := client.Plan("p-1").MoneyMovements.ListByMonth(t.Context(), ynab.NewMonth(2026, time.July))
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/2026-07-01/money_movements", rec.URL.Path)
		require.Equal(t, ynab.ServerKnowledge(5001), sk)
		require.Equal(t, []ynab.MoneyMovement{goldenTopUpMovement()}, movements)

		client2, rec2 := serveFixture(t, "money_movements/groups_by_month.json", 0)
		groups, sk2, err := client2.Plan("p-1").MoneyMovements.ListGroupsByMonth(t.Context(), ynab.CurrentMonth())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/current/money_movement_groups", rec2.URL.Path)
		require.Equal(t, ynab.ServerKnowledge(5003), sk2)
		require.Equal(t, []ynab.MoneyMovementGroup{goldenRebalanceGroup()}, groups)
	})

	t.Run("groups decode", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "money_movements/groups.json", 0)
		groups, sk, err := client.Plan("p-1").MoneyMovements.ListGroups(t.Context())
		require.NoError(t, err)
		require.Equal(t, ynab.ServerKnowledge(5002), sk)
		require.Equal(t, []ynab.MoneyMovementGroup{
			goldenRebalanceGroup(),
			{
				ID:             "mg222222-2222-2222-2222-222222222222",
				GroupCreatedAt: time.Date(2026, time.July, 10, 9, 29, 0, 0, time.UTC),
				Month:          ynab.NewMonth(2026, time.July),
			},
		}, groups)
	})

	t.Run("extreme numerics decode", func(t *testing.T) {
		t.Parallel()

		runExtremeNumericsCase(t, []ynab.MoneyMovement{}, "money_movements/extreme.json", "money_movements")
	})

	t.Run("zero month is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "money_movements/by_month.json", 0)
		_, _, err := client.Plan("p-1").MoneyMovements.ListByMonth(t.Context(), ynab.Month{})
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "MoneyMovements.ListByMonth", argErr.Op)
		require.Equal(t, "month", argErr.Field)

		_, _, err = client.Plan("p-1").MoneyMovements.ListGroupsByMonth(t.Context(), ynab.Month{})
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "MoneyMovements.ListGroupsByMonth", argErr.Op)
		require.Equal(t, "month", argErr.Field)
		require.Empty(t, rec.Method, "no request must be sent for either call")
	})
}
