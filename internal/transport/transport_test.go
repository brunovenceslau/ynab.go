package transport_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/transport"
)

const testToken = "secret-token-do-not-log"

// newCore builds a Core against baseURL with test defaults.
func newCore(t *testing.T, baseURL string) *transport.Core {
	t.Helper()

	u, err := url.Parse(baseURL)
	require.NoError(t, err)

	return &transport.Core{
		HTTPClient: &http.Client{},
		BaseURL:    u,
		UserAgent:  "ynab.go/test",
		Token:      func(context.Context) (string, error) { return testToken, nil },
		Timeout:    5 * time.Second,
		DecodeError: func(status int, body []byte, _ http.Header) error {
			return fmt.Errorf("api error: status=%d body=%s", status, body)
		},
		Logger: slog.New(slog.DiscardHandler),
	}
}

type user struct {
	ID string `json:"id"`
}

func TestDoRequestShape(t *testing.T) {
	t.Parallel()

	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"u-1"}}}`))
	}))
	defer srv.Close()

	c := newCore(t, srv.URL+"/v1")
	q := url.Values{"last_knowledge_of_server": {"42"}}

	v, err := transport.Do[struct {
		User user `json:"user"`
	}](t.Context(), c, http.MethodGet, "plans/p-1/accounts", q, nil)
	require.NoError(t, err)
	require.Equal(t, "u-1", v.User.ID)

	require.Equal(t, http.MethodGet, got.Method)
	require.Equal(t, "/v1/plans/p-1/accounts", got.URL.Path)
	require.Equal(t, "42", got.URL.Query().Get("last_knowledge_of_server"))
	require.Equal(t, []string{"Bearer " + testToken}, got.Header.Values("Authorization"), "exactly one credential header")
	require.Equal(t, "ynab.go/test", got.Header.Get("User-Agent"))
	require.Empty(t, got.Header.Get("Content-Type"), "bodyless request carries no Content-Type")
}

func TestDoWriteBody(t *testing.T) {
	t.Parallel()

	var gotBody []byte
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = mustReadAll(t, r)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"a-1"}}`))
	}))
	defer srv.Close()

	c := newCore(t, srv.URL)
	body := map[string]any{"account": map[string]any{"name": "n"}}

	v, err := transport.Do[struct {
		ID string `json:"id"`
	}](t.Context(), c, http.MethodPost, "plans/p-1/accounts", nil, body)
	require.NoError(t, err)
	require.Equal(t, "a-1", v.ID)
	require.JSONEq(t, `{"account":{"name":"n"}}`, string(gotBody))
	require.Equal(t, "application/json", gotCT)
}

func TestDoErrorEnvelopeRoutesThroughDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
	}{
		{
			name:   "401 envelope",
			status: http.StatusUnauthorized,
			body:   `{"error":{"id":"401","name":"not_authorized","detail":"d"}}`,
		},
		{
			name:   "404.2 envelope",
			status: http.StatusNotFound,
			body:   `{"error":{"id":"404.2","name":"resource_not_found","detail":"d"}}`,
		},
		{name: "garbage body", status: http.StatusInternalServerError, body: `<html>boom`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := newCore(t, srv.URL)
			var decoded struct {
				status int
				body   string
			}
			c.DecodeError = func(status int, body []byte, _ http.Header) error {
				decoded.status, decoded.body = status, string(body)
				return errors.New("mapped")
			}

			_, err := transport.Do[user](t.Context(), c, http.MethodGet, "user", nil, nil)
			require.EqualError(t, err, "mapped", "decoder's error is returned untouched")
			require.Equal(t, tt.status, decoded.status)
			require.Equal(t, tt.body, decoded.body, "decoder sees the raw bytes")
		})
	}
}

func TestDoSuccessIgnoresOptionalHeaders(t *testing.T) {
	t.Parallel()

	// The same payload served verbatim and with every non-essential header
	// stripped must decode identically (the G4 seed).
	const payload = `{"data":{"user":{"id":"u-1"}}}`

	full := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Rate-Limit", "36/200")
		_, _ = w.Write([]byte(payload))
	}))
	defer full.Close()

	stripped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k := range w.Header() {
			w.Header().Del(k)
		}
		w.Header()["Content-Type"] = nil
		w.Header()["Date"] = nil
		_, _ = w.Write([]byte(payload))
	}))
	defer stripped.Close()

	type env struct {
		User user `json:"user"`
	}
	a, err := transport.Do[env](t.Context(), newCore(t, full.URL), http.MethodGet, "user", nil, nil)
	require.NoError(t, err)
	b, err := transport.Do[env](t.Context(), newCore(t, stripped.URL), http.MethodGet, "user", nil, nil)
	require.NoError(t, err)
	require.Equal(t, a, b)
}

func TestDoTransportFailureExposesNetError(t *testing.T) {
	t.Parallel()

	// A dial-refused port must surface a net.Error with Timeout()==false
	// through the public chain — pins the §5.8 errors.As guarantee and the
	// %w wrapping end to end.
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close()) // now the port refuses connections

	c := newCore(t, "http://"+addr)
	_, err = transport.Do[user](t.Context(), c, http.MethodGet, "user", nil, nil)
	require.Error(t, err)

	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.False(t, netErr.Timeout(), "connection refused is not a timeout")
}

func TestDoLimiterAndTokenOrdering(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var order []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		order = append(order, "send")
		mu.Unlock()
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := newCore(t, srv.URL)
	c.Wait = func(context.Context) error {
		mu.Lock()
		order = append(order, "wait")
		mu.Unlock()
		return nil
	}
	c.Token = func(context.Context) (string, error) {
		mu.Lock()
		order = append(order, "token")
		mu.Unlock()
		return testToken, nil
	}

	_, err := transport.Do[struct{}](t.Context(), c, http.MethodGet, "user", nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"wait", "token", "send"}, order)
}

func TestDoHookFailuresAbortAndWrap(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request must be sent when a hook fails")
	}))
	defer srv.Close()

	t.Run("limiter error", func(t *testing.T) {
		t.Parallel()

		c := newCore(t, srv.URL)
		sentinel := errors.New("limiter says no")
		c.Wait = func(context.Context) error { return sentinel }

		_, err := transport.Do[user](t.Context(), c, http.MethodGet, "user", nil, nil)
		require.ErrorIs(t, err, sentinel, "wrapped with %%w")
	})

	t.Run("token source error", func(t *testing.T) {
		t.Parallel()

		c := newCore(t, srv.URL)
		sentinel := errors.New("no token")
		c.Token = func(context.Context) (string, error) { return "", sentinel }

		_, err := transport.Do[user](t.Context(), c, http.MethodGet, "user", nil, nil)
		require.ErrorIs(t, err, sentinel, "wrapped with %%w")
	})
}

// capturingHandler records every slog record's rendered text.
type capturingHandler struct {
	mu    sync.Mutex
	lines []string
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	sb.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		sb.WriteString(" " + a.Key + "=" + a.Value.String())
		return true
	})
	h.mu.Lock()
	h.lines = append(h.lines, sb.String())
	h.mu.Unlock()
	return nil
}
func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func TestDoNeverLogsToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	h := &capturingHandler{}
	c := newCore(t, srv.URL)
	c.Logger = slog.New(h)

	_, err := transport.Do[struct{}](t.Context(), c, http.MethodGet, "user", nil, nil)
	require.NoError(t, err)

	h.mu.Lock()
	defer h.mu.Unlock()
	require.NotEmpty(t, h.lines, "tracing is on at debug level")
	for _, line := range h.lines {
		require.NotContains(t, line, testToken, "the bearer token must never reach a log record")
	}
}

func TestDoUndecodableSuccessBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := transport.Do[user](t.Context(), newCore(t, srv.URL), http.MethodGet, "user", nil, nil)
	require.Error(t, err)

	var syntaxErr *json.SyntaxError
	require.ErrorAs(t, err, &syntaxErr, "decode failures stay inspectable through the chain")
}

func mustReadAll(t *testing.T, r *http.Request) []byte {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	return b
}
