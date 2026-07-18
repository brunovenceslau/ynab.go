package contract_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/contract"
)

const specPath = "../../openapi.yaml"

func TestContractScanSpec(t *testing.T) {
	t.Parallel()

	spec, err := contract.ScanSpec(specPath)
	require.NoError(t, err)

	require.Equal(t, contract.SpecVersion, spec.Version, "the vendored spec pin — restore openapi.yaml if this fails")
	require.Len(t, spec.Ops, 44)

	byID := map[string]contract.SpecOp{}
	for _, op := range spec.Ops {
		byID[op.ID] = op
	}

	// Spot-checks across every tuple field the scanner extracts.
	require.Equal(t, contract.SpecOp{ID: "getUser", Method: "GET", Path: "/user"}, byID["getUser"])
	require.Equal(t, "DELETE", byID["deleteScheduledTransaction"].Method)
	require.Equal(t, "/plans/{plan_id}/months/{month}/categories/{category_id}", byID["updateMonthCategory"].Path)
	require.Equal(t,
		[]string{"since_date", "until_date", "type", "last_knowledge_of_server"},
		byID["getTransactions"].QueryParams)
	require.Equal(t, []string{"include_accounts"}, byID["getPlans"].QueryParams)
	require.Empty(t, byID["createAccount"].QueryParams, "path params are not query params")

	// The delta-endpoint count the API notes ledger records: the spec
	// declares last_knowledge_of_server on 11 operations, not the
	// documentation prose's 9.
	deltas := 0
	for _, op := range spec.Ops {
		for _, p := range op.QueryParams {
			if p == "last_knowledge_of_server" {
				deltas++
			}
		}
	}
	require.Equal(t, 11, deltas)
}

func TestContractDiffHolds(t *testing.T) {
	t.Parallel()

	spec, err := contract.ScanSpec(specPath)
	require.NoError(t, err)
	require.Empty(t, contract.Diff(contract.Table(), spec))
}

func TestContractDiffCatchesMutations(t *testing.T) {
	t.Parallel()

	scan := func(t *testing.T) (*contract.Spec, []contract.Operation) {
		t.Helper()
		spec, err := contract.ScanSpec(specPath)
		require.NoError(t, err)
		return spec, contract.Table()
	}

	t.Run("row removed from table", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		mutated := table[1:]
		problems := contract.Diff(mutated, spec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "unimplemented operation")
	})

	t.Run("phantom row in table", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		mutated := append(slices.Clone(table), contract.Operation{ID: "getBudgetsLegacy", Method: "GET", Path: "/budgets"})
		problems := contract.Diff(mutated, spec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "phantom operation")
	})

	t.Run("wrong verb", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		table[0].Method = "POST"
		problems := contract.Diff(table, spec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "verb")
	})

	t.Run("wrong path", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		table[2].Path = "/budgets/{plan_id}"
		problems := contract.Diff(table, spec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "path")
	})

	t.Run("illegal query param", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		table[0].QueryParams = append(table[0].QueryParams, "page")
		problems := contract.Diff(table, spec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "illegal query param")
	})

	t.Run("missing query param", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		for i := range table {
			if table[i].ID == "getTransactions" {
				table[i].QueryParams = table[i].QueryParams[:1]
			}
		}
		problems := contract.Diff(table, spec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "missing query param")
	})

	t.Run("spec side mutation", func(t *testing.T) {
		t.Parallel()

		spec, table := scan(t)
		spec.Ops[0].Method = "POST"
		require.NotEmpty(t, contract.Diff(table, spec))
	})
}

func TestContractValidateDocLines(t *testing.T) {
	t.Parallel()

	table := contract.Table()

	t.Run("two-method row satisfies one operation", func(t *testing.T) {
		t.Parallel()

		found := map[string][]string{
			"createTransaction": {"TransactionsService.Create", "TransactionsService.CreateBatch"},
		}
		require.Empty(t, contract.ValidateDocLines(table, []string{"createTransaction"}, found))
	})

	t.Run("a listed method without its doc line is caught", func(t *testing.T) {
		t.Parallel()

		// One doc line satisfies the op's existence, but every method the
		// table lists must carry its own line — CreateBatch cannot hide
		// behind Create.
		found := map[string][]string{"createTransaction": {"TransactionsService.Create"}}
		problems := contract.ValidateDocLines(table, []string{"createTransaction"}, found)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "TransactionsService.CreateBatch carries no doc line")
	})

	t.Run("unknown operationId rejected", func(t *testing.T) {
		t.Parallel()

		found := map[string][]string{"getBudgets": {"Client.Budgets"}}
		problems := contract.ValidateDocLines(table, nil, found)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "unknown operationId")
	})

	t.Run("doc line on a method the table does not map", func(t *testing.T) {
		t.Parallel()

		found := map[string][]string{"getUser": {"Client.Whoami"}}
		problems := contract.ValidateDocLines(table, nil, found)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "the table maps it to")
	})

	t.Run("registered row without a doc line fails", func(t *testing.T) {
		t.Parallel()

		problems := contract.ValidateDocLines(table, []string{"getUser"}, nil)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "no doc-line-bearing method")
	})
}
