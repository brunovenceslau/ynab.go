// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func init() {
	registerReadCase(readCase{
		op:      "getUser",
		fixture: "user/get.json",
		model:   ynab.User{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.User(t.Context())
		},
	})
	registerReadCase(readCase{
		op:      "getPlans",
		fixture: "plans/list.json",
		model:   ynab.PlanList{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plans(t.Context())
		},
	})
	registerReadCase(readCase{
		op:      "getPlans",
		variant: "null",
		fixture: "plans/list_null.json",
		model:   ynab.PlanList{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plans(t.Context())
		},
	})
	registerReadCase(readCase{
		op:      "getPlanSettingsById",
		fixture: "plans/settings.json",
		model:   ynab.PlanSettings{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("aa111111-1111-1111-1111-111111111111").Settings(t.Context())
		},
	})
	registerReadCase(readCase{
		op:      "getPlanSettingsById",
		variant: "null",
		fixture: "plans/settings_null.json",
		model:   ynab.PlanSettings{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("aa111111-1111-1111-1111-111111111111").Settings(t.Context())
		},
	})

	registerNullFixture(ynab.PlanList{}, "plans/list_null.json", "")
	registerNullFixture(ynab.PlanSettings{}, "plans/settings_null.json", "settings")
}

// serveFixture builds a client whose base URL serves the fixture and
// records the request.
func serveFixture(t *testing.T, fixture string, status int) (*ynab.Client, *http.Request) {
	t.Helper()

	body := loadFixture(t, fixture)
	rec := &http.Request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*rec = *r.Clone(r.Context())
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled()), rec
}

func TestUser(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "user/get.json", 0)
	u, err := client.User(t.Context())
	require.NoError(t, err)
	require.Equal(t, &ynab.User{ID: "11111111-2222-3333-4444-555555555555"}, u)
	require.Equal(t, "/user", rec.URL.Path)
	require.Equal(t, http.MethodGet, rec.Method)
}

func TestUserConfigErrorBeforeIO(t *testing.T) {
	t.Parallel()

	// Re-proves the construction contract through a real method: the stored option
	// failure surfaces before any I/O.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request must be sent on a config error")
	}))
	t.Cleanup(srv.Close)

	client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithTimeout(-1))
	_, err := client.User(t.Context())

	var argErr *ynab.ArgumentError
	require.ErrorAs(t, err, &argErr)
	require.Equal(t, "WithTimeout", argErr.Field)
}

func TestPlans(t *testing.T) {
	t.Parallel()

	t.Run("golden list", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "plans/list.json", 0)
		got, err := client.Plans(t.Context())
		require.NoError(t, err)

		require.Len(t, got.Plans, 2)
		first := got.Plans[0]
		require.Equal(t, "Family Plan", first.Name)
		require.Equal(t, ynab.NewMonth(2024, time.January), first.FirstMonth)
		require.Equal(t, ynab.NewMonth(2026, time.August), first.LastMonth)
		require.Equal(t, "MM/DD/YYYY", first.DateFormat.Format)
		require.Equal(t, "USD", first.CurrencyFormat.ISOCode)
		require.Equal(t, 2026, first.LastModifiedOn.Year())

		require.NotNil(t, got.DefaultPlan)
		require.Equal(t, first.ID, got.DefaultPlan.ID)

		require.Equal(t, "/plans", rec.URL.Path)
		require.Empty(t, rec.URL.RawQuery, "no options, no parameters")
	})

	t.Run("null default plan and formats", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "plans/list_null.json", 0)
		got, err := client.Plans(t.Context())
		require.NoError(t, err)

		require.Nil(t, got.DefaultPlan)
		require.Len(t, got.Plans, 1)
		require.Nil(t, got.Plans[0].DateFormat)
		require.Nil(t, got.Plans[0].CurrencyFormat)
	})

	t.Run("include accounts option encodes", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "plans/list.json", 0)
		_, err := client.Plans(t.Context(), ynab.IncludeAccounts())
		require.NoError(t, err)
		require.Equal(t, "true", rec.URL.Query().Get("include_accounts"))
	})
}

func TestPlanSettings(t *testing.T) {
	t.Parallel()

	t.Run("golden", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "plans/settings.json", 0)
		got, err := client.Plan(ynab.PlanIDLastUsed).Settings(t.Context())
		require.NoError(t, err)
		require.Equal(t, "DD/MM/YYYY", got.DateFormat.Format)
		require.Equal(t, "EUR", got.CurrencyFormat.ISOCode)
		require.Equal(t, "/plans/last-used/settings", rec.URL.Path)
	})

	t.Run("null formats", func(t *testing.T) {
		t.Parallel()

		client, _ := serveFixture(t, "plans/settings_null.json", 0)
		got, err := client.Plan("p-1").Settings(t.Context())
		require.NoError(t, err)
		require.Nil(t, got.DateFormat)
		require.Nil(t, got.CurrencyFormat)
	})

	t.Run("api error surfaces the taxonomy", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"id":"404.2","name":"resource_not_found","detail":"Plan not found"}}`))
		}))
		t.Cleanup(srv.Close)

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		_, err := client.Plan("nope").Settings(t.Context())
		require.ErrorIs(t, err, ynab.ErrResourceNotFound)
		require.ErrorIs(t, err, ynab.ErrNotFound)
	})
}

func TestPlanHandle(t *testing.T) {
	t.Parallel()

	client := ynab.New("t")
	p := client.Plan(ynab.PlanIDDefault)
	require.Equal(t, ynab.PlanIDDefault, p.ID())

	// Plan IDs with hostile characters cannot traverse into other routes:
	// the wire carries %2E%2E (a raw ".." would have been normalized to
	// /user/settings by the HTTP client before even reaching the server).
	client2, rec := serveFixture(t, "plans/settings.json", 0)
	_, err := client2.Plan("../user").Settings(t.Context())
	require.NoError(t, err)
	require.Equal(t, "/plans/..%2Fuser/settings", rec.URL.EscapedPath())
}
