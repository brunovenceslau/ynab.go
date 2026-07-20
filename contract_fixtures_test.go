// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Gate G5: null fixtures. Nullable is mechanically defined — an exported
// pointer field of a registered response model — and every such field
// must appear with a literal null, at its exact (array-collapsed) path,
// in at least one registered null-variant fixture (*_null.json). Fixtures
// decode strictly (unknown keys are errors), so a fixture that drifts
// from its model shape fails loudly instead of passing vacuously. An
// out-of-range-numeric helper covers the uint8 overflow class.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// nullFixtureEntry ties a response model to a null-variant fixture.
// wrapper is the resource key under the envelope's data holding the model
// ("" when the model is the data object itself).
type nullFixtureEntry struct {
	model   any
	fixture string // path under testdata/, by convention *_null.json
	wrapper string
}

var (
	nullFixturesMu sync.Mutex
	nullFixtures   []nullFixtureEntry
)

// registerNullFixture is called from slice test files' init functions for
// every response model carrying nullable (pointer) fields.
func registerNullFixture(model any, fixture, wrapper string) {
	nullFixturesMu.Lock()
	defer nullFixturesMu.Unlock()
	nullFixtures = append(nullFixtures, nullFixtureEntry{model: model, fixture: fixture, wrapper: wrapper})
}

// TestContractNullFixtures is gate G5 over the registry.
func TestContractNullFixtures(t *testing.T) {
	t.Parallel()

	nullFixturesMu.Lock()
	entries := make([]nullFixtureEntry, len(nullFixtures))
	copy(entries, nullFixtures)
	nullFixturesMu.Unlock()

	loaded := make([]loadedNullFixture, 0, len(entries))
	registered := map[reflect.Type]struct{}{}
	for _, e := range entries {
		raw := modelDocument(t, loadFixture(t, e.fixture), e.wrapper)
		loaded = append(loaded, loadedNullFixture{model: e.model, raw: raw, name: e.fixture})
		registered[normalizeModelType(reflect.TypeOf(e.model))] = struct{}{}
	}
	require.Empty(t, nullCoverageProblems(loaded))

	// Every null fixture must decode strictly into its model: unknown
	// keys are errors, so shape drift cannot pass vacuously.
	for _, l := range loaded {
		require.NoError(t, strictDecode(l.raw, l.model), "fixture %s", l.name)
	}

	// Forcing rule: a registered read model carrying nullable (pointer)
	// fields cannot skip G5 — registration here is not voluntary.
	wireModelsMu.Lock()
	reads := append([]any{}, readModels...)
	wireModelsMu.Unlock()
	for _, m := range reads {
		mt := normalizeModelType(reflect.TypeOf(m))
		if len(pointerFieldPaths(mt)) > 0 {
			require.Contains(t, registered, mt,
				"%s has nullable fields but no registered null fixture", mt)
		}
	}
}

// normalizeModelType unwraps slices, arrays, maps, and pointers so a
// []Account registration and an Account registration land in one group.
func normalizeModelType(t reflect.Type) reflect.Type {
	for {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()
		default:
			return t
		}
	}
}

// loadedNullFixture is a registry entry with its model document extracted.
type loadedNullFixture struct {
	model any
	raw   []byte
	name  string
}

// modelDocument unwraps {"data": ...} and then the resource wrapper key.
func modelDocument(t *testing.T, raw []byte, wrapper string) []byte {
	t.Helper()

	var env struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	require.NotEmpty(t, env.Data, "fixture must carry the {\"data\": ...} envelope")
	if wrapper == "" {
		return env.Data
	}

	var inner map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(env.Data, &inner))
	doc, ok := inner[wrapper]
	require.True(t, ok, "envelope data has no %q key", wrapper)
	return doc
}

// strictDecode unmarshals raw into a new instance of model's type with
// unknown fields disallowed.
func strictDecode(raw []byte, model any) error {
	target := reflect.New(reflect.TypeOf(model)).Interface()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

// nullCoverageProblems checks, per model type, that every pointer field's
// tag path that appears in the model's fixtures carries a literal null in
// at least one of them. A path wholly absent from every fixture belongs
// to a wire shape this model never serves (PlanDetail's flat category
// groups, e.g.) and is exempt — but showing a key without ever showing
// its null is a gap.
func nullCoverageProblems(entries []loadedNullFixture) []string {
	var problems []string

	type modelGroup struct {
		paths   []string
		nulls   map[string]struct{}
		present map[string]struct{}
	}
	groups := map[reflect.Type]*modelGroup{}
	for _, e := range entries {
		mt := normalizeModelType(reflect.TypeOf(e.model))
		g, ok := groups[mt]
		if !ok {
			g = &modelGroup{
				paths:   pointerFieldPaths(mt),
				nulls:   map[string]struct{}{},
				present: map[string]struct{}{},
			}
			groups[mt] = g
		}
		var doc any
		if err := json.Unmarshal(e.raw, &doc); err != nil {
			problems = append(problems, e.name+": fixture is not valid JSON: "+err.Error())
			continue
		}
		collectNullPaths(doc, "$", g.nulls, g.present)
	}

	for mt, g := range groups {
		for _, p := range g.paths {
			_, present := g.present[p]
			_, null := g.nulls[p]
			if present && !null {
				problems = append(problems, mt.String()+": nullable field "+p+" is never null in any registered fixture")
			}
		}
	}
	return problems
}

// pointerFieldPaths walks a model type and returns the array-collapsed
// JSON tag paths of every exported pointer field — the mechanical
// definition of nullable. Paths distinguish nesting ($.payee_id vs
// $.subtransactions.payee_id), so a null at one level can never satisfy
// another.
func pointerFieldPaths(t reflect.Type) []string {
	var paths []string
	seen := map[reflect.Type]struct{}{}
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			walk(t.Elem(), prefix)
			return
		case reflect.Struct:
		default:
			return
		}
		if _, done := seen[t]; done || (t.PkgPath() != "" && !strings.HasPrefix(t.PkgPath(), "pkg.venceslau.dev/ynab")) {
			return
		}
		seen[t] = struct{}{}
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				if f.Anonymous {
					walk(f.Type, prefix)
				}
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			p := prefix
			if !f.Anonymous {
				if name == "" || name == "-" {
					continue
				}
				p = prefix + "." + name
			}
			if f.Type.Kind() == reflect.Pointer {
				paths = append(paths, p)
			}
			walk(f.Type, p)
		}
	}
	walk(t, "$")
	return paths
}

// collectNullPaths records every JSON path present in the document and
// which of them carry a literal null, collapsing array indices so
// fixtures and models align.
func collectNullPaths(doc any, prefix string, nulls, present map[string]struct{}) {
	switch v := doc.(type) {
	case map[string]any:
		for k, child := range v {
			p := prefix + "." + k
			present[p] = struct{}{}
			if child == nil {
				nulls[p] = struct{}{}
			}
			collectNullPaths(child, p, nulls, present)
		}
	case []any:
		for _, child := range v {
			collectNullPaths(child, prefix, nulls, present)
		}
	}
}

// runExtremeNumericsCase asserts extreme numeric magnitudes decode cleanly
// into a model — the uint8-overflow regression class this library's
// predecessor shipped.
func runExtremeNumericsCase(t *testing.T, model any, fixture, wrapper string) {
	t.Helper()

	raw := modelDocument(t, loadFixture(t, fixture), wrapper)
	require.NoError(t, strictDecode(raw, model),
		"extreme numeric fixture %s must decode", fixture)
}

// Self-checks with a synthetic model + fixtures, kept until real slices
// land their own.

type syntheticThing struct {
	Name    string           `json:"name"`
	PayeeID *string          `json:"payee_id"`
	Balance ynab.Milliunits  `json:"balance"`
	Note    *string          `json:"note"`
	Nested  syntheticDetails `json:"details"`
}

type syntheticDetails struct {
	TransferID *string `json:"transfer_id"`
}

func TestContractNullFixturesSelfCheck(t *testing.T) {
	t.Parallel()

	t.Run("complete null coverage passes", func(t *testing.T) {
		t.Parallel()

		entries := []loadedNullFixture{{
			model: syntheticThing{},
			raw:   modelDocument(t, loadFixture(t, "selftest/thing_null.json"), "thing"),
			name:  "selftest/thing_null.json",
		}}
		require.Empty(t, nullCoverageProblems(entries))
	})

	t.Run("uncovered pointer field detected", func(t *testing.T) {
		t.Parallel()

		entries := []loadedNullFixture{{
			model: syntheticThing{},
			raw:   []byte(`{"name":"n","payee_id":null,"note":null,"details":{"transfer_id":"x"}}`),
			name:  "inline",
		}}
		problems := nullCoverageProblems(entries)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "$.details.transfer_id is never null")
	})

	t.Run("null at the wrong nesting level does not satisfy", func(t *testing.T) {
		t.Parallel()

		// transfer_id is null at the TOP level only — the nested pointer
		// stays uncovered. Flat key matching would false-pass here.
		entries := []loadedNullFixture{{
			model: syntheticThing{},
			raw:   []byte(`{"name":"n","payee_id":null,"note":null,"transfer_id":null,"details":{"transfer_id":"x"}}`),
			name:  "inline",
		}}
		problems := nullCoverageProblems(entries)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "$.details.transfer_id is never null")
	})

	t.Run("strict decode rejects shape drift", func(t *testing.T) {
		t.Parallel()

		err := strictDecode([]byte(`{"name":"n","unexpected_key":1}`), syntheticThing{})
		require.Error(t, err)
	})

	t.Run("out of range numerics decode", func(t *testing.T) {
		t.Parallel()
		runExtremeNumericsCase(t, syntheticThing{}, "selftest/thing_extreme.json", "thing")
	})
}

func fixtureModels() map[string]struct {
	wrapper string
	model   any
} {
	m := map[string]struct {
		wrapper string
		model   any
	}{}
	add := func(pattern, wrapper string, model any) {
		m[pattern] = struct {
			wrapper string
			model   any
		}{wrapper, model}
	}

	add("user/get.json", "user", ynab.User{})
	add("plans/list*.json", "", ynab.PlanList{})
	add("plans/settings*.json", "settings", ynab.PlanSettings{})
	add("plans/export*.json", "plan", ynab.PlanDetail{})
	add("accounts/list*.json", "accounts", []ynab.Account{})
	add("accounts/get.json", "account", ynab.Account{})
	add("accounts/create.json", "account", ynab.Account{})
	add("accounts/extreme.json", "account", ynab.Account{})
	add("categories/list*.json", "category_groups", []ynab.CategoryGroup{})
	add("categories/get*.json", "category", ynab.Category{})
	add("categories/create.json", "category", ynab.Category{})
	add("categories/update.json", "category", ynab.Category{})
	add("categories/assign.json", "category", ynab.Category{})
	add("categories/get_for_month.json", "category", ynab.Category{})
	add("categories/extreme.json", "category", ynab.Category{})
	add("categories/group_create.json", "category_group", ynab.CategoryGroup{})
	add("categories/group_rename.json", "category_group", ynab.CategoryGroup{})
	add("months/list*.json", "months", []ynab.MonthSummary{})
	add("months/get*.json", "month", ynab.MonthDetail{})
	add("months/extreme.json", "month", ynab.MonthDetail{})
	add("payees/list*.json", "payees", []ynab.Payee{})
	add("payees/get.json", "payee", ynab.Payee{})
	add("payees/create.json", "payee", ynab.Payee{})
	add("payees/rename.json", "payee", ynab.Payee{})
	add("payee_locations/list.json", "payee_locations", []ynab.PayeeLocation{})
	add("payee_locations/by_payee.json", "payee_locations", []ynab.PayeeLocation{})
	add("payee_locations/get.json", "payee_location", ynab.PayeeLocation{})
	add("money_movements/list*.json", "money_movements", []ynab.MoneyMovement{})
	add("money_movements/by_month.json", "money_movements", []ynab.MoneyMovement{})
	add("money_movements/groups*.json", "money_movement_groups", []ynab.MoneyMovementGroup{})
	add("money_movements/extreme.json", "money_movements", []ynab.MoneyMovement{})
	add("transactions/list*.json", "transactions", []ynab.Transaction{})
	add("transactions/hybrid*.json", "transactions", []ynab.HybridTransaction{})
	add("transactions/get*.json", "transaction", ynab.Transaction{})
	add("transactions/update.json", "transaction", ynab.Transaction{})
	add("transactions/delete.json", "transaction", ynab.Transaction{})
	add("transactions/extreme.json", "transaction", ynab.Transaction{})
	add("transactions/create.json", "transaction", ynab.Transaction{})
	add("transactions/batch*.json", "", ynab.BatchResult{})
	add("transactions/import_*.json", "transaction_ids", []string{})
	add("scheduled/list*.json", "scheduled_transactions", []ynab.ScheduledTransaction{})
	add("scheduled/get.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/create.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/update.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/delete.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	add("scheduled/extreme.json", "scheduled_transaction", ynab.ScheduledTransaction{})
	return m
}

func TestFixturesDecodeStrict(t *testing.T) {
	t.Parallel()

	models := fixtureModels()
	match := func(name string) (string, any, bool) {
		for pattern, mm := range models {
			ok, err := filepath.Match(pattern, name)
			require.NoError(t, err)
			if ok {
				return mm.wrapper, mm.model, true
			}
		}
		return "", nil, false
	}

	var checked int
	err := filepath.WalkDir("testdata", func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() || !strings.HasSuffix(path, ".json") || strings.HasPrefix(path, filepath.Join("testdata", "selftest")) {
			return nil
		}
		rel, err := filepath.Rel("testdata", path)
		require.NoError(t, err)
		rel = filepath.ToSlash(rel)

		wrapper, model, ok := match(rel)
		require.True(t, ok, "fixture %s has no strict-decode mapping — add it to fixtureModels", rel)

		doc := modelDocument(t, loadFixture(t, rel), wrapper)
		require.NoError(t, strictDecode(doc, model), "fixture %s must strict-decode into %T", rel, model)
		checked++
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, checked, 40, "the walk must actually visit the fixture tree")
}
