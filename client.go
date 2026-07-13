// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

// Package ynab implements the client API
package ynab // import "pkg.venceslau.dev/ynab"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"pkg.venceslau.dev/ynab/api"
	"pkg.venceslau.dev/ynab/api/account"
	"pkg.venceslau.dev/ynab/api/budget"
	"pkg.venceslau.dev/ynab/api/category"
	"pkg.venceslau.dev/ynab/api/month"
	"pkg.venceslau.dev/ynab/api/payee"
	"pkg.venceslau.dev/ynab/api/transaction"
	"pkg.venceslau.dev/ynab/api/user"
)

const apiEndpoint = "https://api.youneedabudget.com/v1"

// ClientServicer contract for a client service API
type ClientServicer interface {
	User() *user.Service
	Budget() *budget.Service
	Account() *account.Service
	Category() *category.Service
	Payee() *payee.Service
	Month() *month.Service
	Transaction() *transaction.Service
}

// NewClient facilitates the creation of a new client instance
func NewClient(accessToken string) ClientServicer {
	c := &client{
		accessToken: accessToken,
		client:      http.DefaultClient,
	}

	c.user = user.NewService(c)
	c.budget = budget.NewService(c)
	c.account = account.NewService(c)
	c.category = category.NewService(c)
	c.payee = payee.NewService(c)
	c.month = month.NewService(c)
	c.transaction = transaction.NewService(c)
	return c
}

// client API
type client struct {
	sync.Mutex

	accessToken string

	client    *http.Client

	user        *user.Service
	budget      *budget.Service
	account     *account.Service
	category    *category.Service
	payee       *payee.Service
	month       *month.Service
	transaction *transaction.Service
}

// User returns user.Service API instance
func (c *client) User() *user.Service {
	return c.user
}

// Budget returns budget.Service API instance
func (c *client) Budget() *budget.Service {
	return c.budget
}

// Account returns account.Service API instance
func (c *client) Account() *account.Service {
	return c.account
}

// Category returns category.Service API instance
func (c *client) Category() *category.Service {
	return c.category
}

// Payee returns payee.Service API instance
func (c *client) Payee() *payee.Service {
	return c.payee
}

// Month returns month.Service API instance
func (c *client) Month() *month.Service {
	return c.month
}

// Transaction returns transaction.Service API instance
func (c *client) Transaction() *transaction.Service {
	return c.transaction
}

// GET sends a GET request to the YNAB API
func (c *client) GET(url string, responseModel interface{}) error {
	return c.do(http.MethodGet, url, responseModel, nil)
}

// POST sends a POST request to the YNAB API
func (c *client) POST(url string, responseModel interface{}, requestBody []byte) error {
	return c.do(http.MethodPost, url, responseModel, requestBody)
}

// PUT sends a PUT request to the YNAB API
func (c *client) PUT(url string, responseModel interface{}, requestBody []byte) error {
	return c.do(http.MethodPut, url, responseModel, requestBody)
}

// PATCH sends a PATCH request to the YNAB API
func (c *client) PATCH(url string, responseModel interface{}, requestBody []byte) error {
	return c.do(http.MethodPatch, url, responseModel, requestBody)
}

// DELETE sends a DELETE request to the YNAB API
func (c *client) DELETE(url string, responseModel interface{}) error {
	return c.do(http.MethodDelete, url, responseModel, nil)
}

// do sends a request to the YNAB API
func (c *client) do(method, url string, responseModel interface{}, requestBody []byte) error {
	fullURL := fmt.Sprintf("%s%s", apiEndpoint, url)
	req, err := http.NewRequest(method, fullURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		response := struct {
			Error *api.Error `json:"error"`
		}{}

		if err := json.Unmarshal(body, &response); err != nil {
			// returns a forged *api.Error fore ease of use
			// because either the response body is empty or the response is
			// non compliant with YNAB's API specification
			// https://api.youneedabudget.com/#errors
			apiError := &api.Error{
				ID:     strconv.Itoa(res.StatusCode),
				Name:   "unknown_api_error",
				Detail: "Unknown API error",
			}
			return apiError
		}

		return response.Error
	}

	return json.Unmarshal(body, &responseModel)
}
