// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented(
		"updateTransaction", "updateTransactions", "deleteTransaction", "importTransactions",
	)

	registerWriteCase(writeCase{
		op:     "updateTransaction",
		method: http.MethodPut,
		path:   "/plans/p-1/transactions/tr1",
		body:   `{"transaction":{"memo":"updated memo","approved":false,"flag_color":null}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			update := ynab.TransactionUpdate{
				Memo:      ynab.Set("updated memo"),
				Approved:  ynab.Set(false),
				FlagColor: ynab.SetNull[ynab.FlagColor](), // clear the flag
			}
			_, _, err := c.Plan("p-1").Transactions.Update(t.Context(), "tr1", update)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "updateTransactions",
		method: http.MethodPatch,
		path:   "/plans/p-1/transactions",
		// Each element emits id XOR import_id, exactly as constructed.
		body: `{"transactions":[
			{"id":"tr1","memo":"a"},
			{"import_id":"YNAB:-1:2026-07-10:1","approved":false}
		]}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			patches := []ynab.TransactionPatch{
				ynab.PatchByID("tr1", ynab.TransactionUpdate{Memo: ynab.Set("a")}),
				ynab.PatchByImportID("YNAB:-1:2026-07-10:1",
					ynab.TransactionUpdate{Approved: ynab.Set(false)}),
			}
			_, err := c.Plan("p-1").Transactions.UpdateBatch(t.Context(), patches)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "deleteTransaction",
		method: http.MethodDelete,
		path:   "/plans/p-1/transactions/tr1",
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, _, err := c.Plan("p-1").Transactions.Delete(t.Context(), "tr1")
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "importTransactions",
		method: http.MethodPost,
		path:   "/plans/p-1/transactions/import",
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, err := c.Plan("p-1").Transactions.Import(t.Context())
			require.NoError(t, err)
		},
	})

	registerWriteModel(ynab.TransactionUpdate{
		AccountID:  ynab.Set("ac1"),
		Date:       ynab.Set(ynab.NewDate(2026, time.July, 10)),
		Amount:     ynab.Set(ynab.Milliunits(0)),
		PayeeName:  ynab.SetNull[string](),
		CategoryID: ynab.Set("ca1"),
		Memo:       ynab.Set(""),
		Approved:   ynab.Set(false),
		FlagColor:  ynab.Set(ynab.FlagColorNone),
	})
}

func init() {
	registerIntegrationCase(integrationCase{
		name: "transactions writes create update delete import",
		ops: []string{
			"createTransaction", "updateTransaction", "updateTransactions",
			"deleteTransaction", "importTransactions",
		},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			accounts, _, err := plan.Accounts.List(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, accounts)
			memo := fmt.Sprintf("itest-txn-%d", time.Now().UnixNano())

			cleanupCtx := context.WithoutCancel(t.Context())
			deleteOnCleanup := func(id string) {
				t.Cleanup(func() {
					gone, _, err := plan.Transactions.Delete(cleanupCtx, id)
					require.NoError(t, err, "cleanup must restore the test plan")
					require.True(t, gone.IsDeleted())
				})
			}

			created, _, err := plan.Transactions.Create(t.Context(), ynab.TransactionSpec{
				AccountID: accounts[0].ID,
				Date:      ynab.Today(),
				Amount:    -1000,
				PayeeName: ynab.Set("itest payee"),
				Memo:      ynab.Set(memo),
				Approved:  ynab.Set(false),
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				if _, _, err := plan.Transactions.Delete(cleanupCtx, created.ID); err != nil {
					require.ErrorIs(t, err, ynab.ErrResourceNotFound,
						"cleanup tolerates only the body's own delete")
				}
			})
			require.False(t, created.Approved, "Set(false) survives the live wire")

			batch, err := plan.Transactions.CreateBatch(t.Context(), []ynab.TransactionSpec{{
				AccountID: accounts[0].ID,
				Date:      ynab.Today(),
				Amount:    -2000,
				Memo:      ynab.Set(memo + "-batch"),
			}})
			require.NoError(t, err)
			require.Len(t, batch.TransactionIDs, 1)
			deleteOnCleanup(batch.TransactionIDs[0])

			updated, skAfterUpdate, err := plan.Transactions.Update(t.Context(), created.ID,
				ynab.TransactionUpdate{Memo: ynab.Set(memo + "-upd")})
			require.NoError(t, err)
			require.Equal(t, memo+"-upd", *updated.Memo)

			patched, err := plan.Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
				ynab.PatchByID(batch.TransactionIDs[0],
					ynab.TransactionUpdate{Approved: ynab.Set(true)}),
			})
			require.NoError(t, err)
			require.Len(t, patched.TransactionIDs, 1)

			// Delete in the body so a real delta read observes the tombstone
			// (transactions have no empty-plan fold to dodge); the tolerant
			// cleanup above stays as the safety net.
			gone, _, err := plan.Transactions.Delete(t.Context(), created.ID)
			require.NoError(t, err)
			require.True(t, gone.IsDeleted())

			changes, _, err := plan.Transactions.List(t.Context(),
				ynab.TransactionFilter{Since: skAfterUpdate})
			require.NoError(t, err)
			tombstoned := false
			for _, c := range changes {
				if c.SyncID() == created.ID {
					tombstoned = true
					require.True(t, c.IsDeleted(), "post-delete delta must carry a tombstone")
				}
			}
			require.True(t, tombstoned, "delta read since %d must include the deleted row", skAfterUpdate)

			imported, err := plan.Transactions.Import(t.Context())
			require.NoError(t, err, "an empty import result is a nil-error answer")
			t.Logf("importTransactions returned %d ids", len(imported))
		},
	})
}

func TestTransactionsUpdate(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "transactions/update.json", 0)
	updated, sk, err := client.Plan("p-1").Transactions.Update(t.Context(), "tr1",
		ynab.TransactionUpdate{Memo: ynab.Set("updated memo")})
	require.NoError(t, err)
	require.Equal(t, http.MethodPut, rec.Method, "single update is a PUT")
	require.Equal(t, ynab.ServerKnowledge(6600), sk)
	require.Equal(t, "updated memo", *updated.Memo)
}

func TestTransactionsUpdateBatch(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "transactions/batch.json", 0)
	batch, err := client.Plan("p-1").Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
		ynab.PatchByID("tr1", ynab.TransactionUpdate{Memo: ynab.Set("m")}),
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPatch, rec.Method, "batch update is a PATCH")
	require.Len(t, batch.TransactionIDs, 1)
}

func TestTransactionsDelete(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "transactions/delete.json", 0)
	gone, sk, err := client.Plan("p-1").Transactions.Delete(t.Context(), "tr111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, rec.Method)
	require.Equal(t, ynab.ServerKnowledge(6700), sk)
	require.True(t, gone.IsDeleted(), "Delete returns the final state")
}

func TestTransactionsImport(t *testing.T) {
	t.Parallel()

	t.Run("nothing waiting answers empty with nil error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/import_empty.json", 0)
		ids, err := client.Plan("p-1").Transactions.Import(t.Context())
		require.NoError(t, err)
		require.Empty(t, ids)
		require.Equal(t, "/plans/p-1/transactions/import", rec.URL.Path)
	})

	t.Run("imported ids arrive on 201", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "transactions/import_ids.json", http.StatusCreated)
		ids, err := client.Plan("p-1").Transactions.Import(t.Context())
		require.NoError(t, err)
		require.Equal(t, []string{"tr777777-7777-7777-7777-777777777777"}, ids)
	})
}
