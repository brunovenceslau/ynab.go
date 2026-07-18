package ynab_test

// Gate G4: every registered endpoint case runs twice — verbatim, and with
// every optional response header deleted — and must decode identically.
// Slices register cases like:
//
//	func init() {
//		registerEndpointCase(endpointCase{
//			op: "getPayees", fixture: "payees/list.json",
//			call: func(t *testing.T, c *ynab.Client) (any, error) {
//				payees, _, err := c.Plan("p-1").Payees.List(t.Context())
//				return payees, err
//			},
//		})
//	}

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/transport"
)

// endpointCase is one registered G4 endpoint case: a golden fixture served
// as the response, and a call decoding it through the public client.
type endpointCase struct {
	op      string // operationId (table row)
	variant string // optional: distinguishes fixtures per op
	fixture string // path under testdata/, e.g. "payees/list.json"
	status  int    // response status; 0 means 200
	call    func(t *testing.T, c *ynab.Client) (any, error)
}

var (
	endpointRegistryMu sync.Mutex
	endpointRegistry   []endpointCase
)

// registerEndpointCase is called from slice test files' init functions.
func registerEndpointCase(ec endpointCase) {
	endpointRegistryMu.Lock()
	defer endpointRegistryMu.Unlock()
	endpointRegistry = append(endpointRegistry, ec)
}

// loadFixture reads a golden fixture from testdata/.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "fixture %s", name)
	return raw
}

// fixtureServer serves body on every request. When stripped, every
// non-essential header — including Content-Type and Date — is removed
// before the write.
func fixtureServer(t *testing.T, status int, body []byte, stripped bool) *httptest.Server {
	t.Helper()

	if status == 0 {
		status = http.StatusOK
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if stripped {
			w.Header()["Content-Type"] = nil
			w.Header()["Date"] = nil
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("X-Rate-Limit", "36/200")
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runEndpointTest executes one case twice — verbatim and header-stripped —
// requires both to succeed, and requires identical decoded results. The
// verbatim result feeds the G3 wire-model walk.
func runEndpointTest(t *testing.T, ec endpointCase) {
	t.Helper()

	body := loadFixture(t, ec.fixture)

	decode := func(stripped bool) any {
		srv := fixtureServer(t, ec.status, body, stripped)
		client := ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
		got, err := ec.call(t, client)
		require.NoError(t, err)
		require.NotNil(t, got)
		return got
	}

	verbatim := decode(false)
	strippedResult := decode(true)
	require.Equal(t, verbatim, strippedResult, "optional response headers must never change a decode")

	registerReadModel(verbatim)
}

// TestContractHeaders is gate G4 over the registry.
func TestContractHeaders(t *testing.T) {
	t.Parallel()

	endpointRegistryMu.Lock()
	cases := make([]endpointCase, len(endpointRegistry))
	copy(cases, endpointRegistry)
	endpointRegistryMu.Unlock()

	for _, ec := range cases {
		name := ec.op
		if ec.variant != "" {
			name += "/" + ec.variant
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runEndpointTest(t, ec)
		})
	}
}

// TestContractHeaders429SansRetryAfter pins the issue#38 path at the
// pipeline level: a 429 carrying no Retry-After header still backs off
// and the retried call succeeds.
func TestContractHeaders429SansRetryAfter(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	served := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		served++
		first := served == 1
		mu.Unlock()
		if first {
			w.Header()["Content-Type"] = nil
			w.Header()["Date"] = nil
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"id":"429","name":"too_many_requests","detail":"d"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	core := &transport.Core{
		HTTPClient:  &http.Client{},
		BaseURL:     u,
		UserAgent:   "test",
		Token:       func(context.Context) (string, error) { return "t", nil },
		Timeout:     5 * time.Second,
		DecodeError: ynab.DecodeWireError,
		Retry:       transport.RetryConfig{MaxAttempts: 2, MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond},
		Logger:      discardLogger(),
	}

	v, err := transport.Do[struct {
		OK bool `json:"ok"`
	}](t.Context(), core, http.MethodGet, "p", nil, nil)
	require.NoError(t, err, "a 429 without Retry-After must still be retried")
	require.True(t, v.OK)
}

// The synthetic case keeps the registry loop and both harness runs
// exercised until the first slice lands (Task 16+), then is removed. Its
// op is never marked implemented, so no completeness check counts it.
func init() {
	registerEndpointCase(endpointCase{
		op:      "syntheticEndpointProof",
		fixture: "selftest/thing.json",
		call: func(t *testing.T, c *ynab.Client) (any, error) {
			t.Helper()
			return rawGetJSON(t, ynab.BaseURLOf(c)+"/plans/p-1/things")
		},
	})
}

// discardLogger returns a logger that drops everything.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawGetJSON fetches u and decodes the success envelope into a generic
// map — the stand-in decode until real service methods exist.
func rawGetJSON(t *testing.T, u string) (any, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var env struct {
		Data map[string]any `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&env)
	return env.Data, err
}
