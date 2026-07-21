// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

//go:build integration

package ynab_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mime"
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

	"github.com/stretchr/testify/assert"
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
	violations  []string // per-request invariant violations, asserted once
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

// checkf records a violated per-request invariant. The suite asserts the
// collected set empty once at the end — never FailNow inside RoundTrip.
func (ct *countingTransport) checkf(cond bool, format string, args ...any) {
	if cond {
		return
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.violations = append(ct.violations, fmt.Sprintf(format, args...))
}

// checkRequest collects the per-request invariants only live traffic can
// prove: the packaged default base URL (every fixture test overrides it,
// so https + api.ynab.com is exercised end-to-end only here) and the
// Accept header (asserted in no other test). Authorization and User-Agent
// are re-checked globally as well — unit-pinned per request shape at
// internal/transport/transport_test.go.
func (ct *countingTransport) checkRequest(req *http.Request) {
	ct.checkf(req.URL.Scheme == "https" && req.URL.Host == "api.ynab.com",
		"%s %s escaped the packaged base URL", req.Method, req.URL)
	auth := req.Header.Get("Authorization")
	ct.checkf(strings.HasPrefix(auth, "Bearer ") && len(auth) > len("Bearer "),
		"%s %s carries a malformed Authorization header", req.Method, req.URL.Path)
	ct.checkf(len(req.Header.Values("Authorization")) == 1,
		"%s %s carries %d Authorization headers — exactly one credential header allowed",
		req.Method, req.URL.Path, len(req.Header.Values("Authorization")))
	ct.checkf(strings.HasPrefix(req.Header.Get("User-Agent"), "pkg.venceslau.dev/ynab/"),
		"%s %s carries User-Agent %q", req.Method, req.URL.Path, req.Header.Get("User-Agent"))
	ct.checkf(req.Header.Get("Accept") == "application/json",
		"%s %s carries Accept %q", req.Method, req.URL.Path, req.Header.Get("Accept"))
}

// allViolations returns a snapshot of the collected violations.
func (ct *countingTransport) allViolations() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return slices.Clone(ct.violations)
}

func (ct *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ct.calls.Add(1)
	ct.checkRequest(req)
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

// allowedStatuses maps an operation to the statuses the spec declares
// for it; everything else must answer plain 200. The retry pipeline's
// consumable statuses (429/500/503) are tolerated everywhere because the
// transport records every attempt, including retried ones.
func allowedStatuses(opID string) []int {
	switch opID {
	case "createAccount", "createCategory", "createCategoryGroup", "createPayee",
		"createScheduledTransaction", "createTransaction":
		// The spec declares exactly 201 for every create.
		return []int{201}
	case "importTransactions":
		// 200 "nothing to import" vs 201 "imported" — both documented.
		return []int{200, 201}
	case "getScheduledTransactions":
		// The empty-plan 404 the client folds into an empty list.
		return []int{200, 404}
	case "deleteTransaction", "deleteScheduledTransaction":
		// Tolerant cleanup re-deletes record a real 404 every run.
		return []int{200, 404}
	case "getPayeeLocationById":
		// The suite's deliberate unknown-id 404 probe.
		return []int{200, 404}
	default:
		return []int{200}
	}
}

// assertResponseInvariants checks every recorded response against the
// spec's status taxonomy, requires application/json on 2xx (exactly where
// the client decodes unconditionally), and proves the envelope invariant
// on real traffic. assert, not require: one drifted response must not
// hide the rest of a run that costs a full quota slice to repeat.
func assertResponseInvariants(t *testing.T, transport *countingTransport) {
	t.Helper()
	for _, rc := range transport.recorded() {
		if rc.status == 0 {
			continue // transport error — the case itself reported it
		}
		opID, ok := matchOperation(rc)
		if !ok {
			continue // the per-case window check already failed this
		}
		retryable := rc.status == 429 || rc.status == 500 || rc.status == 503
		assert.True(t, slices.Contains(allowedStatuses(opID), rc.status) || retryable,
			"%s answered undocumented status %d (%s %s)", opID, rc.status, rc.method, rc.path)
		if retryable {
			continue // an interposing edge layer may legitimately answer non-JSON here
		}
		if rc.status >= 200 && rc.status < 300 {
			mt, _, mimeErr := mime.ParseMediaType(rc.contentType)
			if mimeErr != nil {
				assert.Fail(t, "unparseable Content-Type",
					"op %s answered Content-Type %q: %v", opID, rc.contentType, mimeErr)
			} else {
				assert.Equal(t, "application/json", mt,
					"op %s answered Content-Type %q — the client decodes 2xx as JSON unconditionally",
					opID, rc.contentType)
			}
		}
		assertEnvelope(t, opID, rc)
	}
}

// assertEnvelope proves the envelope invariant the client assumes per
// decode, on real traffic: a 2xx always carries data and never error; a
// non-2xx the inverse.
func assertEnvelope(t *testing.T, opID string, rc recordedCall) {
	t.Helper()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rc.body, &envelope); err != nil {
		assert.Fail(t, "non-JSON envelope", "op %s (status %d) answered a non-JSON body: %v",
			opID, rc.status, err)
		return
	}
	dataSet, errSet := rawSet(envelope.Data), rawSet(envelope.Error)
	if rc.status >= 200 && rc.status < 300 {
		assert.True(t, dataSet && !errSet,
			"op %s: a 2xx must carry non-null data and a null error (status %d)", opID, rc.status)
	} else {
		assert.True(t, errSet && !dataSet,
			"op %s: a non-2xx must carry a non-null error and null data (status %d)", opID, rc.status)
	}
}

// rawSet reports a present, non-null JSON value.
func rawSet(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

// assertServerKnowledgeMonotonic requires each operation stream's
// server_knowledge sequence non-decreasing in wall-clock order — the
// documented per-(plan, stream) contract (sync.go). The merged
// cross-stream sequence is only logged: whether the server backs every
// stream with one plan-global counter is unconfirmed, so it must not be
// asserted until real traffic says so.
func assertServerKnowledgeMonotonic(t *testing.T, transport *countingTransport) {
	t.Helper()
	lastSK := map[string]int64{}
	var merged []int64
	for _, rc := range transport.recorded() {
		if rc.sk < 0 || rc.status < 200 || rc.status > 299 {
			continue // skip-tolerant: sk-less envelopes and error answers
		}
		opID, ok := matchOperation(rc)
		if !ok {
			continue
		}
		if prev, seen := lastSK[opID]; seen {
			assert.GreaterOrEqual(t, rc.sk, prev,
				"server_knowledge regressed on %s: %s %s answered %d after %d",
				opID, rc.method, rc.path, rc.sk, prev)
		}
		lastSK[opID] = rc.sk
		merged = append(merged, rc.sk)
	}
	t.Logf("server_knowledge merged cross-stream sequence (%d samples): %v", len(merged), merged)
}

// assertPlanScopedWrites turns the dedicated-test-plan doctrine into an
// assertion: every non-GET request must stay under the test plan's path.
// It catches exactly the two real hazards — an accidental write through
// the read-only PlanIDLastUsed alias, or a hard-coded plan id.
func assertPlanScopedWrites(t *testing.T, transport *countingTransport, planID string) {
	t.Helper()
	prefix := "/v1/plans/" + planID + "/"
	for _, rc := range transport.recorded() {
		if rc.method == http.MethodGet {
			continue
		}
		assert.True(t, strings.HasPrefix(rc.path, prefix),
			"write %s %s escaped the dedicated test plan", rc.method, rc.path)
	}
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
				// assert, not require: FailNow mid-cleanup would report only
				// the first drift and hide the rest — each rerun costs a
				// full quota slice, so every drift must surface in one run.
				for _, rc := range transport.recorded()[before:] {
					opID, ok := matchOperation(rc)
					if !assert.True(t, ok, "case %q sent %s %s, which matches no contract operation",
						c.name, rc.method, rc.path) {
						continue
					}
					assert.True(t, declared[opID],
						"case %q hit %s but does not declare it — its ops list has drifted from its body",
						c.name, opID)
				}
			})
			c.run(t, env) // no t.Parallel: sequential by doctrine
		})
	}

	// Wire-level invariants over the whole recorded run, asserted once.
	require.Empty(t, transport.allViolations(), "per-request invariant violations")
	assertResponseInvariants(t, transport)
	assertServerKnowledgeMonotonic(t, transport)
	assertPlanScopedWrites(t, transport, planID)

	calls := transport.calls.Load()
	t.Logf("live suite made %d requests", calls)
	t.Logf("response header keys observed: %v", transport.headerKeyUnion())
	t.Logf("quota-shaped headers observed (expected none per API_NOTES.md): %v",
		transport.rateHeaderValues())
	require.LessOrEqual(t, calls, int64(120), "the suite must stay well under the 200/h budget")
	require.NotContains(t, logBuf.String(), token, "the token must never reach the logs")
	require.NotContains(t, logBuf.String(), "Bearer ", "the credential scheme must never reach the logs")
	// Exact non-vacuity: every request must have produced exactly one
	// logged request line, tying the WithLogger wiring to the transport's
	// ground-truth counter — the redaction check covered ALL traffic.
	require.Equal(t, calls, int64(strings.Count(logBuf.String(), "ynab: request")),
		"every live request must produce exactly one logged request line")
	require.NotZero(t, logBuf.Len(), "the redaction assertion must not pass vacuously")
}
