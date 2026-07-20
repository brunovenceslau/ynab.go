// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// ClearedStatus is a transaction's cleared state.
type ClearedStatus string

// The three wire cleared states.
const (
	ClearedStatusUncleared  ClearedStatus = "uncleared"
	ClearedStatusCleared    ClearedStatus = "cleared"
	ClearedStatusReconciled ClearedStatus = "reconciled"
)

// Valid reports whether s is one of the documented wire values.
func (s ClearedStatus) Valid() bool {
	switch s {
	case ClearedStatusUncleared, ClearedStatusCleared, ClearedStatusReconciled:
		return true
	default:
		return false
	}
}

// FlagColor is a transaction flag. FlagColorNone ("") is the one enum
// whose zero value is valid — "no flag" on writes. Read models use
// *FlagColor because the wire distinguishes null from a value.
type FlagColor string

// The wire flag colors. FlagColorNone clears the flag on writes.
const (
	FlagColorNone   FlagColor = ""
	FlagColorRed    FlagColor = "red"
	FlagColorOrange FlagColor = "orange"
	FlagColorYellow FlagColor = "yellow"
	FlagColorGreen  FlagColor = "green"
	FlagColorBlue   FlagColor = "blue"
	FlagColorPurple FlagColor = "purple"
)

// Valid reports whether c is one of the documented wire values (the six
// colors or the valid zero FlagColorNone).
func (c FlagColor) Valid() bool {
	switch c {
	case FlagColorNone, FlagColorRed, FlagColorOrange, FlagColorYellow,
		FlagColorGreen, FlagColorBlue, FlagColorPurple:
		return true
	default:
		return false
	}
}

// TransactionType filters transaction lists server-side.
type TransactionType string

// The two wire filter types.
const (
	TransactionTypeUncategorized TransactionType = "uncategorized"
	TransactionTypeUnapproved    TransactionType = "unapproved"
)

// Valid reports whether t is one of the documented wire values.
func (t TransactionType) Valid() bool {
	return t == TransactionTypeUncategorized || t == TransactionTypeUnapproved
}

// HybridType tells whether a hybrid row is a transaction or one leg of a
// split.
type HybridType string

// The two wire hybrid types.
const (
	HybridTypeTransaction    HybridType = "transaction"
	HybridTypeSubtransaction HybridType = "subtransaction"
)

// Valid reports whether t is one of the documented wire values.
func (t HybridType) Valid() bool {
	return t == HybridTypeTransaction || t == HybridTypeSubtransaction
}

// DebtTransactionType is the read-only kind of a debt/loan account
// transaction; null off debt accounts.
type DebtTransactionType string

// The eight wire debt transaction types.
const (
	DebtTransactionTypePayment           DebtTransactionType = "payment"
	DebtTransactionTypeRefund            DebtTransactionType = "refund"
	DebtTransactionTypeFee               DebtTransactionType = "fee"
	DebtTransactionTypeInterest          DebtTransactionType = "interest"
	DebtTransactionTypeEscrow            DebtTransactionType = "escrow"
	DebtTransactionTypeBalanceAdjustment DebtTransactionType = "balanceAdjustment"
	DebtTransactionTypeCredit            DebtTransactionType = "credit"
	DebtTransactionTypeCharge            DebtTransactionType = "charge"
)

// Valid reports whether t is one of the documented wire values.
func (t DebtTransactionType) Valid() bool {
	switch t {
	case DebtTransactionTypePayment, DebtTransactionTypeRefund,
		DebtTransactionTypeFee, DebtTransactionTypeInterest,
		DebtTransactionTypeEscrow, DebtTransactionTypeBalanceAdjustment,
		DebtTransactionTypeCredit, DebtTransactionTypeCharge:
		return true
	default:
		return false
	}
}

// TransactionBase is the transaction shape shared by the
// transaction endpoints and the full-plan export collections.
type TransactionBase struct {
	ID                      string               `json:"id"`
	Date                    Date                 `json:"date"`
	Amount                  Milliunits           `json:"amount"`
	Memo                    *string              `json:"memo"`
	Cleared                 ClearedStatus        `json:"cleared"`
	Approved                bool                 `json:"approved"`
	FlagColor               *FlagColor           `json:"flag_color"`
	FlagName                *string              `json:"flag_name"`
	AccountID               string               `json:"account_id"`
	PayeeID                 *string              `json:"payee_id"`
	CategoryID              *string              `json:"category_id"`
	TransferAccountID       *string              `json:"transfer_account_id"`
	TransferTransactionID   *string              `json:"transfer_transaction_id"`
	MatchedTransactionID    *string              `json:"matched_transaction_id"`
	ImportID                *string              `json:"import_id"`
	ImportPayeeName         *string              `json:"import_payee_name"`
	ImportPayeeNameOriginal *string              `json:"import_payee_name_original"`
	DebtTransactionType     *DebtTransactionType `json:"debt_transaction_type"`
	Deleted                 bool                 `json:"deleted"`
}

// Transaction is a full transaction with its split legs. The
// *_formatted/*_currency companions are computed, read-only display
// fields — do money math on the milliunit integers.
type Transaction struct {
	TransactionBase
	AmountFormatted string           `json:"amount_formatted"`
	AmountCurrency  float64          `json:"amount_currency"`
	AccountName     string           `json:"account_name"`
	PayeeName       *string          `json:"payee_name"`
	CategoryName    *string          `json:"category_name"` // "Split" on split transactions
	Subtransactions []Subtransaction `json:"subtransactions"`
}

// SyncID keys the transaction for MergeByID. Transaction and
// HybridTransaction inherit the adapters by embedding.
func (t TransactionBase) SyncID() string { return t.ID }

// IsDeleted reports a delta tombstone.
func (t TransactionBase) IsDeleted() bool { return t.Deleted }

// SubtransactionBase is the split-leg shape shared by the transaction
// endpoints and the full-plan export collections.
type SubtransactionBase struct {
	ID                    string     `json:"id"`
	TransactionID         string     `json:"transaction_id"`
	Amount                Milliunits `json:"amount"`
	Memo                  *string    `json:"memo"`
	PayeeID               *string    `json:"payee_id"`
	PayeeName             *string    `json:"payee_name"`
	CategoryID            *string    `json:"category_id"`
	CategoryName          *string    `json:"category_name"`
	TransferAccountID     *string    `json:"transfer_account_id"`
	TransferTransactionID *string    `json:"transfer_transaction_id"`
	Deleted               bool       `json:"deleted"`
}

// Subtransaction is one leg of a split transaction.
type Subtransaction struct {
	SubtransactionBase
	AmountFormatted string  `json:"amount_formatted"`
	AmountCurrency  float64 `json:"amount_currency"`
}

// SyncID keys the subtransaction for MergeByID. Subtransaction inherits
// the adapters by embedding.
func (s SubtransactionBase) SyncID() string { return s.ID }

// IsDeleted reports a delta tombstone.
func (s SubtransactionBase) IsDeleted() bool { return s.Deleted }

// HybridTransaction is a row of the category/payee transaction lists:
// either a regular transaction or one leg of a split, per Type.
type HybridTransaction struct {
	TransactionBase
	AmountFormatted     string     `json:"amount_formatted"`
	AmountCurrency      float64    `json:"amount_currency"`
	Type                HybridType `json:"type"`
	ParentTransactionID *string    `json:"parent_transaction_id"` // set only for subtransaction rows
	AccountName         string     `json:"account_name"`
	PayeeName           *string    `json:"payee_name"`
	CategoryName        string     `json:"category_name"`
}

// TransactionFilter tunes the transaction list endpoints. The zero value
// means unfiltered; each set field encodes its exact query parameter.
// The server defaults SinceDate to one year ago when unset.
type TransactionFilter struct {
	SinceDate Date
	UntilDate Date
	Type      TransactionType
	Since     ServerKnowledge
}

// TransactionsService reads and writes the plan's transactions.
type TransactionsService struct {
	plan *Plan
}

// transactionsResult is the shared full-transaction list payload.
type transactionsResult struct {
	Transactions    []Transaction   `json:"transactions"`
	ServerKnowledge ServerKnowledge `json:"server_knowledge"`
}

// hybridResult is the hybrid list payload. Server knowledge is optional
// on this wire shape.
type hybridResult struct {
	Transactions    []HybridTransaction `json:"transactions"`
	ServerKnowledge ServerKnowledge     `json:"server_knowledge"`
}

// List returns the plan's transactions. The server defaults the filter's
// SinceDate to one year ago when unset.
//
// YNAB operationId: getTransactions
func (s *TransactionsService) List(
	ctx context.Context, filter TransactionFilter,
) ([]Transaction, ServerKnowledge, error) {
	data, err := do[transactionsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("transactions"), filter.encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transactions, data.ServerKnowledge, nil
}

// ListByAccount returns one account's transactions.
//
// YNAB operationId: getTransactionsByAccount
func (s *TransactionsService) ListByAccount(
	ctx context.Context, accountID string, filter TransactionFilter,
) ([]Transaction, ServerKnowledge, error) {
	data, err := do[transactionsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("accounts", accountID, "transactions"), filter.encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transactions, data.ServerKnowledge, nil
}

// ListByCategory returns one category's transactions as hybrid rows
// (split legs appear individually). Wire quirk, documented rather than
// papered over: server knowledge is optional on this response and may
// come back 0 — do not advance a cursor from a zero.
//
// YNAB operationId: getTransactionsByCategory
func (s *TransactionsService) ListByCategory(
	ctx context.Context, categoryID string, filter TransactionFilter,
) ([]HybridTransaction, ServerKnowledge, error) {
	data, err := do[hybridResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("categories", categoryID, "transactions"), filter.encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transactions, data.ServerKnowledge, nil
}

// ListByPayee returns one payee's transactions as hybrid rows. The same
// optional-server-knowledge quirk as ListByCategory applies.
//
// YNAB operationId: getTransactionsByPayee
func (s *TransactionsService) ListByPayee(
	ctx context.Context, payeeID string, filter TransactionFilter,
) ([]HybridTransaction, ServerKnowledge, error) {
	data, err := do[hybridResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("payees", payeeID, "transactions"), filter.encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transactions, data.ServerKnowledge, nil
}

// ListByMonth returns one month's transactions. Month accepts
// CurrentMonth.
//
// YNAB operationId: getTransactionsByMonth
func (s *TransactionsService) ListByMonth(
	ctx context.Context, m Month, filter TransactionFilter,
) ([]Transaction, ServerKnowledge, error) {
	if m.IsZero() {
		return nil, 0, zeroMonthError("Transactions.ListByMonth")
	}
	data, err := do[transactionsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("months", m.String(), "transactions"), filter.encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transactions, data.ServerKnowledge, nil
}

// transactionResult is the single-transaction payload.
type transactionResult struct {
	Transaction     *Transaction    `json:"transaction"`
	ServerKnowledge ServerKnowledge `json:"server_knowledge"`
}

// Get returns a single transaction by id. A missing id answers
// [ErrResourceNotFound].
//
// YNAB operationId: getTransactionById
func (s *TransactionsService) Get(ctx context.Context, transactionID string) (*Transaction, ServerKnowledge, error) {
	data, err := do[transactionResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("transactions", transactionID), nil, nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transaction, data.ServerKnowledge, nil
}

// TransactionSpec is the payload for TransactionsService.Create and
// CreateBatch. AccountID, Date, and Amount are required. A split
// transaction sets CategoryID to SetNull and lists its legs in Splits —
// the legs' amounts must sum exactly to Amount (SplitEven always
// satisfies this).
type TransactionSpec struct {
	AccountID string     `json:"account_id"`
	Date      Date       `json:"date"`
	Amount    Milliunits `json:"amount"`

	PayeeID Optional[string] `json:"payee_id,omitzero"`
	// PayeeName resolves or creates a payee by name when PayeeID is not
	// set; bounded at 200 characters.
	PayeeName  Optional[string] `json:"payee_name,omitzero"`
	CategoryID Optional[string] `json:"category_id,omitzero"`
	// Memo is bounded at 500 characters.
	Memo      Optional[string]    `json:"memo,omitzero"`
	Cleared   ClearedStatus       `json:"cleared,omitzero"`
	Approved  Optional[bool]      `json:"approved,omitzero"`
	FlagColor Optional[FlagColor] `json:"flag_color,omitzero"`
	// ImportID marks the transaction imported and deduplicates by
	// (account, import_id); bounded at 36 characters. Convention:
	// "YNAB:<milliunits>:<date>:<n>".
	ImportID Optional[string] `json:"import_id,omitzero"`

	Splits []SubtransactionSpec `json:"subtransactions,omitzero"`
}

// SubtransactionSpec is one leg of a split in TransactionSpec.
type SubtransactionSpec struct {
	Amount     Milliunits       `json:"amount"`
	PayeeID    Optional[string] `json:"payee_id,omitzero"`
	PayeeName  Optional[string] `json:"payee_name,omitzero"`
	CategoryID Optional[string] `json:"category_id,omitzero"`
	Memo       Optional[string] `json:"memo,omitzero"`
}

// Spec-declared transaction write bounds. (The payee-name bound here is
// the transaction field's 200, distinct from the payee entity's 500.)
const (
	importIDMax             = 36
	transactionPayeeNameMax = 200
	memoMax                 = 500
)

// validate applies the spec-stated invariants — and only those — before
// any I/O.
func (s TransactionSpec) validate(op string) error {
	if err := errFirst(
		checkOptRuneMax(op, "import_id", s.ImportID, importIDMax),
		checkOptRuneMax(op, "payee_name", s.PayeeName, transactionPayeeNameMax),
		checkOptRuneMax(op, "memo", s.Memo, memoMax),
	); err != nil {
		return err
	}
	if len(s.Splits) == 0 {
		return nil
	}

	var sum Milliunits
	for i, leg := range s.Splits {
		if err := errFirst(
			checkOptRuneMax(op, "payee_name", leg.PayeeName, transactionPayeeNameMax),
			checkOptRuneMax(op, "memo", leg.Memo, memoMax),
		); err != nil {
			var argErr *ArgumentError
			if errors.As(err, &argErr) {
				argErr.Reason += " (split " + strconv.Itoa(i) + ")"
			}
			return err
		}
		sum = sum.Add(leg.Amount)
	}
	if sum != s.Amount {
		return &ArgumentError{Op: op, Reason: "split amounts must sum exactly to Amount"}
	}
	return nil
}

// errFirst returns the first non-nil error.
func errFirst(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// validate applies the update payload's spec-stated bounds — the same
// maxLengths the create spec declares.
func (u TransactionUpdate) validate(op string) error {
	return errFirst(
		checkOptRuneMax(op, "payee_name", u.PayeeName, transactionPayeeNameMax),
		checkOptRuneMax(op, "memo", u.Memo, memoMax),
	)
}

// BatchResult is what CreateBatch and UpdateBatch return.
// DuplicateImportIDs lists the
// import_ids skipped because the same (account, import_id) already
// existed — a batch-level answer where the single Create returns 409.
type BatchResult struct {
	Transactions       []Transaction   `json:"transactions"`
	TransactionIDs     []string        `json:"transaction_ids"`
	DuplicateImportIDs []string        `json:"duplicate_import_ids"`
	ServerKnowledge    ServerKnowledge `json:"server_knowledge"`
}

// Create adds one transaction (HTTP 201). A duplicate import_id on the
// account answers 409 — errors.Is(err, ErrConflict) — unlike CreateBatch,
// which reports duplicates in BatchResult.DuplicateImportIDs.
//
// YNAB operationId: createTransaction
func (s *TransactionsService) Create(
	ctx context.Context, spec TransactionSpec,
) (*Transaction, ServerKnowledge, error) {
	if err := spec.validate("Transactions.Create"); err != nil {
		return nil, 0, err
	}
	data, err := do[transactionResult](ctx, s.plan.client,
		http.MethodPost, s.plan.path("transactions"), nil, body{"transaction": spec})
	if err != nil {
		return nil, 0, err
	}
	return data.Transaction, data.ServerKnowledge, nil
}

// CreateBatch adds several transactions in one request (HTTP 201).
// Duplicate import_ids do not fail the call: they come back in
// BatchResult.DuplicateImportIDs with a nil error.
//
// YNAB operationId: createTransaction
func (s *TransactionsService) CreateBatch(ctx context.Context, specs []TransactionSpec) (*BatchResult, error) {
	for i, spec := range specs {
		if err := spec.validate("Transactions.CreateBatch"); err != nil {
			var argErr *ArgumentError
			if errors.As(err, &argErr) {
				argErr.Reason += " (spec " + strconv.Itoa(i) + ")"
			}
			return nil, err
		}
	}
	data, err := do[BatchResult](ctx, s.plan.client,
		http.MethodPost, s.plan.path("transactions"), nil, body{"transactions": specs})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// TransactionUpdate is the partial payload for TransactionsService.Update
// and the patches built by PatchByID/PatchByImportID. Unset fields stay
// unchanged on the server; SetNull clears.
type TransactionUpdate struct {
	AccountID  Optional[string]     `json:"account_id,omitzero"`
	Date       Optional[Date]       `json:"date,omitzero"`
	Amount     Optional[Milliunits] `json:"amount,omitzero"`
	PayeeID    Optional[string]     `json:"payee_id,omitzero"`
	PayeeName  Optional[string]     `json:"payee_name,omitzero"`
	CategoryID Optional[string]     `json:"category_id,omitzero"`
	Memo       Optional[string]     `json:"memo,omitzero"`
	Cleared    ClearedStatus        `json:"cleared,omitzero"`
	Approved   Optional[bool]       `json:"approved,omitzero"`
	FlagColor  Optional[FlagColor]  `json:"flag_color,omitzero"`
}

// TransactionPatch is one element of an UpdateBatch. Its identity is
// exactly one of transaction id or import id — both unexported, so the
// XOR holds by construction: build patches only with PatchByID or
// PatchByImportID.
type TransactionPatch struct {
	id       string
	importID string
	TransactionUpdate
}

// PatchByID addresses a batch update by transaction id.
func PatchByID(id string, update TransactionUpdate) TransactionPatch {
	return TransactionPatch{id: id, TransactionUpdate: update}
}

// PatchByImportID addresses a batch update by import id (lookup only —
// changing an import id is not allowed by the API).
func PatchByImportID(importID string, update TransactionUpdate) TransactionPatch {
	return TransactionPatch{importID: importID, TransactionUpdate: update}
}

// MarshalJSON emits the update fields plus exactly one identity key.
func (p TransactionPatch) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(p.TransactionUpdate)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	switch {
	case p.id != "":
		fields["id"], err = json.Marshal(p.id)
	case p.importID != "":
		fields["import_id"], err = json.Marshal(p.importID)
	default:
		return nil, errors.New("ynab: TransactionPatch needs an identity — use PatchByID or PatchByImportID")
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

// Update replaces the given fields of one transaction (HTTP PUT).
//
// YNAB operationId: updateTransaction
func (s *TransactionsService) Update(
	ctx context.Context, transactionID string, update TransactionUpdate,
) (*Transaction, ServerKnowledge, error) {
	if err := update.validate("Transactions.Update"); err != nil {
		return nil, 0, err
	}
	data, err := do[transactionResult](ctx, s.plan.client,
		http.MethodPut, s.plan.path("transactions", transactionID), nil, body{"transaction": update})
	if err != nil {
		return nil, 0, err
	}
	return data.Transaction, data.ServerKnowledge, nil
}

// UpdateBatch patches several transactions in one request. Each patch
// addresses its transaction by id or import id, exactly as constructed.
//
// YNAB operationId: updateTransactions
func (s *TransactionsService) UpdateBatch(ctx context.Context, patches []TransactionPatch) (*BatchResult, error) {
	for i, p := range patches {
		if p.id == "" && p.importID == "" {
			return nil, &ArgumentError{
				Op: "Transactions.UpdateBatch", Field: "id",
				Reason: "patch " + strconv.Itoa(i) + " has an empty identity — use PatchByID or PatchByImportID",
			}
		}
		if err := p.validate("Transactions.UpdateBatch"); err != nil {
			return nil, err
		}
	}
	data, err := do[BatchResult](ctx, s.plan.client,
		http.MethodPatch, s.plan.path("transactions"), nil, body{"transactions": patches})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// Delete removes a transaction and returns its final state with the new
// server knowledge.
//
// YNAB operationId: deleteTransaction
func (s *TransactionsService) Delete(ctx context.Context, transactionID string) (*Transaction, ServerKnowledge, error) {
	data, err := do[transactionResult](ctx, s.plan.client,
		http.MethodDelete, s.plan.path("transactions", transactionID), nil, nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Transaction, data.ServerKnowledge, nil
}

// Import asks the server to import available linked-account transactions
// (bodiless POST). The ids of newly imported transactions are returned;
// an empty slice simply means nothing was waiting — the wire's 200-empty
// vs 201-with-ids split folds into len(ids).
//
// YNAB operationId: importTransactions
func (s *TransactionsService) Import(ctx context.Context) ([]string, error) {
	data, err := do[struct {
		TransactionIDs []string `json:"transaction_ids"`
	}](ctx, s.plan.client, http.MethodPost, s.plan.path("transactions", "import"), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.TransactionIDs, nil
}

// encode renders the filter's query parameters, nil when unfiltered.
func (f TransactionFilter) encode() url.Values {
	var q url.Values
	set := func(k, v string) {
		if q == nil {
			q = url.Values{}
		}
		q.Set(k, v)
	}
	if !f.SinceDate.IsZero() {
		set("since_date", f.SinceDate.String())
	}
	if !f.UntilDate.IsZero() {
		set("until_date", f.UntilDate.String())
	}
	if f.Type != "" {
		set("type", string(f.Type))
	}
	if f.Since != 0 {
		set("last_knowledge_of_server", strconv.FormatInt(int64(f.Since), 10))
	}
	return q
}
