// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Live-integration machinery. Cases are declared here — untagged, so the
// tokenless completeness gate can enumerate them — while the
// runner that actually hits the API lives behind //go:build integration
// and skips cleanly without YNAB_TEST_TOKEN. Suite discipline: sequential
// (never concurrent with itself), well under 200 requests/hour, writes
// create→verify→cleanup on a dedicated test plan only.
//
// Live-unprovable variants, enumerated so the next audit does not
// re-derive them:
//   - importTransactions' 201 branch needs a linked account, which a
//     test plan cannot have — the live case pins the 200-empty branch
//     and its status/result coherence via LastStatus, and stands ready
//     to verify the 201 branch if a linked account ever appears.
//   - payee locations cannot be created through the API (mobile-app
//     writes only), so the populated branch of the payee-locations case
//     is dead on an API-only test plan; the empty branch carries the
//     suite's single 404 probe instead.
//   - the scheduled empty-plan 404 fold is caller-indistinguishable
//     from a plain 200-empty by design; the scheduled case logs which
//     branch really ran via LastStatus.

import (
	"net/http"
	"regexp"
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

	// LastStatus reports the raw HTTP status of the most recent recorded
	// request matching opID — the harness hook that lets a case see the
	// wire's status split the client deliberately folds (import's
	// 200-vs-201, scheduled's 404 fold). Wired by the live runner only;
	// case bodies run nowhere else, so it is never called while nil.
	LastStatus func(opID string) (int, bool)
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

// templateParamRe matches one {param} placeholder in a table path.
var templateParamRe = regexp.MustCompile(`\{[^}]+\}`)

// TestMatchOperationUnambiguous freezes the uniqueness property the live
// runner's first-match-wins matchOperation depends on: no operation's
// path template may match a concrete instance of another same-method
// operation's path. It passes today (verified by pairwise inspection) and
// fails at the exact commit that adds a colliding row to the table —
// e.g. a future GET /plans/{plan_id}/transactions/summary would silently
// mis-attribute to getTransactionById without this gate.
func TestMatchOperationUnambiguous(t *testing.T) {
	t.Parallel()

	table := contract.Table()
	for _, a := range table {
		re := contract.PathRegexp(a.Path)
		for _, b := range table {
			if a.ID == b.ID || a.Method != b.Method {
				continue
			}
			concrete := templateParamRe.ReplaceAllString(b.Path, "x1")
			require.False(t, re.MatchString(concrete),
				"template %s (%s) matches %s's concrete path %s — matchOperation would mis-attribute",
				a.Path, a.ID, b.ID, concrete)
		}
	}
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
