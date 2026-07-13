// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

// Package payee implements payee entities and services
package payee // import "pkg.venceslau.dev/ynab/api/payee"

// Payee represents a payee for a budget
type Payee struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Deleted Deleted payees will only be included in delta requests
	Deleted bool `json:"deleted"`

	// TransferAccountID If a transfer payee, the account_id to which this
	// payee transfers to
	TransferAccountID *string `json:"transfer_account_id"`
}

// Location represents a location for a payee
// When a transaction is saved with an specified payee on the YNAB mobile apps,
// the GPS coordinates for that location are stored, so that the next time
// the user is in the same place (like the Grocery store) YNAB can pre-populate
// nearby payees for you. Locations will not be available for all payees.
type Location struct {
	ID      string `json:"id"`
	PayeeID string `json:"payee_id"`
	// Deleted Deleted payees will only be included in delta requests
	Deleted bool `json:"deleted"`

	Latitude  *float64 `json:"latitude,string"`
	Longitude *float64 `json:"longitude,string"`
}

// SearchResultSnapshot represents a versioned snapshot for a payee search
type SearchResultSnapshot struct {
	Payees          []*Payee
	ServerKnowledge uint64
}
