// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// The G2 write-contract harness. Every write slice registers its cases in
// an init function; TestContractWrites replays them against a recording
// server and asserts verb, path, and byte-equivalent body + emitted key
// set. Adding a case from a slice is a handful of lines:
//
//	func init() {
//		registerWriteCase(writeCase{
//			op: "createPayee", method: http.MethodPost,
//			path: "/plans/p-1/payees",
//			body: `{"payee":{"name":"n"}}`,
//			call: func(t *testing.T, c *ynab.Client) {
//				_, _, err := c.Plan("p-1").Payees.Create(t.Context(), "n")
//				require.NoError(t, err)
//			},
//		})
//	}

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

// writeCase is one registered G2 write-contract case.
type writeCase struct {
	op      string // operationId (must be a table row)
	variant string // distinguishes multiple cases per op (may be empty)
	method  string
	path    string // exact request path, e.g. /plans/p-1/payees
	body    string // exact JSON body; "" asserts a bodiless request
	call    func(t *testing.T, c *ynab.Client)
}

var (
	writeRegistryMu sync.Mutex
	writeRegistry   []writeCase
)

// registerWriteCase is called from slice test files' init functions.
func registerWriteCase(wc writeCase) {
	writeRegistryMu.Lock()
	defer writeRegistryMu.Unlock()
	writeRegistry = append(writeRegistry, wc)
}

// recordedRequest is what the recording server captured.
type recordedRequest struct {
	method string
	path   string
	body   []byte
}

// runWriteCase drives wc.call against a recording server and asserts the
// captured request matches the case exactly.
func runWriteCase(t *testing.T, wc writeCase) {
	t.Helper()

	var mu sync.Mutex
	var rec *recordedRequest
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body) // a short read surfaces as a diff below
		mu.Lock()
		requests++
		rec = &recordedRequest{method: r.Method, path: r.URL.Path, body: b}
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(srv.Close)

	client := ynab.New("test-token", ynab.WithBaseURL(srv.URL), ynab.WithRetryDisabled())
	wc.call(t, client)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, rec, "the call must reach the wire")
	require.Equal(t, 1, requests, "a write must send exactly one request")
	require.Empty(t, diffRecorded(wc, *rec), "recorded %s %s body %s", rec.method, rec.path, rec.body)
}

// diffRecorded compares a recorded request against its case: verb, path,
// then body byte-equivalence plus emitted key set (an accidentally
// omitted Optional is a missing key).
func diffRecorded(wc writeCase, rec recordedRequest) []string {
	var problems []string
	if rec.method != wc.method {
		problems = append(problems, "verb: want "+wc.method+", got "+rec.method)
	}
	if rec.path != wc.path {
		problems = append(problems, "path: want "+wc.path+", got "+rec.path)
	}
	if wc.body == "" {
		if len(rec.body) != 0 {
			problems = append(problems, "unexpected request body: "+string(rec.body))
		}
		return problems
	}
	return append(problems, contract.DiffJSON([]byte(wc.body), rec.body)...)
}

// TestContractWrites is gate G2: registry completeness against the
// table's operation ids, then every registered case replayed byte-exact.
func TestContractWrites(t *testing.T) {
	t.Parallel()

	writeRegistryMu.Lock()
	cases := make([]writeCase, len(writeRegistry))
	copy(cases, writeRegistry)
	writeRegistryMu.Unlock()

	infos := make([]contract.WriteCaseInfo, 0, len(cases))
	for _, wc := range cases {
		infos = append(infos, contract.WriteCaseInfo{OpID: wc.op, HasBody: wc.body != ""})
	}
	require.Empty(t, contract.DiffWriteCoverage(contract.Table(), contract.TableIDs(), infos))

	// Each case's hand-written path must instantiate its operation's
	// spec-diffed table template — the trustworthy third source. A typo
	// copied from the implementation into the case (so recorded and
	// expected agree) fails here, naming the operation.
	rowByID := map[string]contract.Operation{}
	for _, row := range contract.Table() {
		rowByID[row.ID] = row
	}
	for _, wc := range cases {
		row, ok := rowByID[wc.op]
		require.True(t, ok, "write case %s names no table operation", wc.op)
		require.Regexp(t, contract.PathRegexp(row.Path), wc.path,
			"write case %s: path must instantiate the table template %s", wc.op, row.Path)
	}

	for _, wc := range cases {
		name := wc.op
		if wc.variant != "" {
			name += "/" + wc.variant
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runWriteCase(t, wc)
		})
	}
}

// TestContractWritesHarnessSelfCheck proves the harness detects every
// drift class, using synthetic cases — no stub ever touches the real
// registries.
func TestContractWritesHarnessSelfCheck(t *testing.T) {
	t.Parallel()

	base := writeCase{
		op:     "synthetic",
		method: http.MethodPost,
		path:   "/plans/p-1/things",
		body:   `{"thing":{"name":"n","approved":false}}`,
	}
	goodBody := []byte(`{"thing":{"name":"n","approved":false}}`)

	t.Run("clean case passes", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, diffRecorded(base, recordedRequest{method: "POST", path: base.path, body: goodBody}))
	})

	t.Run("wrong verb detected", func(t *testing.T) {
		t.Parallel()
		problems := diffRecorded(base, recordedRequest{method: "PUT", path: base.path, body: goodBody})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "verb")
	})

	t.Run("wrong path detected", func(t *testing.T) {
		t.Parallel()
		problems := diffRecorded(base, recordedRequest{method: "POST", path: "/plans/p-1/thing", body: goodBody})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "path")
	})

	t.Run("omitted Optional detected", func(t *testing.T) {
		t.Parallel()

		// The issue#24 class: approved:false silently dropped.
		rec := recordedRequest{method: "POST", path: base.path, body: []byte(`{"thing":{"name":"n"}}`)}
		problems := diffRecorded(base, rec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "missing key $.thing.approved")
	})

	t.Run("extra key detected", func(t *testing.T) {
		t.Parallel()
		rec := recordedRequest{
			method: "POST",
			path:   base.path,
			body:   []byte(`{"thing":{"name":"n","approved":false,"id":"x"}}`),
		}
		problems := diffRecorded(base, rec)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "unexpected key $.thing.id")
	})

	t.Run("body on a bodiless case detected", func(t *testing.T) {
		t.Parallel()

		bodiless := base
		bodiless.body = ""
		problems := diffRecorded(bodiless, recordedRequest{method: "POST", path: base.path, body: []byte(`{"x":1}`)})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "unexpected request body")
	})
}
