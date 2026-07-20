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
		op:      "getCategories",
		fixture: "categories/list.json",
		model:   []ynab.CategoryGroup{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			groups, _, err := c.Plan("p-1").Categories.List(t.Context())
			return groups, err
		},
	})
	registerReadCase(readCase{
		op:      "getCategories",
		variant: "null",
		fixture: "categories/list_null.json",
		model:   []ynab.CategoryGroup{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			groups, _, err := c.Plan("p-1").Categories.List(t.Context())
			return groups, err
		},
	})
	registerReadCase(readCase{
		op:      "getCategoryById",
		fixture: "categories/get.json",
		model:   ynab.Category{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Categories.Get(t.Context(), "ca111111-1111-1111-1111-111111111111")
		},
	})
	registerReadCase(readCase{
		op:      "getMonthCategoryById",
		fixture: "categories/get_for_month.json",
		model:   ynab.Category{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			m := ynab.NewMonth(2026, time.June)
			return c.Plan("p-1").Categories.GetForMonth(t.Context(), m, "ca111111-1111-1111-1111-111111111111")
		},
	})

	registerNullFixture([]ynab.CategoryGroup{}, "categories/list_null.json", "category_groups")
	registerNullFixture(ynab.Category{}, "categories/get_null.json", "category")

	registerReadCase(readCase{
		op:      "getCategoryById",
		variant: "null",
		fixture: "categories/get_null.json",
		model:   ynab.Category{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Categories.Get(t.Context(), "ca333333-3333-3333-3333-333333333333")
		},
	})

	registerIntegrationCase(integrationCase{
		name: "categories reads",
		ops:  []string{"getCategories", "getCategoryById", "getMonthCategoryById"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			groups, sk, err := plan.Categories.List(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			require.NotEmpty(t, groups)

			var firstCategory *ynab.Category
			for _, g := range groups {
				require.NotEmpty(t, g.ID)
				for _, c := range g.Categories {
					require.Equal(t, g.ID, c.CategoryGroupID)
					if firstCategory == nil && !c.Internal {
						firstCategory = &c
					}
					if c.GoalType != nil {
						require.True(t, c.GoalType.Valid(), "unknown goal type %q", *c.GoalType)
					}
				}
			}
			require.NotNil(t, firstCategory, "a plan always has at least one category")

			got, err := plan.Categories.Get(t.Context(), firstCategory.ID)
			require.NoError(t, err)
			require.Equal(t, firstCategory.ID, got.ID)

			monthly, err := plan.Categories.GetForMonth(t.Context(), ynab.CurrentMonth(), firstCategory.ID)
			require.NoError(t, err)
			require.Equal(t, firstCategory.ID, monthly.ID)
		},
	})
}

func TestCategoriesList(t *testing.T) {
	t.Parallel()

	t.Run("nested tree decode", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/list.json", 0)
		groups, sk, err := client.Plan("p-1").Categories.List(t.Context())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/categories", rec.URL.Path)
		require.Equal(t, ynab.ServerKnowledge(2000), sk)

		// The complete expected tree, transcribed from the fixture. Note
		// the regression pin (first live run) inside the Vacation literal:
		// the deprecated goal_target_month is a calendar DATE mirroring
		// goal_target_date — the real API returns day components in it, so
		// a Month there would fail to decode.
		vacation := ynab.Category{
			CategoryBase: ynab.CategoryBase{
				ID:                     "ca222222-2222-2222-2222-222222222222",
				CategoryGroupID:        "cg222222-2222-2222-2222-222222222222",
				CategoryGroupName:      "Savings Goals",
				Name:                   "Vacation",
				Budgeted:               0,
				Activity:               0,
				Balance:                0,
				GoalType:               ptr(ynab.GoalTypeTBD),
				GoalCreationMonth:      ptr(ynab.NewDate(2026, time.January, 1)),
				GoalTarget:             ptr(ynab.Milliunits(12000000)),
				GoalTargetMonth:        ptr(ynab.NewDate(2027, time.June, 15)),
				GoalTargetDate:         ptr(ynab.NewDate(2027, time.June, 15)),
				GoalPercentageComplete: ptr(10),
				Deleted:                false,
			},
			BalanceFormatted:    "$0.00",
			BalanceCurrency:     0,
			ActivityFormatted:   "$0.00",
			ActivityCurrency:    0,
			BudgetedFormatted:   "$0.00",
			BudgetedCurrency:    0,
			GoalTargetFormatted: ptr("$12,000.00"),
			GoalTargetCurrency:  ptr(12000.0),
		}
		want := []ynab.CategoryGroup{
			{
				ID:         "cg111111-1111-1111-1111-111111111111",
				Name:       "Everyday",
				Categories: []ynab.Category{goldenGroceriesCategory()},
			},
			{
				ID:         "cg222222-2222-2222-2222-222222222222",
				Name:       "Savings Goals",
				Categories: []ynab.Category{vacation},
			},
		}
		require.Equal(t, want, groups)
		require.True(t, groups[0].Categories[0].GoalType.Valid())
	})

	t.Run("delta nests changes in groups — flatten before merging", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/list_delta.json", 0)
		groups, sk, err := client.Plan("p-1").Categories.List(t.Context(), ynab.Since(2000))
		require.NoError(t, err)
		require.Equal(t, "2000", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(2100), sk)

		// The changed category and the tombstone each arrive nested inside
		// their group; flatten before MergeByID.
		var flat []ynab.Category
		for _, g := range groups {
			flat = append(flat, g.Categories...)
		}
		require.Len(t, flat, 2)

		local := map[string]ynab.Category{
			"ca111111-1111-1111-1111-111111111111": {},
			"ca222222-2222-2222-2222-222222222222": {},
		}
		merged := ynab.MergeByID(local, flat)
		require.Len(t, merged, 1, "tombstone deletes, change upserts")
		require.Equal(t, ynab.Milliunits(600000), merged["ca111111-1111-1111-1111-111111111111"].Budgeted)
	})

	t.Run("all-null goals decode", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "categories/list_null.json", 0)
		groups, _, err := client.Plan("p-1").Categories.List(t.Context())
		require.NoError(t, err)

		bare := groups[0].Categories[0]
		require.Nil(t, bare.GoalType)
		require.Nil(t, bare.GoalTarget)
		require.Nil(t, bare.GoalTargetDate)
		require.Nil(t, bare.GoalSnoozedAt)
		require.Nil(t, bare.GoalTargetFormatted)
		require.Nil(t, bare.GoalUnderFundedCurrency)
	})
}

func TestCategoriesGet(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "categories/get.json", 0)
	got, err := client.Plan("p-1").Categories.Get(t.Context(), "ca111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.Equal(t, "/plans/p-1/categories/ca111111-1111-1111-1111-111111111111", rec.URL.Path)
	require.Equal(t, "Groceries", got.Name)
}

func TestCategoriesGetForMonth(t *testing.T) {
	t.Parallel()

	t.Run("concrete month in path", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/get_for_month.json", 0)
		m := ynab.NewMonth(2026, time.June)
		got, err := client.Plan("p-1").Categories.GetForMonth(t.Context(), m, "ca111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/2026-06-01/categories/ca111111-1111-1111-1111-111111111111", rec.URL.Path)
		require.Equal(t, ynab.Milliunits(450000), got.Budgeted)
	})

	t.Run("current month literal in path", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/get_for_month.json", 0)
		_, err := client.Plan("p-1").Categories.GetForMonth(t.Context(), ynab.CurrentMonth(), "ca1")
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/current/categories/ca1", rec.URL.Path)
	})

	t.Run("zero month is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/get_for_month.json", 0)
		_, err := client.Plan("p-1").Categories.GetForMonth(t.Context(), ynab.Month{}, "ca1")

		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "Categories.GetForMonth", argErr.Op)
		require.Equal(t, "month", argErr.Field)
		require.Empty(t, rec.Method, "no request must be sent")
	})
}

func TestCategoriesExtremeNumerics(t *testing.T) {
	t.Parallel()

	runExtremeNumericsCase(t, ynab.Category{}, "categories/extreme.json", "category")
}
