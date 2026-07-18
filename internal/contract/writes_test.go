package contract_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/contract"
)

func TestContractDiffJSON(t *testing.T) {
	t.Parallel()

	t.Run("equivalent modulo key order", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, contract.DiffJSON(
			[]byte(`{"a":1,"b":{"c":true}}`),
			[]byte(`{"b":{"c":true},"a":1}`),
		))
	})

	t.Run("silently omitted Optional is a missing key", func(t *testing.T) {
		t.Parallel()

		problems := contract.DiffJSON(
			[]byte(`{"transaction":{"amount":-220,"approved":false}}`),
			[]byte(`{"transaction":{"amount":-220}}`),
		)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "missing key $.transaction.approved")
	})

	t.Run("extra emitted key", func(t *testing.T) {
		t.Parallel()

		problems := contract.DiffJSON(
			[]byte(`{"a":1}`),
			[]byte(`{"a":1,"debug":true}`),
		)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "unexpected key $.debug")
	})

	t.Run("value drift", func(t *testing.T) {
		t.Parallel()

		problems := contract.DiffJSON([]byte(`{"a":1}`), []byte(`{"a":2}`))
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "values differ")
	})

	t.Run("int64 precision survives", func(t *testing.T) {
		t.Parallel()

		// float64 would collapse these two; json.Number must not.
		problems := contract.DiffJSON(
			[]byte(`{"milliunits":9007199254740993}`),
			[]byte(`{"milliunits":9007199254740992}`),
		)
		require.NotEmpty(t, problems)
	})

	t.Run("nested array keys", func(t *testing.T) {
		t.Parallel()

		problems := contract.DiffJSON(
			[]byte(`{"transactions":[{"amount":1},{"amount":2,"memo":"m"}]}`),
			[]byte(`{"transactions":[{"amount":1},{"amount":2}]}`),
		)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "missing key $.transactions[1].memo")
	})

	t.Run("invalid emitted JSON", func(t *testing.T) {
		t.Parallel()

		problems := contract.DiffJSON([]byte(`{"a":1}`), []byte(`{`))
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "emitted JSON invalid")
	})
}

func TestContractDiffWriteCoverage(t *testing.T) {
	t.Parallel()

	table := contract.Table()

	t.Run("GET operations are outside G2", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, contract.DiffWriteCoverage(table, []string{"getUser"}, nil))
	})

	t.Run("implemented write op without a case", func(t *testing.T) {
		t.Parallel()

		problems := contract.DiffWriteCoverage(table, []string{"createAccount"}, nil)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "createAccount has no G2 write case")
	})

	t.Run("body-carrying op needs a body case", func(t *testing.T) {
		t.Parallel()

		cases := []contract.WriteCaseInfo{{OpID: "createAccount", HasBody: false}}
		problems := contract.DiffWriteCoverage(table, []string{"createAccount"}, cases)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "has no body case")
	})

	t.Run("createTransaction needs exactly two body cases", func(t *testing.T) {
		t.Parallel()

		one := []contract.WriteCaseInfo{{OpID: "createTransaction", HasBody: true}}
		problems := contract.DiffWriteCoverage(table, []string{"createTransaction"}, one)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "exactly 2 body cases")

		two := []contract.WriteCaseInfo{
			{OpID: "createTransaction", HasBody: true},
			{OpID: "createTransaction", HasBody: true},
		}
		require.Empty(t, contract.DiffWriteCoverage(table, []string{"createTransaction"}, two))
	})

	t.Run("bodiless op must stay bodiless", func(t *testing.T) {
		t.Parallel()

		withBody := []contract.WriteCaseInfo{{OpID: "deleteTransaction", HasBody: true}}
		problems := contract.DiffWriteCoverage(table, []string{"deleteTransaction"}, withBody)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "bodiless on the wire")

		clean := []contract.WriteCaseInfo{{OpID: "deleteTransaction", HasBody: false}}
		require.Empty(t, contract.DiffWriteCoverage(table, []string{"deleteTransaction"}, clean))
	})

	t.Run("satisfied registry is clean", func(t *testing.T) {
		t.Parallel()

		cases := []contract.WriteCaseInfo{
			{OpID: "createPayee", HasBody: true},
			{OpID: "updatePayee", HasBody: true},
		}
		require.Empty(t, contract.DiffWriteCoverage(table, []string{"createPayee", "updatePayee", "getPayees"}, cases))
	})
}
