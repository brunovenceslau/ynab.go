package ynab_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"pkg.venceslau.dev/ynab"
)

// ExampleWithBaseURL is the first-class mocking seam: point the client at
// an httptest.Server. Nothing from this library's internals is needed —
// stdlib plus the public constructor is the whole story.
func ExampleWithBaseURL() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"u-123"}}}`))
	}))
	defer srv.Close()

	client := ynab.New("test-token", ynab.WithBaseURL(srv.URL))
	user, err := client.User(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("user:", user.ID)

	// Output:
	// user: u-123
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// ExampleWithHTTPClient is the second mocking seam: install a fake
// http.RoundTripper. No network, no server — responses are fabricated in
// process, again with stdlib only.
func ExampleWithHTTPClient() {
	fake := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		fmt.Println("intercepted:", r.Method, r.URL.Path)
		body := `{"data":{"settings":{"date_format":{"format":"YYYY-MM-DD"},"currency_format":null}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})

	client := ynab.New("test-token", ynab.WithHTTPClient(&http.Client{Transport: fake}))
	settings, err := client.Plan(ynab.PlanIDLastUsed).Settings(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("date format:", settings.DateFormat.Format)

	// Output:
	// intercepted: GET /v1/plans/last-used/settings
	// date format: YYYY-MM-DD
}
