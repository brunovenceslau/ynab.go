package ynab_test

import (
	"net/url"
	"testing"

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
