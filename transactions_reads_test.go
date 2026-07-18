// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented(
		"getTransactions", "getTransactionById", "getTransactionsByAccount",
		"getTransactionsByCategory", "getTransactionsByPayee", "getTransactionsByMonth",
	)

	registerEndpointCase(endpointCase{
		op:      "getTransactions",
		fixture: "transactions/list.json",
		model:   []ynab.Transaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			txns, _, err := c.Plan("p-1").Transactions.List(t.Context(), ynab.TransactionFilter{})
			return txns, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactions",
		variant: "null",
		fixture: "transactions/list_null.json",
		model:   []ynab.Transaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			txns, _, err := c.Plan("p-1").Transactions.List(t.Context(), ynab.TransactionFilter{})
			return txns, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactionById",
		fixture: "transactions/get.json",
		model:   ynab.Transaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			tx, _, err := c.Plan("p-1").Transactions.Get(t.Context(), "tr111111-1111-1111-1111-111111111111")
			return tx, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactionById",
		variant: "null",
		fixture: "transactions/get_null.json",
		model:   ynab.Transaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			tx, _, err := c.Plan("p-1").Transactions.Get(t.Context(), "tr333333-3333-3333-3333-333333333333")
			return tx, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactionsByAccount",
		fixture: "transactions/list.json",
		model:   []ynab.Transaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			txns, _, err := c.Plan("p-1").Transactions.ListByAccount(
				t.Context(), "ac1", ynab.TransactionFilter{})
			return txns, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactionsByCategory",
		fixture: "transactions/hybrid.json",
		model:   []ynab.HybridTransaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			rows, _, err := c.Plan("p-1").Transactions.ListByCategory(
				t.Context(), "ca1", ynab.TransactionFilter{})
			return rows, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactionsByPayee",
		fixture: "transactions/hybrid_no_sk.json",
		model:   []ynab.HybridTransaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			rows, _, err := c.Plan("p-1").Transactions.ListByPayee(
				t.Context(), "pa1", ynab.TransactionFilter{})
			return rows, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getTransactionsByMonth",
		fixture: "transactions/list.json",
		model:   []ynab.Transaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			txns, _, err := c.Plan("p-1").Transactions.ListByMonth(
				t.Context(), ynab.CurrentMonth(), ynab.TransactionFilter{})
			return txns, err
		},
	})

	registerNullFixture([]ynab.Transaction{}, "transactions/list_null.json", "transactions")
	registerNullFixture([]ynab.HybridTransaction{}, "transactions/hybrid_null.json", "transactions")
}

func init() {
	registerIntegrationCase(integrationCase{
		name: "transactions reads",
		ops: []string{
			"getTransactions", "getTransactionById", "getTransactionsByAccount",
			"getTransactionsByCategory", "getTransactionsByPayee", "getTransactionsByMonth",
		},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			today := ynab.Today()
			since := ynab.TransactionFilter{SinceDate: today.AddDays(-30), UntilDate: today}
			txns, sk, err := plan.Transactions.List(t.Context(), since)
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			for _, tx := range txns {
				require.LessOrEqual(t, tx.Date.Compare(today), 0, "until_date must be honored server-side")
			}

			accounts, _, err := plan.Accounts.List(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, accounts)
			_, _, err = plan.Transactions.ListByAccount(t.Context(), accounts[0].ID, since)
			require.NoError(t, err)

			groups, _, err := plan.Categories.List(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, groups)
			require.NotEmpty(t, groups[0].Categories)
			_, _, err = plan.Transactions.ListByCategory(t.Context(), groups[0].Categories[0].ID, since)
			require.NoError(t, err)

			payees, _, err := plan.Payees.List(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, payees)
			_, _, err = plan.Transactions.ListByPayee(t.Context(), payees[0].ID, since)
			require.NoError(t, err)

			_, _, err = plan.Transactions.ListByMonth(t.Context(), ynab.CurrentMonth(), ynab.TransactionFilter{})
			require.NoError(t, err)

			if len(txns) > 0 {
				got, _, err := plan.Transactions.Get(t.Context(), txns[0].ID)
				require.NoError(t, err)
				require.Equal(t, txns[0].ID, got.ID)
			}
		},
	})
}

func TestTransactionsList(t *testing.T) {
	t.Parallel()

	t.Run("filter encodes on the request", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/list.json", 0)
		filter := ynab.TransactionFilter{
			SinceDate: ynab.NewDate(2026, time.January, 1),
			Type:      ynab.TransactionTypeUnapproved,
			Since:     6000,
		}
		txns, sk, err := client.Plan("p-1").Transactions.List(t.Context(), filter)
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/transactions", rec.URL.Path)
		require.Equal(t, "2026-01-01", rec.URL.Query().Get("since_date"))
		require.Equal(t, "unapproved", rec.URL.Query().Get("type"))
		require.Equal(t, "6000", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(6300), sk)
		require.Len(t, txns, 1)
	})

	t.Run("zero filter sends no parameters", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/list.json", 0)
		_, _, err := client.Plan("p-1").Transactions.List(t.Context(), ynab.TransactionFilter{})
		require.NoError(t, err)
		require.Empty(t, rec.URL.RawQuery)
	})
}

func TestTransactionsGet(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "transactions/get.json", 0)
	tx, sk, err := client.Plan("p-1").Transactions.Get(t.Context(), "tr111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.Equal(t, "/plans/p-1/transactions/tr111111-1111-1111-1111-111111111111", rec.URL.Path)
	require.Equal(t, ynab.ServerKnowledge(6000), sk, "Get returns server knowledge")
	require.Len(t, tx.Subtransactions, 2)
}

func TestTransactionsHybridLists(t *testing.T) {
	t.Parallel()

	t.Run("by category with server knowledge", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/hybrid.json", 0)
		rows, sk, err := client.Plan("p-1").Transactions.ListByCategory(
			t.Context(), "ca1", ynab.TransactionFilter{})
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/categories/ca1/transactions", rec.URL.Path)
		require.Equal(t, ynab.ServerKnowledge(6100), sk)
		require.Len(t, rows, 2)
	})

	t.Run("missing server knowledge decodes to zero", func(t *testing.T) {
		t.Parallel()

		// The hybrid wire shape declares server_knowledge optional: a
		// response without the key must yield 0, never an error.
		client, rec := serveFixture(t, "transactions/hybrid_no_sk.json", 0)
		rows, sk, err := client.Plan("p-1").Transactions.ListByPayee(
			t.Context(), "pa1", ynab.TransactionFilter{})
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payees/pa1/transactions", rec.URL.Path)
		require.Zero(t, sk, "absent key decodes to zero — never advance a cursor from it")
		require.Len(t, rows, 1)
	})
}

func TestTransactionsListByMonth(t *testing.T) {
	t.Parallel()

	t.Run("month in path", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/list.json", 0)
		_, _, err := client.Plan("p-1").Transactions.ListByMonth(
			t.Context(), ynab.NewMonth(2026, time.July), ynab.TransactionFilter{})
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/months/2026-07-01/transactions", rec.URL.Path)
	})

	t.Run("zero month is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/list.json", 0)
		_, _, err := client.Plan("p-1").Transactions.ListByMonth(
			t.Context(), ynab.Month{}, ynab.TransactionFilter{})
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Empty(t, rec.Method, "no request must be sent")
	})
}
