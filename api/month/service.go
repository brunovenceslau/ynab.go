// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package month

import (
	"fmt"

	"pkg.venceslau.dev/ynab/api"
)

// NewService facilitates the creation of a new month service instance
func NewService(c api.ClientReader) *Service {
	return &Service{c}
}

// Service wraps YNAB month API endpoints
type Service struct {
	c api.ClientReader
}

// GetMonths fetches the list of months from a budget
// https://api.youneedabudget.com/v1#/Months/getBudgetMonths
func (s *Service) GetMonths(budgetID string, f *api.Filter) (*SearchResultSnapshot, error) {
	resModel := struct {
		Data struct {
			Months          []*Summary `json:"months"`
			ServerKnowledge uint64     `json:"server_knowledge"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/months", budgetID)
	if f != nil {
		url = fmt.Sprintf("%s?%s", url, f.ToQuery())
	}

	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}
	return &SearchResultSnapshot{
		Months:          resModel.Data.Months,
		ServerKnowledge: resModel.Data.ServerKnowledge,
	}, nil
}

// GetMonth fetches a specific month from a budget
// https://api.youneedabudget.com/v1#/Months/getBudgetMonth
func (s *Service) GetMonth(budgetID string, month api.Date) (*Month, error) {
	resModel := struct {
		Data struct {
			Month *Month `json:"month"`
		} `json:"data"`
	}{}

	url := fmt.Sprintf("/budgets/%s/months/%s", budgetID,
		api.DateFormat(month))
	if err := s.c.GET(url, &resModel); err != nil {
		return nil, err
	}
	return resModel.Data.Month, nil
}
