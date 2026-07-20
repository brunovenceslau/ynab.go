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

func TestServerServesFixturesVerbatim(t *testing.T) {
	t.Parallel()

	srv := ynabtest.NewServer(t)

	status, raw := get(t, srv.URL+"/user")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, ynabtest.Fixture(t, "user/get.json"), raw,
		"the fake serves the exact golden fixture bytes")

	status, raw = get(t, srv.URL+"/plans/p-1/accounts")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, ynabtest.Fixture(t, "accounts/list.json"), raw)
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
