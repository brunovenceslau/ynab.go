// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// TestEmptyIDsRejectedPreflight pins the empty-segment guard: an empty id
// argument must fail as *ArgumentError with zero server hits, never
// collapse a path segment and silently shift onto another route.
func TestEmptyIDsRejectedPreflight(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request must be sent for an empty id")
	}))
	t.Cleanup(srv.Close)
	client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	calls := map[string]func() error{
		"empty plan id": func() error {
			_, err := client.Plan("").Settings(t.Context())
			return err
		},
		"empty plan id on export": func() error {
			_, _, err := client.Plan("").Export(t.Context())
			return err
		},
		"empty account id": func() error {
			_, err := client.Plan("p-1").Accounts.Get(t.Context(), "")
			return err
		},
		"empty category id": func() error {
			_, err := client.Plan("p-1").Categories.Get(t.Context(), "")
			return err
		},
		"empty payee id": func() error {
			_, err := client.Plan("p-1").Payees.Get(t.Context(), "")
			return err
		},
		"empty transaction id": func() error {
			_, _, err := client.Plan("p-1").Transactions.Delete(t.Context(), "")
			return err
		},
		"empty scheduled id": func() error {
			_, err := client.Plan("p-1").Scheduled.Get(t.Context(), "")
			return err
		},
		"empty payee location id": func() error {
			_, err := client.Plan("p-1").PayeeLocations.Get(t.Context(), "")
			return err
		},
		"empty payee id on locations list": func() error {
			_, err := client.Plan("p-1").PayeeLocations.ListByPayee(t.Context(), "")
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var argErr *ynab.ArgumentError
			require.ErrorAs(t, call(), &argErr)
		})
	}

	t.Run("empty patch identity", func(t *testing.T) {
		t.Parallel()

		_, err := client.Plan("p-1").Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
			{}, // zero value: neither PatchByID nor PatchByImportID
		})
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Equal(t, "Transactions.UpdateBatch", argErr.Op)
	})
}
