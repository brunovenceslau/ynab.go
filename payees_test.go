// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func init() {
	registerReadCase(readCase{
		op:      "getPayees",
		fixture: "payees/list.json",
		model:   []ynab.Payee{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			payees, _, err := c.Plan("p-1").Payees.List(t.Context())
			return payees, err
		},
	})
	registerReadCase(readCase{
		op:      "getPayees",
		variant: "null",
		fixture: "payees/list_null.json",
		model:   []ynab.Payee{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			payees, _, err := c.Plan("p-1").Payees.List(t.Context())
			return payees, err
		},
	})
	registerReadCase(readCase{
		op:      "getPayeeById",
		fixture: "payees/get.json",
		model:   ynab.Payee{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Payees.Get(t.Context(), "pa111111-1111-1111-1111-111111111111")
		},
	})

	registerNullFixture([]ynab.Payee{}, "payees/list_null.json", "payees")

	registerWriteCase(writeCase{
		op:     "createPayee",
		method: http.MethodPost,
		path:   "/plans/p-1/payees",
		body:   `{"payee":{"name":"New Landlord"}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, _, err := c.Plan("p-1").Payees.Create(t.Context(), "New Landlord")
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "updatePayee",
		method: http.MethodPatch,
		path:   "/plans/p-1/payees/pa1",
		body:   `{"payee":{"name":"Grocery Palace"}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, _, err := c.Plan("p-1").Payees.Rename(t.Context(), "pa1", "Grocery Palace")
			require.NoError(t, err)
		},
	})

	registerIntegrationCase(integrationCase{
		name: "payees reads and writes",
		ops:  []string{"getPayees", "createPayee", "getPayeeById", "updatePayee"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			payees, sk, err := plan.Payees.List(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			require.NotEmpty(t, payees)

			// Payees cannot be deleted through the API: the artifact stays
			// on the dedicated test plan, uniquely named.
			name := fmt.Sprintf("itest-payee-%d", time.Now().UnixNano())
			created, _, err := plan.Payees.Create(t.Context(), name)
			require.NoError(t, err)
			require.Equal(t, name, created.Name)

			got, err := plan.Payees.Get(t.Context(), created.ID)
			require.NoError(t, err)
			require.Equal(t, created.ID, got.ID)

			renamed, _, err := plan.Payees.Rename(t.Context(), created.ID, name+"-renamed")
			require.NoError(t, err)
			require.Equal(t, name+"-renamed", renamed.Name)
		},
	})
}

func TestPayeesList(t *testing.T) {
	t.Parallel()

	t.Run("golden with transfer payee", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payees/list.json", 0)
		payees, sk, err := client.Plan("p-1").Payees.List(t.Context())
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/payees", rec.URL.Path)
		require.Equal(t, ynab.ServerKnowledge(4000), sk)
		require.Equal(t, goldenPayees(), payees)
	})

	t.Run("delta with tombstone", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "payees/list_delta.json", 0)
		payees, sk, err := client.Plan("p-1").Payees.List(t.Context(), ynab.Since(4000))
		require.NoError(t, err)
		require.Equal(t, "4000", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(4100), sk)
		require.Len(t, payees, 1)
		require.True(t, payees[0].IsDeleted())

		local := map[string]ynab.Payee{"pa111111-1111-1111-1111-111111111111": {}}
		require.Empty(t, ynab.MergeByID(local, payees))
	})
}

func TestPayeesGet(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "payees/get.json", 0)
	got, err := client.Plan("p-1").Payees.Get(t.Context(), "pa111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.Equal(t, "/plans/p-1/payees/pa111111-1111-1111-1111-111111111111", rec.URL.Path)
	require.Equal(t, ptr(goldenPayees()[0]), got)
}

func TestPayeesCreate(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "payees/create.json", http.StatusCreated)
	created, sk, err := client.Plan("p-1").Payees.Create(t.Context(), "New Landlord")
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, rec.Method)
	require.Equal(t, "New Landlord", created.Name)
	require.Equal(t, ynab.ServerKnowledge(4200), sk)
}

func TestPayeesRename(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "payees/rename.json", 0)
	renamed, _, err := client.Plan("p-1").Payees.Rename(t.Context(), "pa1", "Grocery Palace")
	require.NoError(t, err)
	require.Equal(t, http.MethodPatch, rec.Method)
	require.Equal(t, "Grocery Palace", renamed.Name)
}

func TestPayeesNameBound(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "payees/create.json", 0)
	long := strings.Repeat("x", 501)

	_, _, err := client.Plan("p-1").Payees.Create(t.Context(), long)
	var argErr *ynab.ArgumentError
	require.ErrorAs(t, err, &argErr)
	require.Equal(t, "name", argErr.Field)

	_, _, err = client.Plan("p-1").Payees.Rename(t.Context(), "pa1", long)
	require.ErrorAs(t, err, &argErr)
	require.Empty(t, rec.Method, "no request must be sent for either call")
}
