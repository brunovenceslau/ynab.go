package contract

import (
	"maps"
	"slices"
)

// implemented records which operationIds have landed. Service slices
// append via MarkImplemented from their test files' init functions; the
// doc-line contract test validates every registered row. Task 31 flips
// the strict 44/44 assertion once the last slice lands.
var implemented = map[string]bool{}

// MarkImplemented registers operationIds as implemented.
func MarkImplemented(ids ...string) {
	for _, id := range ids {
		implemented[id] = true
	}
}

// ImplementedIDs returns the registered operationIds, sorted.
func ImplementedIDs() []string {
	return slices.Sorted(maps.Keys(implemented))
}
