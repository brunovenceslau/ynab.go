// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func TestRawDo(t *testing.T) {
	t.Parallel()

	t.Run("raw bytes pass through untouched in both directions", func(t *testing.T) {
		t.Parallel()

		// Byte-verbatim on purpose: RawDo promises the exact wire bytes
		// (a JSON-equivalence assertion would be too weak), and the served
		// payload even keeps a key outside the envelope to prove no
		// unwrapping happens.
		served := []byte(`{"data":{"anything":"goes"},"extra":"kept"}`)

		var gotBody []byte
		var gotQuery url.Values
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			gotQuery = r.URL.Query()
			_, _ = w.Write(served)
		}))
		t.Cleanup(srv.Close)

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		out, err := client.RawDo(t.Context(), http.MethodPost, "plans/p-1/future_endpoint",
			url.Values{"k": {"v"}}, []byte(`{"raw":true}`))
		require.NoError(t, err)
		require.Equal(t, served, out)
		require.Equal(t, `{"raw":true}`, string(gotBody))
		require.Equal(t, "v", gotQuery.Get("k"))
	})

	t.Run("participates in the retry pipeline", func(t *testing.T) {
		t.Parallel()

		var attempts, waits, tokens atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"id":"429","name":"too_many_requests","detail":"d"}}`))
				return
			}
			_, _ = w.Write([]byte(`ok`))
		}))
		t.Cleanup(srv.Close)

		client := ynab.NewWithTokenSource(
			countingTokenSource{calls: &tokens},
			ynab.WithBaseURL(srv.URL),
			ynab.WithLimiter(countingLimiter{calls: &waits}),
			ynab.WithRetryPolicy(ynab.RetryPolicy{MaxAttempts: 2, MinBackoff: 1, MaxBackoff: 2}),
		)

		out, err := client.RawDo(t.Context(), http.MethodGet, "p", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "ok", string(out))
		require.Equal(t, int32(2), attempts.Load(), "the 429 was retried")
		require.Equal(t, int32(2), waits.Load(), "limiter waits per attempt")
		require.Equal(t, int32(2), tokens.Load(), "token fetched per attempt")
	})

	t.Run("non-2xx maps to the error taxonomy", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"id":"404.2","name":"resource_not_found","detail":"d"}}`))
		}))
		t.Cleanup(srv.Close)

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		_, err := client.RawDo(t.Context(), http.MethodGet, "nope", nil, nil)
		require.ErrorIs(t, err, ynab.ErrResourceNotFound)
	})

	t.Run("transport failures surface as retryable errors", func(t *testing.T) {
		t.Parallel()

		// A server that is already gone: the dial fails, so the error takes
		// DoRaw's transport branch rather than the status-mapping one.
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		_, err := client.RawDo(t.Context(), http.MethodGet, "p", nil, nil)
		require.Error(t, err)
		require.True(t, ynab.IsRetryable(err), "a connection failure is retryable")
		var apiErr *ynab.Error
		require.NotErrorAs(t, err, &apiErr, "a dial failure is transport-class, not a mapped status")
	})

	t.Run("config errors surface before any I/O", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("no request must be sent on a config error")
		}))
		t.Cleanup(srv.Close)

		client := ynab.New("t", ynab.WithBaseURL(srv.URL), ynab.WithTimeout(-1))
		_, err := client.RawDo(t.Context(), http.MethodGet, "p", nil, nil)
		var argErr *ynab.ArgumentError
		require.ErrorAs(t, err, &argErr)
	})
}

// countingLimiter counts Wait calls.
type countingLimiter struct{ calls *atomic.Int32 }

func (l countingLimiter) Wait(context.Context) error { l.calls.Add(1); return nil }

// countingTokenSource counts Token calls.
type countingTokenSource struct{ calls *atomic.Int32 }

func (s countingTokenSource) Token(context.Context) (string, error) { s.calls.Add(1); return "t", nil }
