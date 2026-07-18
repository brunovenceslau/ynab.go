package ynab_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented(
		"createCategory", "updateCategory", "updateMonthCategory",
		"createCategoryGroup", "updateCategoryGroup",
	)

	registerWriteCase(writeCase{
		op:     "createCategory",
		method: http.MethodPost,
		path:   "/plans/p-1/categories",
		body: `{"category":{
			"name":"Emergency Fund",
			"category_group_id":"cg2",
			"note":"rainy day",
			"goal_target":500000,
			"goal_needs_whole_amount":false,
			"goal_frequency":"monthly"
		}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			spec := ynab.CategorySpec{
				Name:                 "Emergency Fund",
				GroupID:              "cg2",
				Note:                 ynab.Set("rainy day"),
				GoalTarget:           ynab.Set(ynab.Milliunits(500000)),
				GoalNeedsWholeAmount: ynab.Set(false), // Set(zero) must survive the wire
				GoalFrequency:        ynab.GoalFrequencyMonthly,
			}
			_, _, err := c.Plan("p-1").Categories.Create(t.Context(), spec)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "updateCategory",
		method: http.MethodPatch,
		path:   "/plans/p-1/categories/ca1",
		body:   `{"category":{"name":"Food","goal_target":null}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			update := ynab.CategoryUpdate{
				Name:       ynab.Set("Food"),
				GoalTarget: ynab.SetNull[ynab.Milliunits](), // clear ≠ omit
			}
			_, _, err := c.Plan("p-1").Categories.Update(t.Context(), "ca1", update)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "updateMonthCategory",
		method: http.MethodPatch,
		path:   "/plans/p-1/months/2026-06-01/categories/ca1",
		// The issue#40 regression: exactly {"category":{"budgeted":n}} —
		// nothing else may ever ride along on an assignment.
		body: `{"category":{"budgeted":750000}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			m := ynab.NewMonth(2026, time.June)
			_, _, err := c.Plan("p-1").Categories.Assign(t.Context(), m, "ca1", 750000)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "createCategoryGroup",
		method: http.MethodPost,
		path:   "/plans/p-1/category_groups",
		body:   `{"category_group":{"name":"Projects"}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, _, err := c.Plan("p-1").Categories.CreateGroup(t.Context(), "Projects")
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "updateCategoryGroup",
		method: http.MethodPatch,
		path:   "/plans/p-1/category_groups/cg5",
		body:   `{"category_group":{"name":"Side Projects"}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, _, err := c.Plan("p-1").Categories.RenameGroup(t.Context(), "cg5", "Side Projects")
			require.NoError(t, err)
		},
	})

	registerWriteModel(ynab.CategorySpec{
		Name:                 "n",
		GroupID:              "g",
		Note:                 ynab.Set("note"),
		GoalTarget:           ynab.Set(ynab.Milliunits(1)),
		GoalTargetDate:       ynab.Set(ynab.NewDate(2027, time.January, 1)),
		GoalNeedsWholeAmount: ynab.Set(false),
		GoalFrequency:        ynab.GoalFrequencyMonthly,
	})
	registerWriteModel(ynab.CategoryUpdate{
		Name:                 ynab.Set(""),
		GroupID:              ynab.Set("g"),
		Note:                 ynab.SetNull[string](),
		GoalTarget:           ynab.SetNull[ynab.Milliunits](),
		GoalTargetDate:       ynab.Set(ynab.NewDate(2027, time.January, 1)),
		GoalNeedsWholeAmount: ynab.Set(true),
	})
}

func init() {
	registerIntegrationCase(integrationCase{
		name: "categories writes",
		ops: []string{
			"createCategory", "updateCategory", "updateMonthCategory",
			"createCategoryGroup", "updateCategoryGroup",
		},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			stamp := time.Now().UnixNano()

			// Categories and groups cannot be deleted through the API: the
			// artifacts stay on the dedicated test plan, uniquely named.
			group, sk, err := plan.Categories.CreateGroup(t.Context(), fmt.Sprintf("itest-%d", stamp))
			require.NoError(t, err)
			require.Positive(t, int64(sk))

			created, _, err := plan.Categories.Create(t.Context(), ynab.CategorySpec{
				Name:    fmt.Sprintf("itest-cat-%d", stamp),
				GroupID: group.ID,
			})
			require.NoError(t, err)

			renamedName := fmt.Sprintf("itest-cat-upd-%d", stamp)
			updated, _, err := plan.Categories.Update(t.Context(), created.ID,
				ynab.CategoryUpdate{Name: ynab.Set(renamedName)})
			require.NoError(t, err)
			require.Equal(t, renamedName, updated.Name)

			assigned, _, err := plan.Categories.Assign(t.Context(), ynab.CurrentMonth(), created.ID, 1000)
			require.NoError(t, err)
			require.Equal(t, ynab.Milliunits(1000), assigned.Budgeted)

			_, _, err = plan.Categories.Assign(t.Context(), ynab.CurrentMonth(), created.ID, 0)
			require.NoError(t, err, "assignment back to zero restores the plan")

			_, _, err = plan.Categories.RenameGroup(t.Context(), group.ID, fmt.Sprintf("itest-done-%d", stamp))
			require.NoError(t, err)
		},
	})
}

func TestCategoriesCreate(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "categories/create.json", http.StatusCreated)
	created, sk, err := client.Plan("p-1").Categories.Create(t.Context(), ynab.CategorySpec{
		Name:    "Emergency Fund",
		GroupID: "cg222222-2222-2222-2222-222222222222",
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, rec.Method)
	require.Equal(t, ynab.ServerKnowledge(2200), sk, "createCategory returns server knowledge, unlike createAccount")
	require.Equal(t, "Emergency Fund", created.Name)
}

func TestCategoriesUpdate(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "categories/update.json", 0)
	updated, sk, err := client.Plan("p-1").Categories.Update(t.Context(), "ca1",
		ynab.CategoryUpdate{Name: ynab.Set("Food")})
	require.NoError(t, err)
	require.Equal(t, http.MethodPatch, rec.Method)
	require.Equal(t, ynab.ServerKnowledge(2300), sk)
	require.Equal(t, "Food", updated.Name)
}

func TestCategoriesAssign(t *testing.T) {
	t.Parallel()

	t.Run("assigns budgeted for the month", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/assign.json", 0)
		m := ynab.NewMonth(2026, time.June)
		got, sk, err := client.Plan("p-1").Categories.Assign(t.Context(), m, "ca1", 750000)
		require.NoError(t, err)
		require.Equal(t, http.MethodPatch, rec.Method)
		require.Equal(t, "/plans/p-1/months/2026-06-01/categories/ca1", rec.URL.Path)
		require.Equal(t, ynab.Milliunits(750000), got.Budgeted)
		require.Equal(t, ynab.ServerKnowledge(2400), sk)
	})

	t.Run("zero month is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/assign.json", 0)
		_, _, err := client.Plan("p-1").Categories.Assign(t.Context(), ynab.Month{}, "ca1", 1)

		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "Categories.Assign", argErr.Op)
		require.Equal(t, "month", argErr.Field)
		require.Empty(t, rec.Method, "no request must be sent")
	})
}

func TestCategoriesGroups(t *testing.T) {
	t.Parallel()

	t.Run("create group", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/group_create.json", http.StatusCreated)
		group, sk, err := client.Plan("p-1").Categories.CreateGroup(t.Context(), "Projects")
		require.NoError(t, err)
		require.Equal(t, http.MethodPost, rec.Method)
		require.Equal(t, "Projects", group.Name)
		require.Equal(t, ynab.ServerKnowledge(2500), sk)
	})

	t.Run("rename group", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/group_rename.json", 0)
		group, _, err := client.Plan("p-1").Categories.RenameGroup(t.Context(), "cg5", "Side Projects")
		require.NoError(t, err)
		require.Equal(t, http.MethodPatch, rec.Method)
		require.Equal(t, "Side Projects", group.Name)
	})

	t.Run("name beyond 50 characters is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "categories/group_create.json", 0)
		long := strings.Repeat("x", 51)

		_, _, err := client.Plan("p-1").Categories.CreateGroup(t.Context(), long)
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "name", argErr.Field)

		_, _, err = client.Plan("p-1").Categories.RenameGroup(t.Context(), "cg5", long)
		require.ErrorAs(t, err, &argErr)
		require.Empty(t, rec.Method, "no request must be sent for either call")
	})
}
