// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynabtest_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/ynabtest"
)

func get(t *testing.T, url string) (int, []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

// TestServerServesFixturesVerbatim pins the fake's complete
// route→fixture mapping: every route, byte-exact, with its status. The
// table is DELIBERATELY a hand-written literal — never derive it from
// contract.Table() or the fake's own registry, because it is the
// anti-circularity witness against exactly those derived routes: a
// derived table would only prove dispatch, not the mapping.
func TestServerServesFixturesVerbatim(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)

	const delta = "?last_knowledge_of_server=1"
	rows := []struct {
		method  string
		url     string // concrete path (plus cursor query for delta rows)
		body    string // request body, for the body-keyed POST pair
		fixture string
		status  int
	}{
		{method: "GET", url: "/user", fixture: "user/get.json", status: 200},
		{method: "GET", url: "/plans", fixture: "plans/list.json", status: 200},
		{method: "GET", url: "/plans/p-1", fixture: "plans/export.json", status: 200},
		{method: "GET", url: "/plans/p-1" + delta, fixture: "plans/export_delta.json", status: 200},
		{method: "GET", url: "/plans/p-1/settings", fixture: "plans/settings.json", status: 200},
		{method: "GET", url: "/plans/p-1/months", fixture: "months/list.json", status: 200},
		{method: "GET", url: "/plans/p-1/months/2026-07-01", fixture: "months/get.json", status: 200},
		{method: "GET", url: "/plans/p-1/accounts", fixture: "accounts/list.json", status: 200},
		{method: "GET", url: "/plans/p-1/accounts" + delta, fixture: "accounts/list_delta.json", status: 200},
		{method: "POST", url: "/plans/p-1/accounts", fixture: "accounts/create.json", status: 201},
		{method: "GET", url: "/plans/p-1/accounts/ac-1", fixture: "accounts/get.json", status: 200},
		{method: "GET", url: "/plans/p-1/categories", fixture: "categories/list.json", status: 200},
		{method: "GET", url: "/plans/p-1/categories" + delta, fixture: "categories/list_delta.json", status: 200},
		{method: "POST", url: "/plans/p-1/categories", fixture: "categories/create.json", status: 201},
		{method: "GET", url: "/plans/p-1/categories/ca-1", fixture: "categories/get.json", status: 200},
		{method: "PATCH", url: "/plans/p-1/categories/ca-1", fixture: "categories/update.json", status: 200},
		{
			method: "GET", url: "/plans/p-1/months/2026-07-01/categories/ca-1",
			fixture: "categories/get_for_month.json", status: 200,
		},
		{
			method: "PATCH", url: "/plans/p-1/months/2026-07-01/categories/ca-1",
			fixture: "categories/assign.json", status: 200,
		},
		{method: "POST", url: "/plans/p-1/category_groups", fixture: "categories/group_create.json", status: 201},
		{method: "PATCH", url: "/plans/p-1/category_groups/cg-1", fixture: "categories/group_rename.json", status: 200},
		{method: "GET", url: "/plans/p-1/payees", fixture: "payees/list.json", status: 200},
		{method: "GET", url: "/plans/p-1/payees" + delta, fixture: "payees/list_delta.json", status: 200},
		{method: "POST", url: "/plans/p-1/payees", fixture: "payees/create.json", status: 201},
		{method: "GET", url: "/plans/p-1/payees/pa-1", fixture: "payees/get.json", status: 200},
		{method: "PATCH", url: "/plans/p-1/payees/pa-1", fixture: "payees/rename.json", status: 200},
		{method: "GET", url: "/plans/p-1/payee_locations", fixture: "payee_locations/list.json", status: 200},
		{method: "GET", url: "/plans/p-1/payee_locations/pl-1", fixture: "payee_locations/get.json", status: 200},
		{
			method: "GET", url: "/plans/p-1/payees/pa-1/payee_locations",
			fixture: "payee_locations/by_payee.json", status: 200,
		},
		{method: "GET", url: "/plans/p-1/money_movements", fixture: "money_movements/list.json", status: 200},
		{
			method: "GET", url: "/plans/p-1/months/2026-07-01/money_movements",
			fixture: "money_movements/by_month.json", status: 200,
		},
		{method: "GET", url: "/plans/p-1/money_movement_groups", fixture: "money_movements/groups.json", status: 200},
		{
			method: "GET", url: "/plans/p-1/months/2026-07-01/money_movement_groups",
			fixture: "money_movements/groups_by_month.json", status: 200,
		},
		{method: "GET", url: "/plans/p-1/transactions", fixture: "transactions/list.json", status: 200},
		{
			method: "POST", url: "/plans/p-1/transactions", body: `{"transaction":{"amount":1}}`,
			fixture: "transactions/create.json", status: 201,
		},
		{
			method: "POST", url: "/plans/p-1/transactions", body: `{"transactions":[{"amount":1}]}`,
			fixture: "transactions/batch.json", status: 201,
		},
		{method: "PATCH", url: "/plans/p-1/transactions", fixture: "transactions/batch.json", status: 200},
		{method: "POST", url: "/plans/p-1/transactions/import", fixture: "transactions/import_ids.json", status: 201},
		{method: "GET", url: "/plans/p-1/transactions/tr-1", fixture: "transactions/get.json", status: 200},
		{method: "PUT", url: "/plans/p-1/transactions/tr-1", fixture: "transactions/update.json", status: 200},
		{method: "DELETE", url: "/plans/p-1/transactions/tr-1", fixture: "transactions/delete.json", status: 200},
		{method: "GET", url: "/plans/p-1/accounts/ac-1/transactions", fixture: "transactions/list.json", status: 200},
		{
			method: "GET", url: "/plans/p-1/categories/ca-1/transactions",
			fixture: "transactions/hybrid.json", status: 200,
		},
		{method: "GET", url: "/plans/p-1/payees/pa-1/transactions", fixture: "transactions/hybrid.json", status: 200},
		{
			method: "GET", url: "/plans/p-1/months/2026-07-01/transactions",
			fixture: "transactions/list.json", status: 200,
		},
		{method: "GET", url: "/plans/p-1/scheduled_transactions", fixture: "scheduled/list.json", status: 200},
		{method: "POST", url: "/plans/p-1/scheduled_transactions", fixture: "scheduled/create.json", status: 201},
		{method: "GET", url: "/plans/p-1/scheduled_transactions/sc-1", fixture: "scheduled/get.json", status: 200},
		{method: "PUT", url: "/plans/p-1/scheduled_transactions/sc-1", fixture: "scheduled/update.json", status: 200},
		{method: "DELETE", url: "/plans/p-1/scheduled_transactions/sc-1", fixture: "scheduled/delete.json", status: 200},
	}

	for _, row := range rows {
		t.Run(row.method+" "+row.url, func(t *testing.T) {
			t.Parallel()

			var reqBody io.Reader
			if row.body != "" {
				reqBody = strings.NewReader(row.body)
			}
			req, err := http.NewRequestWithContext(t.Context(), row.method, srv.URL+row.url, reqBody)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			t.Cleanup(func() { _ = resp.Body.Close() })
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, row.status, resp.StatusCode)
			require.Equal(t, ynabtest.Fixture(t, row.fixture), raw,
				"the fake serves the exact golden fixture bytes")
		})
	}
}

func TestServerDeltaAwareness(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)

	_, full := get(t, srv.URL+"/plans/p-1/accounts")
	_, delta := get(t, srv.URL+"/plans/p-1/accounts?last_knowledge_of_server=1473")
	require.NotEqual(t, full, delta, "a cursor switches to the delta fixture")
	require.Equal(t, ynabtest.Fixture(t, "accounts/list_delta.json"), delta)

	var doc struct {
		Data struct {
			Accounts []struct {
				Deleted bool `json:"deleted"`
			} `json:"accounts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(delta, &doc))
	require.True(t, doc.Data.Accounts[0].Deleted, "the delta fixture carries tombstones")
}

func TestServerFailWith(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	srv.FailWith(http.StatusTooManyRequests, "429", "too_many_requests")

	status, raw := get(t, srv.URL+"/user")
	require.Equal(t, http.StatusTooManyRequests, status)

	var env struct {
		Error struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Detail string `json:"detail"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Equal(t, "429", env.Error.ID)
	require.Equal(t, "too_many_requests", env.Error.Name)

	// The injection is one-shot: the next request succeeds again.
	status, _ = get(t, srv.URL+"/user")
	require.Equal(t, http.StatusOK, status)
}

func TestServerUnknownRouteIs404(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	status, raw := get(t, srv.URL+"/plans/p-1/unicorns")
	require.Equal(t, http.StatusNotFound, status)
	require.Contains(t, string(raw), `"404.1"`)
}

func TestServerBodyKeyDisambiguation(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)
	post := func(body string) []byte {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
			srv.URL+"/plans/p-1/transactions", strings.NewReader(body))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return raw
	}

	single := post(`{"transaction":{"amount":1}}`)
	require.Equal(t, ynabtest.Fixture(t, "transactions/create.json"), single)

	batch := post(`{"transactions":[{"amount":1}]}`)
	require.Equal(t, ynabtest.Fixture(t, "transactions/batch.json"), batch)

	// Neither wrapper key: the fake has no route for it — a loud 404
	// instead of silently picking a shape.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/plans/p-1/transactions", strings.NewReader(`{}`))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestFixtureNamesAllReadable(t *testing.T) {
	t.Parallel()

	names := ynabtest.FixtureNames()
	require.NotEmpty(t, names)
	for _, name := range names {
		require.NotEmpty(t, ynabtest.Fixture(t, name), name)
	}
}
