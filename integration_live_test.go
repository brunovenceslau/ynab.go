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
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// countingTransport counts requests and remembers the server's last
// rate-limit header, so the suite can prove its own budget claims.
type countingTransport struct {
	calls     atomic.Int64
	rateLimit atomic.Value // string, e.g. "36/200"
}

func (ct *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ct.calls.Add(1)
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err == nil {
		if v := resp.Header.Get("X-Rate-Limit"); v != "" {
			ct.rateLimit.Store(v)
		}
	}
	return resp, err
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
