// Command testing-with-mock shows how to point the client at your own
// server for tests — no token, no network, deterministic. This is the
// one place httptest belongs: the subject IS testing. WithBaseURL is the
// same seam the library's own examples use.
//
//	go run ./examples/testing-with-mock
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"pkg.venceslau.dev/ynab"
)

func main() {
	if err := run(); err != nil {
		slog.Error("mock example", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Stand in for api.ynab.com: serve a canned accounts response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":{"accounts":[
			{"name":"Checking","balance":128422,"balance_formatted":"$128.42"},
			{"name":"Savings","balance":900000,"balance_formatted":"$900.00"}
		],"server_knowledge":42}}`)
	}))
	defer srv.Close()

	// WithBaseURL redirects every request to the fake; the token is unused.
	client := ynab.New("test-token", ynab.WithBaseURL(srv.URL))

	accounts, sk, err := client.Plan("plan-id").Accounts.List(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("server_knowledge=%d\n", sk)
	for _, a := range accounts {
		fmt.Printf("%-10s %s (%d milliunits)\n", a.Name, a.BalanceFormatted, a.Balance)
	}
	return nil
}
