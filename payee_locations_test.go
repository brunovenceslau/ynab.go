package ynab_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented("getPayeeLocations", "getPayeeLocationById", "getPayeeLocationsByPayee")

	registerEndpointCase(endpointCase{
		op:      "getPayeeLocations",
		fixture: "payee_locations/list.json",
		model:   []ynab.PayeeLocation{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").PayeeLocations.List(t.Context())
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getPayeeLocationById",
		fixture: "payee_locations/get.json",
		model:   ynab.PayeeLocation{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").PayeeLocations.Get(t.Context(), "pl111111-1111-1111-1111-111111111111")
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getPayeeLocationsByPayee",
		fixture: "payee_locations/by_payee.json",
		model:   []ynab.PayeeLocation{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").PayeeLocations.ListByPayee(t.Context(), "pa111111-1111-1111-1111-111111111111")
		},
	})

	registerIntegrationCase(integrationCase{
		name: "payee locations reads",
		ops:  []string{"getPayeeLocations", "getPayeeLocationById", "getPayeeLocationsByPayee"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			locations, err := plan.PayeeLocations.List(t.Context())
			require.NoError(t, err)

			// A test plan usually has no locations (mobile-app writes only);
			// the list decode itself is the assertion. When one exists,
			// exercise the by-id and by-payee paths too.
			if len(locations) == 0 {
				t.Log("no payee locations on the test plan; list decode covered getPayeeLocations")
				return
			}
			got, err := plan.PayeeLocations.Get(t.Context(), locations[0].ID)
			require.NoError(t, err)
			require.Equal(t, locations[0].ID, got.ID)

			byPayee, err := plan.PayeeLocations.ListByPayee(t.Context(), locations[0].PayeeID)
			require.NoError(t, err)
			require.NotEmpty(t, byPayee)
		},
	})
}

func TestPayeeLocations(t *testing.T) {
	t.Parallel()

	t.Run("list keeps coordinates as strings", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payee_locations/list.json", 0)
		locations, err := client.Plan("p-1").PayeeLocations.List(t.Context())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payee_locations", rec.URL.Path)
		require.Empty(t, rec.URL.RawQuery, "no delta cursor exists for payee locations")
		require.Len(t, locations, 2)
		require.Equal(t, "-23.5505199", locations[0].Latitude)
		require.Equal(t, "-46.6333094", locations[0].Longitude)
	})

	t.Run("get by id", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payee_locations/get.json", 0)
		got, err := client.Plan("p-1").PayeeLocations.Get(t.Context(), "pl111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payee_locations/pl111111-1111-1111-1111-111111111111", rec.URL.Path)
		require.Equal(t, "pa111111-1111-1111-1111-111111111111", got.PayeeID)
	})

	t.Run("list by payee", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payee_locations/by_payee.json", 0)
		locations, err := client.Plan("p-1").PayeeLocations.ListByPayee(
			t.Context(), "pa111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payees/pa111111-1111-1111-1111-111111111111/payee_locations", rec.URL.Path)
		require.Len(t, locations, 1)
	})
}
