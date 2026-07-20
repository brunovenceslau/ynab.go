// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented("getAccounts", "createAccount", "getAccountById")

	registerReadCase(readCase{
		op:      "getAccounts",
		fixture: "accounts/list.json",
		model:   []ynab.Account{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			accounts, _, err := c.Plan("p-1").Accounts.List(t.Context())
			return accounts, err
		},
	})
	registerReadCase(readCase{
		op:      "getAccounts",
		variant: "null",
		fixture: "accounts/list_null.json",
		model:   []ynab.Account{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			accounts, _, err := c.Plan("p-1").Accounts.List(t.Context())
			return accounts, err
		},
	})
	registerReadCase(readCase{
		op:      "getAccountById",
		fixture: "accounts/get.json",
		model:   ynab.Account{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Accounts.Get(t.Context(), "ac111111-1111-1111-1111-111111111111")
		},
	})
	registerReadCase(readCase{
		op:      "getPlans",
		variant: "include_accounts",
		fixture: "plans/list_accounts.json",
		model:   ynab.PlanList{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plans(t.Context(), ynab.IncludeAccounts())
		},
	})

	registerNullFixture([]ynab.Account{}, "accounts/list_null.json", "accounts")

	registerWriteCase(writeCase{
		op:     "createAccount",
		method: http.MethodPost,
		path:   "/plans/p-1/accounts",
		body:   `{"account":{"name":"Vacation Fund","type":"savings","balance":0}}`,
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			spec := ynab.AccountSpec{Name: "Vacation Fund", Type: ynab.SaveAccountTypeSavings, Balance: 0}
			_, err := c.Plan("p-1").Accounts.Create(t.Context(), spec)
			require.NoError(t, err)
		},
	})
	registerWriteModel(ynab.AccountSpec{Name: "n", Type: ynab.SaveAccountTypeChecking, Balance: 0})

	registerIntegrationCase(integrationCase{
		name: "accounts read and create",
		ops:  []string{"getAccounts", "createAccount", "getAccountById"},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			accounts, sk, err := plan.Accounts.List(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			for _, a := range accounts {
				require.NotEmpty(t, a.ID)
				require.True(t, a.Type.Valid(), "unknown account type %q", a.Type)
			}

			// createAccount has no delete counterpart: the account stays on
			// the dedicated test plan, named uniquely to stay identifiable.
			name := fmt.Sprintf("itest-account-%d", time.Now().UnixNano())
			created, err := plan.Accounts.Create(t.Context(), ynab.AccountSpec{
				Name:    name,
				Type:    ynab.SaveAccountTypeChecking,
				Balance: 0,
			})
			require.NoError(t, err)
			require.Equal(t, name, created.Name)

			got, err := plan.Accounts.Get(t.Context(), created.ID)
			require.NoError(t, err)
			require.Equal(t, created.ID, got.ID)
		},
	})
}

func TestAccountsList(t *testing.T) {
	t.Parallel()

	t.Run("golden with loan maps", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "accounts/list.json", 0)
		accounts, sk, err := client.Plan("p-1").Accounts.List(t.Context())
		require.NoError(t, err)
		require.Equal(t, ynab.ServerKnowledge(1473), sk)
		require.Len(t, accounts, 2)
		require.Equal(t, "/plans/p-1/accounts", rec.URL.Path)

		checking := accounts[0]
		require.Equal(t, ynab.AccountTypeChecking, checking.Type)
		require.True(t, checking.Type.Valid())
		require.Equal(t, ynab.Milliunits(123930), checking.Balance)
		require.Equal(t, "primary", *checking.Note)
		require.Equal(t, "$123.93", checking.BalanceFormatted)
		require.InEpsilon(t, 123.93, checking.BalanceCurrency, 1e-9)

		mortgage := accounts[1]
		require.Equal(t, ynab.LoanAccountPeriodicValue{
			"2024-01-01": 3500,
			"2025-06-01": 4250,
		}, mortgage.DebtInterestRates)
		require.Nil(t, mortgage.Note)
		require.Nil(t, mortgage.LastReconciledAt)
	})

	t.Run("delta with tombstone", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "accounts/list_delta.json", 0)
		accounts, sk, err := client.Plan("p-1").Accounts.List(t.Context(), ynab.Since(1473))
		require.NoError(t, err)
		require.Equal(t, "1473", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(1600), sk)
		require.Len(t, accounts, 1)
		require.True(t, accounts[0].IsDeleted())

		// Tombstones drive MergeByID deletion.
		local := map[string]ynab.Account{"ac222222-2222-2222-2222-222222222222": {}}
		merged := ynab.MergeByID(local, accounts)
		require.Empty(t, merged)
	})

	t.Run("unknown enum decodes losslessly", func(t *testing.T) {
		t.Parallel()

		body := loadFixture(t, "accounts/get.json")
		mutated := []byte(string(body))
		mutated = json.RawMessage(replaceOnce(t, mutated, `"checking"`, `"quantumVault"`))
		srv := fixtureServer(t, 0, mutated, false)
		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

		got, err := client.Plan("p-1").Accounts.Get(t.Context(), "ac1")
		require.NoError(t, err, "a new server enum value must never fail a decode")
		require.Equal(t, ynab.AccountType("quantumVault"), got.Type)
		require.False(t, got.Type.Valid())
	})
}

func TestAccountsCreate(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "accounts/create.json", http.StatusCreated)
	spec := ynab.AccountSpec{Name: "Vacation Fund", Type: ynab.SaveAccountTypeSavings, Balance: 0}
	created, err := client.Plan("p-1").Accounts.Create(t.Context(), spec)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, rec.Method)
	require.Equal(t, "ac444444-4444-4444-4444-444444444444", created.ID)
	require.Equal(t, "Vacation Fund", created.Name)
}

func TestAccountsGet(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "accounts/get.json", 0)
	got, err := client.Plan("p-1").Accounts.Get(t.Context(), "ac111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.Equal(t, "/plans/p-1/accounts/ac111111-1111-1111-1111-111111111111", rec.URL.Path)
	require.Equal(t, "Checking", got.Name)
}

func TestAccountsExtremeNumerics(t *testing.T) {
	t.Parallel()

	runOutOfRangeCase(t, ynab.Account{}, "accounts/extreme.json", "account")
}

func TestPlansIncludeAccounts(t *testing.T) {
	t.Parallel()

	// The accounts key appears only with include_accounts (fixture pair).
	client, _ := serveFixture(t, "plans/list_accounts.json", 0)
	with, err := client.Plans(t.Context(), ynab.IncludeAccounts())
	require.NoError(t, err)
	require.Len(t, with.Plans[0].Accounts, 1)
	require.Equal(t, "Checking", with.Plans[0].Accounts[0].Name)

	client2, _ := serveFixture(t, "plans/list.json", 0)
	without, err := client2.Plans(t.Context())
	require.NoError(t, err)
	require.Nil(t, without.Plans[0].Accounts)
}

// replaceOnce is a small fixture-mutation helper.
func replaceOnce(t *testing.T, b []byte, old, newV string) []byte {
	t.Helper()
	s := string(b)
	require.Contains(t, s, old)
	return []byte(strings.Replace(s, old, newV, 1))
}
