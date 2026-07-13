// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package transaction

import (
	"encoding/json"
	"fmt"
	"strings"

	"pkg.venceslau.dev/ynab/api"
)

// NewService facilitates the creation of a new transaction service instance
func NewService(c api.ClientReaderWriter) *Service {
	return &Service{c}
}

// Service wraps YNAB transaction API endpoints
type Service struct {
	c api.ClientReaderWriter
}

// GetTransactions fetches the list of transactions from
// a budget with filtering capabilities
// https://api.youneedabudget.com/v1#/Transactions/getTransactions
func (s *Service) GetTransactions(budgetID string, f *Filter) ([]*Transaction, error) {
	resModel := struct {
		Data struct {
			Transactions []*Transaction `json:"transactions"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions", budgetID)
	if f != nil {
		url = fmt.Sprintf("%s?%s", url, f.ToQuery())
	}

	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return resModel.Data.Transactions, nil
}

// GetTransaction fetches a specific transaction from a budget
// https://api.youneedabudget.com/v1#/Transactions/getTransactionsById
func (s *Service) GetTransaction(budgetID, transactionID string) (*Transaction, error) {
	resModel := struct {
		Data struct {
			Transaction *Transaction `json:"transaction"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions/%s", budgetID, transactionID)
	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}
	return resModel.Data.Transaction, nil
}

// CreateTransaction creates a new transaction for a budget
// https://api.youneedabudget.com/v1#/Transactions/createTransaction
func (s *Service) CreateTransaction(budgetID string,
	p PayloadTransaction) (*OperationSummary, error) {

	return s.CreateTransactions(budgetID, []PayloadTransaction{p})
}

// CreateTransactions creates one or more new transactions for a budget
// https://api.youneedabudget.com/v1#/Transactions/createTransaction
func (s *Service) CreateTransactions(budgetID string,
	p []PayloadTransaction) (*OperationSummary, error) {

	payload := struct {
		Transactions []PayloadTransaction `json:"transactions"`
	}{
		p,
	}

	buf, err := json.Marshal(&payload)
	if err != nil {
		return nil, err
	}

	resModel := struct {
		Data *OperationSummary `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions", budgetID)
	err = s.c.POST(url, &resModel, buf)
	if err != nil {
		return nil, err
	}
	return resModel.Data, nil
}

// BulkCreateTransactions creates multiple transactions for a budget
// https://api.youneedabudget.com/v1#/Transactions/bulkCreateTransactions
// Deprecated: Use transaction.CreateTransactions instead.
func (s *Service) BulkCreateTransactions(budgetID string,
	ps []PayloadTransaction) (*Bulk, error) {

	payload := struct {
		Transactions []PayloadTransaction `json:"transactions"`
	}{
		ps,
	}

	buf, err := json.Marshal(&payload)
	if err != nil {
		return nil, err
	}

	resModel := struct {
		Data struct {
			Bulk *Bulk `json:"bulk"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions/bulk", budgetID)
	if err := s.c.POST(url, &resModel, buf); err != nil {
		return nil, err
	}
	return resModel.Data.Bulk, nil
}

// UpdateTransaction updates a whole transaction for a replacement
// https://api.youneedabudget.com/v1#/Transactions/updateTransaction
func (s *Service) UpdateTransaction(budgetID, transactionID string,
	p PayloadTransaction) (*Transaction, error) {

	payload := struct {
		Transaction *PayloadTransaction `json:"transaction"`
	}{
		&p,
	}

	buf, err := json.Marshal(&payload)
	if err != nil {
		return nil, err
	}

	resModel := struct {
		Data struct {
			Transaction *Transaction `json:"transaction"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions/%s", budgetID, transactionID)
	if err := s.c.PUT(url, &resModel, buf); err != nil {
		return nil, err
	}
	return resModel.Data.Transaction, nil
}

// UpdateTransactions creates one or more new transactions for a budget
// https://api.youneedabudget.com/v1#/Transactions/updateTransactions
func (s *Service) UpdateTransactions(budgetID string,
	p []PayloadTransaction) (*OperationSummary, error) {

	payload := struct {
		Transactions []PayloadTransaction `json:"transactions"`
	}{
		p,
	}

	buf, err := json.Marshal(&payload)
	if err != nil {
		return nil, err
	}

	resModel := struct {
		Data *OperationSummary `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions", budgetID)
	err = s.c.PATCH(url, &resModel, buf)
	if err != nil {
		return nil, err
	}
	return resModel.Data, nil
}

// DeleteTransaction deletes a transaction from a budget
// https://api.youneedabudget.com/v1#/Transactions/deleteTransaction
func (s *Service) DeleteTransaction(budgetID, transactionID string) (*Transaction, error) {
	resModel := struct {
		Data struct {
			Transaction *Transaction `json:"transaction"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/transactions/%s", budgetID, transactionID)
	err := s.c.DELETE(url, &resModel)
	if err != nil {
		return nil, err
	}
	return resModel.Data.Transaction, nil
}

// GetTransactionsByAccount fetches the list of transactions of a specific account
// from a budget with filtering capabilities
// https://api.youneedabudget.com/v1#/Transactions/getTransactionsByAccount
func (s *Service) GetTransactionsByAccount(budgetID, accountID string,
	f *Filter) ([]*Transaction, error) {

	resModel := struct {
		Data struct {
			Transactions []*Transaction `json:"transactions"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/accounts/%s/transactions", budgetID, accountID)
	if f != nil {
		url = fmt.Sprintf("%s?%s", url, f.ToQuery())
	}

	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return resModel.Data.Transactions, nil
}

// GetTransactionsByCategory fetches the list of transactions of a specific category
// from a budget with filtering capabilities
// https://api.youneedabudget.com/v1#/Transactions/getTransactionsByCategory
func (s *Service) GetTransactionsByCategory(budgetID, categoryID string,
	f *Filter) ([]*Hybrid, error) {

	resModel := struct {
		Data struct {
			Transactions []*Hybrid `json:"transactions"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/categories/%s/transactions", budgetID, categoryID)
	if f != nil {
		url = fmt.Sprintf("%s?%s", url, f.ToQuery())
	}

	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return resModel.Data.Transactions, nil
}

// GetTransactionsByPayee fetches the list of transactions of a specific payee
// from a budget with filtering capabilities
// https://api.youneedabudget.com/v1#/Transactions/getTransactionsByPayee
func (s *Service) GetTransactionsByPayee(budgetID, payeeID string,
	f *Filter) ([]*Hybrid, error) {

	resModel := struct {
		Data struct {
			Transactions []*Hybrid `json:"transactions"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/payees/%s/transactions", budgetID, payeeID)
	if f != nil {
		url = fmt.Sprintf("%s?%s", url, f.ToQuery())
	}

	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return resModel.Data.Transactions, nil
}

// GetScheduledTransactions fetches the list of scheduled transactions from
// a budget
// https://api.youneedabudget.com/v1#/Scheduled_Transactions/getScheduledTransactions
func (s *Service) GetScheduledTransactions(budgetID string) ([]*Scheduled, error) {
	resModel := struct {
		Data struct {
			ScheduledTransactions []*Scheduled `json:"scheduled_transactions"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/scheduled_transactions", budgetID)
	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return resModel.Data.ScheduledTransactions, nil
}

// GetScheduledTransaction fetches a specific scheduled transaction from a budget
// https://api.youneedabudget.com/v1#/Scheduled_Transactions/getScheduledTransactionById
func (s *Service) GetScheduledTransaction(budgetID, scheduledTransactionID string) (*Scheduled, error) {
	resModel := struct {
		Data struct {
			ScheduledTransactions *Scheduled `json:"scheduled_transaction"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/scheduled_transactions/%s", budgetID, scheduledTransactionID)
	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}
	return resModel.Data.ScheduledTransactions, nil
}

// Filter represents the optional filter while fetching transactions
type Filter struct {
	Since *api.Date
	Type  *Status
}

// ToQuery returns the filters as a HTTP query string
func (f *Filter) ToQuery() string {
	pairs := make([]string, 0, 2)
	if f.Since != nil && !f.Since.IsZero() {
		pairs = append(pairs, fmt.Sprintf("since_date=%s",
			api.DateFormat(*f.Since)))
	}
	if f.Type != nil {
		pairs = append(pairs, fmt.Sprintf("type=%s", string(*f.Type)))
	}
	return strings.Join(pairs, "&")
}
