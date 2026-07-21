// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

//go:build integration

package ynab_test

// The formal contract loop: every 2xx response the live suite records
// is validated against the vendored OpenAPI schema for its operation
// and status — required members, types, enums, formats, nullability.
// The decode gates prove responses fit OUR models; this proves they fit
// the PUBLISHED contract, catching both server drift the models happen
// to tolerate and models more permissive than the spec.

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specConformanceAllowlist names (operationId, status) pairs the spec
// itself does not describe, each with the live evidence. Empty by
// evidence: the first full validation run covered all 76 recorded
// responses — every (operation, status) pair the suite generates,
// error statuses included, is declared and conformant. A pair that
// gains spec coverage fails the freshness check below.
var specConformanceAllowlist = map[string]string{}

// checkSpecConformance validates every recorded response against the
// vendored spec. Violations are collected and asserted once; a spec
// hole (status undeclared for the operation) must be allowlisted.
func checkSpecConformance(t *testing.T, calls []recordedCall) {
	t.Helper()
	ctx := context.Background()

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("openapi.yaml")
	require.NoError(t, err, "the vendored spec must load")
	require.NoError(t, doc.Validate(ctx), "the vendored spec must be internally valid")
	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	var violations []string
	validated := 0
	for _, rc := range calls {
		if rc.status == 0 || len(rc.body) == 0 {
			continue
		}
		// Organic throttles and server errors may ride edge layers that
		// answer outside the spec — the response-invariant checks exempt
		// them for the same reason.
		if rc.status == http.StatusTooManyRequests || rc.status >= 500 {
			continue
		}
		req, rerr := http.NewRequestWithContext(ctx, rc.method,
			"https://api.ynab.com"+rc.path, nil)
		if rerr != nil {
			continue
		}
		req.URL.RawQuery = rc.query.Encode()
		route, pathParams, rerr := router.FindRoute(req)
		if rerr != nil {
			violations = append(violations,
				rc.method+" "+rc.path+": no spec route ("+rerr.Error()+")")
			continue
		}
		key := operationStatusKey(route, rc.status)
		if _, ok := specConformanceAllowlist[key]; ok {
			continue
		}
		input := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
				Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
			},
			Status: rc.status,
			Header: http.Header{"Content-Type": []string{rc.contentType}},
		}
		input.SetBodyBytes(rc.body)
		if verr := openapi3filter.ValidateResponse(ctx, input); verr != nil {
			violations = append(violations, key+": "+compactErr(verr))
			continue
		}
		validated++
	}
	assert.Empty(t, violations,
		"live responses violating the vendored OpenAPI contract — "+
			"real divergences go to API_NOTES.md, spec holes to the allowlist")
	checkConformanceAllowlistFresh(t, calls)
	t.Logf("spec conformance: %d responses validated against the vendored schema", validated)
}

// checkConformanceAllowlistFresh fails on allowlist rot: an entry whose
// (operation, status) pair never occurred in this run's traffic.
func checkConformanceAllowlistFresh(t *testing.T, calls []recordedCall) {
	t.Helper()
	seen := map[string]struct{}{}
	for _, rc := range calls {
		if op, ok := matchOperation(rc); ok {
			seen[op.ID+" "+strconv.Itoa(rc.status)] = struct{}{}
		}
	}
	for key, reason := range specConformanceAllowlist {
		if _, ok := seen[key]; !ok {
			assert.Fail(t, "stale conformance allowlist entry",
				"%s never occurred in this run (reason was: %s)", key, reason)
		}
	}
}

// operationStatusKey names a validation subject: "<operationId> <status>".
func operationStatusKey(route *routers.Route, status int) string {
	id := route.Operation.OperationID
	if id == "" {
		id = route.Method + " " + route.Path
	}
	return id + " " + strconv.Itoa(status)
}

// compactErr flattens kin-openapi's verbose multiline errors to one line.
func compactErr(err error) string {
	s := strings.Join(strings.Fields(err.Error()), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
