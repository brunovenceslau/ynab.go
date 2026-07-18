// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"context"
	"errors"
	"net/http"
)

// Frequency is a scheduled transaction's repetition cadence (camelCase on
// the wire).
type Frequency string

// The thirteen wire frequencies.
const (
	FrequencyNever           Frequency = "never"
	FrequencyDaily           Frequency = "daily"
	FrequencyWeekly          Frequency = "weekly"
	FrequencyEveryOtherWeek  Frequency = "everyOtherWeek"
	FrequencyTwiceAMonth     Frequency = "twiceAMonth"
	FrequencyEvery4Weeks     Frequency = "every4Weeks"
	FrequencyMonthly         Frequency = "monthly"
	FrequencyEveryOtherMonth Frequency = "everyOtherMonth"
	FrequencyEvery3Months    Frequency = "every3Months"
	FrequencyEvery4Months    Frequency = "every4Months"
	FrequencyTwiceAYear      Frequency = "twiceAYear"
	FrequencyYearly          Frequency = "yearly"
	FrequencyEveryOtherYear  Frequency = "everyOtherYear"
)

// Valid reports whether f is one of the documented wire values.
func (f Frequency) Valid() bool {
	switch f {
	case FrequencyNever, FrequencyDaily, FrequencyWeekly, FrequencyEveryOtherWeek,
		FrequencyTwiceAMonth, FrequencyEvery4Weeks, FrequencyMonthly,
		FrequencyEveryOtherMonth, FrequencyEvery3Months, FrequencyEvery4Months,
		FrequencyTwiceAYear, FrequencyYearly, FrequencyEveryOtherYear:
		return true
	}
	return false
}

// ScheduledTransactionBase is the scheduled-transaction shape
// shared by the scheduled endpoints and the full-plan export collections.
type ScheduledTransactionBase struct {
	ID                string     `json:"id"`
	DateFirst         Date       `json:"date_first"`
	DateNext          Date       `json:"date_next"`
	Frequency         Frequency  `json:"frequency"`
	Amount            Milliunits `json:"amount"`
	Memo              *string    `json:"memo"`
	FlagColor         *FlagColor `json:"flag_color"`
	FlagName          *string    `json:"flag_name"`
	AccountID         string     `json:"account_id"`
	PayeeID           *string    `json:"payee_id"`
	CategoryID        *string    `json:"category_id"`
	TransferAccountID *string    `json:"transfer_account_id"`
	Deleted           bool       `json:"deleted"`
}

// ScheduledTransaction is a full scheduled transaction with its split
// legs.
type ScheduledTransaction struct {
	ScheduledTransactionBase
	AmountFormatted string                    `json:"amount_formatted"`
	AmountCurrency  float64                   `json:"amount_currency"`
	AccountName     string                    `json:"account_name"`
	PayeeName       *string                   `json:"payee_name"`
	CategoryName    *string                   `json:"category_name"` // "Split" on splits
	Subtransactions []ScheduledSubtransaction `json:"subtransactions"`
}

// SyncID keys the scheduled transaction for [MergeByID].
// ScheduledTransaction inherits the adapters by embedding.
func (s ScheduledTransactionBase) SyncID() string { return s.ID }

// IsDeleted reports a delta tombstone.
func (s ScheduledTransactionBase) IsDeleted() bool { return s.Deleted }

// ScheduledSubtransactionBase is the scheduled split-leg shape shared by
// the scheduled endpoints and the full-plan export collections.
type ScheduledSubtransactionBase struct {
	ID                     string     `json:"id"`
	ScheduledTransactionID string     `json:"scheduled_transaction_id"`
	Amount                 Milliunits `json:"amount"`
	Memo                   *string    `json:"memo"`
	PayeeID                *string    `json:"payee_id"`
	PayeeName              *string    `json:"payee_name"`
	CategoryID             *string    `json:"category_id"`
	CategoryName           *string    `json:"category_name"`
	TransferAccountID      *string    `json:"transfer_account_id"`
	Deleted                bool       `json:"deleted"`
}

// ScheduledSubtransaction is one leg of a split scheduled transaction.
type ScheduledSubtransaction struct {
	ScheduledSubtransactionBase
	AmountFormatted string  `json:"amount_formatted"`
	AmountCurrency  float64 `json:"amount_currency"`
}

// SyncID keys the scheduled subtransaction for [MergeByID].
// ScheduledSubtransaction inherits the adapters by embedding.
func (s ScheduledSubtransactionBase) SyncID() string { return s.ID }

// IsDeleted reports a delta tombstone.
func (s ScheduledSubtransactionBase) IsDeleted() bool { return s.Deleted }

// ScheduledTransactionSpec is the payload for Create. AccountID and
// Date are required; the date must not be in the past and at most five
// years out. Split scheduled transactions cannot be created through
// the API.
type ScheduledTransactionSpec struct {
	AccountID string     `json:"account_id"`
	Date      Date       `json:"date"`
	Amount    Milliunits `json:"amount"`
	Frequency Frequency  `json:"frequency,omitzero"`

	PayeeID Optional[string] `json:"payee_id,omitzero"`
	// PayeeName resolves or creates a payee by name when PayeeID is not
	// set; bounded at 200 characters.
	PayeeName  Optional[string] `json:"payee_name,omitzero"`
	CategoryID Optional[string] `json:"category_id,omitzero"`
	// Memo is bounded at 500 characters.
	Memo      Optional[string]    `json:"memo,omitzero"`
	FlagColor Optional[FlagColor] `json:"flag_color,omitzero"`
}

// checkScheduledDate applies the shared date window: not in the past
// and at most five years out, checked against the local calendar day;
// near midnight the server's notion of "today" may differ by one day.
func checkScheduledDate(op string, d Date) error {
	today := Today()
	switch {
	case d.Before(today):
		return &ArgumentError{Op: op, Field: "date", Reason: "must not be in the past"}
	case d.After(today.AddMonths(60)):
		return &ArgumentError{Op: op, Field: "date", Reason: "must be at most 5 years out"}
	}
	return nil
}

// validate applies the spec-stated invariants before any I/O.
func (s ScheduledTransactionSpec) validate(op string) error {
	return errFirst(
		checkScheduledDate(op, s.Date),
		checkOptRuneMax(op, "payee_name", s.PayeeName, txnPayeeNameMax),
		checkOptRuneMax(op, "memo", s.Memo, memoMax),
	)
}

// ScheduledTransactionsService reads and writes the plan's scheduled
// transactions.
type ScheduledTransactionsService struct {
	plan *Plan
}

// scheduledResult is the single-scheduled-transaction payload.
type scheduledResult struct {
	ScheduledTransaction *ScheduledTransaction `json:"scheduled_transaction"`
}

// List returns the plan's scheduled transactions.
//
// Surgical wire normalization: a plan with no scheduled transactions
// answers 404 instead of an empty list — this method, and only this
// method, folds that 404 into ([], 0, nil). The fold cannot distinguish
// an empty plan from a mistyped plan id (both are 404.2 on the wire), so
// a wrong id also yields an empty list here. Every other operation's 404
// still means ErrResourceNotFound (see API_NOTES.md).
//
// YNAB operationId: getScheduledTransactions
func (s *ScheduledTransactionsService) List(
	ctx context.Context, opts ...ListOption,
) ([]ScheduledTransaction, ServerKnowledge, error) {
	data, err := do[struct {
		ScheduledTransactions []ScheduledTransaction `json:"scheduled_transactions"`
		ServerKnowledge       ServerKnowledge        `json:"server_knowledge"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("scheduled_transactions"), applyListOptions(nil, opts), nil)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []ScheduledTransaction{}, 0, nil
		}
		return nil, 0, err
	}
	return data.ScheduledTransactions, data.ServerKnowledge, nil
}

// Create adds a scheduled transaction (HTTP 201).
//
// YNAB operationId: createScheduledTransaction
func (s *ScheduledTransactionsService) Create(
	ctx context.Context, spec ScheduledTransactionSpec,
) (*ScheduledTransaction, error) {
	if err := spec.validate("Scheduled.Create"); err != nil {
		return nil, err
	}
	data, err := do[scheduledResult](ctx, s.plan.client,
		http.MethodPost, s.plan.path("scheduled_transactions"), nil, body{"scheduled_transaction": spec})
	if err != nil {
		return nil, err
	}
	return data.ScheduledTransaction, nil
}

// Get returns a single scheduled transaction by id. A missing id answers
// ErrResourceNotFound — the List-only 404 normalization does not apply.
//
// YNAB operationId: getScheduledTransactionById
func (s *ScheduledTransactionsService) Get(
	ctx context.Context, scheduledTransactionID string,
) (*ScheduledTransaction, error) {
	data, err := do[scheduledResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("scheduled_transactions", scheduledTransactionID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.ScheduledTransaction, nil
}

// ScheduledTransactionUpdate is the partial payload for Update. Despite
// the PUT verb on the wire, the server treats omitted fields as
// unchanged (probed live, 2026-07-18) — so unset Optionals really mean
// "keep", exactly like TransactionUpdate.
type ScheduledTransactionUpdate struct {
	AccountID Optional[string]     `json:"account_id,omitzero"`
	Date      Optional[Date]       `json:"date,omitzero"`
	Amount    Optional[Milliunits] `json:"amount,omitzero"`
	Frequency Frequency            `json:"frequency,omitzero"`

	PayeeID Optional[string] `json:"payee_id,omitzero"`
	// PayeeName resolves or creates a payee by name when PayeeID is not
	// set; bounded at 200 characters.
	PayeeName  Optional[string] `json:"payee_name,omitzero"`
	CategoryID Optional[string] `json:"category_id,omitzero"`
	// Memo is bounded at 500 characters.
	Memo      Optional[string]    `json:"memo,omitzero"`
	FlagColor Optional[FlagColor] `json:"flag_color,omitzero"`
}

// validate applies the spec-stated invariants before any I/O; the date
// window applies only when Date is set to a value.
func (u ScheduledTransactionUpdate) validate(op string) error {
	if d, ok := u.Date.Get(); ok {
		if err := checkScheduledDate(op, d); err != nil {
			return err
		}
	}
	return errFirst(
		checkOptRuneMax(op, "payee_name", u.PayeeName, txnPayeeNameMax),
		checkOptRuneMax(op, "memo", u.Memo, memoMax),
	)
}

// Update changes a scheduled transaction. Unset fields stay unchanged
// on the server; SetNull clears.
//
// YNAB operationId: updateScheduledTransaction
func (s *ScheduledTransactionsService) Update(
	ctx context.Context, scheduledTransactionID string, update ScheduledTransactionUpdate,
) (*ScheduledTransaction, error) {
	if err := update.validate("Scheduled.Update"); err != nil {
		return nil, err
	}
	data, err := do[scheduledResult](ctx, s.plan.client,
		http.MethodPut, s.plan.path("scheduled_transactions", scheduledTransactionID), nil,
		body{"scheduled_transaction": update})
	if err != nil {
		return nil, err
	}
	return data.ScheduledTransaction, nil
}

// Delete removes a scheduled transaction and returns its final state.
//
// YNAB operationId: deleteScheduledTransaction
func (s *ScheduledTransactionsService) Delete(
	ctx context.Context, scheduledTransactionID string,
) (*ScheduledTransaction, error) {
	data, err := do[scheduledResult](ctx, s.plan.client,
		http.MethodDelete, s.plan.path("scheduled_transactions", scheduledTransactionID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.ScheduledTransaction, nil
}
