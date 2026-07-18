package ynab_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"pkg.venceslau.dev/ynab"
)

// tokenStore is a minimal TokenSource. Token runs before every attempt —
// retries included — so an OAuth access token refreshed elsewhere is
// picked up mid-call.
type tokenStore struct{ current string }

func (s *tokenStore) Token(context.Context) (string, error) { return s.current, nil }

// ExampleNewWithTokenSource constructs a client for OAuth tokens that
// expire (YNAB's expire after two hours); a fixed personal-access token
// wants New instead.
func ExampleNewWithTokenSource() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("server saw:", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"u-1"}}}`))
	}))
	defer srv.Close()

	source := &tokenStore{current: "fresh-oauth-token"}
	client := ynab.NewWithTokenSource(source, ynab.WithBaseURL(srv.URL))
	user, err := client.User(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("authenticated as", user.ID)

	// Output:
	// server saw: Bearer fresh-oauth-token
	// authenticated as u-1
}
