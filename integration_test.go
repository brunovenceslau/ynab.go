// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Live-integration machinery. Cases are declared here — untagged, so the
// tokenless completeness gate (Task 40) can enumerate them — while the
// runner that actually hits the API lives behind //go:build integration
// and skips cleanly without YNAB_TEST_TOKEN. Suite discipline: sequential
// (never concurrent with itself), well under 200 requests/hour, writes
// create→verify→cleanup on a dedicated test plan only.

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

// integrationEnv is what a live case gets to work with.
type integrationEnv struct {
	Client *ynab.Client
	PlanID ynab.PlanID
}

// integrationCase declares one live case and the operationIds it covers.
// order sequences the live run explicitly (stable-sorted, default 0):
// registration order is init-order, i.e. file-name alphabetical — an
// invariant a rename would silently break.
type integrationCase struct {
	name  string
	ops   []string
	order int
	run   func(t *testing.T, env integrationEnv)
}

var (
	integrationMu    sync.Mutex
	integrationCases []integrationCase
)

// registerIntegrationCase is called from slice test files' init functions.
func registerIntegrationCase(c integrationCase) {
	integrationMu.Lock()
	defer integrationMu.Unlock()
	integrationCases = append(integrationCases, c)
}

// TestIntegrationCoverage is the tokenless completeness gate (G8's
// second layer): every one of the 44 operations must be covered by at
// least one registered live-integration case, and no case may claim an
// operation the contract table doesn't know. It runs in the normal
// suite — no tag, no token. Limit acknowledged: ops is self-declared
// metadata, so the gate proves a case exists per operation, not that
// the case body still calls it — that half is the live runner's job.
func TestIntegrationCoverage(t *testing.T) {
	t.Parallel()

	integrationMu.Lock()
	cases := make([]integrationCase, len(integrationCases))
	copy(cases, integrationCases)
	integrationMu.Unlock()

	covered := map[string]struct{}{}
	for _, c := range cases {
		for _, op := range c.ops {
			covered[op] = struct{}{}
		}
	}

	table := contract.Table()
	for _, op := range table {
		require.Contains(t, covered, op.ID, "operation %s has no live-integration case", op.ID)
	}
	for op := range covered {
		found := false
		for _, row := range table {
			if row.ID == op {
				found = true
				break
			}
		}
		require.True(t, found, "integration case declares unknown operation %s", op)
	}
	require.Len(t, table, 44)
}

func init() {
	registerIntegrationCase(integrationCase{
		name: "user and plans reads",
		ops:  []string{"getUser", "getPlans", "getPlanSettingsById"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			u, err := env.Client.User(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, u.ID)

			plans, err := env.Client.Plans(t.Context(), ynab.IncludeAccounts())
			require.NoError(t, err)
			require.NotEmpty(t, plans.Plans, "the test token must reach at least the test plan")
			for _, p := range plans.Plans {
				require.NotEmpty(t, p.ID)
				require.NotEmpty(t, p.Name)
				require.NotNil(t, p.Accounts, "IncludeAccounts must embed each plan's accounts")
			}

			settings, err := env.Client.Plan(env.PlanID).Settings(t.Context())
			require.NoError(t, err)
			require.NotNil(t, settings.CurrencyFormat, "a real plan always carries a currency format")
			require.NotEmpty(t, settings.CurrencyFormat.ISOCode)
			require.NotNil(t, settings.DateFormat)

			// The last-used alias, pinned read-only: the dedicated-plan rule
			// only forbids writes through it.
			_, err = env.Client.Plan(ynab.PlanIDLastUsed).Settings(t.Context())
			require.NoError(t, err, "the real server must accept the last-used alias")

			// RawDo, the escape hatch, against the real envelope: raw bytes,
			// no unwrapping.
			raw, err := env.Client.RawDo(t.Context(), http.MethodGet, "user", nil, nil)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"data"`, "RawDo must hand back the untouched envelope")
		},
	})
}
