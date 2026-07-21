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
	"reflect"
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

// skipOrFail skips for missing live credentials — except under
// YNAB_LIVE_REQUIRED=1 (set by this repo's own environments, local and
// CI), where a missing credential is a loud failure: a skip there would
// silently erase live coverage. Third-party clones keep the skip.
func skipOrFail(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("YNAB_LIVE_REQUIRED") == "1" {
		t.Fatalf("YNAB_LIVE_REQUIRED=1 but %s", reason)
	}
	t.Skip(reason)
}

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
	headerKeys  map[string]map[string]struct{} // status class ("2xx", "429", …) → key set
	rateHeaders []string                       // "status Key: value" for names matching rate|limit|quota
	violations  []string                       // per-request invariant violations, asserted once
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
// sets, all under the mutex. Header keys are bucketed by status class:
// if a rate-limit header exists only near the quota (or only on 429s),
// an organic throttle self-documents which headers arrived with it.
func (ct *countingTransport) record(rc recordedCall, header http.Header) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.seen = append(ct.seen, rc)
	if header == nil {
		return
	}
	class := statusClass(rc.status)
	if ct.headerKeys == nil {
		ct.headerKeys = map[string]map[string]struct{}{}
	}
	if ct.headerKeys[class] == nil {
		ct.headerKeys[class] = map[string]struct{}{}
	}
	for key, values := range header {
		ct.headerKeys[class][key] = struct{}{}
		if rateHeaderRe.MatchString(key) {
			ct.rateHeaders = append(ct.rateHeaders,
				class+" "+key+": "+strings.Join(values, ","))
		}
	}
}

// statusClass buckets a status for header discovery: throttle and server
// errors keep their exact code, everything else folds to its class.
func statusClass(status int) string {
	switch {
	case status == 429 || status >= 500:
		return strconv.Itoa(status)
	case status >= 200 && status < 300:
		return "2xx"
	default:
		return "4xx"
	}
}

// recorded returns a snapshot of every call so far.
func (ct *countingTransport) recorded() []recordedCall {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return slices.Clone(ct.seen)
}

// headerKeyUnion returns the sorted union of response-header keys seen,
// prefixed by status class.
func (ct *countingTransport) headerKeyUnion() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	var out []string
	for class, keys := range ct.headerKeys {
		for key := range keys {
			out = append(out, class+" "+key)
		}
	}
	slices.Sort(out)
	return out
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
		"createScheduledTransaction":
		// The spec declares exactly 201 for every create.
		return []int{201}
	case "createTransaction":
		// 201, plus the deliberate duplicate-import_id probe's 409: the
		// transactions-writes case records a real 409 every run.
		return []int{201, 409}
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
		op, ok := matchOperation(rc)
		if !ok {
			continue // the per-case window check already failed this
		}
		opID := op.ID
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
		op, ok := matchOperation(rc)
		if !ok {
			continue
		}
		if prev, seen := lastSK[op.ID]; seen {
			assert.GreaterOrEqual(t, rc.sk, prev,
				"server_knowledge regressed on %s: %s %s answered %d after %d",
				op.ID, rc.method, rc.path, rc.sk, prev)
		}
		lastSK[op.ID] = rc.sk
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

// matchOperation maps a recorded live request to its contract row by
// matching against the table's path templates, after stripping the
// client's /v1 base-path prefix. First match wins — safe because
// TestMatchOperationUnambiguous freezes template uniqueness per method.
func matchOperation(rc recordedCall) (contract.Operation, bool) {
	path := strings.TrimPrefix(rc.path, "/v1")
	for _, op := range contract.Table() {
		if op.Method == rc.method && contract.PathRegexp(op.Path).MatchString(path) {
			return op, true
		}
	}
	return contract.Operation{}, false
}

// checkCaseWindow attributes one case's recorded window to its declared
// operations, both ways: recorded ⊆ ops ∪ condOps AND ops ⊆ recorded —
// a stale ops entry cannot keep the coverage gate green while live
// coverage silently vanishes. Sent query parameters must be declared by
// the matched table row. assert, not require: FailNow mid-cleanup would
// report only the first drift and hide the rest, and each rerun costs a
// full quota slice, so every drift must surface in one run.
func checkCaseWindow(t *testing.T, c integrationCase, window []recordedCall) {
	t.Helper()
	allowed := map[string]bool{}
	for _, op := range c.ops {
		allowed[op] = true
	}
	for _, op := range c.condOps {
		allowed[op] = true
	}
	hit := map[string]bool{}
	for _, rc := range window {
		op, ok := matchOperation(rc)
		if !assert.True(t, ok, "case %q sent %s %s, which matches no contract operation",
			c.name, rc.method, rc.path) {
			continue
		}
		assert.True(t, allowed[op.ID],
			"case %q hit %s but does not declare it — its ops list has drifted from its body",
			c.name, op.ID)
		hit[op.ID] = true
		// Query-param discipline on real traffic: only the table's
		// spec-mirrored parameters may ever leave the client.
		for param := range rc.query {
			assert.Contains(t, op.QueryParams, param,
				"case %q sent undeclared query param %q on %s", c.name, param, op.ID)
		}
	}
	for _, op := range c.ops {
		assert.True(t, hit[op],
			"case %q declares %s but never sent it live — a stale ops entry keeps the "+
				"coverage gate green while live coverage is gone", c.name, op)
	}
}

// liveFieldAllowlist names every wire field a live run is NOT expected
// to observe with a non-trivial value, each with the reason it cannot
// be seeded through the API. Anything else going unobserved fails the
// suite: every reachable field must be exercised live.
var liveFieldAllowlist = map[string]string{
	"payee_locations": "payee locations cannot be created via the API (mobile-app writes only; API_NOTES.md)",
	"flag_name": "flag names are user-defined color labels set only in the app; " +
		"API-set colors carry a null name even on reads (probed live 2026-07-21)",
	"last_reconciled_at": "stamped by the app's reconciliation flow only; " +
		"a reconciled-status write does not touch it (probed live 2026-07-21)",
	"latitude":  "payee-location field — see payee_locations",
	"longitude": "payee-location field — see payee_locations",
	"matched_transaction_id": "set only by the linked-account import pipeline; " +
		"asserted nil on API-created rows",
	"import_payee_name_original": "import-pipeline field — a linkless plan never carries it",
	"original_category_group_id": "deprecated: the server always answers null (documented on the field)",
	"debt_original_balance":      "deprecated: the server always answers null (documented on the field)",
	"debt_interest_rates": "populated only for app-created debt accounts with terms; " +
		"API account creation cannot set them",
	"debt_minimum_payments": "populated only for app-created debt accounts with terms; " +
		"API account creation cannot set them",
	"debt_escrow_amounts": "populated only for app-created debt accounts with terms; " +
		"API account creation cannot set them",
	"debt_transaction_type":   "appears only on transactions of app-created debt accounts",
	"goal_snoozed_at":         "goal snoozing is an app-only action",
	"group_created_at":        "money-movement groups are created by app-side grouped moves only",
	"money_movement_group_id": "money-movement groups are created by app-side grouped moves only",
	"scheduled_subtransactions": "split scheduled transactions cannot be created through the API " +
		"(documented on ScheduledTransactionSpec)",
	"scheduled_transaction_id": "set when the schedule engine spawns a transaction on its date — " +
		"not forceable within a test run",
	"default_plan": "null under a PAT; the OAuth grant's consent-selected default is " +
		"asserted live in TestLiveOAuth instead",
}

// modelFieldTags walks every registered read model and returns the set
// of json tag names the wire can carry.
func modelFieldTags() map[string]struct{} {
	tags := map[string]struct{}{}
	seen := map[reflect.Type]struct{}{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			walk(t.Elem())
			return
		case reflect.Struct:
		default:
			return
		}
		if _, done := seen[t]; done || !strings.HasPrefix(t.PkgPath(), "pkg.venceslau.dev/ynab") {
			return
		}
		seen[t] = struct{}{}
		for i := range t.NumField() {
			f := t.Field(i)
			if f.Anonymous {
				walk(f.Type)
				continue
			}
			tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if tag != "" && tag != "-" {
				tags[tag] = struct{}{}
			}
			walk(f.Type)
		}
	}
	wireModelsMu.Lock()
	models := slices.Clone(readModels)
	wireModelsMu.Unlock()
	for _, m := range models {
		walk(reflect.TypeOf(m))
	}
	return tags
}

// observedKeys collects every JSON key that carried a non-trivial value
// (not null, not "", not [], not {}) anywhere in the recorded bodies.
func observedKeys(calls []recordedCall) map[string]struct{} {
	got := map[string]struct{}{}
	for _, rc := range calls {
		if rc.status < 200 || rc.status > 299 || len(rc.body) == 0 {
			continue
		}
		var doc any
		if json.Unmarshal(rc.body, &doc) == nil {
			walkObserved(doc, got)
		}
	}
	return got
}

// nonTrivial reports whether a decoded JSON value counts as exercised:
// null, "", [], and {} do not.
func nonTrivial(v any) bool {
	switch tv := v.(type) {
	case nil:
		return false
	case string:
		return tv != ""
	case []any:
		return len(tv) > 0
	case map[string]any:
		return len(tv) > 0
	default:
		return true
	}
}

// walkObserved folds every non-trivially-valued key in doc into got.
func walkObserved(v any, got map[string]struct{}) {
	switch tv := v.(type) {
	case map[string]any:
		for k, child := range tv {
			if nonTrivial(child) {
				got[k] = struct{}{}
			}
			walkObserved(child, got)
		}
	case []any:
		for _, child := range tv {
			walkObserved(child, got)
		}
	}
}

// checkAllowlistFresh fails on allowlist rot in both directions: an
// entry no wire model carries, or one the live traffic observed anyway.
func checkAllowlistFresh(t *testing.T, tags, observed map[string]struct{}) {
	t.Helper()
	for tag, reason := range liveFieldAllowlist {
		if _, inModels := tags[tag]; !inModels {
			assert.Fail(t, "stale allowlist entry",
				"%s is in no wire model (reason was: %s)", tag, reason)
		}
		if _, ok := observed[tag]; ok {
			assert.Fail(t, "stale allowlist entry",
				"%s WAS observed live — drop it (reason was: %s)", tag, reason)
		}
	}
}

// checkFieldCoverage is the every-field-live gate: each wire field must
// be observed with a non-trivial value in this run's live traffic, or
// carry an explicit allowlist reason. Both directions: stale allowlist
// entries (field observed anyway, or tag no longer in any model) fail
// too, so the list cannot rot.
func checkFieldCoverage(t *testing.T, calls []recordedCall, casesRan, casesTotal int) {
	t.Helper()
	if casesRan < casesTotal {
		t.Logf("field coverage skipped: partial run (%d/%d cases)", casesRan, casesTotal)
		return
	}
	tags := modelFieldTags()
	observed := observedKeys(calls)

	var missing []string
	for tag := range tags {
		if _, ok := observed[tag]; ok {
			continue
		}
		if _, allowed := liveFieldAllowlist[tag]; allowed {
			continue
		}
		missing = append(missing, tag)
	}
	slices.Sort(missing)
	assert.Empty(t, missing,
		"wire fields never observed with a non-trivial value in live traffic — "+
			"seed them or allowlist with a reason")

	checkAllowlistFresh(t, tags, observed)
	t.Logf("field coverage: %d wire fields, %d observed live, %d allowlisted",
		len(tags), len(observed), len(liveFieldAllowlist))
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
		skipOrFail(t, "YNAB_TEST_TOKEN not set — live integration runs only against a dedicated test plan")
	}
	planID := os.Getenv("YNAB_TEST_PLAN_ID")
	if planID == "" {
		skipOrFail(t, "YNAB_TEST_PLAN_ID not set — the suite writes, and writes only ever touch the dedicated test plan")
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
		// The backward scan picks the FINAL attempt of a retried request.
		LastStatus: func(opID string) (int, bool) {
			for _, rc := range slices.Backward(transport.recorded()) {
				if op, ok := matchOperation(rc); ok && op.ID == opID {
					return rc.status, true
				}
			}
			return 0, false
		},
	}

	integrationMu.Lock()
	cases := make([]integrationCase, len(integrationCases))
	copy(cases, integrationCases)
	integrationMu.Unlock()
	slices.SortStableFunc(cases, func(a, b integrationCase) int { return a.order - b.order })

	// Per-case request accounting: budget regressions must be attributable
	// long before the suite-wide hard cap bursts. Sequential suite, and
	// subtest cleanups finish before t.Run returns — a plain map is safe.
	caseCounts := map[string]int{}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The suite is sequential, so a before/after window attributes
			// every recorded request to this case. The check runs from a
			// Cleanup registered FIRST: cleanups run LIFO, so it sees the
			// case's own cleanup traffic (deletes, restores) too. Both
			// directions are enforced — recorded ⊆ ops ∪ condOps AND
			// ops ⊆ recorded — with set semantics, so retries cannot
			// double-count and a stale ops entry cannot hide lost coverage.
			before := len(transport.recorded())
			t.Cleanup(func() {
				window := transport.recorded()[before:]
				checkCaseWindow(t, c, window)
				caseCounts[c.name] = len(window)
				t.Logf("case %q: %d live requests", c.name, len(window))
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
	for _, name := range slices.Sorted(maps.Keys(caseCounts)) {
		t.Logf("  %3d  %s", caseCounts[name], name)
	}
	casesWithTraffic := 0
	for _, n := range caseCounts {
		if n > 0 {
			casesWithTraffic++
		}
	}
	checkFieldCoverage(t, transport.recorded(), casesWithTraffic, len(cases))
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
