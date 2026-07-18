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
		{ID: "getUser", Method: http.MethodGet, Path: "/user", GoMethods: []string{"Client.User"}},
		{ID: "getPlans", Method: http.MethodGet, Path: "/plans", QueryParams: []string{"include_accounts"}, GoMethods: []string{"Client.Plans"}},
		{ID: "getPlanById", Method: http.MethodGet, Path: "/plans/{plan_id}", QueryParams: deltaParams(), GoMethods: []string{"Plan.Export"}},
		{ID: "getPlanSettingsById", Method: http.MethodGet, Path: "/plans/{plan_id}/settings", GoMethods: []string{"Plan.Settings"}},
		{ID: "getPlanMonths", Method: http.MethodGet, Path: "/plans/{plan_id}/months", QueryParams: deltaParams(), GoMethods: []string{"MonthsService.List"}},
		{ID: "getPlanMonth", Method: http.MethodGet, Path: "/plans/{plan_id}/months/{month}", GoMethods: []string{"MonthsService.Get"}},
		{ID: "getAccounts", Method: http.MethodGet, Path: "/plans/{plan_id}/accounts", QueryParams: deltaParams(), GoMethods: []string{"AccountsService.List"}},
		{ID: "createAccount", Method: http.MethodPost, Path: "/plans/{plan_id}/accounts", GoMethods: []string{"AccountsService.Create"}},
		{ID: "getAccountById", Method: http.MethodGet, Path: "/plans/{plan_id}/accounts/{account_id}", GoMethods: []string{"AccountsService.Get"}},
		{ID: "getCategories", Method: http.MethodGet, Path: "/plans/{plan_id}/categories", QueryParams: deltaParams(), GoMethods: []string{"CategoriesService.List"}},
		{ID: "createCategory", Method: http.MethodPost, Path: "/plans/{plan_id}/categories", GoMethods: []string{"CategoriesService.Create"}},
		{ID: "getCategoryById", Method: http.MethodGet, Path: "/plans/{plan_id}/categories/{category_id}", GoMethods: []string{"CategoriesService.Get"}},
		{ID: "updateCategory", Method: http.MethodPatch, Path: "/plans/{plan_id}/categories/{category_id}", GoMethods: []string{"CategoriesService.Update"}},
		{ID: "getMonthCategoryById", Method: http.MethodGet, Path: "/plans/{plan_id}/months/{month}/categories/{category_id}", GoMethods: []string{"CategoriesService.GetForMonth"}},
		{ID: "updateMonthCategory", Method: http.MethodPatch, Path: "/plans/{plan_id}/months/{month}/categories/{category_id}", GoMethods: []string{"CategoriesService.Assign"}},
		{ID: "createCategoryGroup", Method: http.MethodPost, Path: "/plans/{plan_id}/category_groups", GoMethods: []string{"CategoriesService.CreateGroup"}},
		{ID: "updateCategoryGroup", Method: http.MethodPatch, Path: "/plans/{plan_id}/category_groups/{category_group_id}", GoMethods: []string{"CategoriesService.RenameGroup"}},
		{ID: "getPayees", Method: http.MethodGet, Path: "/plans/{plan_id}/payees", QueryParams: deltaParams(), GoMethods: []string{"PayeesService.List"}},
		{ID: "createPayee", Method: http.MethodPost, Path: "/plans/{plan_id}/payees", GoMethods: []string{"PayeesService.Create"}},
		{ID: "getPayeeById", Method: http.MethodGet, Path: "/plans/{plan_id}/payees/{payee_id}", GoMethods: []string{"PayeesService.Get"}},
		{ID: "updatePayee", Method: http.MethodPatch, Path: "/plans/{plan_id}/payees/{payee_id}", GoMethods: []string{"PayeesService.Rename"}},
		{ID: "getPayeeLocations", Method: http.MethodGet, Path: "/plans/{plan_id}/payee_locations", GoMethods: []string{"PayeeLocationsService.List"}},
		{ID: "getPayeeLocationById", Method: http.MethodGet, Path: "/plans/{plan_id}/payee_locations/{payee_location_id}", GoMethods: []string{"PayeeLocationsService.Get"}},
		{ID: "getPayeeLocationsByPayee", Method: http.MethodGet, Path: "/plans/{plan_id}/payees/{payee_id}/payee_locations", GoMethods: []string{"PayeeLocationsService.ListByPayee"}},
		{ID: "getMoneyMovements", Method: http.MethodGet, Path: "/plans/{plan_id}/money_movements", GoMethods: []string{"MoneyMovementsService.List"}},
		{ID: "getMoneyMovementsByMonth", Method: http.MethodGet, Path: "/plans/{plan_id}/months/{month}/money_movements", GoMethods: []string{"MoneyMovementsService.ListByMonth"}},
		{ID: "getMoneyMovementGroups", Method: http.MethodGet, Path: "/plans/{plan_id}/money_movement_groups", GoMethods: []string{"MoneyMovementsService.ListGroups"}},
		{ID: "getMoneyMovementGroupsByMonth", Method: http.MethodGet, Path: "/plans/{plan_id}/months/{month}/money_movement_groups", GoMethods: []string{"MoneyMovementsService.ListGroupsByMonth"}},
		{ID: "getTransactions", Method: http.MethodGet, Path: "/plans/{plan_id}/transactions", QueryParams: transactionFilterParams(), GoMethods: []string{"TransactionsService.List"}},
		{ID: "createTransaction", Method: http.MethodPost, Path: "/plans/{plan_id}/transactions", GoMethods: []string{"TransactionsService.Create", "TransactionsService.CreateBatch"}},
		{ID: "updateTransactions", Method: http.MethodPatch, Path: "/plans/{plan_id}/transactions", GoMethods: []string{"TransactionsService.UpdateBatch"}},
		{ID: "importTransactions", Method: http.MethodPost, Path: "/plans/{plan_id}/transactions/import", GoMethods: []string{"TransactionsService.Import"}},
		{ID: "getTransactionById", Method: http.MethodGet, Path: "/plans/{plan_id}/transactions/{transaction_id}", GoMethods: []string{"TransactionsService.Get"}},
		{ID: "updateTransaction", Method: http.MethodPut, Path: "/plans/{plan_id}/transactions/{transaction_id}", GoMethods: []string{"TransactionsService.Update"}},
		{ID: "deleteTransaction", Method: http.MethodDelete, Path: "/plans/{plan_id}/transactions/{transaction_id}", GoMethods: []string{"TransactionsService.Delete"}},
		{ID: "getTransactionsByAccount", Method: http.MethodGet, Path: "/plans/{plan_id}/accounts/{account_id}/transactions", QueryParams: transactionFilterParams(), GoMethods: []string{"TransactionsService.ListByAccount"}},
		{ID: "getTransactionsByCategory", Method: http.MethodGet, Path: "/plans/{plan_id}/categories/{category_id}/transactions", QueryParams: transactionFilterParams(), GoMethods: []string{"TransactionsService.ListByCategory"}},
		{ID: "getTransactionsByPayee", Method: http.MethodGet, Path: "/plans/{plan_id}/payees/{payee_id}/transactions", QueryParams: transactionFilterParams(), GoMethods: []string{"TransactionsService.ListByPayee"}},
		{ID: "getTransactionsByMonth", Method: http.MethodGet, Path: "/plans/{plan_id}/months/{month}/transactions", QueryParams: transactionFilterParams(), GoMethods: []string{"TransactionsService.ListByMonth"}},
		{ID: "getScheduledTransactions", Method: http.MethodGet, Path: "/plans/{plan_id}/scheduled_transactions", QueryParams: deltaParams(), GoMethods: []string{"ScheduledTransactionsService.List"}},
		{ID: "createScheduledTransaction", Method: http.MethodPost, Path: "/plans/{plan_id}/scheduled_transactions", GoMethods: []string{"ScheduledTransactionsService.Create"}},
		{ID: "getScheduledTransactionById", Method: http.MethodGet, Path: "/plans/{plan_id}/scheduled_transactions/{scheduled_transaction_id}", GoMethods: []string{"ScheduledTransactionsService.Get"}},
		{ID: "updateScheduledTransaction", Method: http.MethodPut, Path: "/plans/{plan_id}/scheduled_transactions/{scheduled_transaction_id}", GoMethods: []string{"ScheduledTransactionsService.Update"}},
		{ID: "deleteScheduledTransaction", Method: http.MethodDelete, Path: "/plans/{plan_id}/scheduled_transactions/{scheduled_transaction_id}", GoMethods: []string{"ScheduledTransactionsService.Delete"}},
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
