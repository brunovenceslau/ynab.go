// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

//go:build integration

package ynab_test

import (
	"bytes"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

// countingTransport counts requests and records every request/response
// pair — method, path, query, status, content type, body, and any
// server_knowledge the envelope carried — so the runner can attribute
// live traffic to the case that produced it and assert wire invariants
// once, at suite end.
//
// It also collects the union of response-header keys, with values for
// any quota-shaped name: the api.ynab.com prose documents an X-Rate-Limit
// header, but live traffic 2026-07-20 carried no rate-limit header at all
// (see API_NOTES.md), so the suite logs what the server actually sends
// instead of asserting a name it may never see.
type countingTransport struct {
	calls atomic.Int64

	mu          sync.Mutex
	seen        []recordedCall
	headerKeys  map[string]struct{}
	rateHeaders []string // "Key: value" for names matching rate|limit|quota
}

// recordedCall is one live request/response as the transport saw it.
type recordedCall struct {
	method      string
	path        string // includes the client's /v1 base-path prefix
	query       url.Values
	status      int // 0 when the round trip itself failed
	contentType string
	body        []byte
	sk          int64 // envelope server_knowledge; -1 when absent
}

// rateHeaderRe spots quota-shaped response-header names.
var rateHeaderRe = regexp.MustCompile(`(?i)rate|limit|quota`)

// skRe extracts the envelope's server_knowledge wherever it sits. The
// extraction is skip-tolerant by construction: envelopes without the
// field record -1, never a fake 0.
var skRe = regexp.MustCompile(`"server_knowledge":\s*(\d+)`)

func (ct *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ct.calls.Add(1)
	rc := recordedCall{method: req.Method, path: req.URL.Path, query: req.URL.Query(), sk: -1}

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		ct.record(rc, nil)
		return nil, err
	}
	rc.status = resp.StatusCode
	rc.contentType = resp.Header.Get("Content-Type")

	// Tee the body: read it fully, release the real connection, and hand
	// the client a replayed reader (DefaultTransport has already
	// transparently un-gzipped).
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		ct.record(rc, resp.Header)
		return nil, readErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	rc.body = body
	if m := skRe.FindSubmatch(body); m != nil {
		if v, perr := strconv.ParseInt(string(m[1]), 10, 64); perr == nil {
			rc.sk = v
		}
	}
	ct.record(rc, resp.Header)
	return resp, nil
}

// record appends rc and folds the response headers into the discovery
// sets, all under the mutex.
func (ct *countingTransport) record(rc recordedCall, header http.Header) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.seen = append(ct.seen, rc)
	if header == nil {
		return
	}
	if ct.headerKeys == nil {
		ct.headerKeys = map[string]struct{}{}
	}
	for key, values := range header {
		ct.headerKeys[key] = struct{}{}
		if rateHeaderRe.MatchString(key) {
			ct.rateHeaders = append(ct.rateHeaders, key+": "+strings.Join(values, ","))
		}
	}
}

// recorded returns a snapshot of every call so far.
func (ct *countingTransport) recorded() []recordedCall {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return slices.Clone(ct.seen)
}

// headerKeyUnion returns the sorted union of response-header keys seen.
func (ct *countingTransport) headerKeyUnion() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return slices.Sorted(maps.Keys(ct.headerKeys))
}

// rateHeaderValues returns every quota-shaped header observed, with its
// values — expected empty per the 2026-07-20 probe, logged so a renamed
// or returning rate-limit header is spotted in one run.
func (ct *countingTransport) rateHeaderValues() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return slices.Clone(ct.rateHeaders)
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
	t.Logf("live suite made %d requests", calls)
	t.Logf("response header keys observed: %v", transport.headerKeyUnion())
	t.Logf("quota-shaped headers observed (expected none per API_NOTES.md): %v",
		transport.rateHeaderValues())
	require.LessOrEqual(t, calls, int64(120), "the suite must stay well under the 200/h budget")
	require.NotContains(t, logBuf.String(), token, "the token must never reach the logs")
	require.NotZero(t, logBuf.Len(), "the redaction assertion must not pass vacuously")
}
