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
	"pkg.venceslau.dev/ynab/internal/contract"
	"pkg.venceslau.dev/ynab/internal/transport"
)

// endpointCase is one registered G4 endpoint case: a golden fixture served
// as the response, and a call decoding it through the public client.
type endpointCase struct {
	op      string // operationId (table row)
	variant string // optional: distinguishes fixtures per op
	fixture string // path under testdata/, e.g. "payees/list.json"
	status  int    // response status; 0 means 200
	model   any    // the decode-target model, statically feeding the G3 walk
	call    func(t *testing.T, c *ynab.Client) (any, error)
}

var (
	endpointRegistryMu sync.Mutex
	endpointRegistry   []endpointCase
)

// registerEndpointCase is called from slice test files' init functions.
// The case's model feeds the G3 struct lint at registration time —
// statically, so gate coverage can never depend on test scheduling.
func registerEndpointCase(ec endpointCase) {
	endpointRegistryMu.Lock()
	endpointRegistry = append(endpointRegistry, ec)
	endpointRegistryMu.Unlock()

	if ec.model != nil {
		registerReadModel(ec.model)
	}
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
}

// TestContractHeadersStrippedServer proves the stripped fixtureServer
// really omits the optional headers — if the suppression trick ever
// stopped working, the G4 second run would silently duplicate the first.
func TestContractHeadersStrippedServer(t *testing.T) {
	t.Parallel()

	srv := fixtureServer(t, 0, []byte(`{}`), true)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Empty(t, resp.Header.Values("Content-Type"))
	require.Empty(t, resp.Header.Values("Date"))
	require.Empty(t, resp.Header.Values("X-Rate-Limit"))
}

// TestContractHeadersDetectsDependence proves G4's teeth: a decode that
// depends on a response header yields different results between the
// verbatim and stripped runs — exactly what runEndpointTest's equality
// assertion turns into a failure.
func TestContractHeadersDetectsDependence(t *testing.T) {
	t.Parallel()

	headerRead := func(stripped bool) string {
		srv := fixtureServer(t, 0, []byte(`{}`), stripped)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp.Header.Get("X-Rate-Limit")
	}

	require.NotEqual(t, headerRead(false), headerRead(true),
		"a header-dependent decode must diverge between the two runs")
}

// TestContractHeaders is gate G4 over the registry: read-side
// completeness against the G1 implemented registry, then every case run
// twice.
func TestContractHeaders(t *testing.T) {
	t.Parallel()

	endpointRegistryMu.Lock()
	cases := make([]endpointCase, len(endpointRegistry))
	copy(cases, endpointRegistry)
	endpointRegistryMu.Unlock()

	infos := make([]contract.ReadCaseInfo, 0, len(cases))
	for _, ec := range cases {
		infos = append(infos, contract.ReadCaseInfo{OpID: ec.op})
	}
	require.Empty(t, contract.DiffReadCoverage(contract.Table(), contract.ImplementedIDs(), infos))

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

// discardLogger returns a logger that drops everything.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
