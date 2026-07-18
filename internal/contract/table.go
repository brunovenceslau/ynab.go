// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

// Package contract holds the G1 operation table — the library's frozen
// 1:1 coverage contract with the vendored OpenAPI spec — plus the scanner
// and diff machinery the contract tests run against it. It is consumed by
// tests only and takes no dependencies.
package contract

import "net/http"

// Operation is one row of the coverage table: an operationId, its wire
// shape, and the Go methods implementing it (1:N — createTransaction fans
// out to Create and CreateBatch).
type Operation struct {
	ID          string
	Method      string
	Path        string
	QueryParams []string
	GoMethods   []string
}

// SpecVersion is the pinned version of the vendored openapi.yaml. The
// contract tests assert the file still carries it, so an accidental
// update-spec run cannot slip through.
const SpecVersion = "1.86.0"

// Table returns all 44 rows of the coverage contract, transcribed from the
// frozen public surface. Query parameters mirror the vendored spec.
func Table() []Operation {
	return []Operation{
		op("getUser", get, "/user", "Client.User"),
		opQ("getPlans", get, "/plans", []string{"include_accounts"}, "Client.Plans"),
		opQ("getPlanById", get, "/plans/{plan_id}", deltaParams(), "Plan.Export"),
		op("getPlanSettingsById", get, "/plans/{plan_id}/settings", "Plan.Settings"),
		opQ("getPlanMonths", get, "/plans/{plan_id}/months", deltaParams(), "MonthsService.List"),
		op("getPlanMonth", get, "/plans/{plan_id}/months/{month}", "MonthsService.Get"),
		opQ("getAccounts", get, "/plans/{plan_id}/accounts", deltaParams(), "AccountsService.List"),
		op("createAccount", post, "/plans/{plan_id}/accounts", "AccountsService.Create"),
		op("getAccountById", get, "/plans/{plan_id}/accounts/{account_id}", "AccountsService.Get"),
		opQ("getCategories", get, "/plans/{plan_id}/categories", deltaParams(), "CategoriesService.List"),
		op("createCategory", post, "/plans/{plan_id}/categories", "CategoriesService.Create"),
		op("getCategoryById", get, "/plans/{plan_id}/categories/{category_id}", "CategoriesService.Get"),
		op("updateCategory", patch, "/plans/{plan_id}/categories/{category_id}", "CategoriesService.Update"),
		op("getMonthCategoryById", get,
			"/plans/{plan_id}/months/{month}/categories/{category_id}", "CategoriesService.GetForMonth"),
		op("updateMonthCategory", patch,
			"/plans/{plan_id}/months/{month}/categories/{category_id}", "CategoriesService.Assign"),
		op("createCategoryGroup", post, "/plans/{plan_id}/category_groups", "CategoriesService.CreateGroup"),
		op("updateCategoryGroup", patch,
			"/plans/{plan_id}/category_groups/{category_group_id}", "CategoriesService.RenameGroup"),
		opQ("getPayees", get, "/plans/{plan_id}/payees", deltaParams(), "PayeesService.List"),
		op("createPayee", post, "/plans/{plan_id}/payees", "PayeesService.Create"),
		op("getPayeeById", get, "/plans/{plan_id}/payees/{payee_id}", "PayeesService.Get"),
		op("updatePayee", patch, "/plans/{plan_id}/payees/{payee_id}", "PayeesService.Rename"),
		op("getPayeeLocations", get, "/plans/{plan_id}/payee_locations", "PayeeLocationsService.List"),
		op("getPayeeLocationById", get,
			"/plans/{plan_id}/payee_locations/{payee_location_id}", "PayeeLocationsService.Get"),
		op("getPayeeLocationsByPayee", get,
			"/plans/{plan_id}/payees/{payee_id}/payee_locations", "PayeeLocationsService.ListByPayee"),
		op("getMoneyMovements", get, "/plans/{plan_id}/money_movements", "MoneyMovementsService.List"),
		op("getMoneyMovementsByMonth", get,
			"/plans/{plan_id}/months/{month}/money_movements", "MoneyMovementsService.ListByMonth"),
		op("getMoneyMovementGroups", get,
			"/plans/{plan_id}/money_movement_groups", "MoneyMovementsService.ListGroups"),
		op("getMoneyMovementGroupsByMonth", get,
			"/plans/{plan_id}/months/{month}/money_movement_groups", "MoneyMovementsService.ListGroupsByMonth"),
		opQ("getTransactions", get,
			"/plans/{plan_id}/transactions",
			transactionFilterParams(), "TransactionsService.List"),
		op("createTransaction", post,
			"/plans/{plan_id}/transactions", "TransactionsService.Create", "TransactionsService.CreateBatch"),
		op("updateTransactions", patch, "/plans/{plan_id}/transactions", "TransactionsService.UpdateBatch"),
		op("importTransactions", post, "/plans/{plan_id}/transactions/import", "TransactionsService.Import"),
		op("getTransactionById", get, "/plans/{plan_id}/transactions/{transaction_id}", "TransactionsService.Get"),
		op("updateTransaction", put, "/plans/{plan_id}/transactions/{transaction_id}", "TransactionsService.Update"),
		op("deleteTransaction", del, "/plans/{plan_id}/transactions/{transaction_id}", "TransactionsService.Delete"),
		opQ("getTransactionsByAccount", get,
			"/plans/{plan_id}/accounts/{account_id}/transactions",
			transactionFilterParams(), "TransactionsService.ListByAccount"),
		opQ("getTransactionsByCategory", get,
			"/plans/{plan_id}/categories/{category_id}/transactions",
			transactionFilterParams(), "TransactionsService.ListByCategory"),
		opQ("getTransactionsByPayee", get,
			"/plans/{plan_id}/payees/{payee_id}/transactions",
			transactionFilterParams(), "TransactionsService.ListByPayee"),
		opQ("getTransactionsByMonth", get,
			"/plans/{plan_id}/months/{month}/transactions",
			transactionFilterParams(), "TransactionsService.ListByMonth"),
		opQ("getScheduledTransactions", get,
			"/plans/{plan_id}/scheduled_transactions",
			deltaParams(), "ScheduledTransactionsService.List"),
		op("createScheduledTransaction", post,
			"/plans/{plan_id}/scheduled_transactions", "ScheduledTransactionsService.Create"),
		op("getScheduledTransactionById", get,
			"/plans/{plan_id}/scheduled_transactions/{scheduled_transaction_id}", "ScheduledTransactionsService.Get"),
		op("updateScheduledTransaction", put,
			"/plans/{plan_id}/scheduled_transactions/{scheduled_transaction_id}", "ScheduledTransactionsService.Update"),
		op("deleteScheduledTransaction", del,
			"/plans/{plan_id}/scheduled_transactions/{scheduled_transaction_id}", "ScheduledTransactionsService.Delete"),
	}
}

// deltaParams returns a fresh query-param set for the 11 delta-capable
// operations; fresh so mutation in tests cannot alias across rows.
func deltaParams() []string {
	return []string{"last_knowledge_of_server"}
}

// transactionFilterParams returns a fresh query-param set for the five
// filterable transaction list operations.
func transactionFilterParams() []string {
	return []string{"since_date", "until_date", "type", "last_knowledge_of_server"}
}

// Short verb aliases keep the table rows readable under the line limit.
const (
	get   = http.MethodGet
	post  = http.MethodPost
	put   = http.MethodPut
	patch = http.MethodPatch
	del   = http.MethodDelete
)

// op builds a query-less table row.
func op(id, method, path string, goMethods ...string) Operation {
	return Operation{ID: id, Method: method, Path: path, GoMethods: goMethods}
}

// opQ builds a table row carrying allowed query parameters.
func opQ(id, method, path string, queryParams []string, goMethods ...string) Operation {
	return Operation{ID: id, Method: method, Path: path, QueryParams: queryParams, GoMethods: goMethods}
}
