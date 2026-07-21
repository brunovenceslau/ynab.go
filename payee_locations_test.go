// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func init() {
	registerReadCase(readCase{
		op:      "getPayeeLocations",
		fixture: "payee_locations/list.json",
		model:   []ynab.PayeeLocation{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").PayeeLocations.List(t.Context())
		},
	})
	registerReadCase(readCase{
		op:      "getPayeeLocationById",
		fixture: "payee_locations/get.json",
		model:   ynab.PayeeLocation{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").PayeeLocations.Get(t.Context(), "pl111111-1111-1111-1111-111111111111")
		},
	})
	registerReadCase(readCase{
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
		// getPayees: listed unconditionally (hoisted above the branch) to
		// drive the by-payee call against a real id on the empty arm.
		ops: []string{"getPayeeLocations", "getPayeeLocationById", "getPayeeLocationsByPayee", "getPayees"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			locations, err := plan.PayeeLocations.List(t.Context())
			require.NoError(t, err)

			// Hoisted above the branch so getPayees is sent on both arms:
			// the runner requires every declared op on the wire, and this
			// call must not flap if the plan ever gains a location via the
			// mobile app.
			payees, _, err := plan.Payees.List(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, payees, "every plan has default payees")

			// The API cannot create locations (mobile-app writes only), so
			// a test plan is legitimately empty. Every op is still
			// exercised over the real wire either way.
			if len(locations) == 0 {
				// By-payee against a real payee: a 200 with an empty list.
				byPayee, err := plan.PayeeLocations.ListByPayee(t.Context(), payees[0].ID)
				require.NoError(t, err)
				require.Empty(t, byPayee)

				// By-id against a well-formed unknown id: the suite's single
				// 404 probe — the real 404.2 payload must map through BOTH
				// the class and the specific sentinel. The sentinels alone
				// would still pass via the status-class fallback if the
				// sub-code drifted, so the payload itself is pinned too:
				// the only live error payload the suite ever sees.
				_, err = plan.PayeeLocations.Get(t.Context(), "00000000-0000-0000-0000-000000000000")
				require.ErrorIs(t, err, ynab.ErrNotFound)
				require.ErrorIs(t, err, ynab.ErrResourceNotFound)
				var apiErr *ynab.Error
				require.ErrorAs(t, err, &apiErr)
				require.Equal(t, http.StatusNotFound, apiErr.StatusCode)
				require.Equal(t, "404.2", apiErr.ID, "the ID-keyed sentinel table depends on this exact sub-code")
				require.Equal(t, "resource_not_found", apiErr.Name)
				require.NotEmpty(t, apiErr.Detail)
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
		require.Equal(t, goldenPayeeLocations(), locations)
	})

	t.Run("get by id", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payee_locations/get.json", 0)
		got, err := client.Plan("p-1").PayeeLocations.Get(t.Context(), "pl111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payee_locations/pl111111-1111-1111-1111-111111111111", rec.URL.Path)
		require.Equal(t, ptr(goldenPayeeLocations()[0]), got)
	})

	t.Run("list by payee", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payee_locations/by_payee.json", 0)
		locations, err := client.Plan("p-1").PayeeLocations.ListByPayee(
			t.Context(), "pa111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payees/pa111111-1111-1111-1111-111111111111/payee_locations", rec.URL.Path)
		require.Equal(t, goldenPayeeLocations()[:1], locations)
	})
}
