// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Shared golden expected values, transcribed field-by-field from the
// fixtures under testdata/. The golden decode tests assert complete
// structs against these, so a swapped json tag or a dropped mapping on
// ANY field fails a test — hand-picked field subsets cannot catch that
// mutation class. Entities appearing in several fixtures (the list, the
// get, and the plan-export collections share elements by design) reuse
// one constructor, keeping every copy pinned to the same values.

import (
	"time"

	"pkg.venceslau.dev/ynab"
)

// goldenCheckingAccountBase is accounts/list.json's first element (and
// plans/export.json's accounts[0]).
func goldenCheckingAccountBase() ynab.AccountBase {
	return ynab.AccountBase{
		ID:                  "ac111111-1111-1111-1111-111111111111",
		Name:                "Checking",
		Type:                ynab.AccountTypeChecking,
		OnBudget:            true,
		Closed:              false,
		Note:                ptr("primary"),
		Balance:             123930,
		ClearedBalance:      100000,
		UnclearedBalance:    23930,
		TransferPayeeID:     ptr("pa111111-1111-1111-1111-111111111111"),
		DirectImportLinked:  true,
		DirectImportInError: false,
		LastReconciledAt:    ptr(time.Date(2026, time.June, 30, 10, 0, 0, 0, time.UTC)),
		Deleted:             false,
	}
}

// goldenGroceriesCategoryBase is the "Groceries" category shared by
// categories/list.json, months/get.json, and plans/export.json.
func goldenGroceriesCategoryBase() ynab.CategoryBase {
	return ynab.CategoryBase{
		ID:                     "ca111111-1111-1111-1111-111111111111",
		CategoryGroupID:        "cg111111-1111-1111-1111-111111111111",
		CategoryGroupName:      "Everyday",
		Name:                   "Groceries",
		Hidden:                 false,
		Internal:               false,
		Note:                   ptr("weekly shop"),
		Budgeted:               500000,
		Activity:               -123930,
		Balance:                376070,
		GoalType:               ptr(ynab.GoalTypeNEED),
		GoalNeedsWholeAmount:   ptr(false),
		GoalCadence:            ptr(1),
		GoalCadenceFrequency:   ptr(1),
		GoalCreationMonth:      ptr(ynab.NewDate(2025, time.January, 1)),
		GoalTarget:             ptr(ynab.Milliunits(500000)),
		GoalPercentageComplete: ptr(75),
		GoalMonthsToBudget:     ptr(1),
		GoalUnderFunded:        ptr(ynab.Milliunits(123930)),
		GoalOverallFunded:      ptr(ynab.Milliunits(376070)),
		GoalOverallLeft:        ptr(ynab.Milliunits(123930)),
		Deleted:                false,
	}
}

// goldenGroceriesCategory adds the display companions the full Category
// shape carries in categories/list.json and months/get.json.
func goldenGroceriesCategory() ynab.Category {
	return ynab.Category{
		CategoryBase:               goldenGroceriesCategoryBase(),
		BalanceFormatted:           "$376.07",
		BalanceCurrency:            376.07,
		ActivityFormatted:          "-$123.93",
		ActivityCurrency:           -123.93,
		BudgetedFormatted:          "$500.00",
		BudgetedCurrency:           500,
		GoalTargetFormatted:        ptr("$500.00"),
		GoalTargetCurrency:         ptr(500.0),
		GoalUnderFundedFormatted:   ptr("$123.93"),
		GoalUnderFundedCurrency:    ptr(123.93),
		GoalOverallFundedFormatted: ptr("$376.07"),
		GoalOverallFundedCurrency:  ptr(376.07),
		GoalOverallLeftFormatted:   ptr("$123.93"),
		GoalOverallLeftCurrency:    ptr(123.93),
	}
}

// goldenJulyMonthSummaryBase is months/list.json's first element and
// months/get.json's summary core (and plans/export.json's months[0]).
func goldenJulyMonthSummaryBase() ynab.MonthSummaryBase {
	return ynab.MonthSummaryBase{
		Month:        ynab.NewMonth(2026, time.July),
		Note:         ptr("kickoff"),
		Income:       5000000,
		Budgeted:     4500000,
		Activity:     -3200000,
		ToBeBudgeted: 500000,
		AgeOfMoney:   ptr(24),
		Deleted:      false,
	}
}

// goldenJulyMonthSummary adds the display companions.
func goldenJulyMonthSummary() ynab.MonthSummary {
	return ynab.MonthSummary{
		MonthSummaryBase:      goldenJulyMonthSummaryBase(),
		IncomeFormatted:       "$5,000.00",
		IncomeCurrency:        5000,
		BudgetedFormatted:     "$4,500.00",
		BudgetedCurrency:      4500,
		ActivityFormatted:     "-$3,200.00",
		ActivityCurrency:      -3200,
		ToBeBudgetedFormatted: "$500.00",
		ToBeBudgetedCurrency:  500,
	}
}

// goldenPayees is payees/list.json's collection (and
// plans/export.json's payees); element 0 is payees/get.json.
func goldenPayees() []ynab.Payee {
	return []ynab.Payee{
		{
			ID:      "pa111111-1111-1111-1111-111111111111",
			Name:    "Grocer & Co",
			Deleted: false,
		},
		{
			ID:                "pa222222-2222-2222-2222-222222222222",
			Name:              "Transfer : Savings",
			TransferAccountID: ptr("ac222222-2222-2222-2222-222222222222"),
			Deleted:           false,
		},
	}
}

// goldenPayeeLocations is payee_locations/list.json's collection (and
// plans/export.json's payee_locations); element 0 is
// payee_locations/get.json and by_payee.json's only row.
func goldenPayeeLocations() []ynab.PayeeLocation {
	return []ynab.PayeeLocation{
		{
			ID:        "pl111111-1111-1111-1111-111111111111",
			PayeeID:   "pa111111-1111-1111-1111-111111111111",
			Latitude:  "-23.5505199",
			Longitude: "-46.6333094",
			Deleted:   false,
		},
		{
			ID:        "pl222222-2222-2222-2222-222222222222",
			PayeeID:   "pa111111-1111-1111-1111-111111111111",
			Latitude:  "40.7127753",
			Longitude: "-74.0059728",
			Deleted:   false,
		},
	}
}

// goldenGroceryRunTransactionBase is transactions/get.json's base shape
// (and plans/export.json's transactions[0]).
func goldenGroceryRunTransactionBase() ynab.TransactionBase {
	return ynab.TransactionBase{
		ID:                      "tr111111-1111-1111-1111-111111111111",
		Date:                    ynab.NewDate(2026, time.July, 10),
		Amount:                  -294230,
		Memo:                    ptr("groceries run"),
		Cleared:                 ynab.ClearedStatusCleared,
		Approved:                true,
		FlagColor:               ptr(ynab.FlagColorRed),
		FlagName:                ptr("urgent"),
		AccountID:               "ac111111-1111-1111-1111-111111111111",
		PayeeID:                 ptr("pa111111-1111-1111-1111-111111111111"),
		CategoryID:              ptr("ca111111-1111-1111-1111-111111111111"),
		ImportID:                ptr("YNAB:-294230:2026-07-10:1"),
		ImportPayeeName:         ptr("GROCER CO"),
		ImportPayeeNameOriginal: ptr("GROCER*CO 123"),
		Deleted:                 false,
	}
}

// goldenSplitLegBases are transactions/get.json's two subtransaction
// bases; element 0 is plans/export.json's subtransactions[0].
func goldenSplitLegBases() []ynab.SubtransactionBase {
	return []ynab.SubtransactionBase{
		{
			ID:            "st111111-1111-1111-1111-111111111111",
			TransactionID: "tr111111-1111-1111-1111-111111111111",
			Amount:        -147115,
			CategoryID:    ptr("ca111111-1111-1111-1111-111111111111"),
			CategoryName:  ptr("Groceries"),
			Deleted:       false,
		},
		{
			ID:            "st222222-2222-2222-2222-222222222222",
			TransactionID: "tr111111-1111-1111-1111-111111111111",
			Amount:        -147115,
			Memo:          ptr("second leg"),
			CategoryID:    ptr("ca222222-2222-2222-2222-222222222222"),
			CategoryName:  ptr("Vacation"),
			Deleted:       false,
		},
	}
}

// goldenGroceryRunTransaction is transactions/get.json's full shape
// (identical to transactions/list.json's only element).
func goldenGroceryRunTransaction() ynab.Transaction {
	legs := goldenSplitLegBases()
	return ynab.Transaction{
		TransactionBase: goldenGroceryRunTransactionBase(),
		AmountFormatted: "-$294.23",
		AmountCurrency:  -294.23,
		AccountName:     "Checking",
		PayeeName:       ptr("Grocer & Co"),
		CategoryName:    ptr("Split"),
		Subtransactions: []ynab.Subtransaction{
			{SubtransactionBase: legs[0], AmountFormatted: "x", AmountCurrency: -147.115},
			{SubtransactionBase: legs[1], AmountFormatted: "x", AmountCurrency: -147.115},
		},
	}
}

// goldenHybridGroceryRow is the regular-transaction row shared verbatim
// by transactions/hybrid.json (row 0) and
// transactions/hybrid_no_server_knowledge.json (its only row).
func goldenHybridGroceryRow() ynab.HybridTransaction {
	return ynab.HybridTransaction{
		TransactionBase: ynab.TransactionBase{
			ID:                      "tr444444-4444-4444-4444-444444444444",
			Date:                    ynab.NewDate(2026, time.July, 10),
			Amount:                  -294230,
			Memo:                    ptr("groceries run"),
			Cleared:                 ynab.ClearedStatusCleared,
			Approved:                true,
			AccountID:               "ac111111-1111-1111-1111-111111111111",
			PayeeID:                 ptr("pa111111-1111-1111-1111-111111111111"),
			CategoryID:              ptr("ca111111-1111-1111-1111-111111111111"),
			ImportID:                ptr("YNAB:-294230:2026-07-10:1"),
			ImportPayeeName:         ptr("GROCER CO"),
			ImportPayeeNameOriginal: ptr("GROCER*CO 123"),
			Deleted:                 false,
		},
		AmountFormatted: "-$10.00",
		AmountCurrency:  -10,
		Type:            ynab.HybridTypeTransaction,
		AccountName:     "Checking",
		PayeeName:       ptr("Grocer & Co"),
		CategoryName:    "Groceries",
	}
}

// goldenRentScheduledBase is scheduled/get.json's base shape, shared
// with scheduled/list.json's first element and plans/export.json's
// scheduled_transactions[0].
func goldenRentScheduledBase() ynab.ScheduledTransactionBase {
	return ynab.ScheduledTransactionBase{
		ID:         "sc111111-1111-1111-1111-111111111111",
		DateFirst:  ynab.NewDate(2026, time.August, 1),
		DateNext:   ynab.NewDate(2026, time.August, 1),
		Frequency:  ynab.FrequencyMonthly,
		Amount:     -1500000,
		Memo:       ptr("rent"),
		AccountID:  "ac111111-1111-1111-1111-111111111111",
		PayeeID:    ptr("pa555555-5555-5555-5555-555555555555"),
		CategoryID: ptr("ca111111-1111-1111-1111-111111111111"),
		Deleted:    false,
	}
}

// goldenRentScheduled is scheduled/get.json's full shape (and
// scheduled/list.json's first element).
func goldenRentScheduled() ynab.ScheduledTransaction {
	return ynab.ScheduledTransaction{
		ScheduledTransactionBase: goldenRentScheduledBase(),
		AmountFormatted:          "-$1,500.00",
		AmountCurrency:           -1500,
		AccountName:              "Checking",
		PayeeName:                ptr("Landlord"),
		CategoryName:             ptr("Rent"),
		Subtransactions:          []ynab.ScheduledSubtransaction{},
	}
}

// goldenScheduledRentLegBase is scheduled/list.json's first split leg
// (and plans/export.json's scheduled_subtransactions[0]).
func goldenScheduledRentLegBase() ynab.ScheduledSubtransactionBase {
	return ynab.ScheduledSubtransactionBase{
		ID:                     "ss111111-1111-1111-1111-111111111111",
		ScheduledTransactionID: "sc222222-2222-2222-2222-222222222222",
		Amount:                 -750000,
		CategoryID:             ptr("ca111111-1111-1111-1111-111111111111"),
		CategoryName:           ptr("Rent"),
		Deleted:                false,
	}
}

// goldenUSDCurrencyFormat is the USD currency_format block shared by
// plans/list.json, plans/list_accounts.json, and plans/export.json.
func goldenUSDCurrencyFormat() *ynab.CurrencyFormat {
	return &ynab.CurrencyFormat{
		ISOCode:          "USD",
		ExampleFormat:    "123,456.78",
		DecimalDigits:    2,
		DecimalSeparator: ".",
		SymbolFirst:      true,
		GroupSeparator:   ",",
		CurrencySymbol:   "$",
		DisplaySymbol:    true,
	}
}

// goldenFamilyPlanSummary is plans/list.json's first plan, which is
// also its default_plan.
func goldenFamilyPlanSummary() ynab.PlanSummary {
	return ynab.PlanSummary{
		ID:             "aa111111-1111-1111-1111-111111111111",
		Name:           "Family Plan",
		LastModifiedOn: time.Date(2026, time.July, 1, 12, 34, 56, 789000000, time.UTC),
		FirstMonth:     ynab.NewMonth(2024, time.January),
		LastMonth:      ynab.NewMonth(2026, time.August),
		DateFormat:     &ynab.DateFormat{Format: "MM/DD/YYYY"},
		CurrencyFormat: goldenUSDCurrencyFormat(),
	}
}

// goldenSideHustlePlanSummary is plans/list.json's second plan.
func goldenSideHustlePlanSummary() ynab.PlanSummary {
	return ynab.PlanSummary{
		ID:             "bb222222-2222-2222-2222-222222222222",
		Name:           "Side Hustle",
		LastModifiedOn: time.Date(2026, time.June, 15, 8, 0, 0, 0, time.UTC),
		FirstMonth:     ynab.NewMonth(2025, time.May),
		LastMonth:      ynab.NewMonth(2026, time.July),
		DateFormat:     &ynab.DateFormat{Format: "YYYY-MM-DD"},
		CurrencyFormat: &ynab.CurrencyFormat{
			ISOCode:          "JOD",
			ExampleFormat:    "123,456.789",
			DecimalDigits:    3,
			DecimalSeparator: ".",
			SymbolFirst:      false,
			GroupSeparator:   ",",
			CurrencySymbol:   "JD",
			DisplaySymbol:    false,
		},
	}
}

// goldenTopUpMovement is money_movements/list.json's first element (and
// money_movements/by_month.json's only one).
func goldenTopUpMovement() ynab.MoneyMovement {
	return ynab.MoneyMovement{
		ID:                   "mm111111-1111-1111-1111-111111111111",
		Month:                ptr(ynab.NewMonth(2026, time.July)),
		MovedAt:              ptr(time.Date(2026, time.July, 10, 9, 30, 0, 0, time.UTC)),
		Note:                 ptr("topping up groceries"),
		MoneyMovementGroupID: ptr("mg111111-1111-1111-1111-111111111111"),
		PerformedByUserID:    ptr("11111111-2222-3333-4444-555555555555"),
		FromCategoryID:       ptr("ca222222-2222-2222-2222-222222222222"),
		ToCategoryID:         ptr("ca111111-1111-1111-1111-111111111111"),
		Amount:               250000,
		AmountFormatted:      "$250.00",
		AmountCurrency:       250,
	}
}

// goldenRebalanceGroup is money_movements/groups.json's first element
// (and money_movements/groups_by_month.json's only one).
func goldenRebalanceGroup() ynab.MoneyMovementGroup {
	return ynab.MoneyMovementGroup{
		ID:                "mg111111-1111-1111-1111-111111111111",
		GroupCreatedAt:    time.Date(2026, time.July, 10, 9, 29, 0, 0, time.UTC),
		Month:             ynab.NewMonth(2026, time.July),
		Note:              ptr("monthly rebalance"),
		PerformedByUserID: ptr("11111111-2222-3333-4444-555555555555"),
	}
}
