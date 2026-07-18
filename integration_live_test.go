//go:build integration

package ynab_test

import (
	"os"
	"testing"

	"pkg.venceslau.dev/ynab"
)

// TestLiveIntegration runs every registered case against the real API,
// sequentially — the suite is never concurrent with itself. It skips
// cleanly without YNAB_TEST_TOKEN; writes only ever touch the dedicated
// test plan named by YNAB_TEST_PLAN_ID (default: the last-used plan of
// the test token).
func TestLiveIntegration(t *testing.T) {
	token := os.Getenv("YNAB_TEST_TOKEN")
	if token == "" {
		t.Skip("YNAB_TEST_TOKEN not set — live integration runs only against a dedicated test plan")
	}

	planID := ynab.PlanIDLastUsed
	if id := os.Getenv("YNAB_TEST_PLAN_ID"); id != "" {
		planID = ynab.PlanID(id)
	}

	env := integrationEnv{
		Client: ynab.New(token),
		PlanID: planID,
	}

	integrationMu.Lock()
	cases := make([]integrationCase, len(integrationCases))
	copy(cases, integrationCases)
	integrationMu.Unlock()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t, env) // no t.Parallel: sequential by doctrine
		})
	}
}
