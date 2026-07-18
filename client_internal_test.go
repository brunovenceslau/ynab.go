package ynab

// White-box tests for construction defaults and the config-error contract.
// No public I/O method exists yet to observe these through; Task 16's User
// endpoint re-proves the contract through the public surface.

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	c := New("t")
	require.NoError(t, c.configError())
	require.Equal(t, "https://api.ynab.com/v1", c.baseURL.String())
	require.Equal(t, "pkg.venceslau.dev/ynab/"+Version, c.userAgent)
	require.Equal(t, 30*time.Second, c.timeout)
	require.Equal(t, RetryPolicy{MaxAttempts: 3, MinBackoff: time.Second, MaxBackoff: 30 * time.Second, RetryWrites: false}, c.retry)
	require.False(t, c.retryOff)
	require.Same(t, http.DefaultClient, c.httpClient)
	require.Nil(t, c.limiter)
	require.NotNil(t, c.logger)

	tok, err := c.tokenSource.Token(t.Context())
	require.NoError(t, err)
	require.Equal(t, "t", tok)
}

func TestOptionsApply(t *testing.T) {
	t.Parallel()

	hc := &http.Client{}
	lim := waitFunc(func() {})
	lg := slog.New(slog.DiscardHandler)
	p := RetryPolicy{MaxAttempts: 5, MinBackoff: 2 * time.Second, MaxBackoff: 10 * time.Second, RetryWrites: true}

	c := New("t",
		WithHTTPClient(hc),
		WithBaseURL("http://localhost:9999/v1"),
		WithUserAgent("custom/1"),
		WithTimeout(5*time.Second),
		WithRetryPolicy(p),
		WithRetryDisabled(),
		WithLimiter(lim),
		WithLogger(lg),
	)
	require.NoError(t, c.configError())
	require.Same(t, hc, c.httpClient)
	require.Equal(t, "http://localhost:9999/v1", c.baseURL.String())
	require.Equal(t, "custom/1", c.userAgent)
	require.Equal(t, 5*time.Second, c.timeout)
	require.Equal(t, p, c.retry)
	require.True(t, c.retryOff)
	require.NotNil(t, c.limiter)
	require.Same(t, lg, c.logger)
}

// waitFunc adapts a func to Limiter for tests.
type waitFunc func()

func (f waitFunc) Wait(context.Context) error { f(); return nil }

func TestConfigErrorContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opt    Option
		field  string
		reason string
	}{
		{name: "bad base URL", opt: WithBaseURL("://bad"), field: "WithBaseURL"},
		{name: "relative base URL", opt: WithBaseURL("/v1"), field: "WithBaseURL"},
		{name: "non-http scheme", opt: WithBaseURL("ftp://host/v1"), field: "WithBaseURL"},
		{name: "credentials in base URL", opt: WithBaseURL("https://user:pass@host/v1"), field: "WithBaseURL"},
		{name: "query in base URL", opt: WithBaseURL("https://host/v1?x=1"), field: "WithBaseURL"},
		{name: "fragment in base URL", opt: WithBaseURL("https://host/v1#frag"), field: "WithBaseURL"},
		{name: "nil http client", opt: WithHTTPClient(nil), field: "WithHTTPClient"},
		{name: "non-positive timeout", opt: WithTimeout(0), field: "WithTimeout"},
		{name: "zero max attempts", opt: WithRetryPolicy(RetryPolicy{}), field: "WithRetryPolicy"},
		{name: "zero min backoff", opt: WithRetryPolicy(RetryPolicy{MaxAttempts: 1, MaxBackoff: time.Second}), field: "WithRetryPolicy"},
		{name: "inverted backoff", opt: WithRetryPolicy(RetryPolicy{MaxAttempts: 1, MinBackoff: 2 * time.Second, MaxBackoff: time.Second}), field: "WithRetryPolicy"},
		{name: "nil limiter", opt: WithLimiter(nil), field: "WithLimiter"},
		{name: "nil logger", opt: WithLogger(nil), field: "WithLogger"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := New("t", tt.opt)
			err := c.configError()
			require.Error(t, err)

			var argErr *ArgumentError
			require.ErrorAs(t, err, &argErr)
			require.Equal(t, "ynab.New", argErr.Op)
			require.Equal(t, tt.field, argErr.Field, "the error names the failing option")
		})
	}

	t.Run("nil token source trips the contract instead of panicking", func(t *testing.T) {
		t.Parallel()

		var c *Client
		require.NotPanics(t, func() { c = NewWithTokenSource(nil) })

		var argErr *ArgumentError
		require.ErrorAs(t, c.configError(), &argErr)
		require.Equal(t, "ynab.NewWithTokenSource", argErr.Op)
		require.Equal(t, "ts", argErr.Field)
	})

	t.Run("first failure wins", func(t *testing.T) {
		t.Parallel()

		c := New("t", WithBaseURL("://bad"), WithTimeout(0))
		var argErr *ArgumentError
		require.ErrorAs(t, c.configError(), &argErr)
		require.Equal(t, "WithBaseURL", argErr.Field)
	})

	t.Run("failed option never half-applies", func(t *testing.T) {
		t.Parallel()

		c := New("t", WithBaseURL("://bad"))
		require.Equal(t, "https://api.ynab.com/v1", c.baseURL.String(), "no silent fallback, no partial state")
	})
}

func TestClientConcurrentUse(t *testing.T) {
	t.Parallel()

	// Configuration is set-once: concurrent readers must be race-free
	// (verified by -race).
	c := New("t", WithUserAgent("ua/1"))

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.configError()
			_ = c.userAgent
			_ = c.retry
			_, _ = c.tokenSource.Token(context.Background())
		}()
	}
	wg.Wait()
}
