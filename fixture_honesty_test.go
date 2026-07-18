package ynab_test

// The fixture-honesty test: the ynabtest fake server serves the exact
// golden fixture bytes the endpoint tests assert against, and the real
// client decodes them identically through both. Fixtures therefore
// cannot diverge between the fake and the endpoint harness.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/ynabtest"
)

func TestFixtureHonesty(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	viaFake := ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	// Each probe decodes through the fake AND through the endpoint
	// harness's fixture server; the results must be identical.
	probes := []struct {
		name    string
		fixture string
		call    func(t *testing.T, c *ynab.Client) any
	}{
		{
			name: "user", fixture: "user/get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				u, err := c.User(t.Context())
				require.NoError(t, err)
				return u
			},
		},
		{
			name: "accounts list", fixture: "accounts/list.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				accounts, sk, err := c.Plan("p-1").Accounts.List(t.Context())
				require.NoError(t, err)
				return struct {
					Accounts []ynab.Account
					SK       ynab.ServerKnowledge
				}{accounts, sk}
			},
		},
		{
			name: "categories list", fixture: "categories/list.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				groups, sk, err := c.Plan("p-1").Categories.List(t.Context())
				require.NoError(t, err)
				return struct {
					Groups []ynab.CategoryGroup
					SK     ynab.ServerKnowledge
				}{groups, sk}
			},
		},
		{
			name: "plan export", fixture: "plans/export.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				detail, sk, err := c.Plan("p-1").Export(t.Context())
				require.NoError(t, err)
				return struct {
					Detail *ynab.PlanDetail
					SK     ynab.ServerKnowledge
				}{detail, sk}
			},
		},
		{
			name: "month detail", fixture: "months/get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				month, err := c.Plan("p-1").Months.Get(t.Context(), ynab.NewMonth(2026, time.July))
				require.NoError(t, err)
				return month
			},
		},
		{
			name: "transactions list", fixture: "transactions/list.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				txns, sk, err := c.Plan("p-1").Transactions.List(t.Context(), ynab.TransactionFilter{})
				require.NoError(t, err)
				return struct {
					Txns []ynab.Transaction
					SK   ynab.ServerKnowledge
				}{txns, sk}
			},
		},
		{
			name: "scheduled list", fixture: "scheduled/list.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				scheduled, sk, err := c.Plan("p-1").Scheduled.List(t.Context())
				require.NoError(t, err)
				return struct {
					Scheduled []ynab.ScheduledTransaction
					SK        ynab.ServerKnowledge
				}{scheduled, sk}
			},
		},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()

			fromFake := probe.call(t, viaFake)

			direct, _ := serveFixture(t, probe.fixture, 0)
			fromFixture := probe.call(t, direct)

			require.Equal(t, fromFixture, fromFake,
				"the fake and the endpoint harness must decode identically")
		})
	}
}

func TestFixtureHonestyDelta(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	client := ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	accounts, sk, err := client.Plan("p-1").Accounts.List(t.Context(), ynab.Since(1473))
	require.NoError(t, err)
	require.Equal(t, ynab.ServerKnowledge(1600), sk)
	require.Len(t, accounts, 1)
	require.True(t, accounts[0].IsDeleted(), "delta cursors serve tombstoned changes")
}

func TestFixtureHonestyFailWith(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	client := ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	srv.FailWith(403, "403.1", "subscription_lapsed")
	_, err := client.User(t.Context())
	require.ErrorIs(t, err, ynab.ErrSubscriptionLapsed)
	require.ErrorIs(t, err, ynab.ErrForbidden, "injected envelopes are taxonomy-correct")
}
