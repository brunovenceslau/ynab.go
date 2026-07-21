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
		op:      "getPlanById",
		fixture: "plans/export.json",
		model:   ynab.PlanDetail{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			detail, _, err := c.Plan("aa111111-1111-1111-1111-111111111111").Export(t.Context())
			return detail, err
		},
	})
	registerReadCase(readCase{
		op:      "getPlanById",
		variant: "null",
		fixture: "plans/export_null.json",
		model:   ynab.PlanDetail{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			detail, _, err := c.Plan("aa111111-1111-1111-1111-111111111111").Export(t.Context())
			return detail, err
		},
	})

	registerNullFixture(ynab.PlanDetail{}, "plans/export_null.json", "plan")

	registerIntegrationCase(integrationCase{
		name: "full plan export and delta",
		ops:  []string{"getPlanById"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			detail, sk, err := plan.Export(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			require.NotEmpty(t, detail.Accounts)
			require.NotEmpty(t, detail.Categories)
			require.NotEmpty(t, detail.Months)
			assertExportIntegrity(t, detail)

			// The flagship incremental-sync path, live: Delta must accept the
			// cursor, answer a small/empty diff, and advance st.Plan in place.
			st := &ynab.SyncState{PlanID: env.PlanID, Plan: sk}
			delta, err := plan.Delta(t.Context(), st)
			require.NoError(t, err)
			require.GreaterOrEqual(t, int64(st.Plan), int64(sk), "Delta must advance the cursor in place")
			require.Empty(t, delta.Accounts, "nothing changed since the cursor")
		},
	})
}

// liveIDs collects the ids of the collection's non-deleted rows and
// requires them unique — the MergeByID keying assumption. Tombstones are
// excluded: they may legitimately shadow an id.
func liveIDs[T ynab.Syncable](t *testing.T, name string, rows []T) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, row := range rows {
		if row.IsDeleted() {
			continue
		}
		require.False(t, ids[row.SyncID()], "%s: duplicate id %s in a full export", name, row.SyncID())
		ids[row.SyncID()] = true
	}
	return ids
}

// assertExportIntegrity proves the referential integrity of a REAL full
// export — every child's foreign key resolves to an exported parent —
// which fixtures (one golden element per collection) can never establish.
// Deleted rows are skipped on both sides: the server documents tombstones
// only on delta responses, but that exclusion is itself server behavior
// under observation here, so the checks must not silently depend on it.
func assertExportIntegrity(t *testing.T, detail *ynab.PlanDetail) {
	t.Helper()

	accIDs := liveIDs(t, "accounts", detail.Accounts)
	groupIDs := liveIDs(t, "category_groups", detail.CategoryGroups)
	txIDs := liveIDs(t, "transactions", detail.Transactions)
	schedIDs := liveIDs(t, "scheduled_transactions", detail.ScheduledTransactions)
	liveIDs(t, "payees", detail.Payees)
	liveIDs(t, "payee_locations", detail.PayeeLocations)
	liveIDs(t, "categories", detail.Categories)
	liveIDs(t, "subtransactions", detail.Subtransactions)
	liveIDs(t, "scheduled_subtransactions", detail.ScheduledSubtransactions)

	for _, tx := range detail.Transactions {
		if !tx.IsDeleted() {
			require.True(t, accIDs[tx.AccountID],
				"transaction %s references unexported account %s", tx.ID, tx.AccountID)
		}
	}
	for _, c := range detail.Categories {
		if !c.IsDeleted() {
			require.True(t, groupIDs[c.CategoryGroupID],
				"category %s references unexported group %s", c.ID, c.CategoryGroupID)
		}
	}
	for _, leg := range detail.Subtransactions {
		if !leg.IsDeleted() {
			require.True(t, txIDs[leg.TransactionID],
				"subtransaction %s references unexported transaction %s", leg.ID, leg.TransactionID)
		}
	}
	for _, leg := range detail.ScheduledSubtransactions {
		if !leg.IsDeleted() {
			require.True(t, schedIDs[leg.ScheduledTransactionID],
				"scheduled leg %s references unexported scheduled transaction %s",
				leg.ID, leg.ScheduledTransactionID)
		}
	}
}

func TestPlanExport(t *testing.T) {
	t.Parallel()

	t.Run("full export decodes every collection", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "plans/export.json", 0)
		detail, sk, err := client.Plan("aa111111-1111-1111-1111-111111111111").Export(t.Context())
		require.NoError(t, err)
		require.Equal(t, "/plans/aa111111-1111-1111-1111-111111111111", rec.URL.Path)
		require.Equal(t, ynab.ServerKnowledge(8000), sk, "server_knowledge sits beside plan")

		// The scalar header, complete.
		require.Equal(t, "aa111111-1111-1111-1111-111111111111", detail.ID)
		require.Equal(t, "Family Plan", detail.Name)
		require.Equal(t, time.Date(2026, time.July, 1, 12, 34, 56, 789000000, time.UTC), detail.LastModifiedOn)
		require.Equal(t, ynab.NewMonth(2024, time.January), detail.FirstMonth)
		require.Equal(t, ynab.NewMonth(2026, time.August), detail.LastMonth)
		require.Equal(t, &ynab.DateFormat{Format: "MM/DD/YYYY"}, detail.DateFormat)
		require.Equal(t, goldenUSDCurrencyFormat(), detail.CurrencyFormat)

		// A full PlanDetail literal is impractical; instead every
		// collection pins its length AND its complete first element, so
		// each export element shape has one full-value witness.
		require.Len(t, detail.Accounts, 1)
		require.Equal(t, goldenCheckingAccountBase(), detail.Accounts[0])
		require.Equal(t, goldenPayees(), detail.Payees)
		require.Equal(t, goldenPayeeLocations(), detail.PayeeLocations)
		require.Len(t, detail.CategoryGroups, 1)
		require.Equal(t, ynab.CategoryGroup{
			ID:   "cg111111-1111-1111-1111-111111111111",
			Name: "Everyday",
		}, detail.CategoryGroups[0])
		require.Nil(t, detail.CategoryGroups[0].Categories, "export groups arrive flat")
		require.Len(t, detail.Categories, 1)
		require.Equal(t, goldenGroceriesCategoryBase(), detail.Categories[0])
		require.Len(t, detail.Months, 1)
		require.Equal(t, ynab.MonthDetailBase{
			MonthSummaryBase: goldenJulyMonthSummaryBase(),
			Categories:       []ynab.CategoryBase{goldenGroceriesCategoryBase()},
		}, detail.Months[0])
		require.Len(t, detail.Transactions, 1)
		require.Equal(t, goldenGroceryRunTransactionBase(), detail.Transactions[0])
		require.Len(t, detail.Subtransactions, 1)
		require.Equal(t, goldenSplitLegBases()[0], detail.Subtransactions[0])
		require.Len(t, detail.ScheduledTransactions, 1)
		require.Equal(t, goldenRentScheduledBase(), detail.ScheduledTransactions[0])
		require.Len(t, detail.ScheduledSubtransactions, 1)
		require.Equal(t, goldenScheduledRentLegBase(), detail.ScheduledSubtransactions[0])
	})

	t.Run("delta decodes partial collections with tombstones", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "plans/export_delta.json", 0)
		delta, sk, err := client.Plan("p-1").Export(t.Context(), ynab.Since(8000))
		require.NoError(t, err)
		require.Equal(t, "8000", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(8100), sk)
		require.Empty(t, delta.Accounts)
		require.Len(t, delta.Transactions, 1)
		require.True(t, delta.Transactions[0].Deleted, "tombstones arrive inside collections")
	})

	t.Run("summary and detail collections differ by element type", func(t *testing.T) {
		t.Parallel()

		// Compile-level proof of the no-embedding decision: the two
		// Accounts fields hold different element types without shadowing.
		var summary ynab.PlanSummary
		var detail ynab.PlanDetail
		fullTyped := func([]ynab.Account) {}
		baseTyped := func([]ynab.AccountBase) {}
		fullTyped(summary.Accounts)
		baseTyped(detail.Accounts)
		require.Equal(t, summary.ID, detail.ID, "both expose the shared scalar core")
	})
}
