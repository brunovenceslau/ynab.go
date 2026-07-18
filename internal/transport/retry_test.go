package transport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/transport"
)

// retryCore builds a Core with a deterministic pipeline: jitter pinned to
// its upper bound (Rand=1) and backoffs measured, not slept, unless a test
// overrides.
func retryCore(t *testing.T, baseURL string, cfg transport.RetryConfig) *transport.Core {
	t.Helper()

	c := newCore(t, baseURL)
	c.Retry = cfg
	c.Rand = func() float64 { return 1 }
	return c
}

// flakyServer fails with `failures` responses of status `failStatus`, then
// succeeds. It counts attempts.
func flakyServer(t *testing.T, failures, failStatus int, hdr http.Header) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if int(n) <= failures {
			for k, vs := range hdr {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(failStatus)
			_, _ = w.Write([]byte(`{"error":{"id":"x","name":"x","detail":"x"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &attempts
}

type okPayload struct {
	OK bool `json:"ok"`
}

func TestRetry429AnyVerb(t *testing.T) {
	t.Parallel()

	// 429 is rejected before processing, so even POST retries — and a 429
	// without Retry-After still backs off (the issue#38 seed).
	srv, attempts := flakyServer(t, 2, http.StatusTooManyRequests, nil)

	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 4 * time.Millisecond}
	v, err := transport.Do[okPayload](t.Context(), retryCore(t, srv.URL, cfg), http.MethodPost, "p", nil, map[string]int{"a": 1})
	require.NoError(t, err)
	require.True(t, v.OK)
	require.Equal(t, int32(3), attempts.Load())
}

func TestRetryRetryAfterHonored(t *testing.T) {
	t.Parallel()

	// With backoff bounds at 1–2ms, only an honored Retry-After: 1 can
	// explain a ≥1s wait between the 429 and the retry.
	hdr := http.Header{}
	hdr.Set("Retry-After", "1")
	srv, attempts := flakyServer(t, 1, http.StatusTooManyRequests, hdr)

	cfg := transport.RetryConfig{MaxAttempts: 2, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	start := time.Now()
	v, err := transport.Do[okPayload](t.Context(), retryCore(t, srv.URL, cfg), http.MethodGet, "p", nil, nil)
	require.NoError(t, err)
	require.True(t, v.OK)
	require.Equal(t, int32(2), attempts.Load())
	require.GreaterOrEqual(t, time.Since(start), time.Second)
}

func TestRetry500VerbRules(t *testing.T) {
	t.Parallel()

	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}

	t.Run("POST not retried by default", func(t *testing.T) {
		t.Parallel()

		srv, attempts := flakyServer(t, 1, http.StatusInternalServerError, nil)
		c := retryCore(t, srv.URL, cfg)
		var decodedStatus int
		c.DecodeError = func(status int, _ []byte, _ http.Header) error {
			decodedStatus = status
			return errors.New("mapped")
		}

		_, err := transport.Do[okPayload](t.Context(), c, http.MethodPost, "p", nil, map[string]int{"a": 1})
		require.Error(t, err)
		require.Equal(t, int32(1), attempts.Load(), "a timed-out write may have applied — one attempt only")
		require.Equal(t, http.StatusInternalServerError, decodedStatus)
	})

	t.Run("POST retried with RetryWrites", func(t *testing.T) {
		t.Parallel()

		srv, attempts := flakyServer(t, 1, http.StatusInternalServerError, nil)
		rw := cfg
		rw.RetryWrites = true

		v, err := transport.Do[okPayload](t.Context(), retryCore(t, srv.URL, rw), http.MethodPost, "p", nil, map[string]int{"a": 1})
		require.NoError(t, err)
		require.True(t, v.OK)
		require.Equal(t, int32(2), attempts.Load())
	})

	t.Run("GET retried by default", func(t *testing.T) {
		t.Parallel()

		srv, attempts := flakyServer(t, 1, http.StatusServiceUnavailable, nil)

		v, err := transport.Do[okPayload](t.Context(), retryCore(t, srv.URL, cfg), http.MethodGet, "p", nil, nil)
		require.NoError(t, err)
		require.True(t, v.OK)
		require.Equal(t, int32(2), attempts.Load())
	})

	t.Run("DELETE retried by default", func(t *testing.T) {
		t.Parallel()

		srv, attempts := flakyServer(t, 1, http.StatusInternalServerError, nil)

		v, err := transport.Do[okPayload](t.Context(), retryCore(t, srv.URL, cfg), http.MethodDelete, "p", nil, nil)
		require.NoError(t, err)
		require.True(t, v.OK)
		require.Equal(t, int32(2), attempts.Load())
	})
}

func TestRetryDisabled(t *testing.T) {
	t.Parallel()

	srv, attempts := flakyServer(t, 1, http.StatusTooManyRequests, nil)
	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond, Disabled: true}
	c := retryCore(t, srv.URL, cfg)
	c.DecodeError = func(int, []byte, http.Header) error { return errors.New("mapped") }

	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Equal(t, int32(1), attempts.Load(), "WithRetryDisabled short-circuits to exactly one attempt")
}

func TestRetryCancelDuringBackoffReturnsPromptly(t *testing.T) {
	t.Parallel()

	// A 10s backoff canceled after ~30ms must return promptly with
	// context.Canceled — the reused-timer select path.
	srv, _ := flakyServer(t, 5, http.StatusTooManyRequests, nil)
	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: 10 * time.Second, MaxBackoff: 10 * time.Second}
	c := retryCore(t, srv.URL, cfg)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := transport.Do[okPayload](ctx, c, http.MethodGet, "p", nil, nil)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, elapsed, 2*time.Second, "cancellation must never be stalled by backoff")
}

func TestRetryPerAttemptDeadlineChain(t *testing.T) {
	t.Parallel()

	// A server stalling past the per-attempt timeout yields
	// errors.Is(err, context.DeadlineExceeded) through the public chain
	// after retries exhaust — the §5.8 deadline guarantee.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(srv.Close)

	cfg := transport.RetryConfig{MaxAttempts: 2, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	c := retryCore(t, srv.URL, cfg)
	c.Timeout = 50 * time.Millisecond

	start := time.Now()
	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 3*time.Second, "two 50ms attempts plus tiny backoff")
}

func TestRetryPerAttemptOrdering(t *testing.T) {
	t.Parallel()

	// Wait → Token → send runs per attempt, not once per call.
	srv, _ := flakyServer(t, 1, http.StatusTooManyRequests, nil)
	cfg := transport.RetryConfig{MaxAttempts: 2, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	c := retryCore(t, srv.URL, cfg)

	var mu sync.Mutex
	var order []string
	record := func(step string) {
		mu.Lock()
		order = append(order, step)
		mu.Unlock()
	}
	c.Wait = func(context.Context) error { record("wait"); return nil }
	c.Token = func(context.Context) (string, error) { record("token"); return testToken, nil }

	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"wait", "token", "wait", "token"}, order)
}

func TestRetryHookErrorsAbortMidPipeline(t *testing.T) {
	t.Parallel()

	// A limiter failure on the second attempt aborts instead of retrying,
	// surfacing the wrapped hook error.
	srv, attempts := flakyServer(t, 3, http.StatusTooManyRequests, nil)
	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	c := retryCore(t, srv.URL, cfg)

	sentinel := errors.New("limiter blew up")
	var calls atomic.Int32
	c.Wait = func(context.Context) error {
		if calls.Add(1) == 2 {
			return sentinel
		}
		return nil
	}

	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, int32(1), attempts.Load())
}

func TestRetryBackoffBounds(t *testing.T) {
	t.Parallel()

	// With Rand pinned to 1 the i-th retry waits min(Max, Min·2^i); with
	// Rand pinned to 0 it waits exactly Min. Proven through wall clock at
	// millisecond scale.
	srv, _ := flakyServer(t, 2, http.StatusServiceUnavailable, nil)
	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: 30 * time.Millisecond, MaxBackoff: 40 * time.Millisecond}
	c := retryCore(t, srv.URL, cfg) // Rand = 1 → delays 40ms+40ms (capped)

	start := time.Now()
	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.NoError(t, err)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 80*time.Millisecond, "two capped 40ms backoffs")
	require.Less(t, elapsed, 5*time.Second)

	// url.Values encoding stays stable through retries (regression guard
	// for query reuse across attempts).
	srv2, _ := flakyServer(t, 1, http.StatusServiceUnavailable, nil)
	c2 := retryCore(t, srv2.URL, cfg)
	c2.Rand = func() float64 { return 0 }
	q := url.Values{"k": {"v"}}
	start = time.Now()
	_, err = transport.Do[okPayload](t.Context(), c2, http.MethodGet, "p", q, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond, "Rand=0 waits exactly MinBackoff")
}

func TestRetryExhaustionReturnsLastError(t *testing.T) {
	t.Parallel()

	srv, attempts := flakyServer(t, 99, http.StatusServiceUnavailable, nil)
	cfg := transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	c := retryCore(t, srv.URL, cfg)
	var decodedStatus int
	c.DecodeError = func(status int, _ []byte, _ http.Header) error {
		decodedStatus = status
		return errors.New("mapped")
	}

	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Equal(t, int32(3), attempts.Load())
	require.Equal(t, http.StatusServiceUnavailable, decodedStatus, "the final 503 is decoded, not swallowed")
}
