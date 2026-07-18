package contract

import (
	"fmt"
	"slices"
)

// ValidateDocLines cross-checks the `// YNAB operationId:` doc lines found
// in the root package against the table and the implemented registry:
// every found id must be a table row and every found method must belong to
// that row; every registered row must have at least one doc-line-bearing
// method. The op→method relation is 1:N — never a bijection.
func ValidateDocLines(table []Operation, implemented []string, found map[string][]string) []string {
	var problems []string

	byID := make(map[string]Operation, len(table))
	for _, op := range table {
		byID[op.ID] = op
	}

	for id, methods := range found {
		row, ok := byID[id]
		if !ok {
			problems = append(problems, fmt.Sprintf("doc line names unknown operationId %s (methods %v)", id, methods))
			continue
		}
		for _, m := range methods {
			if !slices.Contains(row.GoMethods, m) {
				problems = append(problems, fmt.Sprintf("%s: doc line on %s, but the table maps it to %v", id, m, row.GoMethods))
			}
		}
	}

	for _, id := range implemented {
		if _, ok := byID[id]; !ok {
			problems = append(problems, fmt.Sprintf("registered operationId %s has no table row", id))
			continue
		}
		if len(found[id]) == 0 {
			problems = append(problems, fmt.Sprintf("registered operationId %s has no doc-line-bearing method", id))
		}
	}
	return problems
}
