// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// decodeFixture strictly decodes a wrapped fixture document — unknown
// keys fail, so a typo'd fixture cannot pass vacuously.
func decodeFixture[T any](t *testing.T, fixture, wrapper string) T {
	t.Helper()

	raw := loadFixture(t, fixture)
	var env map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &env))
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(env["data"], &data))

	var v T
	dec := json.NewDecoder(bytes.NewReader(data[wrapper]))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&v))
	return v
}

func TestTransactionModels(t *testing.T) {
	t.Parallel()

	t.Run("full transaction with subtransactions", func(t *testing.T) {
		t.Parallel()

		tx := decodeFixture[ynab.Transaction](t, "transactions/get.json", "transaction")
		require.Equal(t, goldenGroceryRunTransaction(), tx)
		require.True(t, tx.Cleared.Valid())

		require.Len(t, tx.Subtransactions, 2)
		sum := ynab.Milliunits(0)
		for _, leg := range tx.Subtransactions {
			require.Equal(t, tx.ID, leg.TransactionID)
			sum = sum.Add(leg.Amount)
		}
		require.Equal(t, tx.Amount, sum, "split legs sum to the parent amount")
		require.Equal(t, "second leg", *tx.Subtransactions[1].Memo)
	})

	t.Run("all-null variant decodes", func(t *testing.T) {
		t.Parallel()

		tx := decodeFixture[ynab.Transaction](t, "transactions/get_null.json", "transaction")
		require.Nil(t, tx.Memo)
		require.Nil(t, tx.FlagColor, "null flag_color is nil, never the empty FlagColorNone")
		require.Nil(t, tx.PayeeID)
		require.Nil(t, tx.CategoryID)
		require.Nil(t, tx.ImportID)
		require.Nil(t, tx.CategoryName)
		require.False(t, tx.Approved)
		require.Nil(t, tx.Subtransactions[0].CategoryID)
	})

	t.Run("hybrid rows carry both type values", func(t *testing.T) {
		t.Parallel()

		rows := decodeFixture[[]ynab.HybridTransaction](t, "transactions/hybrid.json", "transactions")

		leg := ynab.HybridTransaction{
			TransactionBase: ynab.TransactionBase{
				ID:                      "st555555-5555-5555-5555-555555555555",
				Date:                    ynab.NewDate(2026, time.July, 10),
				Amount:                  -5000,
				Memo:                    ptr("groceries run"),
				Cleared:                 ynab.ClearedStatusCleared,
				Approved:                true,
				FlagColor:               ptr(ynab.FlagColorRed),
				FlagName:                ptr("urgent"),
				AccountID:               "ac111111-1111-1111-1111-111111111111",
				PayeeID:                 ptr("pa111111-1111-1111-1111-111111111111"),
				CategoryID:              ptr("ca111111-1111-1111-1111-111111111111"),
				ImportID:                ptr("YNAB:-294230:2026-07-10:1"),
				ImportPayeeName:         ptr("GROCER CO"),
				ImportPayeeNameOriginal: ptr("GROCER*CO 123"),
				Deleted:                 false,
			},
			AmountFormatted:     "-$5.00",
			AmountCurrency:      -5,
			Type:                ynab.HybridTypeSubtransaction,
			ParentTransactionID: ptr("tr444444-4444-4444-4444-444444444444"),
			AccountName:         "Checking",
			CategoryName:        "Vacation",
		}
		require.Equal(t, []ynab.HybridTransaction{goldenHybridGroceryRow(), leg}, rows)
		require.True(t, rows[0].Type.Valid())
	})

	t.Run("unknown enum values decode losslessly", func(t *testing.T) {
		t.Parallel()

		raw := loadFixture(t, "transactions/get.json")
		mutated := replaceOnce(t, raw, `"cleared": "cleared"`, `"cleared": "quantum"`)
		mutated = replaceOnce(t, mutated, `"flag_color": "red"`, `"flag_color": "ultraviolet"`)

		var env struct {
			Data struct {
				Transaction ynab.Transaction `json:"transaction"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(mutated, &env))
		require.Equal(t, ynab.ClearedStatus("quantum"), env.Data.Transaction.Cleared)
		require.False(t, env.Data.Transaction.Cleared.Valid())
		require.Equal(t, ynab.FlagColor("ultraviolet"), *env.Data.Transaction.FlagColor)
		require.False(t, env.Data.Transaction.FlagColor.Valid())
	})

	t.Run("extreme numerics decode", func(t *testing.T) {
		t.Parallel()
		runExtremeNumericsCase(t, ynab.Transaction{}, "transactions/extreme.json", "transaction")
	})
}

func TestFlagColor(t *testing.T) {
	t.Parallel()

	// Null vs "" are different facts: null means "no flag on the wire",
	// FlagColorNone ("") is the deliberate clear-the-flag write value.
	var withNull struct {
		FlagColor *ynab.FlagColor `json:"flag_color"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"flag_color":null}`), &withNull))
	require.Nil(t, withNull.FlagColor)

	var withEmpty struct {
		FlagColor *ynab.FlagColor `json:"flag_color"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"flag_color":""}`), &withEmpty))
	require.NotNil(t, withEmpty.FlagColor)
	require.Equal(t, ynab.FlagColorNone, *withEmpty.FlagColor)
	require.True(t, withEmpty.FlagColor.Valid(), "the empty flag is the one valid zero enum")
}

func TestClearedStatus(t *testing.T) {
	t.Parallel()

	for _, s := range []ynab.ClearedStatus{
		ynab.ClearedStatusUncleared, ynab.ClearedStatusCleared, ynab.ClearedStatusReconciled,
	} {
		require.True(t, s.Valid(), s)
	}
	require.False(t, ynab.ClearedStatus("pending").Valid())
	require.False(t, ynab.ClearedStatus("").Valid(), "unlike FlagColor, the zero cleared status is invalid")
}

func TestTransactionFilterEncode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter ynab.TransactionFilter
		want   url.Values
	}{
		{name: "zero filter encodes nothing", filter: ynab.TransactionFilter{}, want: nil},
		{
			name:   "since date",
			filter: ynab.TransactionFilter{SinceDate: ynab.NewDate(2026, time.January, 1)},
			want:   url.Values{"since_date": {"2026-01-01"}},
		},
		{
			name:   "until date",
			filter: ynab.TransactionFilter{UntilDate: ynab.NewDate(2026, time.June, 30)},
			want:   url.Values{"until_date": {"2026-06-30"}},
		},
		{
			name:   "type",
			filter: ynab.TransactionFilter{Type: ynab.TransactionTypeUnapproved},
			want:   url.Values{"type": {"unapproved"}},
		},
		{
			name:   "delta cursor",
			filter: ynab.TransactionFilter{Since: 6000},
			want:   url.Values{"last_knowledge_of_server": {"6000"}},
		},
		{
			name: "combined",
			filter: ynab.TransactionFilter{
				SinceDate: ynab.NewDate(2026, time.January, 1),
				UntilDate: ynab.NewDate(2026, time.June, 30),
				Type:      ynab.TransactionTypeUncategorized,
				Since:     42,
			},
			want: url.Values{
				"since_date":               {"2026-01-01"},
				"until_date":               {"2026-06-30"},
				"type":                     {"uncategorized"},
				"last_knowledge_of_server": {"42"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ynab.EncodeTransactionFilter(tt.filter))
		})
	}
}

func TestUpdateBatchAnnotatesPatchIndex(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request must be sent on a pre-flight failure")
	}))
	t.Cleanup(srv.Close)
	client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	long := strings.Repeat("x", 501)
	_, err := client.Plan("p-1").Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
		ynab.PatchByID("tr1", ynab.TransactionUpdate{Memo: ynab.Set("ok")}),
		ynab.PatchByID("tr2", ynab.TransactionUpdate{Memo: ynab.Set(long)}),
	})
	var argErr *ynab.ArgumentError
	require.ErrorAs(t, err, &argErr)
	require.Contains(t, argErr.Reason, "(patch 1)", "the failing element must be named, like CreateBatch's (spec N)")
}

func TestUpdateBatchBoundsImportIDIdentity(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request must be sent on a pre-flight failure")
	}))
	t.Cleanup(srv.Close)
	client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	// 37 code points: one past the spec's 36. The bound counts runes, not
	// bytes — ééé… would be 74 bytes at 37 runes and must fail identically.
	_, err := client.Plan("p-1").Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
		ynab.PatchByID("tr1", ynab.TransactionUpdate{Memo: ynab.Set("ok")}),
		ynab.PatchByImportID(strings.Repeat("é", 37), ynab.TransactionUpdate{}),
	})
	var argErr *ynab.ArgumentError
	require.ErrorAs(t, err, &argErr)
	require.Equal(t, "import_id", argErr.Field)
	require.Contains(t, argErr.Reason, "(patch 1)")
}

func TestUpdateBatchIDIdentityIsUnbounded(t *testing.T) {
	t.Parallel()

	// The spec declares maxLength on import_id only; a long transaction id
	// must reach the wire. The fake captures the body, because "unbounded"
	// means sent intact — a request merely going out would also pass if a
	// refactor silently dropped or truncated the identity.
	bodyCh := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodyCh <- b
		_, _ = w.Write([]byte(`{"data":{"transactions":[],"transaction_ids":[],` +
			`"duplicate_import_ids":[],"server_knowledge":1}}`))
	}))
	t.Cleanup(srv.Close)
	client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	_, err := client.Plan("p-1").Transactions.UpdateBatch(t.Context(), []ynab.TransactionPatch{
		ynab.PatchByID(strings.Repeat("x", 100), ynab.TransactionUpdate{}),
		// And the boundary itself: exactly 36 runes of import_id pass.
		ynab.PatchByImportID(strings.Repeat("é", 36), ynab.TransactionUpdate{}),
	})
	require.NoError(t, err)

	body := string(<-bodyCh)
	require.Contains(t, body, strings.Repeat("x", 100), "the unbounded id must reach the wire intact")
	require.Contains(t, body, strings.Repeat("é", 36), "the boundary import_id must reach the wire intact")
}
