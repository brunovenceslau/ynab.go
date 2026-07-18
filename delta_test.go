package ynab_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/ynabtest"
)

func TestSyncStateJSON(t *testing.T) {
	t.Parallel()

	t.Run("frozen wire format", func(t *testing.T) {
		t.Parallel()

		st := ynab.SyncState{
			PlanID:       "p-1",
			Plan:         8000,
			Accounts:     1473,
			Transactions: 6300,
		}
		raw, err := json.Marshal(st)
		require.NoError(t, err)
		require.JSONEq(t, `{"plan_id":"p-1","plan":8000,"accounts":1473,"transactions":6300}`, string(raw),
			"lowercase keys, zero cursors omitted")

		var back ynab.SyncState
		require.NoError(t, json.Unmarshal(raw, &back))
		require.Equal(t, st, back)
	})

	t.Run("zero state emits only the plan id", func(t *testing.T) {
		t.Parallel()

		raw, err := json.Marshal(ynab.SyncState{PlanID: "p-1"})
		require.NoError(t, err)
		require.JSONEq(t, `{"plan_id":"p-1"}`, string(raw))
	})
}

func TestPlanDelta(t *testing.T) {
	t.Parallel()

	t.Run("full read then delta, advancing in place", func(t *testing.T) {
		t.Parallel()

		srv := ynabtest.NewServer(t)
		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		plan := client.Plan("p-1")

		st := &ynab.SyncState{}
		full, err := plan.Delta(t.Context(), st)
		require.NoError(t, err)
		require.NotEmpty(t, full.Accounts, "first call is the full read")
		require.Equal(t, ynab.PlanID("p-1"), st.PlanID)
		require.Equal(t, ynab.ServerKnowledge(8000), st.Plan, "st advances in place")

		delta, err := plan.Delta(t.Context(), st)
		require.NoError(t, err)
		require.Empty(t, delta.Accounts, "the cursor switches to the delta response")
		require.Len(t, delta.Transactions, 1)
		require.True(t, delta.Transactions[0].Deleted, "tombstones arrive inside collections")
		require.Equal(t, ynab.ServerKnowledge(8100), st.Plan)

		// The per-service cursors are separate spaces and stay untouched.
		require.Zero(t, st.Accounts)
		require.Zero(t, st.Transactions)

		// Folding the delta into a local store via MergeByID.
		local := ynab.MergeByID(nil, full.Transactions)
		merged := ynab.MergeByID(local, delta.Transactions)
		require.Empty(t, merged, "the tombstone removed the only transaction")
	})

	t.Run("the persisted cursor reaches the wire", func(t *testing.T) {
		t.Parallel()

		// A recording server pins the exact last_knowledge_of_server the
		// second Delta sends — the fixture switch alone cannot see a
		// wrong value.
		client, rec := serveFixture(t, "plans/export.json", 0)
		st := &ynab.SyncState{PlanID: "p-1", Plan: 8000}
		_, err := client.Plan("p-1").Delta(t.Context(), st)
		require.NoError(t, err)
		require.Equal(t, "8000", rec.URL.Query().Get("last_knowledge_of_server"))
	})

	t.Run("an unattributed cursor is rejected", func(t *testing.T) {
		t.Parallel()

		// A cursor without a plan id cannot be verified against this
		// handle — the plan-mismatch guard must not be bypassable by
		// blanking the id.
		client := ynab.New("t", ynab.WithBaseURL("http://127.0.0.1:1"), ynab.WithRetryDisabled())
		st := &ynab.SyncState{Plan: 100}
		_, err := client.Plan("p-1").Delta(t.Context(), st)
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
	})

	t.Run("state does not advance on failure", func(t *testing.T) {
		t.Parallel()

		// The corruption class this pins: a failed request must leave the
		// persisted cursor exactly as it was.
		srv := ynabtest.NewServer(t)
		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		srv.FailWith(500, "500", "internal_server_error")

		st := &ynab.SyncState{PlanID: "p-1", Plan: 8000, Accounts: 1473}
		before := *st
		_, err := client.Plan("p-1").Delta(t.Context(), st)
		require.ErrorIs(t, err, ynab.ErrServerError)
		require.Equal(t, before, *st, "st must be untouched after a failed Delta")
	})

	t.Run("nil state is a pre-flight error", func(t *testing.T) {
		t.Parallel()

		// Unroutable base URL: if the guard ever regressed this fails
		// hermetically instead of dialing out.
		client := ynab.New("t", ynab.WithBaseURL("http://127.0.0.1:1"), ynab.WithRetryDisabled())
		_, err := client.Plan("p-1").Delta(t.Context(), nil)
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
	})

	t.Run("another plan's cursor is rejected", func(t *testing.T) {
		t.Parallel()

		// Reusing plan A's cursor against plan B would silently corrupt a
		// local store; the mismatch fails before any I/O.
		client := ynab.New("t", ynab.WithBaseURL("http://127.0.0.1:1"), ynab.WithRetryDisabled())
		st := &ynab.SyncState{PlanID: "plan-a", Plan: 100}
		_, err := client.Plan("plan-b").Delta(t.Context(), st)
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
		require.Contains(t, argErr.Reason, "plan-a")
	})
}
