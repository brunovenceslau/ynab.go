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
)

func init() {
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
		// getAccounts anchors the created rows; getTransactions covers
		// both the type=unapproved filter and the post-delete delta read
		// observing the tombstones; getTransactionById is the
		// unconditional persistence read-back.
		ops: []string{
			"createTransaction", "updateTransaction", "updateTransactions",
			"deleteTransaction", "importTransactions",
			"getAccounts", "getTransactions", "getTransactionById",
		},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			accounts, _, err := plan.Accounts.List(t.Context())
			require.NoError(t, err)
			// The transfer probe needs a second account; every accounts-case
			// run creates one itest account, so the dedicated plan has ≥2.
			require.GreaterOrEqual(t, len(accounts), 2,
				"the transfer probe needs two accounts — run the accounts case on this plan at least once")
			memo := fmt.Sprintf("itest-txn-%d", time.Now().UnixNano())
			date := ynab.Today() // hoisted: re-evaluating Today() in asserts would race midnight

			cleanupCtx := context.WithoutCancel(t.Context())
			deleteOnCleanup := func(id string) {
				t.Cleanup(func() {
					gone, _, err := plan.Transactions.Delete(cleanupCtx, id)
					require.NoError(t, err, "cleanup must restore the test plan")
					require.True(t, gone.IsDeleted())
				})
			}

			// The import_id convention "YNAB:<milliunits>:<date>:<n>" with
			// the distinctive amount -1037: an import-marked create is
			// subject to the server's match-to-user-entered rule (same
			// account, same amount, date ±10 days), and no other row ever
			// carries -1037 — so a leftover from a failed prior cleanup can
			// never auto-match. The id must be unique PER RUN, not per day:
			// import_id uniqueness survives transaction deletion server-side
			// (probed live 2026-07-20 — see API_NOTES.md), so a
			// date-deterministic id 409s on the very next same-day run.
			importID := fmt.Sprintf("itest:%d", time.Now().UnixNano())
			created, _, err := plan.Transactions.Create(t.Context(), ynab.TransactionSpec{
				AccountID: accounts[0].ID,
				Date:      date,
				Amount:    -1037,
				PayeeName: ynab.Set("itest payee"),
				Memo:      ynab.Set(memo),
				Cleared:   ynab.ClearedStatusCleared,
				Approved:  ynab.Set(false),
				ImportID:  ynab.Set(importID),
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				if _, _, err := plan.Transactions.Delete(cleanupCtx, created.ID); err != nil {
					require.ErrorIs(t, err, ynab.ErrResourceNotFound,
						"cleanup tolerates only the body's own delete")
				}
			})
			// Full echo of every sent field, plus the server-side effects
			// only live traffic can prove.
			require.False(t, created.Approved, "Set(false) survives the live wire")
			require.Equal(t, ynab.Milliunits(-1037), created.Amount)
			require.Equal(t, date, created.Date)
			require.Equal(t, accounts[0].ID, created.AccountID)
			require.NotNil(t, created.Memo)
			require.Equal(t, memo, *created.Memo)
			require.Equal(t, ynab.ClearedStatusCleared, created.Cleared, "sent cleared status must round-trip")
			require.NotNil(t, created.PayeeName)
			require.Equal(t, "itest payee", *created.PayeeName)
			require.NotNil(t, created.PayeeID, "payee_name resolution must yield a payee id")
			require.NotNil(t, created.ImportID)
			require.Equal(t, importID, *created.ImportID)
			require.Nil(t, created.MatchedTransactionID,
				"the distinctive -1037 amount must never auto-match — matching drift must be visible")

			// The duplicate (account, import_id) single create: the spec's
			// 409, never before observed from the real server. Pin the
			// status and the sentinel; log the payload id so drift in the
			// sub-code is visible. A 409 creates nothing — no cleanup.
			_, _, err = plan.Transactions.Create(t.Context(), ynab.TransactionSpec{
				AccountID: accounts[0].ID,
				Date:      date,
				Amount:    -1037,
				ImportID:  ynab.Set(importID),
			})
			require.ErrorIs(t, err, ynab.ErrConflict, "a duplicate import_id on the account must answer 409")
			var apiErr *ynab.Error
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, http.StatusConflict, apiErr.StatusCode)
			require.NotEmpty(t, apiErr.ID)
			t.Logf("duplicate-import_id 409 payload: id=%q name=%q", apiErr.ID, apiErr.Name)

			// Batch create with a deliberate duplicate row: same-account
			// dedup must skip it AND report it — both halves pin the rule.
			batch, err := plan.Transactions.CreateBatch(t.Context(), []ynab.TransactionSpec{
				{
					AccountID: accounts[0].ID,
					Date:      date,
					Amount:    -2000,
					Memo:      ynab.Set(memo + "-batch"),
				},
				{AccountID: accounts[0].ID, Date: date, Amount: -1037, ImportID: ynab.Set(importID)},
			})
			require.NoError(t, err)
			require.Len(t, batch.TransactionIDs, 1, "only the non-duplicate row may be created")
			require.Equal(t, []string{importID}, batch.DuplicateImportIDs,
				"the server must skip the existing (account, import_id)")
			deleteOnCleanup(batch.TransactionIDs[0])

			// type=unapproved: whether the server actually filters is
			// provable only while created (Approved false) exists.
			unapproved, _, err := plan.Transactions.List(t.Context(), ynab.TransactionFilter{
				SinceDate: date.AddDays(-1),
				Type:      ynab.TransactionTypeUnapproved,
			})
			require.NoError(t, err)
			unapprovedIDs := make([]string, 0, len(unapproved))
			for _, tx := range unapproved {
				require.False(t, tx.Approved, "type=unapproved must answer only unapproved rows")
				unapprovedIDs = append(unapprovedIDs, tx.ID)
			}
			require.Contains(t, unapprovedIDs, created.ID)

			updated, skAfterUpdate, err := plan.Transactions.Update(t.Context(), created.ID,
				ynab.TransactionUpdate{
					Memo:    ynab.Set(memo + "-upd"),
					Cleared: ynab.ClearedStatusReconciled, // plain omitzero field, not Optional
				})
			require.NoError(t, err)
			require.Equal(t, memo+"-upd", *updated.Memo)
			require.Equal(t, ynab.ClearedStatusReconciled, updated.Cleared,
				"the API must accept a direct write to reconciled")
			// Partial-PUT semantics: unset fields must stay unchanged.
			// Compared against created.* so these asserts can never drift
			// from the create spec.
			require.Equal(t, created.Amount, updated.Amount, "unset Amount must survive a partial PUT")
			require.Equal(t, created.Date, updated.Date)
			require.Equal(t, created.AccountID, updated.AccountID)
			require.False(t, updated.Approved, "unset Approved must stay unchanged")
			require.Equal(t, created.PayeeName, updated.PayeeName)

			// Read-back: the server STORED the fields — strictly stronger
			// than the update response's echo.
			got, _, err := plan.Transactions.Get(t.Context(), created.ID)
			require.NoError(t, err)
			require.Equal(t, created.ID, got.ID)
			require.NotNil(t, got.Memo)
			require.Equal(t, memo+"-upd", *got.Memo)
			require.Equal(t, created.Amount, got.Amount)
			require.Equal(t, created.Date, got.Date)
			require.False(t, got.Approved)
			require.NotNil(t, got.ImportID)
			require.Equal(t, importID, *got.ImportID, "import_id must persist to a read-back")

			// PatchByImportID is deliberately absent: the live server
			// answers 400 "transaction does not exist" for API-created
			// transactions carrying the import_id — the lookup reaches only
			// import-pipeline rows, uncreatable on this plan (probed live
			// 2026-07-20; see API_NOTES.md and the known-unprovable list).
			patched, err := plan.Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
				ynab.PatchByID(batch.TransactionIDs[0],
					ynab.TransactionUpdate{Approved: ynab.Set(true)}),
			})
			require.NoError(t, err)
			require.Len(t, patched.TransactionIDs, 1)
			// The returned rows prove the batch PATCH applied the set field
			// and left the unset ones untouched — the batch-path
			// unset-means-unchanged proof, distinct from the single PUT's.
			require.Len(t, patched.Transactions, 1)
			rows := map[string]ynab.Transaction{}
			for _, row := range patched.Transactions {
				rows[row.ID] = row
			}
			batchRow, ok := rows[batch.TransactionIDs[0]]
			require.True(t, ok)
			require.True(t, batchRow.Approved, "the patched field must be applied")
			require.NotNil(t, batchRow.Memo)
			require.Equal(t, memo+"-batch", *batchRow.Memo, "unset memo must survive the batch PATCH")
			require.Equal(t, ynab.Milliunits(-2000), batchRow.Amount, "unset amount must survive the batch PATCH")

			// Split create: leg id minting, leg↔parent linkage, and
			// end-to-end sum acceptance are server-only truths.
			split, _, err := plan.Transactions.Create(t.Context(), ynab.TransactionSpec{
				AccountID:  accounts[0].ID,
				Date:       date,
				Amount:     -3000,
				CategoryID: ynab.SetNull[string](),
				Memo:       ynab.Set(memo + "-split"),
				Splits: []ynab.SubtransactionSpec{
					{Amount: -1500, Memo: ynab.Set("leg-a")},
					{Amount: -1500},
				},
			})
			require.NoError(t, err)
			deleteOnCleanup(split.ID)
			require.Len(t, split.Subtransactions, 2)
			for _, leg := range split.Subtransactions {
				require.NotEmpty(t, leg.ID, "the server must mint leg ids")
				require.Equal(t, split.ID, leg.TransactionID)
				require.Equal(t, ynab.Milliunits(-1500), leg.Amount)
			}

			// Transfer via the target account's transfer payee: mirror-row
			// creation and linkage are server-only.
			target := accounts[1]
			require.NotNil(t, target.TransferPayeeID, "every account carries a transfer payee")
			transfer, _, err := plan.Transactions.Create(t.Context(), ynab.TransactionSpec{
				AccountID: accounts[0].ID,
				Date:      date,
				Amount:    -5000,
				PayeeID:   ynab.Set(*target.TransferPayeeID),
			})
			require.NoError(t, err)
			require.NotNil(t, transfer.TransferAccountID, "a transfer must link the target account")
			require.Equal(t, target.ID, *transfer.TransferAccountID)
			require.NotNil(t, transfer.TransferTransactionID, "a transfer must link its mirror row")
			mirrorID := *transfer.TransferTransactionID
			// Whether deleting one side cascades to the other is unobserved
			// server behavior: the delta read below reports it, and this
			// guarded cleanup removes the mirror only when the server did
			// not — so a red first run can never strand a row.
			mirrorTombstoned := false
			t.Cleanup(func() {
				if mirrorTombstoned {
					return
				}
				if _, _, err := plan.Transactions.Delete(cleanupCtx, mirrorID); err != nil {
					require.ErrorIs(t, err, ynab.ErrResourceNotFound,
						"mirror cleanup tolerates only a server-side cascade")
				}
			})
			gone, _, err := plan.Transactions.Delete(t.Context(), transfer.ID)
			require.NoError(t, err)
			require.True(t, gone.IsDeleted())

			// Delete in the body so a real delta read observes the
			// tombstones (transactions have no empty-plan fold to dodge);
			// the tolerant cleanups above stay as the safety net.
			gone, _, err = plan.Transactions.Delete(t.Context(), created.ID)
			require.NoError(t, err)
			require.True(t, gone.IsDeleted())

			changes, _, err := plan.Transactions.List(t.Context(),
				ynab.TransactionFilter{Since: skAfterUpdate})
			require.NoError(t, err)
			tombstoned := map[string]bool{}
			for _, c := range changes {
				if c.IsDeleted() {
					tombstoned[c.SyncID()] = true
				}
			}
			require.True(t, tombstoned[created.ID],
				"delta read since %d must include the deleted row", skAfterUpdate)
			require.True(t, tombstoned[transfer.ID],
				"post-delete delta must carry the transfer's own tombstone")
			mirrorTombstoned = tombstoned[mirrorID]
			t.Logf("transfer mirror %s cascade-deleted by the server: %v", mirrorID, mirrorTombstoned)

			imported, err := plan.Transactions.Import(t.Context())
			require.NoError(t, err, "an empty import result is a nil-error answer")
			// Status/result coherence for the fold the client performs:
			// 200 must mean no ids, 201 must mean ids — proving the
			// 200-empty vs 201-with-ids fold is lossless. On the unlinked
			// test plan the 200-empty branch runs; the 201 arm stands
			// ready if a linked account ever appears.
			st, ok := env.LastStatus("importTransactions")
			require.True(t, ok, "the import request must be recorded")
			if st == http.StatusCreated {
				require.NotEmpty(t, imported, "a 201 import must carry ids")
			} else {
				require.Equal(t, http.StatusOK, st)
				require.Empty(t, imported, "a 200 import means nothing was waiting")
			}
			t.Logf("importTransactions answered %d with %d ids", st, len(imported))
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
