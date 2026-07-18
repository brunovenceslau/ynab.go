package ynab_test

// Tests added by the phase-3 ship review: merge-key adapters, enum
// tables, per-endpoint error propagation, and strict decoding of every
// golden fixture — the systemic net against typo'd fixture keys.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// TestSyncableAdapters pins every SyncID/IsDeleted pair: a wrong merge
// key would silently corrupt users' delta stores and no wire gate can
// see it.
func TestSyncableAdapters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    ynab.Syncable
		id   string
	}{
		{name: "Account", s: ynab.Account{AccountBase: ynab.AccountBase{ID: "a", Deleted: true}}, id: "a"},
		{name: "Category", s: ynab.Category{CategoryBase: ynab.CategoryBase{ID: "c", Deleted: true}}, id: "c"},
		{name: "CategoryGroup", s: ynab.CategoryGroup{ID: "g", Deleted: true}, id: "g"},
		{name: "Payee", s: ynab.Payee{ID: "p", Deleted: true}, id: "p"},
		{name: "PayeeLocation", s: ynab.PayeeLocation{ID: "l", Deleted: true}, id: "l"},
		{
			name: "MonthSummary",
			s: ynab.MonthSummary{MonthSummaryBase: ynab.MonthSummaryBase{
				Month: ynab.NewMonth(2026, time.July), Deleted: true,
			}},
			id: "2026-07-01",
		},
		{
			name: "Transaction",
			s:    ynab.Transaction{TransactionBase: ynab.TransactionBase{ID: "t", Deleted: true}},
			id:   "t",
		},
		{
			name: "Subtransaction",
			s:    ynab.Subtransaction{SubtransactionBase: ynab.SubtransactionBase{ID: "s", Deleted: true}},
			id:   "s",
		},
		{
			name: "HybridTransaction",
			s: ynab.HybridTransaction{
				TransactionBase:     ynab.TransactionBase{ID: "h", Deleted: true},
				ParentTransactionID: ptr("NOT-the-key"),
			},
			id: "h",
		},
		{
			name: "ScheduledTransaction",
			s: ynab.ScheduledTransaction{
				ScheduledTransactionBase: ynab.ScheduledTransactionBase{ID: "sc", Deleted: true},
			},
			id: "sc",
		},
		{
			name: "ScheduledSubtransaction",
			s: ynab.ScheduledSubtransaction{
				ScheduledSubtransactionBase: ynab.ScheduledSubtransactionBase{ID: "ss", Deleted: true},
			},
			id: "ss",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.id, tt.s.SyncID())
			require.True(t, tt.s.IsDeleted())
		})
	}
}

func ptr[T any](v T) *T { return &v }

// TestEnumValidTables covers every enum's Valid across its full value set
// plus an unknown.
func TestEnumValidTables(t *testing.T) {
	t.Parallel()

	t.Run("SaveAccountType", func(t *testing.T) {
		t.Parallel()
		for _, v := range []ynab.SaveAccountType{
			ynab.SaveAccountTypeChecking, ynab.SaveAccountTypeSavings, ynab.SaveAccountTypeCash,
			ynab.SaveAccountTypeCreditCard, ynab.SaveAccountTypeOtherAsset, ynab.SaveAccountTypeOtherLiability,
		} {
			require.True(t, v.Valid(), v)
		}
		require.False(t, ynab.SaveAccountType("mortgage").Valid(), "loan types cannot be created")
	})

	t.Run("GoalType and GoalFrequency", func(t *testing.T) {
		t.Parallel()
		for _, v := range []ynab.GoalType{
			ynab.GoalTypeTB, ynab.GoalTypeTBD, ynab.GoalTypeMF, ynab.GoalTypeNEED, ynab.GoalTypeDEBT,
		} {
			require.True(t, v.Valid(), v)
		}
		require.False(t, ynab.GoalType("SAVE").Valid())

		for _, v := range []ynab.GoalFrequency{
			ynab.GoalFrequencyMonthly, ynab.GoalFrequencyWeekly, ynab.GoalFrequencyYearly,
		} {
			require.True(t, v.Valid(), v)
		}
		require.False(t, ynab.GoalFrequency("daily").Valid())
	})

	t.Run("TransactionType and DebtTransactionType", func(t *testing.T) {
		t.Parallel()
		require.True(t, ynab.TransactionTypeUncategorized.Valid())
		require.True(t, ynab.TransactionTypeUnapproved.Valid())
		require.False(t, ynab.TransactionType("cleared").Valid())

		for _, v := range []ynab.DebtTransactionType{
			ynab.DebtTransactionTypePayment, ynab.DebtTransactionTypeRefund,
			ynab.DebtTransactionTypeFee, ynab.DebtTransactionTypeInterest,
			ynab.DebtTransactionTypeEscrow, ynab.DebtTransactionTypeBalanceAdjustment,
			ynab.DebtTransactionTypeCredit, ynab.DebtTransactionTypeCharge,
		} {
			require.True(t, v.Valid(), v)
		}
		require.False(t, ynab.DebtTransactionType("penalty").Valid())
	})

	t.Run("Frequency", func(t *testing.T) {
		t.Parallel()
		for _, v := range []ynab.Frequency{
			ynab.FrequencyNever, ynab.FrequencyDaily, ynab.FrequencyWeekly,
			ynab.FrequencyEveryOtherWeek, ynab.FrequencyTwiceAMonth, ynab.FrequencyEvery4Weeks,
			ynab.FrequencyMonthly, ynab.FrequencyEveryOtherMonth, ynab.FrequencyEvery3Months,
			ynab.FrequencyEvery4Months, ynab.FrequencyTwiceAYear, ynab.FrequencyYearly,
			ynab.FrequencyEveryOtherYear,
		} {
			require.True(t, v.Valid(), v)
		}
		require.False(t, ynab.Frequency("fortnightly").Valid())
	})
}

// TestContractEndpointErrorPropagation drives every registered endpoint
// case against a 500 and requires the error to surface — closing the
// transport-error branch of every service method mechanically.
func TestContractEndpointErrorPropagation(t *testing.T) {
	t.Parallel()

	endpointRegistryMu.Lock()
	cases := make([]endpointCase, len(endpointRegistry))
	copy(cases, endpointRegistry)
	endpointRegistryMu.Unlock()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"id":"500","name":"internal_server_error","detail":"boom"}}`))
	}))
	t.Cleanup(srv.Close)

	for _, ec := range cases {
		name := ec.op
		if ec.variant != "" {
			name += "/" + ec.variant
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
			_, err := ec.call(t, client)
			require.ErrorIs(t, err, ynab.ErrServerError, "the 500 must propagate through the method")
		})
	}
}

// fixtureModels maps every fixture file to its decode target: wrapper key
// under the envelope's data plus the public model. TestFixturesDecodeStrict
// strict-decodes each one, so a typo'd fixture key can never silently
// leave a pointer field nil.
func fixtureModels() map[string]struct {
	wrapper string
	model   any
} {
	m := map[string]struct {
		wrapper string
		model   any
	}{}
	add := func(pattern, wrapper string, model any) {
		m[pattern] = struct {
			wrapper string
			model   any
		}{wrapper, model}
	}

	add("user/get.json", "user", ynab.User{})
	add("plans/list*.json", "", ynab.PlanList{})
	add("plans/settings*.json", "settings", ynab.PlanSettings{})
	add("plans/export*.json", "plan", ynab.PlanDetail{})
	add("accounts/list*.json", "accounts", []ynab.Account{})
	add("accounts/get.json", "account", ynab.Account{})
	add("accounts/create.json", "account", ynab.Account{})
	add("accounts/extreme.json", "account", ynab.Account{})
	add("categories/list*.json", "category_groups", []ynab.CategoryGroup{})
	add("categories/get*.json", "category", ynab.Category{})
	add("categories/create.json", "category", ynab.Category{})
	add("categories/update.json", "category", ynab.Category{})
	add("categories/assign.json", "category", ynab.Category{})
	add("categories/month_get.json", "category", ynab.Category{})
	add("categories/extreme.json", "category", ynab.Category{})
	add("categories/group_create.json", "category_group", ynab.CategoryGroup{})
	add("categories/group_rename.json", "category_group", ynab.CategoryGroup{})
	add("months/list*.json", "months", []ynab.MonthSummary{})
	add("months/get*.json", "month", ynab.MonthDetail{})
	add("payees/list*.json", "payees", []ynab.Payee{})
	add("payees/get.json", "payee", ynab.Payee{})
	add("payees/create.json", "payee", ynab.Payee{})
	add("payees/rename.json", "payee", ynab.Payee{})
	add("payee_locations/list.json", "payee_locations", []ynab.PayeeLocation{})
	add("payee_locations/by_payee.json", "payee_locations", []ynab.PayeeLocation{})
	add("payee_locations/get.json", "payee_location", ynab.PayeeLocation{})
	add("money_movements/list.json", "money_movements", []ynab.MoneyMovement{})
	add("money_movements/by_month.json", "money_movements", []ynab.MoneyMovement{})
	add("money_movements/groups.json", "money_movement_groups", []ynab.MoneyMovementGroup{})
	add("money_movements/groups_by_month.json", "money_movement_groups", []ynab.MoneyMovementGroup{})
	add("transactions/list*.json", "transactions", []ynab.Transaction{})
	add("transactions/hybrid*.json", "transactions", []ynab.HybridTransaction{})
	add("transactions/get*.json", "transaction", ynab.Transaction{})
	add("transactions/update.json", "transaction", ynab.Transaction{})
	add("transactions/delete.json", "transaction", ynab.Transaction{})
	add("transactions/extreme.json", "transaction", ynab.Transaction{})
	add("transactions/create.json", "transaction", ynab.Transaction{})
	add("transactions/create_batch*.json", "", ynab.BatchResult{})
	add("transactions/import_*.json", "transaction_ids", []string{})
	add("scheduled/list.json", "scheduled_transactions", []ynab.ScheduledTransaction{})
	add("scheduled/get.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/create.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/update.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/delete.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	return m
}

func TestFixturesDecodeStrict(t *testing.T) {
	t.Parallel()

	models := fixtureModels()
	match := func(name string) (string, any, bool) {
		for pattern, mm := range models {
			ok, err := filepath.Match(pattern, name)
			require.NoError(t, err)
			if ok {
				return mm.wrapper, mm.model, true
			}
		}
		return "", nil, false
	}

	var checked int
	err := filepath.WalkDir("testdata", func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() || !strings.HasSuffix(path, ".json") || strings.HasPrefix(path, filepath.Join("testdata", "selftest")) {
			return nil
		}
		rel, err := filepath.Rel("testdata", path)
		require.NoError(t, err)
		rel = filepath.ToSlash(rel)

		wrapper, model, ok := match(rel)
		require.True(t, ok, "fixture %s has no strict-decode mapping — add it to fixtureModels", rel)

		doc := modelDocument(t, loadFixture(t, rel), wrapper)
		require.NoError(t, strictDecode(doc, model), "fixture %s must strict-decode into %T", rel, model)
		checked++
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, checked, 40, "the walk must actually visit the fixture tree")
}
