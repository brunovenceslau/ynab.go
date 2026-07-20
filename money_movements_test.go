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
		op:      "getMoneyMovementGroupsByMonth",
		fixture: "money_movements/groups_by_month.json",
		model:   []ynab.MoneyMovementGroup{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			groups, _, err := c.Plan("p-1").MoneyMovements.ListGroupsByMonth(t.Context(), ynab.CurrentMonth())
			return groups, err
		},
	})

	registerNullFixture([]ynab.MoneyMovement{}, "money_movements/list.json", "money_movements")
	registerNullFixture([]ynab.MoneyMovementGroup{}, "money_movements/groups.json", "money_movement_groups")

	registerIntegrationCase(integrationCase{
		name: "money movements reads",
		ops: []string{
			"getMoneyMovements", "getMoneyMovementsByMonth",
			"getMoneyMovementGroups", "getMoneyMovementGroupsByMonth",
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

			_, _, err = plan.MoneyMovements.ListByMonth(t.Context(), ynab.CurrentMonth())
			require.NoError(t, err)

			_, _, err = plan.MoneyMovements.ListGroups(t.Context())
			require.NoError(t, err)

			_, _, err = plan.MoneyMovements.ListGroupsByMonth(t.Context(), ynab.CurrentMonth())
			require.NoError(t, err)
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
		require.Len(t, movements, 2)

		full := movements[0]
		require.Equal(t, ynab.Milliunits(250000), full.Amount)
		require.Equal(t, ynab.NewMonth(2026, time.July), *full.Month)
		require.Equal(t, "topping up groceries", *full.Note)

		bare := movements[1]
		require.Nil(t, bare.Month)
		require.Nil(t, bare.MovedAt)
		require.Nil(t, bare.FromCategoryID)
		require.Nil(t, bare.ToCategoryID)
	})

	t.Run("month variants hit the month paths", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "money_movements/by_month.json", 0)
		_, _, err := client.Plan("p-1").MoneyMovements.ListByMonth(t.Context(), ynab.NewMonth(2026, time.July))
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/2026-07-01/money_movements", rec.URL.Path)

		client2, rec2 := serveFixture(t, "money_movements/groups_by_month.json", 0)
		_, _, err = client2.Plan("p-1").MoneyMovements.ListGroupsByMonth(t.Context(), ynab.CurrentMonth())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/current/money_movement_groups", rec2.URL.Path)
	})

	t.Run("groups decode", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "money_movements/groups.json", 0)
		groups, sk, err := client.Plan("p-1").MoneyMovements.ListGroups(t.Context())
		require.NoError(t, err)
		require.Equal(t, ynab.ServerKnowledge(5002), sk)
		require.Len(t, groups, 2)
		require.Equal(t, "monthly rebalance", *groups[0].Note)
		require.Nil(t, groups[1].Note)
	})

	t.Run("zero month is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "money_movements/by_month.json", 0)
		_, _, err := client.Plan("p-1").MoneyMovements.ListByMonth(t.Context(), ynab.Month{})
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)

		_, _, err = client.Plan("p-1").MoneyMovements.ListGroupsByMonth(t.Context(), ynab.Month{})
		require.ErrorAs(t, err, &argErr)
		require.Empty(t, rec.Method, "no request must be sent for either call")
	})
}
