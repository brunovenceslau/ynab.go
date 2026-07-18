package ynab

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pkg.venceslau.dev/ynab/internal/transport"
)

// PlanID identifies a plan. Besides concrete UUIDs, the API accepts two
// literals, PlanIDLastUsed and PlanIDDefault.
type PlanID string

const (
	// PlanIDLastUsed resolves server-side to the last used plan.
	PlanIDLastUsed PlanID = "last-used"
	// PlanIDDefault resolves server-side to the default plan (OAuth
	// default-plan selection; equivalent to last-used otherwise).
	PlanIDDefault PlanID = "default"
)

// Plan is the handle every plan-scoped operation hangs off:
// client.Plan(id).Accounts.List(ctx), and so on for each service field.
// Client.Plan performs no I/O. The zero Plan is unusable by design —
// handles come only from Client.Plan, so the bound id can never drift
// from its services.
type Plan struct {
	id     PlanID
	client *Client

	// Accounts reads and creates the plan's accounts.
	Accounts *AccountsService
	// Categories reads and writes the plan's categories.
	Categories *CategoriesService
	// Months reads the plan's months.
	Months *MonthsService
	// Payees reads and writes the plan's payees.
	Payees *PayeesService
	// PayeeLocations reads the plan's payee locations (no delta support).
	PayeeLocations *PayeeLocationsService
	// MoneyMovements reads the plan's money movements.
	MoneyMovements *MoneyMovementsService
	// Transactions reads and writes the plan's transactions.
	Transactions *TransactionsService
	// Scheduled reads and writes the plan's scheduled transactions.
	Scheduled *ScheduledTransactionsService
}

// Plan returns the handle for id. No I/O happens; the id is validated by
// the server on first use.
func (c *Client) Plan(id PlanID) *Plan {
	p := &Plan{id: id, client: c}
	p.Accounts = &AccountsService{plan: p}
	p.Categories = &CategoriesService{plan: p}
	p.Months = &MonthsService{plan: p}
	p.Payees = &PayeesService{plan: p}
	p.PayeeLocations = &PayeeLocationsService{plan: p}
	p.MoneyMovements = &MoneyMovementsService{plan: p}
	p.Transactions = &TransactionsService{plan: p}
	p.Scheduled = &ScheduledTransactionsService{plan: p}
	return p
}

// ID returns the plan id this handle is bound to.
func (p *Plan) ID() PlanID {
	return p.id
}

// path builds a plan-scoped API path with escaped segments.
func (p *Plan) path(segments ...string) string {
	return transport.JoinPath(append([]string{"plans", string(p.id)}, segments...)...)
}

// User is the authenticated user.
type User struct {
	ID string `json:"id"`
}

// DateFormat is the plan-level date rendering metadata. The object can be
// null at its use sites, which model it as *DateFormat.
type DateFormat struct {
	Format string `json:"format"`
}

// planCore holds the scalar plan fields PlanSummary and PlanDetail
// share. The two types deliberately do not embed one another: their
// collection element types differ (full models vs export *Base shapes),
// so each declares its own correctly-typed collections around this core.
type planCore struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	LastModifiedOn time.Time       `json:"last_modified_on"`
	FirstMonth     Month           `json:"first_month"`
	LastMonth      Month           `json:"last_month"`
	DateFormat     *DateFormat     `json:"date_format"`
	CurrencyFormat *CurrencyFormat `json:"currency_format"`
}

// PlanSummary is one plan in a plan list. DateFormat and CurrencyFormat
// are null when unavailable.
type PlanSummary struct {
	planCore

	// Accounts is populated only when Plans is called with
	// IncludeAccounts; otherwise the key is absent and the slice nil.
	Accounts []Account `json:"accounts"`
}

// PlanDetail is the full-plan export Plan.Export returns: every
// collection at once, entities in their export *Base shapes. Category
// groups arrive flat here — their Categories slices stay nil because the
// categories collection carries every category directly.
type PlanDetail struct {
	planCore

	Accounts                 []AccountBase                     `json:"accounts"`
	Payees                   []Payee                           `json:"payees"`
	PayeeLocations           []PayeeLocation                   `json:"payee_locations"`
	CategoryGroups           []CategoryGroup                   `json:"category_groups"`
	Categories               []CategoryBase                    `json:"categories"`
	Months                   []MonthDetailBase                 `json:"months"`
	Transactions             []TransactionSummaryBase          `json:"transactions"`
	Subtransactions          []SubTransactionBase              `json:"subtransactions"`
	ScheduledTransactions    []ScheduledTransactionSummaryBase `json:"scheduled_transactions"`
	ScheduledSubtransactions []ScheduledSubTransactionBase     `json:"scheduled_subtransactions"`
}

// PlanList is the result of Client.Plans. DefaultPlan is null unless the
// token's OAuth grant selected a default plan.
type PlanList struct {
	Plans       []PlanSummary `json:"plans"`
	DefaultPlan *PlanSummary  `json:"default_plan"`
}

// PlanSettings are the plan's rendering settings; both members are null
// when unavailable.
type PlanSettings struct {
	DateFormat     *DateFormat     `json:"date_format"`
	CurrencyFormat *CurrencyFormat `json:"currency_format"`
}

// PlansOption tunes Client.Plans.
type PlansOption struct {
	apply func(url.Values)
}

// IncludeAccounts asks the server to embed each plan's accounts in the
// summaries.
func IncludeAccounts() PlansOption {
	return PlansOption{apply: func(q url.Values) {
		q.Set("include_accounts", "true")
	}}
}

// User returns the authenticated user.
//
// YNAB operationId: getUser
func (c *Client) User(ctx context.Context) (*User, error) {
	data, err := do[struct {
		User *User `json:"user"`
	}](ctx, c, http.MethodGet, "user", nil, nil)
	if err != nil {
		return nil, err
	}
	return data.User, nil
}

// Plans lists the plans the token can reach.
//
// YNAB operationId: getPlans
func (c *Client) Plans(ctx context.Context, opts ...PlansOption) (*PlanList, error) {
	var q url.Values
	for _, opt := range opts {
		if opt.apply == nil {
			continue
		}
		if q == nil {
			q = url.Values{}
		}
		opt.apply(q)
	}

	data, err := do[PlanList](ctx, c, http.MethodGet, "plans", q, nil)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// Export returns the full plan — every collection in one request. With
// Since, only entities changed after the cursor are returned, deletions
// arriving as tombstones inside their collections.
//
// YNAB operationId: getPlanById
func (p *Plan) Export(ctx context.Context, opts ...ListOption) (*PlanDetail, ServerKnowledge, error) {
	data, err := do[struct {
		Plan            *PlanDetail     `json:"plan"`
		ServerKnowledge ServerKnowledge `json:"server_knowledge"`
	}](ctx, p.client, http.MethodGet, transport.JoinPath("plans", string(p.id)), applyListOptions(nil, opts), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Plan, data.ServerKnowledge, nil
}

// Delta is the one-request full-plan delta: an Export since st's plan
// cursor, advancing *st in place on success (st.Plan moves to the new
// server knowledge; the per-service cursors are separate spaces and stay
// untouched). A zero st.Plan performs the initial full read. st must
// carry this plan's id — reusing another plan's cursor would silently
// corrupt a local store, so it is rejected pre-flight.
func (p *Plan) Delta(ctx context.Context, st *SyncState) (*PlanDetail, error) {
	if st == nil {
		return nil, &ArgumentError{Op: "Plan.Delta", Field: "st", Reason: "sync state must not be nil"}
	}
	if st.PlanID != "" && st.PlanID != p.id {
		return nil, &ArgumentError{
			Op: "Plan.Delta", Field: "st",
			Reason: "sync state belongs to plan " + string(st.PlanID) + ", not " + string(p.id),
		}
	}

	var opts []ListOption
	if st.Plan != 0 {
		opts = append(opts, Since(st.Plan))
	}
	detail, sk, err := p.Export(ctx, opts...)
	if err != nil {
		return nil, err
	}
	st.PlanID = p.id
	st.Plan = sk
	return detail, nil
}

// Settings returns the plan's date and currency settings.
//
// YNAB operationId: getPlanSettingsById
func (p *Plan) Settings(ctx context.Context) (*PlanSettings, error) {
	data, err := do[struct {
		Settings *PlanSettings `json:"settings"`
	}](ctx, p.client, http.MethodGet, p.path("settings"), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.Settings, nil
}

// nameOnlyBody builds the {"<wrapper>":{"name":...}} payload the group
// and payee writes share.
func nameOnlyBody(wrapper, name string) body {
	return body{wrapper: map[string]any{"name": name}}
}

// do funnels every operation through the config-error contract, the
// empty-segment guard, and the transport pipeline.
func do[T any](ctx context.Context, c *Client, method, path string, q url.Values, body any) (T, error) {
	var zero T
	if err := c.configError(); err != nil {
		return zero, err
	}
	// An empty id argument would collapse a path segment and silently
	// shift the request onto a different route (Plan("").Settings would
	// hit the plans list). Reject it before any I/O instead.
	if path == "" || strings.HasSuffix(path, "/") || strings.Contains(path, "//") {
		return zero, &ArgumentError{Op: method + " " + path, Reason: "an id argument is empty"}
	}
	return transport.Do[T](ctx, c.core, method, path, q, body)
}
