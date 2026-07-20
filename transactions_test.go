// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"bytes"
	"encoding/json"
	"net/url"
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
		require.Equal(t, ynab.NewDate(2026, time.July, 10), tx.Date)
		require.Equal(t, ynab.Milliunits(-294230), tx.Amount)
		require.Equal(t, ynab.ClearedStatusCleared, tx.Cleared)
		require.True(t, tx.Cleared.Valid())
		require.True(t, tx.Approved)
		require.Equal(t, ynab.FlagColorRed, *tx.FlagColor)
		require.Equal(t, "urgent", *tx.FlagName)
		require.Equal(t, "YNAB:-294230:2026-07-10:1", *tx.ImportID)
		require.Equal(t, "Split", *tx.CategoryName)

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
		require.Len(t, rows, 2)

		regular, leg := rows[0], rows[1]
		require.Equal(t, ynab.HybridTypeTransaction, regular.Type)
		require.True(t, regular.Type.Valid())
		require.Nil(t, regular.ParentTransactionID)

		require.Equal(t, ynab.HybridTypeSubtransaction, leg.Type)
		require.Equal(t, regular.ID, *leg.ParentTransactionID)
		require.Equal(t, "Vacation", leg.CategoryName)
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
