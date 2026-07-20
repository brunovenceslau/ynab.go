// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"context"
	"net/http"
	"time"
)

// MoneyMovement is one recorded move of money between categories.
type MoneyMovement struct {
	ID                   string     `json:"id"`
	Month                *Month     `json:"month"`
	MovedAt              *time.Time `json:"moved_at"`
	Note                 *string    `json:"note"`
	MoneyMovementGroupID *string    `json:"money_movement_group_id"`
	PerformedByUserID    *string    `json:"performed_by_user_id"`
	FromCategoryID       *string    `json:"from_category_id"`
	ToCategoryID         *string    `json:"to_category_id"`
	Amount               Milliunits `json:"amount"`
	AmountFormatted      string     `json:"amount_formatted"`
	AmountCurrency       float64    `json:"amount_currency"`
}

// MoneyMovementGroup is a named batch of money movements.
type MoneyMovementGroup struct {
	ID                string    `json:"id"`
	GroupCreatedAt    time.Time `json:"group_created_at"`
	Month             Month     `json:"month"`
	Note              *string   `json:"note"`
	PerformedByUserID *string   `json:"performed_by_user_id"`
}

// MoneyMovementsService reads the plan's money movements. Wire asymmetry,
// documented rather than papered over: these endpoints return server
// knowledge but accept no delta cursor — there are no list options.
type MoneyMovementsService struct {
	plan *Plan
}

// moneyMovementsResult is the shared movements response payload.
type moneyMovementsResult struct {
	MoneyMovements  []MoneyMovement `json:"money_movements"`
	ServerKnowledge ServerKnowledge `json:"server_knowledge"`
}

// moneyMovementGroupsResult is the shared groups response payload.
type moneyMovementGroupsResult struct {
	MoneyMovementGroups []MoneyMovementGroup `json:"money_movement_groups"`
	ServerKnowledge     ServerKnowledge      `json:"server_knowledge"`
}

// List returns all money movements of the plan.
//
// YNAB operationId: getMoneyMovements
func (s *MoneyMovementsService) List(ctx context.Context) ([]MoneyMovement, ServerKnowledge, error) {
	data, err := do[moneyMovementsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("money_movements"), nil, nil)
	if err != nil {
		return nil, 0, err
	}
	return data.MoneyMovements, data.ServerKnowledge, nil
}

// ListByMonth returns the money movements of one month. Month accepts
// [CurrentMonth]; a zero Month is a pre-flight [*ArgumentError].
//
// YNAB operationId: getMoneyMovementsByMonth
func (s *MoneyMovementsService) ListByMonth(ctx context.Context, m Month) ([]MoneyMovement, ServerKnowledge, error) {
	if m.IsZero() {
		return nil, 0, zeroMonthError("MoneyMovements.ListByMonth")
	}
	data, err := do[moneyMovementsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("months", m.String(), "money_movements"), nil, nil)
	if err != nil {
		return nil, 0, err
	}
	return data.MoneyMovements, data.ServerKnowledge, nil
}

// ListGroups returns all money movement groups of the plan.
//
// YNAB operationId: getMoneyMovementGroups
func (s *MoneyMovementsService) ListGroups(ctx context.Context) ([]MoneyMovementGroup, ServerKnowledge, error) {
	data, err := do[moneyMovementGroupsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("money_movement_groups"), nil, nil)
	if err != nil {
		return nil, 0, err
	}
	return data.MoneyMovementGroups, data.ServerKnowledge, nil
}

// ListGroupsByMonth returns the money movement groups of one month.
// Month accepts [CurrentMonth]; a zero Month is a pre-flight
// [*ArgumentError].
//
// YNAB operationId: getMoneyMovementGroupsByMonth
func (s *MoneyMovementsService) ListGroupsByMonth(
	ctx context.Context, m Month,
) ([]MoneyMovementGroup, ServerKnowledge, error) {
	if m.IsZero() {
		return nil, 0, zeroMonthError("MoneyMovements.ListGroupsByMonth")
	}
	data, err := do[moneyMovementGroupsResult](ctx, s.plan.client,
		http.MethodGet, s.plan.path("months", m.String(), "money_movement_groups"), nil, nil)
	if err != nil {
		return nil, 0, err
	}
	return data.MoneyMovementGroups, data.ServerKnowledge, nil
}
