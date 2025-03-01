// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package category_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/jarcoal/httpmock.v1"

	"github.com/brunomvsouza/ynab.go"
	"github.com/brunomvsouza/ynab.go/api"
	"github.com/brunomvsouza/ynab.go/api/category"
)

func TestService_GetCategories(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/categories"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "category_groups": [
      {
        "id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
        "name": "Credit Card Payments",
        "hidden": false,
        "deleted": false,
        "categories": [
          {
            "id": "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
            "category_group_id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
            "name": "MasterCard",
            "hidden": false,
            "original_category_group_id": null,
            "note": null,
            "budgeted": 0,
            "activity": 12190,
            "balance": 18740,
            "deleted": false,
            "goal_type": "TB",
            "goal_creation_month": "2018-04-01",
            "goal_target": 18740,
            "goal_target_month": "2018-05-01",
            "goal_percentage_complete": 20
          }
        ]
      }
		],
		"server_knowledge": 10
	}
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	f := &api.Filter{LastKnowledgeOfServer: 10}
	snapshot, err := client.Category().GetCategories("aa248caa-eed7-4575-a990-717386438d2c", f)
	assert.NoError(t, err)

	var (
		expectedGoalTarget             int64  = 18740
		expectedGoalPercentageComplete uint16 = 20
	)
	expectedGoalCreationMonth, err := api.DateFromString("2018-04-01")
	assert.NoError(t, err)
	expectedGoalTargetMonth, err := api.DateFromString("2018-05-01")
	assert.NoError(t, err)

	expected := &category.SearchResultSnapshot{
		GroupWithCategories: []*category.GroupWithCategories{
			{
				ID:      "13419c12-78d3-4818-a5dc-601b2b8a6064",
				Name:    "Credit Card Payments",
				Hidden:  false,
				Deleted: false,
				Categories: []*category.Category{
					{
						ID:                     "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
						CategoryGroupID:        "13419c12-78d3-4818-a5dc-601b2b8a6064",
						Name:                   "MasterCard",
						Hidden:                 false,
						Budgeted:               int64(0),
						Activity:               int64(12190),
						Balance:                int64(18740),
						Deleted:                false,
						GoalType:               category.GoalTargetCategoryBalance.Pointer(),
						GoalCreationMonth:      &expectedGoalCreationMonth,
						GoalTargetMonth:        &expectedGoalTargetMonth,
						GoalTarget:             &expectedGoalTarget,
						GoalPercentageComplete: &expectedGoalPercentageComplete,
					},
				},
			},
		},
		ServerKnowledge: 10,
	}

	assert.Equal(t, expected, snapshot)
}

func TestService_GetCategory(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/categories/13419c12-78d3-4a26-82ca-1cde7aa1d6f8"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "category": {
			"id": "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
			"category_group_id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
			"name": "MasterCard",
			"hidden": false,
			"original_category_group_id": null,
			"note": null,
			"budgeted": 0,
			"activity": 12190,
			"balance": 18740,
			"deleted": false,
			"goal_type": "TB",
			"goal_creation_month": "2018-04-01",
			"goal_target": 18740,
			"goal_target_month": "2018-05-01",
			"goal_percentage_complete": 20
    }
	}
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	c, err := client.Category().GetCategory(
		"aa248caa-eed7-4575-a990-717386438d2c",
		"13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
	)
	assert.NoError(t, err)

	var (
		expectedGoalTarget             int64  = 18740
		expectedGoalPercentageComplete uint16 = 20
	)
	expectedGoalCreationMonth, err := api.DateFromString("2018-04-01")
	assert.NoError(t, err)
	expectedGoalTargetMonth, err := api.DateFromString("2018-05-01")
	assert.NoError(t, err)

	expected := &category.Category{
		ID:                     "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		CategoryGroupID:        "13419c12-78d3-4818-a5dc-601b2b8a6064",
		Name:                   "MasterCard",
		Hidden:                 false,
		Budgeted:               int64(0),
		Activity:               int64(12190),
		Balance:                int64(18740),
		Deleted:                false,
		GoalType:               category.GoalTargetCategoryBalance.Pointer(),
		GoalCreationMonth:      &expectedGoalCreationMonth,
		GoalTargetMonth:        &expectedGoalTargetMonth,
		GoalTarget:             &expectedGoalTarget,
		GoalPercentageComplete: &expectedGoalPercentageComplete,
	}
	assert.Equal(t, expected, c)
}

func TestService_GetCategoryForMonth(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/months/2018-01-01/categories/13419c12-78d3-4a26-82ca-1cde7aa1d6f8"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "category": {
			"id": "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
			"category_group_id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
			"name": "MasterCard",
			"hidden": false,
			"original_category_group_id": null,
			"note": null,
			"budgeted": 0,
			"activity": 12190,
			"balance": 18740,
			"deleted": false,
			"goal_type": "TB",
			"goal_creation_month": "2018-04-01",
			"goal_target": 18740,
			"goal_target_month": "2018-05-01",
			"goal_percentage_complete": 20
    }
	}
}
		`)
			return res, nil
		},
	)

	date, err := api.DateFromString("2018-01-01")
	assert.NoError(t, err)

	client := ynab.NewClient("")
	c, err := client.Category().GetCategoryForMonth(
		"aa248caa-eed7-4575-a990-717386438d2c",
		"13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		date,
	)
	assert.NoError(t, err)

	var (
		expectedGoalTarget             int64  = 18740
		expectedGoalPercentageComplete uint16 = 20
	)
	expectedGoalCreationMonth, err := api.DateFromString("2018-04-01")
	assert.NoError(t, err)
	expectedGoalTargetMonth, err := api.DateFromString("2018-05-01")
	assert.NoError(t, err)

	expected := &category.Category{
		ID:                     "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		CategoryGroupID:        "13419c12-78d3-4818-a5dc-601b2b8a6064",
		Name:                   "MasterCard",
		Hidden:                 false,
		Budgeted:               int64(0),
		Activity:               int64(12190),
		Balance:                int64(18740),
		Deleted:                false,
		GoalType:               category.GoalTargetCategoryBalance.Pointer(),
		GoalCreationMonth:      &expectedGoalCreationMonth,
		GoalTargetMonth:        &expectedGoalTargetMonth,
		GoalTarget:             &expectedGoalTarget,
		GoalPercentageComplete: &expectedGoalPercentageComplete,
	}
	assert.Equal(t, expected, c)
}

func TestService_GetCategoryForCurrentMonth(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/months/current/categories/13419c12-78d3-4a26-82ca-1cde7aa1d6f8"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "category": {
			"id": "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
			"category_group_id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
			"name": "MasterCard",
			"hidden": false,
			"original_category_group_id": null,
			"note": null,
			"budgeted": 0,
			"activity": 12190,
			"balance": 18740,
			"deleted": false,
			"goal_type": "TB",
			"goal_creation_month": "2018-04-01",
			"goal_target": 18740,
			"goal_target_month": "2018-05-01",
			"goal_percentage_complete": 20
    }
	}
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	c, err := client.Category().GetCategoryForCurrentMonth(
		"aa248caa-eed7-4575-a990-717386438d2c",
		"13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
	)
	assert.NoError(t, err)

	var (
		expectedGoalTarget             int64  = 18740
		expectedGoalPercentageComplete uint16 = 20
	)
	expectedGoalCreationMonth, err := api.DateFromString("2018-04-01")
	assert.NoError(t, err)
	expectedGoalTargetMonth, err := api.DateFromString("2018-05-01")
	assert.NoError(t, err)

	expected := &category.Category{
		ID:                     "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		CategoryGroupID:        "13419c12-78d3-4818-a5dc-601b2b8a6064",
		Name:                   "MasterCard",
		Hidden:                 false,
		Budgeted:               int64(0),
		Activity:               int64(12190),
		Balance:                int64(18740),
		Deleted:                false,
		GoalType:               category.GoalTargetCategoryBalance.Pointer(),
		GoalCreationMonth:      &expectedGoalCreationMonth,
		GoalTargetMonth:        &expectedGoalTargetMonth,
		GoalTarget:             &expectedGoalTarget,
		GoalPercentageComplete: &expectedGoalPercentageComplete,
	}
	assert.Equal(t, expected, c)
}

func TestService_UpdateCategoryForMonth(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	payload := category.PayloadMonthCategory{
		Budgeted: 1000,
	}

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/months/0001-01-01/categories/13419c12-78d3-4a26-82ca-1cde7aa1d6f8"
	httpmock.RegisterResponder(http.MethodPatch, url,
		func(req *http.Request) (*http.Response, error) {
			resModel := struct {
				MonthCategory *category.PayloadMonthCategory `json:"category"`
			}{}
			err := json.NewDecoder(req.Body).Decode(&resModel)
			assert.NoError(t, err)
			assert.Equal(t, &payload, resModel.MonthCategory)

			res := httpmock.NewStringResponse(200, `{
  "data": {
    "category": {
			"id": "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
			"category_group_id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
			"name": "MasterCard",
			"hidden": false,
			"original_category_group_id": null,
			"note": null,
			"budgeted": 1000,
			"activity": 12190,
			"balance": 18740,
			"deleted": false,
			"goal_type": "TB",
			"goal_creation_month": "2018-04-01",
			"goal_target": 18740,
			"goal_target_month": "2018-05-01",
			"goal_percentage_complete": 20
    }
	}
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	c, err := client.Category().UpdateCategoryForMonth(
		"aa248caa-eed7-4575-a990-717386438d2c",
		"13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		api.Date{},
		payload,
	)
	assert.NoError(t, err)

	var (
		expectedGoalTarget             int64  = 18740
		expectedGoalPercentageComplete uint16 = 20
	)
	expectedGoalCreationMonth, err := api.DateFromString("2018-04-01")
	assert.NoError(t, err)
	expectedGoalTargetMonth, err := api.DateFromString("2018-05-01")
	assert.NoError(t, err)

	expected := &category.Category{
		ID:                     "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		CategoryGroupID:        "13419c12-78d3-4818-a5dc-601b2b8a6064",
		Name:                   "MasterCard",
		Hidden:                 false,
		Budgeted:               int64(1000),
		Activity:               int64(12190),
		Balance:                int64(18740),
		Deleted:                false,
		GoalType:               category.GoalTargetCategoryBalance.Pointer(),
		GoalCreationMonth:      &expectedGoalCreationMonth,
		GoalTargetMonth:        &expectedGoalTargetMonth,
		GoalTarget:             &expectedGoalTarget,
		GoalPercentageComplete: &expectedGoalPercentageComplete,
	}
	assert.Equal(t, expected, c)
}

func TestService_UpdateCategoryForCurrentMonth(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	payload := category.PayloadMonthCategory{
		Budgeted: 1000,
	}

	url := "https://api.youneedabudget.com/v1/budgets/aa248caa-eed7-4575-a990-717386438d2c/months/current/categories/13419c12-78d3-4a26-82ca-1cde7aa1d6f8"
	httpmock.RegisterResponder(http.MethodPatch, url,
		func(req *http.Request) (*http.Response, error) {
			resModel := struct {
				MonthCategory *category.PayloadMonthCategory `json:"category"`
			}{}
			err := json.NewDecoder(req.Body).Decode(&resModel)
			assert.NoError(t, err)
			assert.Equal(t, &payload, resModel.MonthCategory)

			res := httpmock.NewStringResponse(200, `{
  "data": {
    "category": {
			"id": "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
			"category_group_id": "13419c12-78d3-4818-a5dc-601b2b8a6064",
			"name": "MasterCard",
			"hidden": false,
			"original_category_group_id": null,
			"note": null,
			"budgeted": 1000,
			"activity": 12190,
			"balance": 18740,
			"deleted": false,
			"goal_type": "TB",
			"goal_creation_month": "2018-04-01",
			"goal_target": 18740,
			"goal_target_month": "2018-05-01",
			"goal_percentage_complete": 20
    }
	}
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	c, err := client.Category().UpdateCategoryForCurrentMonth(
		"aa248caa-eed7-4575-a990-717386438d2c",
		"13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		payload,
	)
	assert.NoError(t, err)

	var (
		expectedGoalTarget             int64  = 18740
		expectedGoalPercentageComplete uint16 = 20
	)
	expectedGoalCreationMonth, err := api.DateFromString("2018-04-01")
	assert.NoError(t, err)
	expectedGoalTargetMonth, err := api.DateFromString("2018-05-01")
	assert.NoError(t, err)

	expected := &category.Category{
		ID:                     "13419c12-78d3-4a26-82ca-1cde7aa1d6f8",
		CategoryGroupID:        "13419c12-78d3-4818-a5dc-601b2b8a6064",
		Name:                   "MasterCard",
		Hidden:                 false,
		Budgeted:               int64(1000),
		Activity:               int64(12190),
		Balance:                int64(18740),
		Deleted:                false,
		GoalType:               category.GoalTargetCategoryBalance.Pointer(),
		GoalCreationMonth:      &expectedGoalCreationMonth,
		GoalTargetMonth:        &expectedGoalTargetMonth,
		GoalTarget:             &expectedGoalTarget,
		GoalPercentageComplete: &expectedGoalPercentageComplete,
	}
	assert.Equal(t, expected, c)
}
