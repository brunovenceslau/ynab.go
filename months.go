package ynab

import (
	"context"
	"net/http"
)

// MonthSummaryBase is the month shape shared by the months endpoints and
// the full-plan export collections.
type MonthSummaryBase struct {
	Month        Month      `json:"month"`
	Note         *string    `json:"note"`
	Income       Milliunits `json:"income"`
	Budgeted     Milliunits `json:"budgeted"`
	Activity     Milliunits `json:"activity"`
	ToBeBudgeted Milliunits `json:"to_be_budgeted"`
	AgeOfMoney   *int       `json:"age_of_money"`
	Deleted      bool       `json:"deleted"`
}

// MonthSummary is one month in a month list. The *_formatted/*_currency
// companions are computed, read-only display fields.
type MonthSummary struct {
	MonthSummaryBase
	IncomeFormatted       string  `json:"income_formatted"`
	IncomeCurrency        float64 `json:"income_currency"`
	BudgetedFormatted     string  `json:"budgeted_formatted"`
	BudgetedCurrency      float64 `json:"budgeted_currency"`
	ActivityFormatted     string  `json:"activity_formatted"`
	ActivityCurrency      float64 `json:"activity_currency"`
	ToBeBudgetedFormatted string  `json:"to_be_budgeted_formatted"`
	ToBeBudgetedCurrency  float64 `json:"to_be_budgeted_currency"`
}

// SyncID keys the month for MergeByID.
func (m MonthSummary) SyncID() string { return m.Month.String() }

// IsDeleted reports a delta tombstone.
func (m MonthSummary) IsDeleted() bool { return m.Deleted }

// MonthDetailBase is the export shape for months in PlanDetail: the base
// summary plus CategoryBase collections.
type MonthDetailBase struct {
	MonthSummaryBase
	Categories []CategoryBase `json:"categories"`
}

// MonthDetail is a single month with its categories, amounts specific to
// the requested month.
type MonthDetail struct {
	MonthSummary
	Categories []Category `json:"categories"`
}

// MonthsService reads the plan's months.
type MonthsService struct {
	plan *Plan
}

// List returns the plan's month summaries.
//
// YNAB operationId: getPlanMonths
func (s *MonthsService) List(ctx context.Context, opts ...ListOption) ([]MonthSummary, ServerKnowledge, error) {
	data, err := do[struct {
		Months          []MonthSummary  `json:"months"`
		ServerKnowledge ServerKnowledge `json:"server_knowledge"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("months"), applyListOptions(nil, opts), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Months, data.ServerKnowledge, nil
}

// Get returns a single month with its categories. Month accepts
// CurrentMonth — the server resolves its own current month.
//
// YNAB operationId: getPlanMonth
func (s *MonthsService) Get(ctx context.Context, m Month) (*MonthDetail, error) {
	if m.IsZero() {
		return nil, zeroMonthError("Months.Get")
	}
	data, err := do[struct {
		Month *MonthDetail `json:"month"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("months", m.String()), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.Month, nil
}
