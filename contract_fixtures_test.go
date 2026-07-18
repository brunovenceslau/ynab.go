package ynab_test

// Gate G5: null fixtures. Nullable is mechanically defined — an exported
// pointer field of a registered response model — and every such field
// must appear with a literal null in at least one registered null-variant
// fixture (*_null.json). An out-of-range-numeric helper covers the uint8
// overflow class: extreme int64 magnitudes must decode cleanly.

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// nullFixtureEntry ties a response model to a null-variant fixture.
type nullFixtureEntry struct {
	model   any
	fixture string // path under testdata/, by convention *_null.json
}

var (
	nullFixturesMu sync.Mutex
	nullFixtures   []nullFixtureEntry
)

// registerNullFixture is called from slice test files' init functions for
// every response model carrying nullable (pointer) fields.
func registerNullFixture(model any, fixture string) {
	nullFixturesMu.Lock()
	defer nullFixturesMu.Unlock()
	nullFixtures = append(nullFixtures, nullFixtureEntry{model: model, fixture: fixture})
}

// TestContractNullFixtures is gate G5 over the registry.
func TestContractNullFixtures(t *testing.T) {
	t.Parallel()

	nullFixturesMu.Lock()
	entries := make([]nullFixtureEntry, len(nullFixtures))
	copy(entries, nullFixtures)
	nullFixturesMu.Unlock()

	loaded := make([]loadedNullFixture, 0, len(entries))
	for _, e := range entries {
		loaded = append(loaded, loadedNullFixture{model: e.model, raw: loadFixture(t, e.fixture), name: e.fixture})
	}
	require.Empty(t, nullCoverageProblems(loaded))

	// Every null fixture must also decode cleanly into its model.
	for _, l := range loaded {
		target := reflect.New(reflect.TypeOf(l.model)).Interface()
		require.NoError(t, json.Unmarshal(extractData(t, l.raw), target), "fixture %s", l.name)
	}
}

// loadedNullFixture is a registry entry with its fixture bytes read.
type loadedNullFixture struct {
	model any
	raw   []byte
	name  string
}

// nullCoverageProblems checks, per model type, that every pointer field's
// JSON key appears with a literal null in at least one of its fixtures.
func nullCoverageProblems(entries []loadedNullFixture) []string {
	var problems []string

	type modelGroup struct {
		tags  []string
		nulls map[string]bool
	}
	groups := map[reflect.Type]*modelGroup{}
	for _, e := range entries {
		mt := reflect.TypeOf(e.model)
		g, ok := groups[mt]
		if !ok {
			g = &modelGroup{tags: pointerFieldTags(mt), nulls: map[string]bool{}}
			groups[mt] = g
		}
		var doc any
		if err := json.Unmarshal(e.raw, &doc); err != nil {
			problems = append(problems, e.name+": fixture is not valid JSON: "+err.Error())
			continue
		}
		collectNullKeys(doc, g.nulls)
	}

	for mt, g := range groups {
		for _, tag := range g.tags {
			if !g.nulls[tag] {
				problems = append(problems, mt.String()+": nullable field "+tag+" is never null in any registered fixture")
			}
		}
	}
	return problems
}

// pointerFieldTags walks a model type and returns the JSON tag names of
// every exported pointer field — the mechanical definition of nullable.
func pointerFieldTags(t reflect.Type) []string {
	var tags []string
	seen := map[reflect.Type]bool{}
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
		if seen[t] || (t.PkgPath() != "" && !strings.HasPrefix(t.PkgPath(), "pkg.venceslau.dev/ynab")) {
			return
		}
		seen[t] = true
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			if f.Type.Kind() == reflect.Pointer {
				if name, _, _ := strings.Cut(f.Tag.Get("json"), ","); name != "" && name != "-" {
					tags = append(tags, name)
				}
			}
			walk(f.Type)
		}
	}
	walk(t)
	return tags
}

// collectNullKeys records every JSON key whose value is a literal null.
func collectNullKeys(doc any, out map[string]bool) {
	switch v := doc.(type) {
	case map[string]any:
		for k, child := range v {
			if child == nil {
				out[k] = true
			}
			collectNullKeys(child, out)
		}
	case []any:
		for _, child := range v {
			collectNullKeys(child, out)
		}
	}
}

// extractData unwraps a fixture's {"data": ...} envelope for direct model
// decodes; fixtures without the envelope are returned whole.
func extractData(t *testing.T, raw []byte) []byte {
	t.Helper()

	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Data) > 0 {
		return env.Data
	}
	return raw
}

// runOutOfRangeCase asserts extreme numeric magnitudes decode cleanly
// into a model — the uint8-overflow class of the pr-era regressions.
func runOutOfRangeCase(t *testing.T, model any, fixture string) {
	t.Helper()

	target := reflect.New(reflect.TypeOf(model)).Interface()
	require.NoError(t, json.Unmarshal(extractData(t, loadFixture(t, fixture)), target),
		"extreme numeric fixture %s must decode", fixture)
}

// Self-checks with a synthetic model + fixtures, kept until real slices
// land their own (Task 16+).

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
			raw:   loadFixture(t, "selftest/thing_null.json"),
			name:  "selftest/thing_null.json",
		}}
		require.Empty(t, nullCoverageProblems(entries))
	})

	t.Run("uncovered pointer field detected", func(t *testing.T) {
		t.Parallel()

		entries := []loadedNullFixture{{
			model: syntheticThing{},
			raw:   []byte(`{"data":{"thing":{"name":"n","payee_id":null,"note":null,"details":{"transfer_id":"x"}}}}`),
			name:  "inline",
		}}
		problems := nullCoverageProblems(entries)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "transfer_id is never null")
	})

	t.Run("out of range numerics decode", func(t *testing.T) {
		t.Parallel()
		runOutOfRangeCase(t, syntheticThing{}, "selftest/thing_extreme.json")
	})
}

func init() {
	// Synthetic G5 registration, removed when the first slice lands.
	registerNullFixture(syntheticThing{}, "selftest/thing_null.json")
}
