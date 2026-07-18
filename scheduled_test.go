package ynab_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

func init() {
	contract.MarkImplemented(
		"getScheduledTransactions", "createScheduledTransaction",
		"getScheduledTransactionById", "updateScheduledTransaction",
		"deleteScheduledTransaction",
	)

	registerEndpointCase(endpointCase{
		op:      "getScheduledTransactions",
		fixture: "scheduled/list.json",
		model:   []ynab.ScheduledTransaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			scheduled, _, err := c.Plan("p-1").Scheduled.List(t.Context())
			return scheduled, err
		},
	})
	registerEndpointCase(endpointCase{
		op:      "getScheduledTransactionById",
		fixture: "scheduled/get.json",
		model:   ynab.ScheduledTransaction{},
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return c.Plan("p-1").Scheduled.Get(t.Context(), "sc111111-1111-1111-1111-111111111111")
		},
	})

	registerNullFixture([]ynab.ScheduledTransaction{}, "scheduled/list.json", "scheduled_transactions")

	// The scheduled date window is validated against the clock, so the
	// byte-exact G2 bodies carry a date computed once at registration.
	schedDate := ynab.Today().AddDays(30)
	registerWriteCase(writeCase{
		op:     "createScheduledTransaction",
		method: http.MethodPost,
		path:   "/plans/p-1/scheduled_transactions",
		body: fmt.Sprintf(`{"scheduled_transaction":{
			"account_id":"ac1","date":%q,"amount":-1500000,
			"frequency":"monthly","payee_name":"Landlord","memo":""
		}}`, schedDate),
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			spec := ynab.ScheduledTransactionSpec{
				AccountID: "ac1",
				Date:      schedDate,
				Amount:    -1500000,
				Frequency: ynab.FrequencyMonthly,
				PayeeName: ynab.Set("Landlord"),
				Memo:      ynab.Set(""), // Set(zero) must survive the wire
			}
			_, err := c.Plan("p-1").Scheduled.Create(t.Context(), spec)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "updateScheduledTransaction",
		method: http.MethodPut,
		path:   "/plans/p-1/scheduled_transactions/sc1",
		body: fmt.Sprintf(`{"scheduled_transaction":{
			"account_id":"ac1","date":%q,"amount":-1600000,"frequency":"monthly"
		}}`, schedDate),
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			spec := ynab.ScheduledTransactionSpec{
				AccountID: "ac1",
				Date:      schedDate,
				Amount:    -1600000,
				Frequency: ynab.FrequencyMonthly,
			}
			_, err := c.Plan("p-1").Scheduled.Update(t.Context(), "sc1", spec)
			require.NoError(t, err)
		},
	})
	registerWriteCase(writeCase{
		op:     "deleteScheduledTransaction",
		method: http.MethodDelete,
		path:   "/plans/p-1/scheduled_transactions/sc1",
		call: func(t *testing.T, c *ynab.Client) {
			t.Helper()
			_, err := c.Plan("p-1").Scheduled.Delete(t.Context(), "sc1")
			require.NoError(t, err)
		},
	})

	registerWriteModel(ynab.ScheduledTransactionSpec{
		AccountID: "ac1",
		Date:      ynab.Today().AddDays(30),
		Amount:    -1,
		Frequency: ynab.FrequencyMonthly,
		PayeeID:   ynab.Set("p"),
		PayeeName: ynab.SetNull[string](),
		Memo:      ynab.Set(""),
		FlagColor: ynab.Set(ynab.FlagColorNone),
	})
}

func init() {
	registerIntegrationCase(integrationCase{
		name: "scheduled transactions lifecycle",
		ops: []string{
			"getScheduledTransactions", "createScheduledTransaction",
			"getScheduledTransactionById", "updateScheduledTransaction",
			"deleteScheduledTransaction",
		},
		run: func(t *testing.T, env integrationEnv) {
			t.Helper()

			plan := env.Client.Plan(env.PlanID)
			initial, _, err := plan.Scheduled.List(t.Context())
			require.NoError(t, err, "empty-plan 404 must fold into an empty list")
			require.NotNil(t, initial, "the fold must answer an empty slice, never nil")

			accounts, _, err := plan.Accounts.List(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, accounts)

			memo := fmt.Sprintf("itest-sched-%d", time.Now().UnixNano())
			created, err := plan.Scheduled.Create(t.Context(), ynab.ScheduledTransactionSpec{
				AccountID: accounts[0].ID,
				Date:      ynab.Today().AddDays(30),
				Amount:    -1000,
				Frequency: ynab.FrequencyMonthly,
				PayeeName: ynab.Set("itest payee"),
				Memo:      ynab.Set(memo),
				FlagColor: ynab.Set(ynab.FlagColorRed),
			})
			require.NoError(t, err)
			cleanupCtx := context.WithoutCancel(t.Context())
			t.Cleanup(func() {
				if _, err := plan.Scheduled.Delete(cleanupCtx, created.ID); err != nil {
					require.ErrorIs(t, err, ynab.ErrResourceNotFound,
						"cleanup tolerates only the body's own delete")
				}
			})
			require.Equal(t, ynab.FrequencyMonthly, created.Frequency)
			require.True(t, created.Frequency.Valid(), "unknown frequency %q", created.Frequency)
			require.NotNil(t, created.FlagColor)
			require.Equal(t, ynab.FlagColorRed, *created.FlagColor, "Set(flag) must round-trip the live wire")
			require.NotNil(t, created.Memo)
			require.Equal(t, memo, *created.Memo)

			// A sentinel row keeps the plan non-empty during the post-delete
			// delta read below: without it the empty-plan 404 fold could
			// swallow the tombstone. LIFO cleanup deletes it first.
			sentinel, err := plan.Scheduled.Create(t.Context(), ynab.ScheduledTransactionSpec{
				AccountID: accounts[0].ID,
				Date:      ynab.Today().AddDays(30),
				Amount:    -1000,
				Frequency: ynab.FrequencyMonthly,
				Memo:      ynab.Set(memo + "-sentinel"),
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				gone, err := plan.Scheduled.Delete(cleanupCtx, sentinel.ID)
				require.NoError(t, err, "cleanup must restore the test plan")
				require.True(t, gone.IsDeleted())
			})

			got, err := plan.Scheduled.Get(t.Context(), created.ID)
			require.NoError(t, err)
			require.Equal(t, created.ID, got.ID)

			updated, err := plan.Scheduled.Update(t.Context(), created.ID, ynab.ScheduledTransactionSpec{
				AccountID: accounts[0].ID,
				Date:      ynab.Today().AddDays(45),
				Amount:    -2000,
				Frequency: ynab.FrequencyMonthly,
				Memo:      ynab.Set(memo + "-upd"),
				FlagColor: ynab.SetNull[ynab.FlagColor](),
			})
			require.NoError(t, err)
			require.Equal(t, ynab.Milliunits(-2000), updated.Amount)
			require.Nil(t, updated.FlagColor, "SetNull must clear the flag on the real server")

			// The success branch must carry a positive delta cursor and the
			// row; a real delta read then observes the tombstone.
			listed, sk, err := plan.Scheduled.List(t.Context())
			require.NoError(t, err)
			require.Positive(t, int64(sk))
			store := ynab.MergeByID(nil, listed)
			require.Contains(t, store, created.ID)

			// Delete in the body so the tombstone is observable through a
			// delta read; the tolerant cleanup above stays as the safety net.
			gone, err := plan.Scheduled.Delete(t.Context(), created.ID)
			require.NoError(t, err)
			require.True(t, gone.IsDeleted())

			changes, _, err := plan.Scheduled.List(t.Context(), ynab.Since(sk))
			require.NoError(t, err)
			tombstoned := false
			for _, c := range changes {
				if c.SyncID() == created.ID {
					tombstoned = true
					require.True(t, c.IsDeleted(), "post-delete delta must carry a tombstone")
				}
			}
			require.True(t, tombstoned, "delta read since %d must include the deleted row", sk)
			store = ynab.MergeByID(store, changes)
			require.NotContains(t, store, created.ID, "MergeByID must fold the live tombstone")
		},
	})
}

func TestScheduledList(t *testing.T) {
	t.Parallel()

	t.Run("golden with split and null variants", func(t *testing.T) {
		t.Parallel()

		client, rec := serveFixture(t, "scheduled/list.json", 0)
		scheduled, sk, err := client.Plan("p-1").Scheduled.List(t.Context(), ynab.Since(6900))
		require.NoError(t, err)
		require.Equal(t, "/plans/p-1/scheduled_transactions", rec.URL.Path)
		require.Equal(t, "6900", rec.URL.Query().Get("last_knowledge_of_server"))
		require.Equal(t, ynab.ServerKnowledge(7000), sk)
		require.Len(t, scheduled, 3)

		rent := scheduled[0]
		require.Equal(t, ynab.FrequencyMonthly, rent.Frequency)
		require.True(t, rent.Frequency.Valid())
		require.Equal(t, ynab.NewDate(2026, time.August, 1), rent.DateNext)

		split := scheduled[1]
		require.Equal(t, "Split", *split.CategoryName)
		require.Len(t, split.Subtransactions, 2)
		require.Equal(t, split.ID, split.Subtransactions[0].ScheduledTransactionID)

		bare := scheduled[2]
		require.Nil(t, bare.Memo)
		require.Nil(t, bare.PayeeID)
	})

	t.Run("empty-plan 404 folds into an empty list", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"id":"404.2","name":"resource_not_found","detail":"No scheduled transactions"}}`))
		}))
		t.Cleanup(srv.Close)

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		scheduled, sk, err := client.Plan("p-1").Scheduled.List(t.Context())
		require.NoError(t, err, "the surgical normalization — List only")
		require.NotNil(t, scheduled)
		require.Empty(t, scheduled)
		require.Zero(t, sk)

		// The surgical contrast: Get on the same wire answer stays an error.
		_, err = client.Plan("p-1").Scheduled.Get(t.Context(), "sc-missing")
		require.ErrorIs(t, err, ynab.ErrResourceNotFound)
	})
}

func TestScheduledCreate(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "scheduled/create.json", http.StatusCreated)
	created, err := client.Plan("p-1").Scheduled.Create(t.Context(), ynab.ScheduledTransactionSpec{
		AccountID: "ac1",
		Date:      ynab.Today().AddDays(10),
		Amount:    -1500000,
		Frequency: ynab.FrequencyMonthly,
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, rec.Method)
	require.Equal(t, "sc444444-4444-4444-4444-444444444444", created.ID)
}

func TestScheduledDateWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		date ynab.Date
	}{
		{name: "past date", date: ynab.Today().AddDays(-1)},
		{name: "beyond five years", date: ynab.Today().AddMonths(61)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("no request must be sent on a pre-flight failure")
			}))
			t.Cleanup(srv.Close)
			client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())

			spec := ynab.ScheduledTransactionSpec{AccountID: "ac1", Date: tt.date, Amount: -1}
			_, err := client.Plan("p-1").Scheduled.Create(t.Context(), spec)
			var argErr *ynab.ArgumentError
			require.ErrorAs(t, err, &argErr)
			require.Equal(t, "date", argErr.Field)

			_, err = client.Plan("p-1").Scheduled.Update(t.Context(), "sc1", spec)
			require.ErrorAs(t, err, &argErr)
		})
	}
}

func TestScheduledUpdateDelete(t *testing.T) {
	t.Parallel()

	client, rec := serveFixture(t, "scheduled/update.json", 0)
	updated, err := client.Plan("p-1").Scheduled.Update(t.Context(), "sc111111-1111-1111-1111-111111111111",
		ynab.ScheduledTransactionSpec{
			AccountID: "ac1", Date: ynab.Today().AddDays(5), Amount: -1600000,
		})
	require.NoError(t, err)
	require.Equal(t, http.MethodPut, rec.Method, "scheduled update is a full-spec PUT")
	require.Equal(t, ynab.Milliunits(-1600000), updated.Amount)

	client2, rec2 := serveFixture(t, "scheduled/delete.json", 0)
	gone, err := client2.Plan("p-1").Scheduled.Delete(t.Context(), "sc111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, rec2.Method)
	require.True(t, gone.IsDeleted())
}
