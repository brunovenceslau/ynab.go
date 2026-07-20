// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package contract

import (
	"fmt"
	"net/http"
	"slices"
)

// DiffSpec compares the coverage table against the scanned spec both ways and
// returns every discrepancy in human-readable form. An empty result means
// the contract holds: same operation set, same verb and path per
// operation, same query-parameter set per operation.
func DiffSpec(table []Operation, spec *Spec) []string {
	var problems []string

	byIDTable := make(map[string]Operation, len(table))
	for _, op := range table {
		if _, dup := byIDTable[op.ID]; dup {
			problems = append(problems, "table: duplicate operationId "+op.ID)
		}
		byIDTable[op.ID] = op
	}
	byIDSpec := make(map[string]SpecOp, len(spec.Ops))
	for _, op := range spec.Ops {
		if _, dup := byIDSpec[op.ID]; dup {
			problems = append(problems, "spec: duplicate operationId "+op.ID)
		}
		byIDSpec[op.ID] = op
	}

	for _, op := range table {
		specOp, ok := byIDSpec[op.ID]
		if !ok {
			problems = append(problems, fmt.Sprintf("phantom operation: table has %s, spec does not", op.ID))
			continue
		}
		if op.Method != specOp.Method {
			problems = append(problems, fmt.Sprintf("%s: table verb %s, spec verb %s", op.ID, op.Method, specOp.Method))
		}
		if op.Path != specOp.Path {
			problems = append(problems, fmt.Sprintf("%s: table path %s, spec path %s", op.ID, op.Path, specOp.Path))
		}
		// The bodilessOps declaration stays the readable local truth, but
		// it must agree with the spec's requestBody presence on every
		// non-GET operation.
		if op.Method != http.MethodGet && specOp.HasBody == isBodiless(op.ID) {
			problems = append(problems, fmt.Sprintf(
				"%s: spec requestBody presence %v contradicts bodilessOps (bodiless=%v)",
				op.ID, specOp.HasBody, isBodiless(op.ID)))
		}
		problems = append(problems, diffParams(op.ID, op.QueryParams, specOp.QueryParams)...)
	}
	for _, specOp := range spec.Ops {
		if _, ok := byIDTable[specOp.ID]; !ok {
			problems = append(problems, fmt.Sprintf("unimplemented operation: spec has %s, table does not", specOp.ID))
		}
	}
	return problems
}

// diffParams compares query-parameter sets in both directions: a table
// param the spec lacks is an illegal parameter; a spec param the table
// lacks is missing coverage.
func diffParams(opID string, table, spec []string) []string {
	var problems []string
	for _, p := range table {
		if !slices.Contains(spec, p) {
			problems = append(problems, fmt.Sprintf("%s: illegal query param %q (not in spec)", opID, p))
		}
	}
	for _, p := range spec {
		if !slices.Contains(table, p) {
			problems = append(problems, fmt.Sprintf("%s: missing query param %q (spec declares it)", opID, p))
		}
	}
	return problems
}
