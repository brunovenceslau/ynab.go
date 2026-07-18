package transport_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/transport"
)

// TestRetryTransportFailureWriteNotRetried pins the write-safety contract
// for transport failures (not just 5xx statuses): a POST that dies on the
// wire may already have applied server-side and must not be re-sent.
func TestRetryTransportFailureWriteNotRetried(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close()) // refuses connections from here on

	c := newCore(t, "http://"+addr)
	c.Retry = transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}

	var waits atomic.Int32
	c.Wait = func(context.Context) error { waits.Add(1); return nil }

	_, err = transport.Do[okPayload](t.Context(), c, http.MethodPost, "p", nil, map[string]int{"a": 1})
	require.Error(t, err)
	require.Equal(t, int32(1), waits.Load(), "exactly one attempt: the write may have applied")

	// The same failure on GET does retry.
	waits.Store(0)
	_, err = transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Equal(t, int32(3), waits.Load(), "idempotent verbs exhaust MaxAttempts")
}

// TestRetryCallerCancelDuringAttempt pins that a caller's own context
// ending mid-attempt aborts the pipeline instead of burning retries.
func TestRetryCallerCancelDuringAttempt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel() // the caller gives up while the server holds the request
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(srv.Close)

	c := newCore(t, srv.URL)
	c.Retry = transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	var waits atomic.Int32
	c.Wait = func(context.Context) error { waits.Add(1); return nil }

	_, err := transport.Do[okPayload](ctx, c, http.MethodGet, "p", nil, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int32(1), waits.Load(), "no retry against a canceled caller context")
}

// TestRetryAfterBeyondCapStopsPipeline pins the bounded-wait contract: a
// server demanding an hour-long wait gets its 429 surfaced immediately,
// with Error semantics left to the injected decoder.
func TestRetryAfterBeyondCapStopsPipeline(t *testing.T) {
	t.Parallel()

	hdr := http.Header{}
	hdr.Set("Retry-After", "3600")
	srv, attempts := flakyServer(t, 99, http.StatusTooManyRequests, hdr)

	c := retryCore(t, srv.URL, transport.RetryConfig{MaxAttempts: 3, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond})
	var decodedStatus int
	c.DecodeError = func(status int, _ []byte, _ http.Header) error {
		decodedStatus = status
		return errors.New("mapped")
	}

	start := time.Now()
	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusTooManyRequests, decodedStatus)
	require.Equal(t, int32(1), attempts.Load(), "no second attempt after an over-cap Retry-After")
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestDoMissingDataKeyIsLoud(t *testing.T) {
	t.Parallel()

	// The envelope invariant: a 2xx always carries data. A bare {} must
	// fail loudly, never yield a silently zero-valued result.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	_, err := transport.Do[okPayload](t.Context(), newCore(t, srv.URL), http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no data key")
}

func TestDoInvalidMethod(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("an unbuildable request must never be sent")
	}))
	t.Cleanup(srv.Close)

	_, err := transport.Do[okPayload](t.Context(), newCore(t, srv.URL), "BAD METHOD", "p", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build request")
}

func TestDoEncodeBodyFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request must be sent when the body cannot encode")
	}))
	t.Cleanup(srv.Close)

	_, err := transport.Do[okPayload](t.Context(), newCore(t, srv.URL), http.MethodPost, "p", nil, map[string]any{"bad": make(chan int)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "encode request body")
}

func TestDoBodyReadFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(`{"da`)) // then return: 5 of 100 promised bytes
	}))
	t.Cleanup(srv.Close)

	_, err := transport.Do[okPayload](t.Context(), newCore(t, srv.URL), http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read response")
}

func TestDoResponseBodyLimit(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ok":true},"pad":"` + strings.Repeat("x", 100) + `"}`))
	}))
	t.Cleanup(srv.Close)

	c := newCore(t, srv.URL)
	c.MaxResponseBytes = 16
	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds 16 bytes")
}

// TestRetryDefaultJitter exercises the default rand source (no injected
// Rand): delays stay inside the documented half-open interval by wall
// clock bounds.
func TestRetryDefaultJitter(t *testing.T) {
	t.Parallel()

	srv, _ := flakyServer(t, 1, http.StatusServiceUnavailable, nil)
	c := newCore(t, srv.URL)
	c.Retry = transport.RetryConfig{MaxAttempts: 2, MinBackoff: 20 * time.Millisecond, MaxBackoff: 40 * time.Millisecond}

	start := time.Now()
	_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
}

func TestJoinPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want string
	}{
		{name: "plain segments", in: []string{"plans", "p-1", "accounts"}, want: "plans/p-1/accounts"},
		{name: "space escaped", in: []string{"plans", "p 1"}, want: "plans/p%201"},
		{name: "slash in segment cannot reroute", in: []string{"plans", "p-1/transactions"}, want: "plans/p-1%2Ftransactions"},
		{name: "dot-dot segment neutralized", in: []string{"plans", "..", "user"}, want: "plans/%2E%2E/user"},
		{name: "dot segment neutralized", in: []string{"plans", "."}, want: "plans/%2E"},
		{name: "question mark escaped", in: []string{"p?x=1"}, want: "p%3Fx=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, transport.JoinPath(tt.in...))
		})
	}
}

// TestDoNeverLogsTokenOnErrorPaths extends the redaction guarantee to the
// retry and failure paths, for logs AND error strings.
func TestDoNeverLogsTokenOnErrorPaths(t *testing.T) {
	t.Parallel()

	h := &capturingHandler{}

	t.Run("retry then success", func(t *testing.T) {
		t.Parallel()

		srv, _ := flakyServer(t, 1, http.StatusTooManyRequests, nil)
		c := retryCore(t, srv.URL, transport.RetryConfig{MaxAttempts: 2, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond})
		c.Logger = slog.New(h)
		_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
		require.NoError(t, err)
	})

	t.Run("decode error path", func(t *testing.T) {
		t.Parallel()

		srv, _ := flakyServer(t, 99, http.StatusInternalServerError, nil)
		c := newCore(t, srv.URL)
		c.Logger = slog.New(h)
		_, err := transport.Do[okPayload](t.Context(), c, http.MethodPost, "p", nil, map[string]int{"a": 1})
		require.Error(t, err)
		require.NotContains(t, err.Error(), testToken)
	})

	t.Run("transport failure path", func(t *testing.T) {
		t.Parallel()

		c := newCore(t, "http://127.0.0.1:1")
		c.Logger = slog.New(h)
		_, err := transport.Do[okPayload](t.Context(), c, http.MethodGet, "p", nil, nil)
		require.Error(t, err)
		require.NotContains(t, err.Error(), testToken)
	})

	t.Cleanup(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, line := range h.lines {
			require.NotContains(t, line, testToken)
		}
	})
}
