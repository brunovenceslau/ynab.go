// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package main

// The mock example is fully offline and deterministic, so unlike the
// live examples it can be executed and verified — its output is pinned.

func Example_run() {
	if err := run(); err != nil {
		panic(err)
	}
	// Output:
	// server_knowledge=42
	// Checking   $128.42 (128422 milliunits)
	// Savings    $900.00 (900000 milliunits)
}
