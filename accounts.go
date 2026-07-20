// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"context"
	"net/http"
	"time"
)

// AccountType is an account's read-side type. Thirteen values exist on
// the wire; unknown future values decode losslessly with Valid() false.
type AccountType string

// The thirteen wire account types.
const (
	AccountTypeChecking       AccountType = "checking"
	AccountTypeSavings        AccountType = "savings"
	AccountTypeCash           AccountType = "cash"
	AccountTypeCreditCard     AccountType = "creditCard"
	AccountTypeLineOfCredit   AccountType = "lineOfCredit"
	AccountTypeOtherAsset     AccountType = "otherAsset"
	AccountTypeOtherLiability AccountType = "otherLiability"
	AccountTypeMortgage       AccountType = "mortgage"
	AccountTypeAutoLoan       AccountType = "autoLoan"
	AccountTypeStudentLoan    AccountType = "studentLoan"
	AccountTypePersonalLoan   AccountType = "personalLoan"
	AccountTypeMedicalDebt    AccountType = "medicalDebt"
	AccountTypeOtherDebt      AccountType = "otherDebt"
)

// Valid reports whether t is one of the documented wire values.
func (t AccountType) Valid() bool {
	switch t {
	case AccountTypeChecking, AccountTypeSavings, AccountTypeCash,
		AccountTypeCreditCard, AccountTypeLineOfCredit, AccountTypeOtherAsset,
		AccountTypeOtherLiability, AccountTypeMortgage, AccountTypeAutoLoan,
		AccountTypeStudentLoan, AccountTypePersonalLoan,
		AccountTypeMedicalDebt, AccountTypeOtherDebt:
		return true
	default:
		return false
	}
}

// AccountSpecType is an account type accepted by Create — deliberately a
// distinct type from AccountType: the seven loan/debt read types cannot
// compile into a create call.
type AccountSpecType string

// The six account types the API accepts on creation.
const (
	AccountSpecTypeChecking       AccountSpecType = "checking"
	AccountSpecTypeSavings        AccountSpecType = "savings"
	AccountSpecTypeCash           AccountSpecType = "cash"
	AccountSpecTypeCreditCard     AccountSpecType = "creditCard"
	AccountSpecTypeOtherAsset     AccountSpecType = "otherAsset"
	AccountSpecTypeOtherLiability AccountSpecType = "otherLiability"
)

// Valid reports whether t is one of the documented create values.
func (t AccountSpecType) Valid() bool {
	switch t {
	case AccountSpecTypeChecking, AccountSpecTypeSavings, AccountSpecTypeCash,
		AccountSpecTypeCreditCard, AccountSpecTypeOtherAsset, AccountSpecTypeOtherLiability:
		return true
	default:
		return false
	}
}

// LoanAccountPeriodicValue maps effective dates to milliunit values for
// loan accounts (interest rates, minimum payments, escrow amounts). The
// whole map is null for non-loan accounts.
type LoanAccountPeriodicValue map[string]Milliunits

// AccountBase is the account shape shared by the accounts endpoints and
// the full-plan export collections.
type AccountBase struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	Type                AccountType `json:"type"`
	OnBudget            bool        `json:"on_budget"`
	Closed              bool        `json:"closed"`
	Note                *string     `json:"note"`
	Balance             Milliunits  `json:"balance"`
	ClearedBalance      Milliunits  `json:"cleared_balance"`
	UnclearedBalance    Milliunits  `json:"uncleared_balance"`
	TransferPayeeID     *string     `json:"transfer_payee_id"`
	DirectImportLinked  bool        `json:"direct_import_linked"`
	DirectImportInError bool        `json:"direct_import_in_error"`
	LastReconciledAt    *time.Time  `json:"last_reconciled_at"`
	// Deprecated: the server always answers null; kept for wire fidelity.
	DebtOriginalBalance *Milliunits              `json:"debt_original_balance"`
	DebtInterestRates   LoanAccountPeriodicValue `json:"debt_interest_rates"`
	DebtMinimumPayments LoanAccountPeriodicValue `json:"debt_minimum_payments"`
	DebtEscrowAmounts   LoanAccountPeriodicValue `json:"debt_escrow_amounts"`
	Deleted             bool                     `json:"deleted"`
}

// Account is a plan account. The *_formatted/*_currency companions are
// computed, read-only display fields — do money math on the milliunit
// integers, never on the float companions.
type Account struct {
	AccountBase
	BalanceFormatted          string  `json:"balance_formatted"`
	BalanceCurrency           float64 `json:"balance_currency"`
	ClearedBalanceFormatted   string  `json:"cleared_balance_formatted"`
	ClearedBalanceCurrency    float64 `json:"cleared_balance_currency"`
	UnclearedBalanceFormatted string  `json:"uncleared_balance_formatted"`
	UnclearedBalanceCurrency  float64 `json:"uncleared_balance_currency"`
}

// SyncID keys the account for MergeByID. Account inherits the adapters
// by embedding.
func (a AccountBase) SyncID() string { return a.ID }

// IsDeleted reports a delta tombstone.
func (a AccountBase) IsDeleted() bool { return a.Deleted }

// AccountSpec is the payload for AccountsService.Create. All three fields
// are required by the API.
type AccountSpec struct {
	Name    string          `json:"name"`
	Type    AccountSpecType `json:"type"`
	Balance Milliunits      `json:"balance"`
}

// AccountsService reads and creates the plan's accounts.
type AccountsService struct {
	plan *Plan
}

// List returns the plan's accounts. With Since, only accounts changed
// after the cursor are returned, deletions arriving as tombstones.
//
// YNAB operationId: getAccounts
func (s *AccountsService) List(ctx context.Context, opts ...ListOption) ([]Account, ServerKnowledge, error) {
	data, err := do[struct {
		Accounts        []Account       `json:"accounts"`
		ServerKnowledge ServerKnowledge `json:"server_knowledge"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("accounts"), applyListOptions(nil, opts), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Accounts, data.ServerKnowledge, nil
}

// Create adds an unlinked account to the plan and returns it (HTTP 201).
// Wire asymmetry, documented rather than papered over: unlike other
// creates, createAccount returns no server knowledge — advance your
// cursor with the next List.
//
// YNAB operationId: createAccount
func (s *AccountsService) Create(ctx context.Context, spec AccountSpec) (*Account, error) {
	data, err := do[struct {
		Account *Account `json:"account"`
	}](ctx, s.plan.client, http.MethodPost, s.plan.path("accounts"), nil, body{"account": spec})
	if err != nil {
		return nil, err
	}
	return data.Account, nil
}

// Get returns a single account by id. A missing id answers
// [ErrResourceNotFound].
//
// YNAB operationId: getAccountById
func (s *AccountsService) Get(ctx context.Context, accountID string) (*Account, error) {
	data, err := do[struct {
		Account *Account `json:"account"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("accounts", accountID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.Account, nil
}
