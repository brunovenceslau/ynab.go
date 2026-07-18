package ynab_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/contract"
)

// TestContractSpecDiff is gate G1: the 44-row operation table diffs clean
// against the vendored spec in both directions — no unimplemented op, no
// phantom op, no verb/path drift, no illegal query param.
func TestContractSpecDiff(t *testing.T) {
	t.Parallel()

	spec, err := contract.ScanSpec("openapi.yaml")
	require.NoError(t, err)
	require.Equal(t, contract.SpecVersion, spec.Version,
		"vendored spec must stay pinned; if update-spec ran, git checkout -- openapi.yaml")
	require.Len(t, spec.Ops, 44)
	require.Len(t, contract.Table(), 44)

	require.Empty(t, contract.Diff(contract.Table(), spec))
}

// TestContractDocLines scans the root package for `// YNAB operationId:`
// trailing doc lines and validates them against the table and the
// implemented registry.
func TestContractDocLines(t *testing.T) {
	t.Parallel()

	found := scanDocLines(t)
	require.Empty(t, contract.ValidateDocLines(contract.Table(), contract.ImplementedIDs(), found))
}

// scanDocLines parses the root package (non-test files) and returns
// operationId → methods bearing its doc line, methods named as
// "ReceiverType.Method".
func scanDocLines(t *testing.T) map[string][]string {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	require.NoError(t, err)

	pkg, ok := pkgs["ynab"]
	require.True(t, ok, "root package parses")

	const marker = "// YNAB operationId: "
	found := map[string][]string{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Doc != nil {
				for _, c := range fn.Doc.List {
					if rest, ok := strings.CutPrefix(c.Text, marker); ok {
						id := strings.TrimSpace(rest)
						found[id] = append(found[id], methodName(fn))
					}
				}
			}
		}
	}
	return found
}

// methodName renders a FuncDecl as "ReceiverType.Method" (or just the
// function name when there is no receiver).
func methodName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if ident, ok := recv.(*ast.Ident); ok {
		return ident.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}
