package ynab

import (
	"context"
	"net/http"
	"time"
)

// GoalType is a category goal's read-side type. Null on the wire means
// "no goal", modeled as *GoalType.
type GoalType string

// The five wire goal types.
const (
	GoalTypeTB   GoalType = "TB"   // Target Category Balance
	GoalTypeTBD  GoalType = "TBD"  // Target Category Balance by Date
	GoalTypeMF   GoalType = "MF"   // Monthly Funding
	GoalTypeNEED GoalType = "NEED" // Plan Your Spending
	GoalTypeDEBT GoalType = "DEBT" // Debt payoff
)

// Valid reports whether t is one of the documented wire values.
func (t GoalType) Valid() bool {
	switch t {
	case GoalTypeTB, GoalTypeTBD, GoalTypeMF, GoalTypeNEED, GoalTypeDEBT:
		return true
	}
	return false
}

// GoalFrequency configures a recurring NEED target on category writes
// (write-only; requires a goal target, cannot combine with a target date).
type GoalFrequency string

// The three wire goal frequencies.
const (
	GoalFrequencyMonthly GoalFrequency = "monthly"
	GoalFrequencyWeekly  GoalFrequency = "weekly"
	GoalFrequencyYearly  GoalFrequency = "yearly"
)

// Valid reports whether f is one of the documented wire values.
func (f GoalFrequency) Valid() bool {
	switch f {
	case GoalFrequencyMonthly, GoalFrequencyWeekly, GoalFrequencyYearly:
		return true
	}
	return false
}

// CategoryBase is the category shape shared by the categories endpoints
// and the full-plan export collections. Amounts are specific to the
// current plan month unless read through GetForMonth.
type CategoryBase struct {
	ID                      string      `json:"id"`
	CategoryGroupID         string      `json:"category_group_id"`
	CategoryGroupName       string      `json:"category_group_name"`
	Name                    string      `json:"name"`
	Hidden                  bool        `json:"hidden"`
	Internal                bool        `json:"internal"`
	OriginalCategoryGroupID *string     `json:"original_category_group_id"` // deprecated: always null
	Note                    *string     `json:"note"`
	Budgeted                Milliunits  `json:"budgeted"`
	Activity                Milliunits  `json:"activity"`
	Balance                 Milliunits  `json:"balance"`
	GoalType                *GoalType   `json:"goal_type"`
	GoalNeedsWholeAmount    *bool       `json:"goal_needs_whole_amount"`
	GoalDay                 *int        `json:"goal_day"`
	GoalCadence             *int        `json:"goal_cadence"`
	GoalCadenceFrequency    *int        `json:"goal_cadence_frequency"`
	GoalCreationMonth       *Month      `json:"goal_creation_month"`
	GoalTarget              *Milliunits `json:"goal_target"`
	GoalTargetMonth         *Month      `json:"goal_target_month"` // deprecated: use GoalTargetDate
	GoalTargetDate          *Date       `json:"goal_target_date"`
	GoalPercentageComplete  *int        `json:"goal_percentage_complete"`
	GoalMonthsToBudget      *int        `json:"goal_months_to_budget"`
	GoalUnderFunded         *Milliunits `json:"goal_under_funded"`
	GoalOverallFunded       *Milliunits `json:"goal_overall_funded"`
	GoalOverallLeft         *Milliunits `json:"goal_overall_left"`
	GoalSnoozedAt           *time.Time  `json:"goal_snoozed_at"`
	Deleted                 bool        `json:"deleted"`
}

// Category is a plan category. The *_formatted/*_currency companions are
// computed, read-only display fields — do money math on the milliunit
// integers, never on the float companions.
type Category struct {
	CategoryBase
	BalanceFormatted           string   `json:"balance_formatted"`
	BalanceCurrency            float64  `json:"balance_currency"`
	ActivityFormatted          string   `json:"activity_formatted"`
	ActivityCurrency           float64  `json:"activity_currency"`
	BudgetedFormatted          string   `json:"budgeted_formatted"`
	BudgetedCurrency           float64  `json:"budgeted_currency"`
	GoalTargetFormatted        *string  `json:"goal_target_formatted"`
	GoalTargetCurrency         *float64 `json:"goal_target_currency"`
	GoalUnderFundedFormatted   *string  `json:"goal_under_funded_formatted"`
	GoalUnderFundedCurrency    *float64 `json:"goal_under_funded_currency"`
	GoalOverallFundedFormatted *string  `json:"goal_overall_funded_formatted"`
	GoalOverallFundedCurrency  *float64 `json:"goal_overall_funded_currency"`
	GoalOverallLeftFormatted   *string  `json:"goal_overall_left_formatted"`
	GoalOverallLeftCurrency    *float64 `json:"goal_overall_left_currency"`
}

// SyncID keys the category for MergeByID. Category inherits the
// adapters by embedding.
func (c CategoryBase) SyncID() string { return c.ID }

// IsDeleted reports a delta tombstone.
func (c CategoryBase) IsDeleted() bool { return c.Deleted }

// CategoryGroup is a category group with its nested categories, as
// getCategories returns them.
type CategoryGroup struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Hidden     bool       `json:"hidden"`
	Internal   bool       `json:"internal"`
	Deleted    bool       `json:"deleted"`
	Categories []Category `json:"categories"`
}

// SyncID keys the group for MergeByID.
func (g CategoryGroup) SyncID() string { return g.ID }

// IsDeleted reports a delta tombstone.
func (g CategoryGroup) IsDeleted() bool { return g.Deleted }

// CategorySpec is the payload for CategoriesService.Create. Name and
// GroupID are required; an internal category group may not be specified.
type CategorySpec struct {
	Name    string           `json:"name"`
	GroupID string           `json:"category_group_id"`
	Note    Optional[string] `json:"note,omitzero"`

	// GoalTarget creates a monthly goal when the category has none
	// (defaulting to NEED, or MF for Credit Card Payment categories).
	GoalTarget           Optional[Milliunits] `json:"goal_target,omitzero"`
	GoalTargetDate       Optional[Date]       `json:"goal_target_date,omitzero"`
	GoalNeedsWholeAmount Optional[bool]       `json:"goal_needs_whole_amount,omitzero"`

	// GoalFrequency configures a recurring NEED target; requires
	// GoalTarget and cannot be combined with GoalTargetDate. The zero
	// value is omitted.
	GoalFrequency GoalFrequency `json:"goal_frequency,omitzero"`
}

// CategoryUpdate is the partial payload for CategoriesService.Update.
// Unset fields stay unchanged on the server; SetNull clears — for
// GoalTarget, SetNull removes an existing target (omit ≠ clear!).
type CategoryUpdate struct {
	Name                 Optional[string]     `json:"name,omitzero"`
	GroupID              Optional[string]     `json:"category_group_id,omitzero"`
	Note                 Optional[string]     `json:"note,omitzero"`
	GoalTarget           Optional[Milliunits] `json:"goal_target,omitzero"`
	GoalTargetDate       Optional[Date]       `json:"goal_target_date,omitzero"`
	GoalNeedsWholeAmount Optional[bool]       `json:"goal_needs_whole_amount,omitzero"`
	GoalFrequency        GoalFrequency        `json:"goal_frequency,omitzero"`
}

// categoryGroupNameMax is the spec-stated bound on group names.
const categoryGroupNameMax = 50

// CategoriesService reads and writes the plan's categories.
type CategoriesService struct {
	plan *Plan
}

// List returns the plan's category groups with their categories nested
// inside. Delta reads nest changed categories inside their groups too:
// flatten every group's Categories before merging by id — merging groups
// wholesale would drop unchanged categories from unchanged groups.
//
// YNAB operationId: getCategories
func (s *CategoriesService) List(ctx context.Context, opts ...ListOption) ([]CategoryGroup, ServerKnowledge, error) {
	data, err := do[struct {
		CategoryGroups  []CategoryGroup `json:"category_groups"`
		ServerKnowledge ServerKnowledge `json:"server_knowledge"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("categories"), applyListOptions(nil, opts), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.CategoryGroups, data.ServerKnowledge, nil
}

// Get returns a single category by id, with amounts for the current
// plan month.
//
// YNAB operationId: getCategoryById
func (s *CategoriesService) Get(ctx context.Context, categoryID string) (*Category, error) {
	data, err := do[struct {
		Category *Category `json:"category"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("categories", categoryID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.Category, nil
}

// GetForMonth returns a single category with amounts specific to month.
// Month accepts CurrentMonth.
//
// YNAB operationId: getMonthCategoryById
func (s *CategoriesService) GetForMonth(ctx context.Context, m Month, categoryID string) (*Category, error) {
	if m.IsZero() {
		return nil, zeroMonthError("Categories.GetForMonth")
	}
	data, err := do[struct {
		Category *Category `json:"category"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("months", m.String(), "categories", categoryID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.Category, nil
}

// categoryResult is the shared save-category response payload.
type categoryResult struct {
	Category        *Category       `json:"category"`
	ServerKnowledge ServerKnowledge `json:"server_knowledge"`
}

// Create adds a category (HTTP 201) and returns it with the new server
// knowledge — unlike createAccount, this create does return a cursor.
//
// YNAB operationId: createCategory
func (s *CategoriesService) Create(ctx context.Context, spec CategorySpec) (*Category, ServerKnowledge, error) {
	data, err := do[categoryResult](ctx, s.plan.client,
		http.MethodPost, s.plan.path("categories"), nil, body{"category": spec})
	if err != nil {
		return nil, 0, err
	}
	return data.Category, data.ServerKnowledge, nil
}

// Update patches a category. Unset fields stay unchanged; SetNull on
// GoalTarget clears an existing target.
//
// YNAB operationId: updateCategory
func (s *CategoriesService) Update(
	ctx context.Context, categoryID string, update CategoryUpdate,
) (*Category, ServerKnowledge, error) {
	data, err := do[categoryResult](ctx, s.plan.client,
		http.MethodPatch, s.plan.path("categories", categoryID), nil, body{"category": update})
	if err != nil {
		return nil, 0, err
	}
	return data.Category, data.ServerKnowledge, nil
}

// Assign sets the budgeted amount for a category in the given month.
//
// Only the assigned amount can change through this operation; YNAB
// computes activity and available from it. Month accepts CurrentMonth.
//
// YNAB operationId: updateMonthCategory
func (s *CategoriesService) Assign(
	ctx context.Context, m Month, categoryID string, budgeted Milliunits,
) (*Category, ServerKnowledge, error) {
	if m.IsZero() {
		return nil, 0, zeroMonthError("Categories.Assign")
	}
	data, err := do[categoryResult](ctx, s.plan.client,
		http.MethodPatch, s.plan.path("months", m.String(), "categories", categoryID), nil,
		body{"category": map[string]any{"budgeted": budgeted}})
	if err != nil {
		return nil, 0, err
	}
	return data.Category, data.ServerKnowledge, nil
}

// categoryGroupResult is the shared save-category-group response payload.
type categoryGroupResult struct {
	CategoryGroup   *CategoryGroup  `json:"category_group"`
	ServerKnowledge ServerKnowledge `json:"server_knowledge"`
}

// CreateGroup adds a category group (HTTP 201). The name is bounded at 50
// characters by the API; longer names fail pre-flight as *ArgumentError.
//
// YNAB operationId: createCategoryGroup
func (s *CategoriesService) CreateGroup(ctx context.Context, name string) (*CategoryGroup, ServerKnowledge, error) {
	return s.saveGroup(ctx, "Categories.CreateGroup", http.MethodPost, s.plan.path("category_groups"), name)
}

// RenameGroup renames a category group. The same 50-character bound as
// CreateGroup applies pre-flight.
//
// YNAB operationId: updateCategoryGroup
func (s *CategoriesService) RenameGroup(
	ctx context.Context, groupID, name string,
) (*CategoryGroup, ServerKnowledge, error) {
	return s.saveGroup(ctx, "Categories.RenameGroup", http.MethodPatch, s.plan.path("category_groups", groupID), name)
}

// saveGroup validates the shared name bound and performs a group write.
func (s *CategoriesService) saveGroup(
	ctx context.Context, op, method, path, name string,
) (*CategoryGroup, ServerKnowledge, error) {
	if err := checkRuneMax(op, "name", name, categoryGroupNameMax); err != nil {
		return nil, 0, err
	}
	data, err := do[categoryGroupResult](ctx, s.plan.client, method, path, nil,
		nameOnlyBody("category_group", name))
	if err != nil {
		return nil, 0, err
	}
	return data.CategoryGroup, data.ServerKnowledge, nil
}
