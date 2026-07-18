package ynab

import (
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
	}
	return false
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
	}
	return false
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
	}
	return false
}

// TransactionSummaryBase is the transaction shape shared by the
// transaction endpoints and the full-plan export collections.
type TransactionSummaryBase struct {
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
	TransactionSummaryBase
	AmountFormatted string           `json:"amount_formatted"`
	AmountCurrency  float64          `json:"amount_currency"`
	AccountName     string           `json:"account_name"`
	PayeeName       *string          `json:"payee_name"`
	CategoryName    *string          `json:"category_name"` // "Split" on split transactions
	Subtransactions []Subtransaction `json:"subtransactions"`
}

// SyncID keys the transaction for MergeByID.
func (t Transaction) SyncID() string { return t.ID }

// IsDeleted reports a delta tombstone.
func (t Transaction) IsDeleted() bool { return t.Deleted }

// SubTransactionBase is the split-leg shape shared by the transaction
// endpoints and the full-plan export collections.
type SubTransactionBase struct {
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
	SubTransactionBase
	AmountFormatted string  `json:"amount_formatted"`
	AmountCurrency  float64 `json:"amount_currency"`
}

// SyncID keys the subtransaction for MergeByID.
func (s Subtransaction) SyncID() string { return s.ID }

// IsDeleted reports a delta tombstone.
func (s Subtransaction) IsDeleted() bool { return s.Deleted }

// HybridTransaction is a row of the category/payee transaction lists:
// either a regular transaction or one leg of a split, per Type.
type HybridTransaction struct {
	TransactionSummaryBase
	AmountFormatted     string     `json:"amount_formatted"`
	AmountCurrency      float64    `json:"amount_currency"`
	Type                HybridType `json:"type"`
	ParentTransactionID *string    `json:"parent_transaction_id"` // set only for subtransaction rows
	AccountName         string     `json:"account_name"`
	PayeeName           *string    `json:"payee_name"`
	CategoryName        string     `json:"category_name"`
}

// SyncID keys the hybrid row for MergeByID.
func (t HybridTransaction) SyncID() string { return t.ID }

// IsDeleted reports a delta tombstone.
func (t HybridTransaction) IsDeleted() bool { return t.Deleted }

// TransactionFilter tunes the transaction list endpoints. The zero value
// means unfiltered; each set field encodes its exact query parameter.
// The server defaults SinceDate to one year ago when unset.
type TransactionFilter struct {
	SinceDate Date
	UntilDate Date
	Type      TransactionType
	Since     ServerKnowledge
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
