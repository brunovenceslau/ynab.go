package ynab_test

// Live-integration machinery. Cases are declared here — untagged, so the
// tokenless completeness gate (Task 40) can enumerate them — while the
// runner that actually hits the API lives behind //go:build integration
// and skips cleanly without YNAB_TEST_TOKEN. Suite discipline: sequential
// (never concurrent with itself), well under 200 requests/hour, writes
// create→verify→cleanup on a dedicated test plan only.

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// integrationEnv is what a live case gets to work with.
type integrationEnv struct {
	Client *ynab.Client
	PlanID ynab.PlanID
}

// integrationCase declares one live case and the operationIds it covers.
type integrationCase struct {
	name string
	ops  []string
	run  func(t *testing.T, env integrationEnv)
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

func init() {
	registerIntegrationCase(integrationCase{
		name: "user and plans reads",
		ops:  []string{"getUser", "getPlans", "getPlanSettingsById"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			u, err := env.Client.User(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, u.ID)

			plans, err := env.Client.Plans(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, plans.Plans, "the test token must reach at least the test plan")
			for _, p := range plans.Plans {
				require.NotEmpty(t, p.ID)
				require.NotEmpty(t, p.Name)
			}

			settings, err := env.Client.Plan(env.PlanID).Settings(t.Context())
			require.NoError(t, err)
			require.NotNil(t, settings)
		},
	})
}
