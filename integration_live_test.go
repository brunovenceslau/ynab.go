// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

//go:build integration

package ynab_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

// countingTransport counts requests, remembers the server's last
// rate-limit header, and records every (method, path) pair so the
// runner can attribute live traffic to the case that produced it.
type countingTransport struct {
	calls     atomic.Int64
	rateLimit atomic.Value // string, e.g. "36/200"

	mu   sync.Mutex
	seen []recordedCall
}

// recordedCall is one live request as the transport saw it.
type recordedCall struct {
	method string
	path   string // includes the client's /v1 base-path prefix
}

func (ct *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ct.calls.Add(1)
	ct.mu.Lock()
	ct.seen = append(ct.seen, recordedCall{method: req.Method, path: req.URL.Path})
	ct.mu.Unlock()
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err == nil {
		if v := resp.Header.Get("X-Rate-Limit"); v != "" {
			ct.rateLimit.Store(v)
		}
	}
	return resp, err
}

// recorded returns a snapshot of every call so far.
func (ct *countingTransport) recorded() []recordedCall {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return slices.Clone(ct.seen)
}

// matchOperation maps a recorded live request to its operationId by
// matching against the contract table's path templates, after stripping
// the client's /v1 base-path prefix.
func matchOperation(rc recordedCall) (string, bool) {
	path := strings.TrimPrefix(rc.path, "/v1")
	for _, op := range contract.Table() {
		if op.Method == rc.method && contract.PathRegexp(op.Path).MatchString(path) {
			return op.ID, true
		}
	}
	return "", false
}

// TestLiveIntegration runs every registered case against the real API,
// sequentially — the suite is never concurrent with itself. It skips
// cleanly without YNAB_TEST_TOKEN, and REQUIRES YNAB_TEST_PLAN_ID: the
// suite writes, and writes only ever touch the dedicated test plan —
// defaulting to the token's last-used plan would silently redirect them
// wherever the token was last used.
func TestLiveIntegration(t *testing.T) {
	token := os.Getenv("YNAB_TEST_TOKEN")
	if token == "" {
		t.Skip("YNAB_TEST_TOKEN not set — live integration runs only against a dedicated test plan")
	}
	planID := os.Getenv("YNAB_TEST_PLAN_ID")
	if planID == "" {
		t.Skip("YNAB_TEST_PLAN_ID not set — the suite writes, and writes only ever touch the dedicated test plan")
	}

	// The counting transport and a captured debug log turn two promises
	// into assertions: the request budget, and token redaction on real
	// traffic. The suite is sequential, so the plain buffer is safe.
	transport := &countingTransport{}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	env := integrationEnv{
		Client: ynab.New(token,
			ynab.WithHTTPClient(&http.Client{Transport: transport}),
			ynab.WithLogger(logger),
		),
		PlanID: ynab.PlanID(planID),
	}

	integrationMu.Lock()
	cases := make([]integrationCase, len(integrationCases))
	copy(cases, integrationCases)
	integrationMu.Unlock()
	slices.SortStableFunc(cases, func(a, b integrationCase) int { return a.order - b.order })

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The suite is sequential, so a before/after window attributes
			// every recorded request to this case. The check runs from a
			// Cleanup registered FIRST: cleanups run LIFO, so it sees the
			// case's own cleanup traffic (deletes, restores) too. Every
			// recorded operation must be one the case declares — set
			// semantics, so retries cannot double-count.
			before := len(transport.recorded())
			t.Cleanup(func() {
				declared := map[string]bool{}
				for _, op := range c.ops {
					declared[op] = true
				}
				for _, rc := range transport.recorded()[before:] {
					opID, ok := matchOperation(rc)
					require.True(t, ok, "case %q sent %s %s, which matches no contract operation",
						c.name, rc.method, rc.path)
					require.True(t, declared[opID],
						"case %q hit %s but does not declare it — its ops list has drifted from its body",
						c.name, opID)
				}
			})
			c.run(t, env) // no t.Parallel: sequential by doctrine
		})
	}

	calls := transport.calls.Load()
	rate, _ := transport.rateLimit.Load().(string)
	t.Logf("live suite made %d requests (server X-Rate-Limit: %s)", calls, rate)
	require.LessOrEqual(t, calls, int64(120), "the suite must stay well under the 200/h budget")
	require.NotContains(t, logBuf.String(), token, "the token must never reach the logs")
	require.NotZero(t, logBuf.Len(), "the redaction assertion must not pass vacuously")
}
