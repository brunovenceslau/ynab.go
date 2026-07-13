// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

// Package category implements category entities and services
package category // import "pkg.venceslau.dev/ynab/api/category"

import "pkg.venceslau.dev/ynab/api"

// Category represents a category for a budget
type Category struct {
	ID              string `json:"id"`
	CategoryGroupID string `json:"category_group_id"`
	Name            string `json:"name"`
	Hidden          bool   `json:"hidden"`
	// Budgeted Budgeted amount in current month in milliunits format
	Budgeted int64 `json:"budgeted"`
	// Activity Activity amount in current month in milliunits format
	Activity int64 `json:"activity"`
	// Balance Balance in current month in milliunits format
	Balance int64 `json:"balance"`
	// Deleted Deleted category groups will only be included in delta requests
	Deleted bool `json:"deleted"`

	Note *string `json:"note"`
	// OriginalCategoryGroupID If category is hidden this is the ID of the category
	// group it originally belonged to before it was hidden
	OriginalCategoryGroupID *string `json:"original_category_group_id"`

	GoalType *Goal `json:"goal_type"`
	// GoalCreationMonth the month a goal was created
	GoalCreationMonth *api.Date `json:"goal_creation_month"`
	// GoalTarget the goal target amount in milliunits
	GoalTarget *int64 `json:"goal_target"`
	// GoalTargetMonth if the goal type is GoalTargetCategoryBalanceByDate,
	// this is the target month for the goal to be completed
	GoalTargetMonth *api.Date `json:"goal_target_month"`
	// GoalPercentageComplete the percentage completion of the goal
	GoalPercentageComplete *uint16 `json:"goal_percentage_complete"`
}

// Group represents a resumed category group for a budget
type Group struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
	// Deleted Deleted category groups will only be included in delta requests
	Deleted bool `json:"deleted"`
}

// GroupWithCategories represents a category group for a budget
type GroupWithCategories struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
	// Deleted Deleted category groups will only be included in delta requests
	Deleted bool `json:"deleted"`

	Categories []*Category `json:"categories"`
}

// SearchResultSnapshot represents a versioned snapshot for an account search
type SearchResultSnapshot struct {
	GroupWithCategories []*GroupWithCategories
	ServerKnowledge     uint64
}
