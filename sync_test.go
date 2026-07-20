// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// entity is a minimal Syncable for merge tests.
type entity struct {
	ID      string
	Name    string
	Deleted bool
}

func (e entity) SyncID() string  { return e.ID }
func (e entity) IsDeleted() bool { return e.Deleted }

func TestMergeByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		local   map[string]entity
		changes []entity
		want    map[string]entity
	}{
		{
			name:    "nil local allocates and upserts",
			local:   nil,
			changes: []entity{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}},
			want:    map[string]entity{"a": {ID: "a", Name: "A"}, "b": {ID: "b", Name: "B"}},
		},
		{
			name:    "tombstone deletes",
			local:   map[string]entity{"a": {ID: "a", Name: "A"}, "b": {ID: "b", Name: "B"}},
			changes: []entity{{ID: "a", Deleted: true}},
			want:    map[string]entity{"b": {ID: "b", Name: "B"}},
		},
		{
			name:    "upsert overwrites",
			local:   map[string]entity{"a": {ID: "a", Name: "old"}},
			changes: []entity{{ID: "a", Name: "new"}},
			want:    map[string]entity{"a": {ID: "a", Name: "new"}},
		},
		{
			name:    "tombstone for unknown id is a no-op",
			local:   map[string]entity{"a": {ID: "a", Name: "A"}},
			changes: []entity{{ID: "ghost", Deleted: true}},
			want:    map[string]entity{"a": {ID: "a", Name: "A"}},
		},
		{
			name:    "empty changes returns local unchanged",
			local:   map[string]entity{"a": {ID: "a", Name: "A"}},
			changes: nil,
			want:    map[string]entity{"a": {ID: "a", Name: "A"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ynab.MergeByID(tt.local, tt.changes)
			require.Equal(t, tt.want, got)
			if tt.local != nil {
				require.Equal(t, tt.want, tt.local, "merge mutates and returns the same map")
			}
		})
	}

	t.Run("pointer elements", func(t *testing.T) {
		t.Parallel()

		got := ynab.MergeByID(nil, []*entity{{ID: "a", Name: "A"}, {ID: "b", Deleted: true}})
		require.Len(t, got, 1)
		require.Equal(t, "A", got["a"].Name)
	})
}

func TestSince(t *testing.T) {
	t.Parallel()

	q := url.Values{}
	ynab.ApplyListOptions(q, ynab.Since(1473))
	require.Equal(t, "1473", q.Get("last_knowledge_of_server"))

	q = url.Values{}
	ynab.ApplyListOptions(q, ynab.Since(0))
	require.Equal(t, "0", q.Get("last_knowledge_of_server"), "explicit zero cursor is sent, not omitted")

	q = url.Values{}
	ynab.ApplyListOptions(q)
	require.Empty(t, q, "no options, no parameters")

	var zero ynab.ListOption
	q = url.Values{}
	require.NotPanics(t, func() { ynab.ApplyListOptions(q, zero) }, "zero ListOption is inert")
	require.Empty(t, q)

	got := ynab.ApplyListOptions(nil, ynab.Since(7))
	require.Equal(t, "7", got.Get("last_knowledge_of_server"), "nil query map self-allocates")
}

// TestSyncableAdapters pins every SyncID/IsDeleted pair: a wrong merge
// key would silently corrupt users' delta stores and no wire gate can
// see it.
func TestSyncableAdapters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    ynab.Syncable
		id   string
	}{
		{name: "Account", s: ynab.Account{AccountBase: ynab.AccountBase{ID: "a", Deleted: true}}, id: "a"},
		{name: "Category", s: ynab.Category{CategoryBase: ynab.CategoryBase{ID: "c", Deleted: true}}, id: "c"},
		{name: "CategoryGroup", s: ynab.CategoryGroup{ID: "g", Deleted: true}, id: "g"},
		{name: "Payee", s: ynab.Payee{ID: "p", Deleted: true}, id: "p"},
		{name: "PayeeLocation", s: ynab.PayeeLocation{ID: "l", Deleted: true}, id: "l"},
		{
			name: "MonthSummary",
			s: ynab.MonthSummary{MonthSummaryBase: ynab.MonthSummaryBase{
				Month: ynab.NewMonth(2026, time.July), Deleted: true,
			}},
			id: "2026-07-01",
		},
		{
			name: "Transaction",
			s:    ynab.Transaction{TransactionBase: ynab.TransactionBase{ID: "t", Deleted: true}},
			id:   "t",
		},
		{
			name: "Subtransaction",
			s:    ynab.Subtransaction{SubtransactionBase: ynab.SubtransactionBase{ID: "s", Deleted: true}},
			id:   "s",
		},
		{
			name: "HybridTransaction",
			s: ynab.HybridTransaction{
				TransactionBase:     ynab.TransactionBase{ID: "h", Deleted: true},
				ParentTransactionID: ptr("NOT-the-key"),
			},
			id: "h",
		},
		{
			name: "ScheduledTransaction",
			s: ynab.ScheduledTransaction{
				ScheduledTransactionBase: ynab.ScheduledTransactionBase{ID: "sc", Deleted: true},
			},
			id: "sc",
		},
		{
			name: "ScheduledSubtransaction",
			s: ynab.ScheduledSubtransaction{
				ScheduledSubtransactionBase: ynab.ScheduledSubtransactionBase{ID: "ss", Deleted: true},
			},
			id: "ss",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.id, tt.s.SyncID())
			require.True(t, tt.s.IsDeleted())
		})
	}
}

func ptr[T any](v T) *T { return &v }

// TestEnumValidTables covers every enum's Valid across its full value set
