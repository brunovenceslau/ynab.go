// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// The fixture-honesty test: the ynabtest fake server serves the exact
// golden fixture bytes the read cases assert against, and the real
// client decodes them identically through both. Fixtures therefore
// cannot diverge between the fake and the read harness.

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

	// Each probe decodes through the fake AND through the read
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
		{
			name: "plans list", fixture: "plans/list.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				plans, err := c.Plans(t.Context())
				require.NoError(t, err)
				return plans
			},
		},
		{
			name: "plan settings", fixture: "plans/settings.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				settings, err := c.Plan("p-1").Settings(t.Context())
				require.NoError(t, err)
				return settings
			},
		},
		{
			name: "account get", fixture: "accounts/get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				account, err := c.Plan("p-1").Accounts.Get(t.Context(), "ac1")
				require.NoError(t, err)
				return account
			},
		},
		{
			name: "payee get", fixture: "payees/get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				payee, err := c.Plan("p-1").Payees.Get(t.Context(), "pa1")
				require.NoError(t, err)
				return payee
			},
		},
		{
			name: "payee locations by payee", fixture: "payee_locations/by_payee.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				locations, err := c.Plan("p-1").PayeeLocations.ListByPayee(t.Context(), "pa1")
				require.NoError(t, err)
				return locations
			},
		},
		{
			name: "months list", fixture: "months/list.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				months, sk, err := c.Plan("p-1").Months.List(t.Context())
				require.NoError(t, err)
				return struct {
					Months []ynab.MonthSummary
					SK     ynab.ServerKnowledge
				}{months, sk}
			},
		},
		{
			name: "month category get", fixture: "categories/month_get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				category, err := c.Plan("p-1").Categories.GetForMonth(t.Context(), ynab.CurrentMonth(), "ca1")
				require.NoError(t, err)
				return category
			},
		},
		{
			name: "money movements by month", fixture: "money_movements/by_month.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				movements, _, err := c.Plan("p-1").MoneyMovements.ListByMonth(t.Context(), ynab.CurrentMonth())
				require.NoError(t, err)
				return movements
			},
		},
		{
			name: "money movement groups", fixture: "money_movements/groups.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				groups, _, err := c.Plan("p-1").MoneyMovements.ListGroups(t.Context())
				require.NoError(t, err)
				return groups
			},
		},
		{
			name: "hybrid by category", fixture: "transactions/hybrid.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				rows, _, err := c.Plan("p-1").Transactions.ListByCategory(t.Context(), "ca1", ynab.TransactionFilter{})
				require.NoError(t, err)
				return rows
			},
		},
		{
			name: "transaction get", fixture: "transactions/get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				tx, _, err := c.Plan("p-1").Transactions.Get(t.Context(), "tr1")
				require.NoError(t, err)
				return tx
			},
		},
		{
			name: "scheduled get", fixture: "scheduled/get.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				scheduled, err := c.Plan("p-1").Scheduled.Get(t.Context(), "sc1")
				require.NoError(t, err)
				return scheduled
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
