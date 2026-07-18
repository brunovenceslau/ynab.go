package ynab

import (
	"context"
	"net/http"
)

// Payee is a plan payee. TransferAccountID is set only on transfer payees.
type Payee struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	TransferAccountID *string `json:"transfer_account_id"`
	Deleted           bool    `json:"deleted"`
}

// SyncID keys the payee for MergeByID.
func (p Payee) SyncID() string { return p.ID }

// IsDeleted reports a delta tombstone.
func (p Payee) IsDeleted() bool { return p.Deleted }

// payeeNameMax is the spec-stated bound on payee names.
const payeeNameMax = 500

// PayeesService reads and writes the plan's payees.
type PayeesService struct {
	plan *Plan
}

// List returns the plan's payees.
//
// YNAB operationId: getPayees
func (s *PayeesService) List(ctx context.Context, opts ...ListOption) ([]Payee, ServerKnowledge, error) {
	data, err := do[struct {
		Payees          []Payee         `json:"payees"`
		ServerKnowledge ServerKnowledge `json:"server_knowledge"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("payees"), applyListOptions(nil, opts), nil)
	if err != nil {
		return nil, 0, err
	}
	return data.Payees, data.ServerKnowledge, nil
}

// Create adds a payee (HTTP 201) and returns it with the new server
// knowledge. Names are bounded at 500 characters by the API; longer
// names fail pre-flight as *ArgumentError.
//
// YNAB operationId: createPayee
func (s *PayeesService) Create(ctx context.Context, name string) (*Payee, ServerKnowledge, error) {
	return s.save(ctx, "Payees.Create", http.MethodPost, s.plan.path("payees"), name)
}

// Get returns a single payee by id.
//
// YNAB operationId: getPayeeById
func (s *PayeesService) Get(ctx context.Context, payeeID string) (*Payee, error) {
	data, err := do[struct {
		Payee *Payee `json:"payee"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("payees", payeeID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.Payee, nil
}

// Rename changes a payee's name. The same 500-character bound as Create
// applies pre-flight.
//
// YNAB operationId: updatePayee
func (s *PayeesService) Rename(ctx context.Context, payeeID, name string) (*Payee, ServerKnowledge, error) {
	return s.save(ctx, "Payees.Rename", http.MethodPatch, s.plan.path("payees", payeeID), name)
}

// save validates the shared name bound and performs a payee write.
func (s *PayeesService) save(ctx context.Context, op, method, path, name string) (*Payee, ServerKnowledge, error) {
	if len(name) > payeeNameMax {
		return nil, 0, &ArgumentError{Op: op, Field: "name", Reason: "must be at most 500 characters"}
	}
	data, err := do[struct {
		Payee           *Payee          `json:"payee"`
		ServerKnowledge ServerKnowledge `json:"server_knowledge"`
	}](ctx, s.plan.client, method, path, nil, nameOnlyBody("payee", name))
	if err != nil {
		return nil, 0, err
	}
	return data.Payee, data.ServerKnowledge, nil
}
