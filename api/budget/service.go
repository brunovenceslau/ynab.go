// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package budget

import (
	"fmt"

	"pkg.venceslau.dev/ynab/api"
)

// NewService facilitates the creation of a new budget service instance
func NewService(c api.ClientReader) *Service {
	return &Service{c}
}

// Service wraps YNAB budget API endpoints
type Service struct {
	c api.ClientReader
}

// GetBudgets fetches the list of budgets of the logger in user
// https://api.youneedabudget.com/v1#/Budgets/getBudgets
func (s *Service) GetBudgets() ([]*Summary, error) {
	resModel := struct {
		Data struct {
			Budgets []*Summary `json:"budgets"`
		} `json:"data"`
	}{}

	if err := s.c.GET("/budgets", &resModel); err != nil {
		return nil, err
	}
	return resModel.Data.Budgets, nil
}

// GetBudget fetches a single budget with all related entities,
// effectively a full budget export with filtering capabilities
// https://api.youneedabudget.com/v1#/Budgets/getBudgetById
func (s *Service) GetBudget(budgetID string, f *api.Filter) (*Snapshot, error) {
	resModel := struct {
		Data struct {
			Budget          *Budget `json:"budget"`
			ServerKnowledge uint64  `json:"server_knowledge"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s", budgetID)
	if f != nil {
		url = fmt.Sprintf("%s?%s", url, f.ToQuery())
	}

	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return &Snapshot{
		Budget:          resModel.Data.Budget,
		ServerKnowledge: resModel.Data.ServerKnowledge,
	}, nil
}

// GetLastUsedBudget fetches the last used budget with all related
// entities, effectively a full budget export with filtering capabilities
// https://api.youneedabudget.com/v1#/Budgets/getBudgetById
func (s *Service) GetLastUsedBudget(f *api.Filter) (*Snapshot, error) {
	const lastUsedBudgetID = "last-used"
	return s.GetBudget(lastUsedBudgetID, f)
}

// GetBudgetSettings fetches a budget settings
// https://api.youneedabudget.com/v1#/Budgets/getBudgetSettingsById
func (s *Service) GetBudgetSettings(budgetID string) (*Settings, error) {
	resModel := struct {
		Data struct {
			Settings *Settings `json:"settings"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/settings", budgetID)
	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}

	return resModel.Data.Settings, nil
}
