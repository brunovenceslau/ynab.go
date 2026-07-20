// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// The fixture-honesty gate: the ynabtest fake server serves the exact
// golden fixture bytes the endpoint tests assert against, proven by
// decoding through both and requiring identical results. Coverage is
// derived, not hand-kept: every registered G4 read case whose fixture
// the fake serves is probed automatically — including the sixteen
// non-GET operations, whose success decodes register read cases in
// contract_complete_test.go — hand probes add the four delta streams,
// and a closing completeness check requires every FixtureNames() entry to be
// probed — a fake route whose fixture no probe decodes fails loudly.
// (Route→fixture mapping itself is pinned byte-level by the literal
// table in internal/ynabtest/server_test.go; shared fixtures across
// routes are that layer's job, not this one's.)

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/ynabtest"
)

// honestyProbe decodes one fixture through any client; the test runs it
// against the fake and against the read harness's fixture server.
type honestyProbe struct {
	name    string
	fixture string
	status  int // the status the fixture server answers; 0 means 200
	call    func(t *testing.T, c *ynab.Client) any
}

// derivedReadProbes turns the G4 read registry into probes, one per
// registered case whose fixture the fake serves. The served filter
// mechanically excludes the *_null/extreme/no-server-knowledge variants
// the fake has no route for.
func derivedReadProbes(served map[string]bool) []honestyProbe {
	readRegistryMu.Lock()
	cases := make([]readCase, len(readRegistry))
	copy(cases, readRegistry)
	readRegistryMu.Unlock()

	var probes []honestyProbe
	for _, rc := range cases {
		if rc.status != 0 || !served[rc.fixture] {
			continue
		}
		name := rc.op
		if rc.variant != "" {
			name += "/" + rc.variant
		}
		probes = append(probes, honestyProbe{
			name:    name,
			fixture: rc.fixture,
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				got, err := rc.call(t, c)
				require.NoError(t, err)
				require.NotNil(t, got)
				return got
			},
		})
	}
	return probes
}

// deltaProbes covers the four delta streams the fake switches to on a
// cursor — the read registry cannot reach them because no registered
// case passes [ynab.Since].
func deltaProbes() []honestyProbe {
	return []honestyProbe{
		{
			name: "getPlanById/delta", fixture: "plans/export_delta.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				detail, sk, err := c.Plan("p-1").Export(t.Context(), ynab.Since(1))
				require.NoError(t, err)
				return pairWithSK(detail, sk)
			},
		},
		{
			name: "getAccounts/delta", fixture: "accounts/list_delta.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				accounts, sk, err := c.Plan("p-1").Accounts.List(t.Context(), ynab.Since(1))
				require.NoError(t, err)
				return pairWithSK(accounts, sk)
			},
		},
		{
			name: "getCategories/delta", fixture: "categories/list_delta.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				groups, sk, err := c.Plan("p-1").Categories.List(t.Context(), ynab.Since(1))
				require.NoError(t, err)
				return pairWithSK(groups, sk)
			},
		},
		{
			name: "getPayees/delta", fixture: "payees/list_delta.json",
			call: func(t *testing.T, c *ynab.Client) any {
				t.Helper()
				payees, sk, err := c.Plan("p-1").Payees.List(t.Context(), ynab.Since(1))
				require.NoError(t, err)
				return pairWithSK(payees, sk)
			},
		},
	}
}

// pairWithSK bundles a decoded value with its server knowledge so the
// equality covers both returns.
func pairWithSK(v any, sk ynab.ServerKnowledge) any {
	return struct {
		Value any
		SK    ynab.ServerKnowledge
	}{v, sk}
}

func TestFixtureHonesty(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	viaFake := ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

	served := map[string]bool{}
	for _, name := range ynabtest.FixtureNames() {
		served[name] = true
	}
	probes := append(derivedReadProbes(served), deltaProbes()...)

	// The self-enforcing half of the header's claim: every fixture the
	// fake can serve is decoded by at least one probe — a new fake route
	// cannot ship without a probe here.
	probed := map[string]bool{}
	for _, probe := range probes {
		require.True(t, served[probe.fixture],
			"probe %s decodes %s, which the fake does not serve", probe.name, probe.fixture)
		probed[probe.fixture] = true
	}
	for _, name := range ynabtest.FixtureNames() {
		require.True(t, probed[name], "fake fixture %s has no honesty probe", name)
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()

			fromFake := probe.call(t, viaFake)

			direct, _ := serveFixture(t, probe.fixture, probe.status)
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
