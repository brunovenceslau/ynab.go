// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package contract

import "net/http"

// ReadCaseInfo summarizes one registered G4 read case for the
// read-side completeness check.
type ReadCaseInfo struct {
	OpID string
}

// DiffReadCoverage checks the G4 registry against the given operation
// ids (the table's, in the gates): every GET operation needs at least
// one read case, so an op cannot skip the header-stripped harness.
// (Write ops replay through G2 instead.)
func DiffReadCoverage(table []Operation, ids []string, cases []ReadCaseInfo) []string {
	var problems []string

	byID := make(map[string]Operation, len(table))
	for _, op := range table {
		byID[op.ID] = op
	}
	count := map[string]int{}
	for _, c := range cases {
		count[c.OpID]++
	}

	for _, id := range ids {
		row, ok := byID[id]
		if !ok || row.Method != http.MethodGet {
			continue
		}
		if count[id] == 0 {
			problems = append(problems, "implemented read operation "+id+" has no G4 read case")
		}
	}
	return problems
}
