// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// The enum nets: every Valid() table pinned against its const block. The
// member lists are AST-derived from the package source, so a new const
// missing from Valid(), or a new enum type missing from the dispatch
// below, fails loudly by name instead of drifting.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// enumValidDispatch is the one remaining hand list — type names, not
// members — mapping every string enum bearing a Valid method to it.
var enumValidDispatch = map[string]func(string) bool{
	"AccountType":         func(s string) bool { return ynab.AccountType(s).Valid() },
	"AccountSpecType":     func(s string) bool { return ynab.AccountSpecType(s).Valid() },
	"ClearedStatus":       func(s string) bool { return ynab.ClearedStatus(s).Valid() },
	"FlagColor":           func(s string) bool { return ynab.FlagColor(s).Valid() },
	"TransactionType":     func(s string) bool { return ynab.TransactionType(s).Valid() },
	"HybridType":          func(s string) bool { return ynab.HybridType(s).Valid() },
	"DebtTransactionType": func(s string) bool { return ynab.DebtTransactionType(s).Valid() },
	"GoalType":            func(s string) bool { return ynab.GoalType(s).Valid() },
	"GoalFrequency":       func(s string) bool { return ynab.GoalFrequency(s).Valid() },
	"Frequency":           func(s string) bool { return ynab.Frequency(s).Valid() },
}

// TestEnumValidTables covers every enum's Valid across its full declared
// value set: the members come from the const blocks via the AST scan, so
// the test cannot hand-repeat (and silently drift from) the source.
func TestEnumValidTables(t *testing.T) {
	t.Parallel()

	enums := scanEnumConsts(t)
	require.NotEmpty(t, enums, "the AST scan must find the enum const blocks")

	// The dispatch must be complete in both directions: a new enum type
	// without an entry fails here by name, as does a stale entry.
	for name := range enums {
		require.Contains(t, enumValidDispatch, name,
			"enum %s has a Valid method but no dispatch entry — add it to enumValidDispatch", name)
	}
	for name := range enumValidDispatch {
		members, ok := enums[name]
		require.True(t, ok, "dispatch entry %s matches no scanned enum const block", name)
		require.NotEmpty(t, members, "enum %s declares no const members", name)
	}

	for name, members := range enums {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			valid := enumValidDispatch[name]
			for _, m := range members {
				require.True(t, valid(m), "declared const %q must satisfy %s.Valid", m, name)
			}
			require.False(t, valid("__nope__"), "%s.Valid must reject undeclared values", name)
		})
	}
}

// scanEnumConsts parses the root package (non-test files, the same walk
// as scanDocLines) and returns, per exported string-typed enum bearing a
// Valid method, the string values of its declared constants.
func scanEnumConsts(t *testing.T) map[string][]string {
	t.Helper()

	files := parseRootPackage(t)
	enumTypes := scanEnumTypes(files)

	enums := map[string][]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			d, ok := decl.(*ast.GenDecl)
			if ok && d.Tok == token.CONST {
				collectEnumConstDecl(t, d, enumTypes, enums)
			}
		}
	}
	return enums
}

// parseRootPackage parses every non-test .go file of the root package.
func parseRootPackage(t *testing.T) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		files = append(files, file)
	}
	return files
}

// scanEnumTypes returns the enum type names: exported string-typed type
// declarations intersected with Valid-method receiver types.
func scanEnumTypes(files []*ast.File) map[string]bool {
	stringTypes := map[string]bool{}
	hasValid := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				collectStringTypeDecl(d, stringTypes)
			case *ast.FuncDecl:
				if d.Name.Name == "Valid" && d.Recv != nil && len(d.Recv.List) == 1 {
					if ident, ok := d.Recv.List[0].Type.(*ast.Ident); ok {
						hasValid[ident.Name] = true
					}
				}
			}
		}
	}

	enumTypes := map[string]bool{}
	for name := range stringTypes {
		if hasValid[name] {
			enumTypes[name] = true
		}
	}
	return enumTypes
}

// collectStringTypeDecl records d's exported `type X string` declarations.
func collectStringTypeDecl(d *ast.GenDecl, stringTypes map[string]bool) {
	if d.Tok != token.TYPE {
		return
	}
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || !ts.Name.IsExported() {
			continue
		}
		if ident, ok := ts.Type.(*ast.Ident); ok && ident.Name == "string" {
			stringTypes[ts.Name.Name] = true
		}
	}
}

// collectEnumConstDecl appends the unquoted values of d's const specs
// typed as one of the enum types.
func collectEnumConstDecl(t *testing.T, d *ast.GenDecl, enumTypes map[string]bool, enums map[string][]string) {
	t.Helper()

	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		ident, ok := vs.Type.(*ast.Ident)
		if !ok || !enumTypes[ident.Name] {
			continue
		}
		for _, v := range vs.Values {
			lit, ok := v.(*ast.BasicLit)
			require.True(t, ok, "enum %s const must be a basic literal", ident.Name)
			require.Equal(t, token.STRING, lit.Kind)
			value, err := strconv.Unquote(lit.Value)
			require.NoError(t, err)
			enums[ident.Name] = append(enums[ident.Name], value)
		}
	}
}
