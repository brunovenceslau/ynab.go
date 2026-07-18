// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Gate G7's zero-dependency half: the non-test dependency closure of
// package ynab must be standard library only. testify exists solely in
// test files, which this closure excludes.

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZeroRuntimeDeps(t *testing.T) {
	t.Parallel()

	out, err := exec.CommandContext(t.Context(),
		"go", "list", "-deps", "pkg.venceslau.dev/ynab").CombinedOutput()
	require.NoError(t, err, "go list -deps: %s", out)

	var offenders []string
	for line := range strings.Lines(string(out)) {
		pkg := strings.TrimSpace(line)
		switch {
		case pkg == "":
		case strings.HasPrefix(pkg, "pkg.venceslau.dev/ynab"):
		case !strings.Contains(strings.SplitN(pkg, "/", 2)[0], "."):
			// Standard library import paths have no dot in their first
			// element (net/http, encoding/json); module paths always do.
		default:
			offenders = append(offenders, pkg)
		}
	}
	require.Empty(t, offenders, "package ynab must have zero runtime dependencies")
}
