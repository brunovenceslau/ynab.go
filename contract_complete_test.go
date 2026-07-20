// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Task 31: the strict completeness flip. Every table row must be
// implemented with doc-line-bearing methods (1:N), every write op G2-
// registered, every op G4-covered, every pointer-bearing model G5-
// covered. The G4 endpoint cases for the sixteen non-GET operations live
// here: their success responses run through the header-stripped
// double-run like every read.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	for _, ec := range writeOpReadCases() {
		registerReadCase(ec)
	}

	registerNullFixture(ynab.BatchResult{}, "transactions/create_batch_null.json", "")
}

// writeOpReadCases returns the read cases for the sixteen non-GET
// operations — their success responses run header-stripped like any
// read (G4).
func writeOpReadCases() []readCase {
	sched := ynab.Today().AddDays(30)

	return []readCase{
		{
			op: "createAccount", fixture: "accounts/create.json", model: ynab.Account{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				spec := ynab.AccountSpec{Name: "Vacation Fund", Type: ynab.SaveAccountTypeSavings}
				return c.Plan("p-1").Accounts.Create(t.Context(), spec)
			},
		},
		{
			op: "createCategory", fixture: "categories/create.json", model: ynab.Category{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				cat, _, err := c.Plan("p-1").Categories.Create(t.Context(),
					ynab.CategorySpec{Name: "n", GroupID: "g"})
				return cat, err
			},
		},
		{
			op: "updateCategory", fixture: "categories/update.json", model: ynab.Category{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				cat, _, err := c.Plan("p-1").Categories.Update(t.Context(), "ca1",
					ynab.CategoryUpdate{Name: ynab.Set("Food")})
				return cat, err
			},
		},
		{
			op: "updateMonthCategory", fixture: "categories/assign.json", model: ynab.Category{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				cat, _, err := c.Plan("p-1").Categories.Assign(t.Context(), ynab.CurrentMonth(), "ca1", 750000)
				return cat, err
			},
		},
		{
			op: "createCategoryGroup", fixture: "categories/group_create.json", model: ynab.CategoryGroup{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				group, _, err := c.Plan("p-1").Categories.CreateGroup(t.Context(), "Projects")
				return group, err
			},
		},
		{
			op: "updateCategoryGroup", fixture: "categories/group_rename.json", model: ynab.CategoryGroup{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				group, _, err := c.Plan("p-1").Categories.RenameGroup(t.Context(), "cg5", "Side Projects")
				return group, err
			},
		},
		{
			op: "createPayee", fixture: "payees/create.json", model: ynab.Payee{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				payee, _, err := c.Plan("p-1").Payees.Create(t.Context(), "New Landlord")
				return payee, err
			},
		},
		{
			op: "updatePayee", fixture: "payees/rename.json", model: ynab.Payee{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				payee, _, err := c.Plan("p-1").Payees.Rename(t.Context(), "pa1", "Grocery Palace")
				return payee, err
			},
		},
		{
			op: "createTransaction", fixture: "transactions/create.json", model: ynab.Transaction{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				tx, _, err := c.Plan("p-1").Transactions.Create(t.Context(), ynab.TransactionSpec{
					AccountID: "ac1", Date: ynab.NewDate(2026, time.July, 10), Amount: -294230,
				})
				return tx, err
			},
		},
		{
			op: "updateTransaction", fixture: "transactions/update.json", model: ynab.Transaction{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				tx, _, err := c.Plan("p-1").Transactions.Update(t.Context(), "tr1",
					ynab.TransactionUpdate{Memo: ynab.Set("updated memo")})
				return tx, err
			},
		},
		{
			op: "updateTransactions", fixture: "transactions/create_batch.json", model: ynab.BatchResult{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				return c.Plan("p-1").Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
					ynab.PatchByID("tr1", ynab.TransactionUpdate{Memo: ynab.Set("m")}),
				})
			},
		},
		{
			op: "deleteTransaction", fixture: "transactions/delete.json", model: ynab.Transaction{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				tx, _, err := c.Plan("p-1").Transactions.Delete(t.Context(), "tr1")
				return tx, err
			},
		},
		{
			op: "importTransactions", fixture: "transactions/import_ids.json", model: []string{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				return c.Plan("p-1").Transactions.Import(t.Context())
			},
		},
		{
			op: "createScheduledTransaction", fixture: "scheduled/create.json", model: ynab.ScheduledTransaction{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				return c.Plan("p-1").Scheduled.Create(t.Context(), ynab.ScheduledTransactionSpec{
					AccountID: "ac1", Date: sched, Amount: -1500000,
				})
			},
		},
		{
			op: "updateScheduledTransaction", fixture: "scheduled/update.json", model: ynab.ScheduledTransaction{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				return c.Plan("p-1").Scheduled.Update(t.Context(), "sc1", ynab.ScheduledTransactionUpdate{
					Amount: ynab.Set(ynab.Milliunits(-1600000)),
				})
			},
		},
		{
			op: "deleteScheduledTransaction", fixture: "scheduled/delete.json", model: ynab.ScheduledTransaction{},
			call: func(t *testing.T, c *ynab.Client) (any, error) {
				t.Helper()
				return c.Plan("p-1").Scheduled.Delete(t.Context(), "sc1")
			},
		},
	}
}

// TestContractComplete is the strict 44/44 assertion, active now that the
// last operation landed: every table row implemented (op 30's single row
// satisfied by Create and CreateBatch — 1:N, never a bijection), every
// operation G4-covered, no phantom registrations.
func TestContractComplete(t *testing.T) {
	t.Parallel()

	table := contract.Table()
	require.Len(t, table, 44)

	implemented := map[string]struct{}{}
	for _, id := range contract.ImplementedIDs() {
		implemented[id] = struct{}{}
	}
	require.Len(t, implemented, 44, "all 44 operations registered as implemented")

	byID := map[string]struct{}{}
	for _, op := range table {
		byID[op.ID] = struct{}{}
		require.Contains(t, implemented, op.ID, "table row %s not implemented", op.ID)
	}
	for id := range implemented {
		require.Contains(t, byID, id, "phantom implemented operation %s", id)
	}

	// G4 completeness over the whole table: every operation — reads and
	// writes alike — has at least one header-stripped endpoint case.
	readRegistryMu.Lock()
	covered := map[string]int{}
	for _, ec := range readRegistry {
		covered[ec.op]++
	}
	readRegistryMu.Unlock()
	for _, op := range table {
		require.Positive(t, covered[op.ID], "operation %s has no G4 read case", op.ID)
	}
}

// TestContractRegressionAudit proves the three named postmortem
// regressions exist as labeled tests in this tree.
func TestContractRegressionAudit(t *testing.T) {
	t.Parallel()

	labels := map[string]bool{"issue#24": false, "issue#38": false, "issue#40": false}

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, e := range entries {
		// This file's own label map would satisfy the audit vacuously.
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") || e.Name() == "contract_complete_test.go" {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(e.Name()))
		require.NoError(t, err)
		for label := range labels {
			if strings.Contains(string(raw), label) {
				labels[label] = true
			}
		}
	}
	for label, found := range labels {
		require.True(t, found, "no test carries the %s regression label", label)
	}
}
