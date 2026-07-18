package ynab

import (
	"context"
	"net/http"
)

// PayeeLocation is a geographic location for a payee. Latitude and
// longitude arrive as strings on the wire and are kept verbatim.
type PayeeLocation struct {
	ID        string `json:"id"`
	PayeeID   string `json:"payee_id"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Deleted   bool   `json:"deleted"`
}

// SyncID keys the location for MergeByID.
func (l PayeeLocation) SyncID() string { return l.ID }

// IsDeleted reports a delta tombstone.
func (l PayeeLocation) IsDeleted() bool { return l.Deleted }

// PayeeLocationsService reads the plan's payee locations. These endpoints
// have no delta support: no cursor is accepted and no server knowledge is
// returned.
type PayeeLocationsService struct {
	plan *Plan
}

// List returns all payee locations of the plan.
//
// YNAB operationId: getPayeeLocations
func (s *PayeeLocationsService) List(ctx context.Context) ([]PayeeLocation, error) {
	data, err := do[struct {
		PayeeLocations []PayeeLocation `json:"payee_locations"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("payee_locations"), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.PayeeLocations, nil
}

// Get returns a single payee location by id.
//
// YNAB operationId: getPayeeLocationById
func (s *PayeeLocationsService) Get(ctx context.Context, payeeLocationID string) (*PayeeLocation, error) {
	data, err := do[struct {
		PayeeLocation *PayeeLocation `json:"payee_location"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("payee_locations", payeeLocationID), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.PayeeLocation, nil
}

// ListByPayee returns the locations recorded for one payee.
//
// YNAB operationId: getPayeeLocationsByPayee
func (s *PayeeLocationsService) ListByPayee(ctx context.Context, payeeID string) ([]PayeeLocation, error) {
	data, err := do[struct {
		PayeeLocations []PayeeLocation `json:"payee_locations"`
	}](ctx, s.plan.client, http.MethodGet, s.plan.path("payees", payeeID, "payee_locations"), nil, nil)
	if err != nil {
		return nil, err
	}
	return data.PayeeLocations, nil
}
