package contract

import (
	"fmt"
	"net/http"
)

// WriteCaseInfo summarizes one registered G2 write case for the
// completeness check.
type WriteCaseInfo struct {
	OpID    string
	HasBody bool
}

// bodilessOps are the three non-GET operations that carry no request body
// (wire truth from the vendored spec).
var bodilessOps = map[string]bool{
	"deleteTransaction":          true,
	"deleteScheduledTransaction": true,
	"importTransactions":         true,
}

// DiffWriteCoverage checks the G2 registry against the G1 implemented
// registry: every implemented non-GET operation needs at least one case;
// every body-carrying one needs at least one body case; createTransaction
// needs exactly two body cases (single and batch) while remaining one G1
// row.
func DiffWriteCoverage(table []Operation, implemented []string, cases []WriteCaseInfo) []string {
	var problems []string

	byID := make(map[string]Operation, len(table))
	for _, op := range table {
		byID[op.ID] = op
	}

	total := map[string]int{}
	bodies := map[string]int{}
	for _, c := range cases {
		total[c.OpID]++
		if c.HasBody {
			bodies[c.OpID]++
		}
	}

	for _, id := range implemented {
		row, ok := byID[id]
		if !ok || row.Method == http.MethodGet {
			continue // unknown ids are the doc-line check's problem; GETs are not G2's
		}
		if total[id] == 0 {
			problems = append(problems, "implemented write operation "+id+" has no G2 write case")
			continue
		}
		switch {
		case id == "createTransaction":
			if bodies[id] != 2 {
				problems = append(problems, fmt.Sprintf(
					"createTransaction needs exactly 2 body cases (single+batch), has %d", bodies[id]))
			}
		case bodilessOps[id]:
			if bodies[id] != 0 {
				problems = append(problems, id+" is bodiless on the wire but has a body case")
			}
		case bodies[id] == 0:
			problems = append(problems, "body-carrying operation "+id+" has no body case")
		}
	}
	return problems
}
