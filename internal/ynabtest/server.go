// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

// Package ynabtest is the in-repo fake YNAB API used by this module's own
// endpoint and consumer-seam tests. It serves the same golden fixtures the
// endpoint tests assert against — the fixture-honesty test keeps the two
// from ever diverging — and understands delta cursors and error
// injection. Delta cursors switch to *_delta fixtures on the four
// streams that have them (plan export, accounts, categories, payees);
// the other delta-capable streams serve their full list regardless of
// cursor. Method and path derive from contract.Table, so the fake cannot
// drift from the coverage contract. Internal on purpose: the public
// mocking seams are WithBaseURL+httptest and WithHTTPClient, not a
// second public surface.
package ynabtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"

	"pkg.venceslau.dev/ynab/internal/contract"
)

// fixtureSpec is the fake's own per-operation data: which fixture(s) an
// operation serves and how.
type fixtureSpec struct {
	fixture string
	// deltaFixture is served instead when last_knowledge_of_server is
	// present and non-empty.
	deltaFixture string
	status       int // 0 means 200
	// bodyKey selects between specs sharing method+path by a top-level
	// request body key (createTransaction single vs batch).
	bodyKey string
}

// fixtures is the one hand-maintained axis, keyed by operationId; verb
// and path come from contract.Table. routes panics on a key without a
// table row or a table row without a key.
var fixtures = map[string][]fixtureSpec{
	"getUser":                       {{fixture: "user/get.json"}},
	"getPlans":                      {{fixture: "plans/list.json"}},
	"getPlanById":                   {{fixture: "plans/export.json", deltaFixture: "plans/export_delta.json"}},
	"getPlanSettingsById":           {{fixture: "plans/settings.json"}},
	"getPlanMonths":                 {{fixture: "months/list.json"}},
	"getPlanMonth":                  {{fixture: "months/get.json"}},
	"getAccounts":                   {{fixture: "accounts/list.json", deltaFixture: "accounts/list_delta.json"}},
	"createAccount":                 {{fixture: "accounts/create.json", status: http.StatusCreated}},
	"getAccountById":                {{fixture: "accounts/get.json"}},
	"getCategories":                 {{fixture: "categories/list.json", deltaFixture: "categories/list_delta.json"}},
	"createCategory":                {{fixture: "categories/create.json", status: http.StatusCreated}},
	"getCategoryById":               {{fixture: "categories/get.json"}},
	"updateCategory":                {{fixture: "categories/update.json"}},
	"getMonthCategoryById":          {{fixture: "categories/get_for_month.json"}},
	"updateMonthCategory":           {{fixture: "categories/assign.json"}},
	"createCategoryGroup":           {{fixture: "categories/group_create.json", status: http.StatusCreated}},
	"updateCategoryGroup":           {{fixture: "categories/group_rename.json"}},
	"getPayees":                     {{fixture: "payees/list.json", deltaFixture: "payees/list_delta.json"}},
	"createPayee":                   {{fixture: "payees/create.json", status: http.StatusCreated}},
	"getPayeeById":                  {{fixture: "payees/get.json"}},
	"updatePayee":                   {{fixture: "payees/rename.json"}},
	"getPayeeLocations":             {{fixture: "payee_locations/list.json"}},
	"getPayeeLocationById":          {{fixture: "payee_locations/get.json"}},
	"getPayeeLocationsByPayee":      {{fixture: "payee_locations/by_payee.json"}},
	"getMoneyMovements":             {{fixture: "money_movements/list.json"}},
	"getMoneyMovementsByMonth":      {{fixture: "money_movements/by_month.json"}},
	"getMoneyMovementGroups":        {{fixture: "money_movements/groups.json"}},
	"getMoneyMovementGroupsByMonth": {{fixture: "money_movements/groups_by_month.json"}},
	"getTransactions":               {{fixture: "transactions/list.json"}},
	"createTransaction": {
		{fixture: "transactions/create.json", status: http.StatusCreated, bodyKey: "transaction"},
		{fixture: "transactions/batch.json", status: http.StatusCreated, bodyKey: "transactions"},
	},
	"updateTransactions":          {{fixture: "transactions/batch.json"}},
	"importTransactions":          {{fixture: "transactions/import_ids.json", status: http.StatusCreated}},
	"getTransactionById":          {{fixture: "transactions/get.json"}},
	"updateTransaction":           {{fixture: "transactions/update.json"}},
	"deleteTransaction":           {{fixture: "transactions/delete.json"}},
	"getTransactionsByAccount":    {{fixture: "transactions/list.json"}},
	"getTransactionsByCategory":   {{fixture: "transactions/hybrid.json"}},
	"getTransactionsByPayee":      {{fixture: "transactions/hybrid.json"}},
	"getTransactionsByMonth":      {{fixture: "transactions/list.json"}},
	"getScheduledTransactions":    {{fixture: "scheduled/list.json"}},
	"createScheduledTransaction":  {{fixture: "scheduled/create.json", status: http.StatusCreated}},
	"getScheduledTransactionById": {{fixture: "scheduled/get.json"}},
	"updateScheduledTransaction":  {{fixture: "scheduled/update.json"}},
	"deleteScheduledTransaction":  {{fixture: "scheduled/delete.json"}},
}

// route maps one request shape to its fixture spec.
type route struct {
	method  string
	pattern *regexp.Regexp
	fixtureSpec
}

// routes derives the fake API's surface from the contract table, compiled
// once. It panics on any table/fixtures mismatch, in either direction.
var routes = sync.OnceValue(func() []route {
	table := contract.Table()
	if len(fixtures) != len(table) {
		panic(fmt.Sprintf("ynabtest: %d fixture entries for %d table operations", len(fixtures), len(table)))
	}
	rts := make([]route, 0, len(table)+1)
	for _, op := range table {
		specs, ok := fixtures[op.ID]
		if !ok {
			panic("ynabtest: table operation " + op.ID + " has no fixture entry")
		}
		pattern := contract.PathRegexp(op.Path)
		for _, fs := range specs {
			rts = append(rts, route{method: op.Method, pattern: pattern, fixtureSpec: fs})
		}
	}
	return rts
})

// Server is a fake YNAB API bound to the module's golden fixtures.
type Server struct {
	// URL is the base URL to hand to ynab.WithBaseURL.
	URL string

	tb  testing.TB
	srv *httptest.Server

	mu         sync.Mutex
	failStatus int
	failID     string
	failName   string
}

// NewServer starts a fake API. With a non-nil tb the server closes with
// the test and fixture failures fail it; a nil tb (Example functions)
// panics on fixture failures and the caller must Close the server.
func NewServer(tb testing.TB) *Server {
	s := &Server{tb: tb}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	s.URL = s.srv.URL
	if tb != nil {
		tb.Helper()
		tb.Cleanup(s.srv.Close)
	}
	return s
}

// Close shuts the fake down — needed only for the nil-tb form.
func (s *Server) Close() {
	s.srv.Close()
}

// FailWith makes the next request answer a taxonomy-correct error
// envelope with the given status and error id, then resets. The
// injection is consumed by the first attempt — pair retryable statuses
// with ynab.WithRetryDisabled or the retry pipeline will eat them.
func (s *Server) FailWith(status int, id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failStatus, s.failID, s.failName = status, id, name
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	failStatus, failID, failName := s.failStatus, s.failID, s.failName
	s.failStatus, s.failID, s.failName = 0, "", ""
	s.mu.Unlock()

	if failStatus != 0 {
		w.WriteHeader(failStatus)
		envelope := map[string]any{"error": map[string]string{
			"id": failID, "name": failName, "detail": "injected by ynabtest.FailWith",
		}}
		_ = json.NewEncoder(w).Encode(envelope)
		return
	}

	rt, ok := s.resolve(r)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"id":"404.1","name":"not_found","detail":"ynabtest: no route"}}`))
		return
	}

	fixture := rt.fixture
	if rt.deltaFixture != "" && r.URL.Query().Get("last_knowledge_of_server") != "" {
		fixture = rt.deltaFixture
	}

	// Read before writing any header, and never t.Fatalf from the serve
	// goroutine — a broken fixture answers 500 with the reason instead.
	raw, err := os.ReadFile(filepath.Join(testdataDir(), fixture))
	if err != nil {
		if s.tb != nil {
			s.tb.Errorf("ynabtest: fixture %s: %v", fixture, err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w,
			`{"error":{"id":"500","name":"internal_server_error","detail":"ynabtest: fixture %s unreadable"}}`, fixture)
		return
	}
	status := rt.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// resolve finds the route for a request, disambiguating shared
// method+path routes by the request body's top-level key; the body is
// restored for any downstream reader.
func (s *Server) resolve(r *http.Request) (route, bool) {
	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var bodyKeys map[string]json.RawMessage
	_ = json.Unmarshal(raw, &bodyKeys)

	for _, rt := range routes() {
		if rt.method != r.Method || !rt.pattern.MatchString(r.URL.Path) {
			continue
		}
		if rt.bodyKey != "" {
			if _, has := bodyKeys[rt.bodyKey]; !has {
				continue
			}
		}
		return rt, true
	}
	return route{}, false
}

// Fixture reads a golden fixture from the module's testdata directory —
// the same files the endpoint tests assert against. A nil tb panics on
// read failure.
func Fixture(tb testing.TB, name string) []byte {
	raw, err := os.ReadFile(filepath.Join(testdataDir(), name))
	if err != nil {
		if tb == nil {
			panic(fmt.Sprintf("ynabtest: fixture %s: %v", name, err))
		}
		tb.Helper()
		tb.Fatalf("ynabtest: fixture %s: %v", name, err)
	}
	return raw
}

// FixtureNames lists every fixture the fake server can serve.
func FixtureNames() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, rt := range routes() {
		for _, f := range []string{rt.fixture, rt.deltaFixture} {
			if _, dup := seen[f]; f != "" && !dup {
				seen[f] = struct{}{}
				names = append(names, f)
			}
		}
	}
	return names
}

// testdataDir locates the module root's testdata directory relative to
// this source file, so callers in any package resolve the same files.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}
