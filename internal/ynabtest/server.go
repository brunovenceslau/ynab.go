// Package ynabtest is the in-repo fake YNAB API used by this module's own
// endpoint and consumer-seam tests. It serves the same golden fixtures the
// endpoint tests assert against — the fixture-honesty test keeps the two
// from ever diverging — and understands delta cursors and error
// injection. Delta cursors switch to *_delta fixtures on the four
// streams that have them (plan export, accounts, categories, payees);
// the other delta-capable streams serve their full list regardless of
// cursor. Internal on purpose: the public mocking seams are
// WithBaseURL+httptest and WithHTTPClient, not a second public surface.
package ynabtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"
)

// route maps one request shape to its fixture.
type route struct {
	method  string
	pattern *regexp.Regexp
	fixture string
	// deltaFixture is served instead when last_knowledge_of_server is
	// present and non-empty.
	deltaFixture string
	status       int // 0 means 200
	// bodyKey selects between routes sharing method+path by a top-level
	// request body key (createTransaction single vs batch).
	bodyKey string
}

// routes returns the fake API's surface, one line per wire shape,
// compiled once.
var routes = sync.OnceValue(func() []route {
	r := func(method, pattern, fixture string) route {
		return route{method: method, pattern: regexp.MustCompile("^" + pattern + "$"), fixture: fixture}
	}
	created := func(rt route) route { rt.status = http.StatusCreated; return rt }
	delta := func(rt route, fixture string) route { rt.deltaFixture = fixture; return rt }
	withBodyKey := func(rt route, key string) route { rt.bodyKey = key; return rt }

	const plan = `/plans/[^/]+`
	return []route{
		r(http.MethodGet, `/user`, "user/get.json"),
		r(http.MethodGet, `/plans`, "plans/list.json"),
		delta(r(http.MethodGet, plan, "plans/export.json"), "plans/export_delta.json"),
		r(http.MethodGet, plan+`/settings`, "plans/settings.json"),
		delta(r(http.MethodGet, plan+`/accounts`, "accounts/list.json"), "accounts/list_delta.json"),
		created(r(http.MethodPost, plan+`/accounts`, "accounts/create.json")),
		r(http.MethodGet, plan+`/accounts/[^/]+`, "accounts/get.json"),
		delta(r(http.MethodGet, plan+`/categories`, "categories/list.json"), "categories/list_delta.json"),
		created(r(http.MethodPost, plan+`/categories`, "categories/create.json")),
		r(http.MethodGet, plan+`/categories/[^/]+`, "categories/get.json"),
		r(http.MethodPatch, plan+`/categories/[^/]+`, "categories/update.json"),
		r(http.MethodGet, plan+`/months/[^/]+/categories/[^/]+`, "categories/month_get.json"),
		r(http.MethodPatch, plan+`/months/[^/]+/categories/[^/]+`, "categories/assign.json"),
		created(r(http.MethodPost, plan+`/category_groups`, "categories/group_create.json")),
		r(http.MethodPatch, plan+`/category_groups/[^/]+`, "categories/group_rename.json"),
		delta(r(http.MethodGet, plan+`/payees`, "payees/list.json"), "payees/list_delta.json"),
		created(r(http.MethodPost, plan+`/payees`, "payees/create.json")),
		r(http.MethodGet, plan+`/payees/[^/]+`, "payees/get.json"),
		r(http.MethodPatch, plan+`/payees/[^/]+`, "payees/rename.json"),
		r(http.MethodGet, plan+`/payee_locations`, "payee_locations/list.json"),
		r(http.MethodGet, plan+`/payee_locations/[^/]+`, "payee_locations/get.json"),
		r(http.MethodGet, plan+`/payees/[^/]+/payee_locations`, "payee_locations/by_payee.json"),
		r(http.MethodGet, plan+`/months`, "months/list.json"),
		r(http.MethodGet, plan+`/months/[^/]+`, "months/get.json"),
		r(http.MethodGet, plan+`/money_movements`, "money_movements/list.json"),
		r(http.MethodGet, plan+`/months/[^/]+/money_movements`, "money_movements/by_month.json"),
		r(http.MethodGet, plan+`/money_movement_groups`, "money_movements/groups.json"),
		r(http.MethodGet, plan+`/months/[^/]+/money_movement_groups`, "money_movements/groups_by_month.json"),
		r(http.MethodGet, plan+`/transactions`, "transactions/list.json"),
		withBodyKey(created(r(http.MethodPost, plan+`/transactions`, "transactions/create.json")), "transaction"),
		withBodyKey(created(r(http.MethodPost, plan+`/transactions`, "transactions/create_batch.json")), "transactions"),
		r(http.MethodPatch, plan+`/transactions`, "transactions/create_batch.json"),
		created(r(http.MethodPost, plan+`/transactions/import`, "transactions/import_ids.json")),
		r(http.MethodGet, plan+`/transactions/[^/]+`, "transactions/get.json"),
		r(http.MethodPut, plan+`/transactions/[^/]+`, "transactions/update.json"),
		r(http.MethodDelete, plan+`/transactions/[^/]+`, "transactions/delete.json"),
		r(http.MethodGet, plan+`/accounts/[^/]+/transactions`, "transactions/list.json"),
		r(http.MethodGet, plan+`/categories/[^/]+/transactions`, "transactions/hybrid.json"),
		r(http.MethodGet, plan+`/payees/[^/]+/transactions`, "transactions/hybrid.json"),
		r(http.MethodGet, plan+`/months/[^/]+/transactions`, "transactions/list.json"),
		r(http.MethodGet, plan+`/scheduled_transactions`, "scheduled/list.json"),
		created(r(http.MethodPost, plan+`/scheduled_transactions`, "scheduled/create.json")),
		r(http.MethodGet, plan+`/scheduled_transactions/[^/]+`, "scheduled/get.json"),
		r(http.MethodPut, plan+`/scheduled_transactions/[^/]+`, "scheduled/update.json"),
		r(http.MethodDelete, plan+`/scheduled_transactions/[^/]+`, "scheduled/delete.json"),
	}
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
// method+path routes by the request body's top-level key. Note: this
// drains r.Body — if the fake ever grows request recording, buffer and
// restore the body here first.
func (s *Server) resolve(r *http.Request) (route, bool) {
	var bodyKeys map[string]json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&bodyKeys)

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
	seen := map[string]bool{}
	var names []string
	for _, rt := range routes() {
		for _, f := range []string{rt.fixture, rt.deltaFixture} {
			if f != "" && !seen[f] {
				seen[f] = true
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
