// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented("createTransaction")

	// Op 30 fans out to two methods and requires exactly two G2 body
	// cases: the single {"transaction":...} and the batch
	// {"transactions":[...]} shapes.
	registerWriteCase(writeCase{
		op:      "createTransaction",
		variant: "single",
		method:  http.MethodPost,
		path:    "/plans/p-1/transactions",
		// The issue#24 class assertion: approved:false (a Set zero value)
		// must be present, and unset Optionals (payee_id, flag_color, …)
		// must be absent from the emitted key set.
		body: `{"transaction":{
			"account_id":"ac1",
			"date":"2026-07-10",
			"amount":-294230,
			"payee_name":"Grocer & Co",
			"memo":"weekly shop",
			"cleared":"cleared",
			"approved":false,
			"import_id":"YNAB:-294230:2026-07-10:1"
		}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			spec := ynab.TransactionSpec{
				AccountID: "ac1",
				Date:      ynab.NewDate(2026, time.July, 10),
				Amount:    -294230,
				PayeeName: ynab.Set("Grocer & Co"),
				Memo:      ynab.Set("weekly shop"),
				Cleared:   ynab.ClearedStatusCleared,
				Approved:  ynab.Set(false),
				ImportID:  ynab.Set("YNAB:-294230:2026-07-10:1"),
			}
			_, _, err := c.Plan("p-1").Transactions.Create(t.Context(), spec)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:      "createTransaction",
		variant: "batch",
		method:  http.MethodPost,
		path:    "/plans/p-1/transactions",
		body: `{"transactions":[
			{"account_id":"ac1","date":"2026-07-10","amount":-100,"category_id":null,
			 "subtransactions":[{"amount":-34,"memo":"a"},{"amount":-33},{"amount":-33}]},
			{"account_id":"ac1","date":"2026-07-11","amount":5000,"approved":false}
		]}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			split := ynab.Milliunits(-100).SplitEven(3)
			specs := []ynab.TransactionSpec{
				{
					AccountID:  "ac1",
					Date:       ynab.NewDate(2026, time.July, 10),
					Amount:     -100,
					CategoryID: ynab.SetNull[string](),
					Splits: []ynab.SubtransactionSpec{
						{Amount: split[0], Memo: ynab.Set("a")},
						{Amount: split[1]},
						{Amount: split[2]},
					},
				},
				{
					AccountID: "ac1",
					Date:      ynab.NewDate(2026, time.July, 11),
					Amount:    5000,
					Approved:  ynab.Set(false),
				},
			}
			_, err := c.Plan("p-1").Transactions.CreateBatch(t.Context(), specs)
			require.NoError(t, err)
		},
	})

	registerWriteModel(ynab.TransactionSpec{
		AccountID: "ac1",
		Date:      ynab.NewDate(2026, time.July, 10),
		Amount:    -1,
		PayeeID:   ynab.Set("p"),
		PayeeName: ynab.SetNull[string](),
		Memo:      ynab.Set(""),
		Approved:  ynab.Set(false),
		FlagColor: ynab.Set(ynab.FlagColorRed),
		ImportID:  ynab.Set("i"),
	})
	registerWriteModel(ynab.SubtransactionSpec{
		Amount:     -1,
		CategoryID: ynab.Set(""),
		Memo:       ynab.SetNull[string](),
	})
}

func TestTransactionsCreate(t *testing.T) {
	t.Parallel()

	t.Run("single create decodes 201", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "transactions/create.json", http.StatusCreated)
		created, sk, err := client.Plan("p-1").Transactions.Create(t.Context(), ynab.TransactionSpec{
			AccountID: "ac1", Date: ynab.NewDate(2026, time.July, 10), Amount: -294230,
		})
		require.NoError(t, err)
		require.Equal(t, http.MethodPost, rec.Method)
		require.Equal(t, ynab.ServerKnowledge(6400), sk)
		require.Equal(t, "tr555555-5555-5555-5555-555555555555", created.ID)
	})

	t.Run("duplicate import_id answers 409 ErrConflict", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"id":"409","name":"conflict","detail":"Duplicate import id"}}`))
		}))
		t.Cleanup(srv.Close)

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		_, _, err := client.Plan("p-1").Transactions.Create(t.Context(), ynab.TransactionSpec{
			AccountID: "ac1", Date: ynab.NewDate(2026, time.July, 10), Amount: -1,
			ImportID: ynab.Set("dup"),
		})
		require.ErrorIs(t, err, ynab.ErrConflict)
	})

	t.Run("batch duplicates fill DuplicateImportIDs with nil error", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "transactions/create_batch_dups.json", http.StatusCreated)
		batch, err := client.Plan("p-1").Transactions.CreateBatch(t.Context(), []ynab.TransactionSpec{
			{AccountID: "ac1", Date: ynab.NewDate(2026, time.July, 10), Amount: -1},
		})
		require.NoError(t, err, "batch duplicates are data, not an error")
		require.Equal(t, []string{"YNAB:-294230:2026-07-10:1"}, batch.DuplicateImportIDs)
		require.Empty(t, batch.TransactionIDs)
	})
}

func TestTransactionsCreatePreflight(t *testing.T) {
	t.Parallel()

	newSpec := func() ynab.TransactionSpec {
		return ynab.TransactionSpec{
			AccountID: "ac1", Date: ynab.NewDate(2026, time.July, 10), Amount: -100,
		}
	}

	tests := []struct {
		name   string
		mutate func(*ynab.TransactionSpec)
		field  string
	}{
		{
			name:   "import id beyond 36 characters",
			mutate: func(s *ynab.TransactionSpec) { s.ImportID = ynab.Set(strings.Repeat("x", 37)) },
			field:  "import_id",
		},
		{
			name:   "payee name beyond 200 characters",
			mutate: func(s *ynab.TransactionSpec) { s.PayeeName = ynab.Set(strings.Repeat("x", 201)) },
			field:  "payee_name",
		},
		{
			name:   "memo beyond 500 characters",
			mutate: func(s *ynab.TransactionSpec) { s.Memo = ynab.Set(strings.Repeat("x", 501)) },
			field:  "memo",
		},
		{
			name: "split amounts must sum to Amount",
			mutate: func(s *ynab.TransactionSpec) {
				s.Splits = []ynab.SubtransactionSpec{{Amount: -50}, {Amount: -49}}
			},
			field: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("no request must be sent on a pre-flight failure")
			}))
			t.Cleanup(srv.Close)
			client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

			spec := newSpec()
			tt.mutate(&spec)

			_, _, err := client.Plan("p-1").Transactions.Create(t.Context(), spec)
			var argErr *ynab.ArgumentError
			require.ErrorAs(t, err, &argErr)
			require.Equal(t, tt.field, argErr.Field)

			_, err = client.Plan("p-1").Transactions.CreateBatch(t.Context(), []ynab.TransactionSpec{spec})
			require.ErrorAs(t, err, &argErr, "batch validates every spec before any I/O")
		})
	}
}

func TestSplitEvenAlwaysValidates(t *testing.T) {
	t.Parallel()

	// Property: SplitEven output always passes the split-sum pre-flight.
	amounts := []ynab.Milliunits{-294230, -1, 0, 1, 100, -100, 999999999, -999999937}
	for _, amount := range amounts {
		for n := 1; n <= 7; n++ {
			legs := amount.SplitEven(n)
			splits := make([]ynab.SubtransactionSpec, len(legs))
			for i, leg := range legs {
				splits[i] = ynab.SubtransactionSpec{Amount: leg}
			}
			spec := ynab.TransactionSpec{
				AccountID: "ac1", Date: ynab.NewDate(2026, time.July, 10),
				Amount: amount, CategoryID: ynab.SetNull[string](), Splits: splits,
			}
			require.NoError(t, ynab.ValidateTransactionSpec(spec), "amount=%d n=%d", amount, n)
		}
	}
}
