// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package user

import (
	"pkg.venceslau.dev/ynab/api"
)

// NewService facilitates the creation of a new user service instance
func NewService(c api.ClientReader) *Service {
	return &Service{c}
}

// Service wraps YNAB user API endpoints
type Service struct {
	c api.ClientReader
}

// GetUser fetches information about the authenticated user
// https://api.youneedabudget.com/v1#/User/getUser
func (s *Service) GetUser() (*User, error) {
	resModel := struct {
		Data struct {
			User *User `json:"user"`
		} `json:"data"`
	}{}

	if err := s.c.GET("/user", &resModel); err != nil {
		return nil, err
	}
	return resModel.Data.User, nil
}
