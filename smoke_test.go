//go:build smoke

package ynab_test

// The weekly read-only smoke against the real API (gate G8's first
// layer): decode + taxonomy assertions only, never a write. Runs with
// YNAB_TEST_TOKEN; skips cleanly without it.

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func TestLiveSmoke(t *testing.T) {
	token := os.Getenv("YNAB_TEST_TOKEN")
	if token == "" {
		t.Skip("YNAB_TEST_TOKEN not set — the smoke runs read-only against the real API")
	}

	client := ynab.New(token)
	ctx := t.Context()

	user, err := client.User(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, user.ID)

	plans, err := client.Plans(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, plans.Plans)

	plan := client.Plan(ynab.PlanIDLastUsed)

	accounts, accountsSK, err := plan.Accounts.List(ctx)
	require.NoError(t, err)
	require.Positive(t, int64(accountsSK))
	for _, a := range accounts {
		require.True(t, a.Type.Valid(), "unknown account type %q — spec drift?", a.Type)
	}

	groups, _, err := plan.Categories.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, groups)

	months, _, err := plan.Months.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, months)

	// Delta round-trip: a read from the fresh cursor answers quickly and
	// near-empty.
	delta, deltaSK, err := plan.Accounts.List(ctx, ynab.Since(accountsSK))
	require.NoError(t, err)
	require.GreaterOrEqual(t, int64(deltaSK), int64(accountsSK))
	require.Empty(t, delta, "nothing changed between the two reads")

	// Taxonomy smoke: a nonsense plan id must answer through the sentinel
	// taxonomy, never a decode error.
	_, err = client.Plan("00000000-0000-0000-0000-000000000000").Settings(ctx)
	require.Error(t, err)
	require.True(t, errors.Is(err, ynab.ErrNotFound) || errors.Is(err, ynab.ErrBadRequest),
		"expected a taxonomy answer, got: %v", err)
}
