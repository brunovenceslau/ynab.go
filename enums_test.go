// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// The enum nets: every Valid() table pinned against its const block.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func TestEnumValidTables(t *testing.T) {
	t.Parallel()

	t.Run("AccountSpecType", func(t *testing.T) {
		t.Parallel()
		for _, v := range []ynab.AccountSpecType{
			ynab.AccountSpecTypeChecking, ynab.AccountSpecTypeSavings, ynab.AccountSpecTypeCash,
			ynab.AccountSpecTypeCreditCard, ynab.AccountSpecTypeOtherAsset, ynab.AccountSpecTypeOtherLiability,
		} {
			require.True(t, v.Valid(), v)
		}
		require.False(t, ynab.AccountSpecType("mortgage").Valid(), "loan types cannot be created")
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

// TestContractReadErrorPropagation drives every registered endpoint
// case against a 500 and requires the error to surface — closing the
