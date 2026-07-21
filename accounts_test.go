// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func init() {
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
			spec := ynab.AccountSpec{Name: "Vacation Fund", Type: ynab.AccountSpecTypeSavings, Balance: 0}
			_, err := c.Plan("p-1").Accounts.Create(t.Context(), spec)
			require.NoError(t, err)
		},
	})
	registerWriteModel(ynab.AccountSpec{Name: "n", Type: ynab.AccountSpecTypeChecking, Balance: 0})

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
			// Type savings: the byte-exact G2 case already pins the savings
			// encoding client-side, so this create proves the server-side
			// half — acceptance and echo of a second enum member — at the
			// same accumulation cost (still one account per run).
			name := fmt.Sprintf("itest-account-%d", time.Now().UnixNano())
			created, err := plan.Accounts.Create(t.Context(), ynab.AccountSpec{
				Name:    name,
				Type:    ynab.AccountSpecTypeSavings,
				Balance: 0,
			})
			require.NoError(t, err)
			require.Equal(t, name, created.Name)
			require.Equal(t, ynab.AccountTypeSavings, created.Type, "sent type must round-trip")
			require.Zero(t, created.Balance)
			require.False(t, created.Closed)
			require.True(t, created.OnBudget, "a savings account is a budget account")
			require.NotNil(t, created.TransferPayeeID, "the server must mint a transfer payee")
			require.False(t, created.Deleted)

			// Read-back persistence — the server stored what it echoed.
			got, err := plan.Accounts.Get(t.Context(), created.ID)
			require.NoError(t, err)
			require.Equal(t, created.ID, got.ID)
			require.Equal(t, name, got.Name, "the created name must persist to a read-back")
			require.Equal(t, created.Type, got.Type)
			require.Equal(t, created.Balance, got.Balance)
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
		require.Equal(t, "/plans/p-1/accounts", rec.URL.Path)

		// The complete expected value, transcribed from the fixture: every
		// field is load-bearing, so a swapped json tag cannot survive.
		want := []ynab.Account{
			{
				AccountBase:               goldenCheckingAccountBase(),
				BalanceFormatted:          "$123.93",
				BalanceCurrency:           123.93,
				ClearedBalanceFormatted:   "$100.00",
				ClearedBalanceCurrency:    100,
				UnclearedBalanceFormatted: "$23.93",
				UnclearedBalanceCurrency:  23.93,
			},
			{
				AccountBase: ynab.AccountBase{
					ID:               "ac222222-2222-2222-2222-222222222222",
					Name:             "Mortgage",
					Type:             ynab.AccountTypeMortgage,
					OnBudget:         false,
					Closed:           false,
					Balance:          -395032000,
					ClearedBalance:   -395032000,
					UnclearedBalance: 0,
					TransferPayeeID:  ptr("pa222222-2222-2222-2222-222222222222"),
					DebtInterestRates: ynab.LoanAccountPeriodicValue{
						"2024-01-01": 3500,
						"2025-06-01": 4250,
					},
					DebtMinimumPayments: ynab.LoanAccountPeriodicValue{"2024-01-01": 1500000},
					DebtEscrowAmounts:   ynab.LoanAccountPeriodicValue{"2024-01-01": 250000},
					Deleted:             false,
				},
				BalanceFormatted:          "-$395,032.00",
				BalanceCurrency:           -395032,
				ClearedBalanceFormatted:   "-$395,032.00",
				ClearedBalanceCurrency:    -395032,
				UnclearedBalanceFormatted: "$0.00",
				UnclearedBalanceCurrency:  0,
			},
		}
		require.Equal(t, want, accounts)
		require.True(t, accounts[0].Type.Valid())
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
	spec := ynab.AccountSpec{Name: "Vacation Fund", Type: ynab.AccountSpecTypeSavings, Balance: 0}
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

	runExtremeNumericsCase(t, ynab.Account{}, "accounts/extreme.json", "account")
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
