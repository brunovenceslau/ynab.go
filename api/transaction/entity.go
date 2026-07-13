// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

// Package transaction implements transaction entities and services
package transaction // import "pkg.venceslau.dev/ynab/api/transaction"

import "pkg.venceslau.dev/ynab/api"

// Transaction represents a full transaction for a budget
type Transaction struct {
	ID   string   `json:"id"`
	Date api.Date `json:"date"`
	// Amount Transaction amount in milliunits format
	Amount    int64          `json:"amount"`
	Cleared   ClearingStatus `json:"cleared"`
	Approved  bool           `json:"approved"`
	AccountID string         `json:"account_id"`
	// Deleted Deleted transactions will only be included in delta requests
	Deleted         bool              `json:"deleted"`
	AccountName     string            `json:"account_name"`
	SubTransactions []*SubTransaction `json:"subtransactions"`

	Memo              *string    `json:"memo"`
	FlagColor         *FlagColor `json:"flag_color"`
	PayeeID           *string    `json:"payee_id"`
	CategoryID        *string    `json:"category_id"`
	TransferAccountID *string    `json:"transfer_account_id"`
	// ImportID If the Transaction was imported, this field is a unique (by account) import
	// identifier. If this transaction was imported through File Based Import or
	// Direct Import and not through the API, the import_id will have the format:
	// 'YNAB:[milliunit_amount]:[iso_date]:[occurrence]'. For example, a transaction
	// dated 2015-12-30 in the amount of -$294.23 USD would have an import_id of
	// 'YNAB:-294230:2015-12-30:1’. If a second transaction on the same account
	// was imported and had the same date and same amount, its import_id would
	// be 'YNAB:-294230:2015-12-30:2’.
	ImportID     *string `json:"import_id"`
	PayeeName    *string `json:"payee_name"`
	CategoryName *string `json:"category_name"`
}

// Summary represents the summary of a transaction for a budget
type Summary struct {
	ID   string   `json:"id"`
	Date api.Date `json:"date"`
	// Amount Transaction amount in milliunits format
	Amount    int64          `json:"amount"`
	Cleared   ClearingStatus `json:"cleared"`
	Approved  bool           `json:"approved"`
	AccountID string         `json:"account_id"`
	// Deleted Deleted transactions will only be included in delta requests
	Deleted bool `json:"deleted"`

	Memo              *string    `json:"memo"`
	FlagColor         *FlagColor `json:"flag_color"`
	PayeeID           *string    `json:"payee_id"`
	CategoryID        *string    `json:"category_id"`
	TransferAccountID *string    `json:"transfer_account_id"`

	// ImportID If the Transaction was imported, this field is a unique (by account) import
	// identifier. If this transaction was imported through File Based Import or
	// Direct Import and not through the API, the import_id will have the format:
	// 'YNAB:[milliunit_amount]:[iso_date]:[occurrence]'. For example, a transaction
	// dated 2015-12-30 in the amount of -$294.23 USD would have an import_id of
	// 'YNAB:-294230:2015-12-30:1’. If a second transaction on the same account
	// was imported and had the same date and same amount, its import_id would
	// be 'YNAB:-294230:2015-12-30:2’.
	ImportID *string `json:"import_id"`
}

// SubTransaction represents a sub-transaction for a transaction
type SubTransaction struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	// Amount sub-transaction amount in milliunits format
	Amount int64 `json:"amount"`
	// Deleted Deleted sub-transactions will only be included in delta requests.
	Deleted bool `json:"deleted"`

	Memo       *string `json:"memo"`
	PayeeID    *string `json:"payee_id"`
	CategoryID *string `json:"category_id"`
	// TransferAccountID If a transfer, the account_id which the
	// sub-transaction transfers to
	TransferAccountID *string `json:"transfer_account_id"`
}

// Hybrid represents a hybrid transaction
type Hybrid struct {
	ID   string   `json:"id"`
	Date api.Date `json:"date"`
	// Amount Transaction amount in milliunits format
	Amount      int64          `json:"amount"`
	Cleared     ClearingStatus `json:"cleared"`
	Approved    bool           `json:"approved"`
	AccountID   string         `json:"account_id"`
	AccountName string         `json:"account_name"`
	// Deleted Deleted transactions will only be included in delta requests
	Deleted bool `json:"deleted"`
	Type    Type `json:"type"`

	Memo              *string    `json:"memo"`
	FlagColor         *FlagColor `json:"flag_color"`
	PayeeID           *string    `json:"payee_id"`
	CategoryID        *string    `json:"category_id"`
	TransferAccountID *string    `json:"transfer_account_id"`
	// ImportID If the Transaction was imported, this field is a unique (by account) import
	// identifier. If this transaction was imported through File Based Import or
	// Direct Import and not through the API, the import_id will have the format:
	// 'YNAB:[milliunit_amount]:[iso_date]:[occurrence]'. For example, a transaction
	// dated 2015-12-30 in the amount of -$294.23 USD would have an import_id of
	// 'YNAB:-294230:2015-12-30:1’. If a second transaction on the same account
	// was imported and had the same date and same amount, its import_id would
	// be 'YNAB:-294230:2015-12-30:2’.
	ImportID *string `json:"import_id"`
	// ParentTransactionID For subtransaction types, this is the id of the parent transaction
	// For transaction types, this id will be always be null
	ParentTransactionID *string `json:"parent_transaction_id"`
	PayeeName           *string `json:"payee_name"`
	CategoryName        *string `json:"category_name"`
}

// Scheduled represents a scheduled transaction for a budget
type Scheduled struct {
	ID        string             `json:"id"`
	DateFirst api.Date           `json:"date_first"`
	DateNext  api.Date           `json:"date_next"`
	Frequency ScheduledFrequency `json:"frequency"`
	// Amount The scheduled transaction amount in milliunits format
	Amount    int64  `json:"amount"`
	AccountID string `json:"account_id"`
	// Deleted Deleted scheduled transactions will only be included in delta requests.
	Deleted         bool                       `json:"deleted"`
	AccountName     string                     `json:"account_name"`
	SubTransactions []*ScheduledSubTransaction `json:"subtransactions"`

	Memo       *string    `json:"memo"`
	FlagColor  *FlagColor `json:"flag_color"`
	PayeeID    *string    `json:"payee_id"`
	CategoryID *string    `json:"category_id"`
	// TransferAccountID If a transfer, the account_id which the scheduled
	// transaction transfers to
	TransferAccountID *string `json:"transfer_account_id"`
	PayeeName         *string `json:"payee_name"`
	CategoryName      *string `json:"category_name"`
}

// ScheduledSummary represents the summary of a scheduled transaction for a budget
type ScheduledSummary struct {
	ID        string             `json:"id"`
	DateFirst api.Date           `json:"date_first"`
	DateNext  api.Date           `json:"date_next"`
	Frequency ScheduledFrequency `json:"frequency"`
	// Amount The scheduled transaction amount in milliunits format
	Amount    int64  `json:"amount"`
	AccountID string `json:"account_id"`
	// Deleted Deleted scheduled transactions will only be included in delta requests.
	Deleted bool `json:"deleted"`

	Memo       *string    `json:"memo"`
	FlagColor  *FlagColor `json:"flag_color"`
	PayeeID    *string    `json:"payee_id"`
	CategoryID *string    `json:"category_id"`
	// TransferAccountID If a transfer, the account_id which the scheduled
	// transaction transfers to
	TransferAccountID *string `json:"transfer_account_id"`
}

// ScheduledSubTransaction represents a scheduled sub-transaction for
// a scheduled transaction
type ScheduledSubTransaction struct {
	ID                     string `json:"id"`
	ScheduledTransactionID string `json:"scheduled_transaction_id"`
	// Amount The scheduled sub-transaction amount in milliunits format
	Amount int64 `json:"amount"`
	// Deleted Deleted scheduled sub-transactions will only be included in delta requests
	Deleted bool `json:"deleted"`

	Memo       *string `json:"memo"`
	PayeeID    *string `json:"payee_id"`
	CategoryID *string `json:"category_id"`
	// TransferAccountID If a transfer, the account_id which the scheduled
	// subtransaction transfers to
	TransferAccountID *string `json:"transfer_account_id"`
}

// Bulk represents the output of transactions being created in bulk mode
// Deprecated: Use BulkTransactions instead.
type Bulk struct {
	// TransactionIDs The list of Transaction IDs that were created
	TransactionIDs []string `json:"transaction_ids"`
	// DuplicateImportIDs If any Transactions were not created because they had an
	// import ID matching a transaction already on the same account, the
	// specified import IDs will be included in this list
	DuplicateImportIDs []string `json:"duplicate_import_ids"`
}

// OperationSummary represents the output of transactions being created
type OperationSummary struct {
	// TransactionIDs The list of Transaction IDs that were created
	TransactionIDs []string `json:"transaction_ids"`
	// DuplicateImportIDs If any Transactions were not created because they had an
	// import ID matching a transaction already on the same account, the
	// specified import IDs will be included in this list
	DuplicateImportIDs []string `json:"duplicate_import_ids"`
	// Transactions If multiple transactions were specified, the transactions that were saved
	Transactions []*Transaction `json:"transactions"`
	// Transactions If a single transaction was specified, the transaction that was saved
	Transaction *Transaction `json:"transaction"`
}
