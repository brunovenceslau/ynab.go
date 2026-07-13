package account_test

import (
	"net/http"
	"testing"

	"pkg.venceslau.dev/ynab/api"

	"github.com/stretchr/testify/assert"
	"gopkg.in/jarcoal/httpmock.v1"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/api/account"
)

func TestService_GetAccounts(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/bbdccdb0-9007-42aa-a6fe-02a3e94476be/accounts"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {

			res := httpmock.NewStringResponse(200, `{
  "data": {
    "accounts": [
			{
				"id": "aa248caa-eed7-4575-a990-717386438d2c",
				"name": "Test Account 2",
				"type": "savings",
				"on_budget": false,
				"closed": true,
				"note": "omg omg omg",
				"balance": -123930,
				"cleared_balance": -123930,
				"uncleared_balance": 0,
				"deleted": false
			}
    ],
    "server_knowledge": 10
  }
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	f := &api.Filter{LastKnowledgeOfServer: 10}
	snapshot, err := client.Account().GetAccounts("bbdccdb0-9007-42aa-a6fe-02a3e94476be", f)
	assert.NoError(t, err)

	note := "omg omg omg"
	expected := &account.SearchResultSnapshot{
		Accounts: []*account.Account{
			{
				ID:               "aa248caa-eed7-4575-a990-717386438d2c",
				Name:             "Test Account 2",
				Type:             account.TypeSavings,
				OnBudget:         false,
				Closed:           true,
				Note:             &note,
				Balance:          int64(-123930),
				ClearedBalance:   int64(-123930),
				UnclearedBalance: int64(0),
				Deleted:          false,
			},
		},
		ServerKnowledge: 10,
	}
	assert.Equal(t, expected, snapshot)
}

func TestService_GetAccount(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://api.youneedabudget.com/v1/budgets/bbdccdb0-9007-42aa-a6fe-02a3e94476be/accounts/aa248caa-eed7-4575-a990-717386438d2c"
	httpmock.RegisterResponder(http.MethodGet, url,
		func(req *http.Request) (*http.Response, error) {
			res := httpmock.NewStringResponse(200, `{
  "data": {
    "account": {
      "id": "aa248caa-eed7-4575-a990-717386438d2c",
      "name": "Test Account",
      "type": "checking",
      "on_budget": true,
      "closed": true,
			"note": "omg omg omg",
      "balance": 0,
      "cleared_balance": 0,
      "uncleared_balance": 0,
      "deleted": false
    }
  }
}
		`)
			return res, nil
		},
	)

	client := ynab.NewClient("")
	a, err := client.Account().GetAccount(
		"bbdccdb0-9007-42aa-a6fe-02a3e94476be",
		"aa248caa-eed7-4575-a990-717386438d2c",
	)
	assert.NoError(t, err)

	note := "omg omg omg"
	expected := &account.Account{
		ID:               "aa248caa-eed7-4575-a990-717386438d2c",
		Name:             "Test Account",
		Type:             account.TypeChecking,
		OnBudget:         true,
		Note:             &note,
		Closed:           true,
		Balance:          int64(0),
		ClearedBalance:   int64(0),
		UnclearedBalance: int64(0),
		Deleted:          false,
	}
	assert.Equal(t, expected, a)
}
