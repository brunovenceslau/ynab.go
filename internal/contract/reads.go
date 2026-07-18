package contract

import "net/http"

// ReadCaseInfo summarizes one registered G4 endpoint case for the
// read-side completeness check.
type ReadCaseInfo struct {
	OpID string
}

// DiffReadCoverage checks the G4 registry against the G1 implemented
// registry: every implemented GET operation needs at least one endpoint
// case, so a slice cannot mark an op implemented while skipping the
// header-stripped harness. (Write ops replay through G2 instead.)
func DiffReadCoverage(table []Operation, implemented []string, cases []ReadCaseInfo) []string {
	var problems []string

	byID := make(map[string]Operation, len(table))
	for _, op := range table {
		byID[op.ID] = op
	}
	count := map[string]int{}
	for _, c := range cases {
		count[c.OpID]++
	}

	for _, id := range implemented {
		row, ok := byID[id]
		if !ok || row.Method != http.MethodGet {
			continue
		}
		if count[id] == 0 {
			problems = append(problems, "implemented read operation "+id+" has no G4 endpoint case")
		}
	}
	return problems
}
