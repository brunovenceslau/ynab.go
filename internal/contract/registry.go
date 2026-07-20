// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package contract

import (
	"maps"
	"slices"
	"sync"
)

// implemented records which operationIds have landed. Service slices
// append via MarkImplemented from their test files' init functions; the
// doc-line contract test validates every registered row. Task 31 flips
// the strict 44/44 assertion once the last slice lands.
var (
	implementedMu sync.Mutex
	implemented   = map[string]struct{}{}
)

// MarkImplemented registers operationIds as implemented.
func MarkImplemented(ids ...string) {
	implementedMu.Lock()
	defer implementedMu.Unlock()
	for _, id := range ids {
		implemented[id] = struct{}{}
	}
}

// ImplementedIDs returns the registered operationIds, sorted.
func ImplementedIDs() []string {
	implementedMu.Lock()
	defer implementedMu.Unlock()
	return slices.Sorted(maps.Keys(implemented))
}
