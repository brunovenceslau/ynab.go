// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Gate G3: struct lint over wire models. Scope is registry-driven — the
// types transitively reachable from registered decode targets (G4
// endpoint registry), registered write-model instances (G2), and the
// standing wire value types. Config/error/filter structs fall outside the
// walk by construction, never by a name-based exclusion list.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

var (
	wireModelsMu sync.Mutex
	// readModels are registered decode targets (zero values are fine).
	readModels []any
	// writeModels are registered fully-populated write-model instances:
	// every Optional field set, at least one of them Set(zero-of-T) — the
	// issue#24 net.
	writeModels []any
)

// registerReadModel adds a decode target to the G3 walk. The G4 endpoint
// harness calls it for every registered endpoint case.
func registerReadModel(v any) {
	wireModelsMu.Lock()
	defer wireModelsMu.Unlock()
	readModels = append(readModels, v)
}

// registerWriteModel adds a populated write-model instance to the G3 walk
// and its omitzero round-trip.
func registerWriteModel(v any) {
	wireModelsMu.Lock()
	defer wireModelsMu.Unlock()
	writeModels = append(writeModels, v)
}

func init() {
	// Standing wire value types in scope from day one.
	registerReadModel(ynab.CurrencyFormat{})

	// Synthetic write model keeping the round-trip machinery exercised
	// until the first real write slice lands (Task 16+), then removed.
	registerWriteModel(goodWireModel{Name: "n", Note: ynab.Set("m"), Approved: ynab.Set(false)})
}

// TestContractStructs is gate G3.
func TestContractStructs(t *testing.T) {
	t.Parallel()

	wireModelsMu.Lock()
	reads := append([]any{}, readModels...)
	writes := append([]any{}, writeModels...)
	wireModelsMu.Unlock()

	var roots []reflect.Type
	for _, v := range append(append([]any{}, reads...), writes...) {
		roots = append(roots, reflect.TypeOf(v))
	}
	require.Empty(t, structLintProblems(roots))

	for _, w := range writes {
		require.Empty(t, writeModelProblems(w), "%T", w)
	}
}

// structLintProblems walks every module struct type reachable from roots
// and enforces the tag rules: every exported serialized field carries an
// explicit JSON tag; omitempty is forbidden on Optional fields (it never
// omits a struct); Optional fields must carry omitzero.
func structLintProblems(roots []reflect.Type) []string {
	var problems []string
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
		// Module types and anonymous structs (PkgPath "") are in scope;
		// stdlib and third-party types are not.
		if seen[t] || (t.PkgPath() != "" && !strings.HasPrefix(t.PkgPath(), "pkg.venceslau.dev/ynab")) {
			return
		}
		seen[t] = true
		if isOptionalType(t) {
			// Optional's own internals are not a wire struct, but the
			// wrapped type is: walk what the value field carries.
			walk(t.Field(0).Type)
			return
		}

		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				// encoding/json still promotes and serializes the exported
				// fields of an unexported embedded struct — those must not
				// escape the lint.
				if f.Anonymous {
					walk(f.Type)
				}
				continue
			}
			problems = append(problems, fieldProblems(t, f)...)
			walk(f.Type)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return problems
}

// fieldProblems applies the per-field tag rules.
func fieldProblems(owner reflect.Type, f reflect.StructField) []string {
	var problems []string
	loc := owner.String() + "." + f.Name

	tag, ok := f.Tag.Lookup("json")
	if f.Anonymous && !ok {
		return nil // embedded Base structs flatten; their own fields are walked
	}
	name, opts, _ := strings.Cut(tag, ",")
	switch {
	case !ok || name == "":
		problems = append(problems, loc+": serialized field needs an explicit JSON tag name")
	case name == "-":
		return nil
	}

	if isOptionalType(f.Type) {
		if tagHasOption(opts, "omitempty") {
			problems = append(problems, loc+": omitempty on Optional never omits — use omitzero")
		}
		if !tagHasOption(opts, "omitzero") {
			problems = append(problems, loc+": Optional field must carry omitzero")
		}
	}
	return problems
}

func tagHasOption(opts, want string) bool {
	for opt := range strings.SplitSeq(opts, ",") {
		if opt == want {
			return true
		}
	}
	return false
}

func isOptionalType(t reflect.Type) bool {
	return t.PkgPath() == "pkg.venceslau.dev/ynab" && strings.HasPrefix(t.Name(), "Optional[")
}

// writeModelProblems marshals a registered write-model instance and
// checks the omitzero contract through the public Optional API: every
// non-unset Optional field must be present in the emitted JSON, and at
// least one field must be Set(zero-of-T) — emitted, not omitted.
func writeModelProblems(v any) []string {
	raw, err := json.Marshal(v)
	if err != nil {
		return []string{fmt.Sprintf("%T: marshal: %v", v, err)}
	}
	var emitted map[string]any
	if err := json.Unmarshal(raw, &emitted); err != nil {
		return []string{fmt.Sprintf("%T: emitted JSON is not an object: %v", v, err)}
	}

	var problems []string
	setZeroFields := 0
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !f.IsExported() || !isOptionalType(f.Type) {
			continue
		}
		problem, setZero := optionalEmitProblem(rt, f, rv.Field(i), emitted)
		if problem != "" {
			problems = append(problems, problem)
		}
		if setZero {
			setZeroFields++
		}
	}
	if setZeroFields == 0 && hasOptionalField(rt) {
		problems = append(problems,
			rt.String()+": register the instance with at least one Set(zero-of-T) field — the issue#24 net")
	}
	return problems
}

// optionalEmitProblem checks one Optional field's emit/omit behavior via
// the public API and reports whether it is a Set(zero-of-T) field.
func optionalEmitProblem(
	rt reflect.Type, f reflect.StructField, field reflect.Value, emitted map[string]any,
) (problem string, setZero bool) {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	_, present := emitted[name]

	if field.MethodByName("IsZero").Call(nil)[0].Bool() {
		if present {
			return rt.String() + "." + f.Name + ": unset Optional was emitted", false
		}
		return "", false
	}
	if !present {
		return rt.String() + "." + f.Name + ": set Optional was omitted (issue#24 class)", false
	}

	got := field.MethodByName("Get").Call(nil)
	return "", got[1].Bool() && got[0].IsZero()
}

// hasOptionalField reports whether a write model carries any Optional
// field at all — all-plain payloads (AccountSpec) have nothing for the
// set-zero net to catch.
func hasOptionalField(t reflect.Type) bool {
	for i := range t.NumField() {
		if isOptionalType(t.Field(i).Type) {
			return true
		}
	}
	return false
}

// Self-checks: planted violations in local types prove every rule bites;
// existing config structs (RetryPolicy, Error) stay out of scope
// structurally.

type badUntagged struct {
	Budgeted int64 // no tag: the "Budgeted"-class accident
}

// badOmitemptyType is built at runtime: a source-level `omitempty` tag on
// an Optional would be rewritten to omitzero by `go fix`, deleting the
// planted violation this self-check exists to detect.
func badOmitemptyType() reflect.Type {
	return reflect.StructOf([]reflect.StructField{{
		Name: "Note",
		Type: reflect.TypeFor[ynab.Optional[string]](),
		Tag:  `json:"note,omitempty"`,
	}})
}

type badMissingOmitzero struct {
	Note ynab.Optional[string] `json:"note"`
}

// badBase is an unexported embedded struct whose exported field json still
// promotes and serializes.
type badBase struct {
	Budgeted int64
}

type badEmbedding struct {
	badBase
	ID string `json:"id"`
}

type badOptionalWrapped struct {
	Sub ynab.Optional[badBase] `json:"sub,omitzero"`
}

type goodWireModel struct {
	Name     string                `json:"name"`
	Note     ynab.Optional[string] `json:"note,omitzero"`
	Approved ynab.Optional[bool]   `json:"approved,omitzero"`
}

func TestContractStructsSelfCheck(t *testing.T) {
	t.Parallel()

	t.Run("untagged serialized field detected", func(t *testing.T) {
		t.Parallel()

		problems := structLintProblems([]reflect.Type{reflect.TypeFor[badUntagged]()})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "explicit JSON tag")
	})

	t.Run("omitempty on Optional detected", func(t *testing.T) {
		t.Parallel()

		problems := structLintProblems([]reflect.Type{badOmitemptyType()})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "omitempty on Optional")
	})

	t.Run("missing omitzero detected", func(t *testing.T) {
		t.Parallel()

		problems := structLintProblems([]reflect.Type{reflect.TypeFor[badMissingOmitzero]()})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "must carry omitzero")
	})

	t.Run("clean wire model passes", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, structLintProblems([]reflect.Type{reflect.TypeFor[goodWireModel]()}))
	})

	t.Run("unexported embedded struct cannot hide untagged fields", func(t *testing.T) {
		t.Parallel()

		// json promotes badBase's exported field: {"Budgeted":...} — the
		// walk must descend into unexported embedded types.
		problems := structLintProblems([]reflect.Type{reflect.TypeFor[badEmbedding]()})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "explicit JSON tag")
	})

	t.Run("struct wrapped in Optional is still walked", func(t *testing.T) {
		t.Parallel()

		problems := structLintProblems([]reflect.Type{reflect.TypeFor[badOptionalWrapped]()})
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "explicit JSON tag")
	})

	t.Run("config structs are structurally out of scope", func(t *testing.T) {
		t.Parallel()

		// RetryPolicy has untagged fields by design; it must not be
		// reachable from any registered wire model. Prove the walk from
		// the current registries never visits it.
		wireModelsMu.Lock()
		roots := make([]reflect.Type, 0, len(readModels)+len(writeModels))
		for _, v := range append(append([]any{}, readModels...), writeModels...) {
			roots = append(roots, reflect.TypeOf(v))
		}
		wireModelsMu.Unlock()

		require.Empty(t, structLintProblems(roots),
			"RetryPolicy/Error/TransactionFilter untagged fields must never surface here")
	})

	t.Run("set zero survives the round trip", func(t *testing.T) {
		t.Parallel()

		good := goodWireModel{Name: "n", Note: ynab.Set("memo"), Approved: ynab.Set(false)}
		require.Empty(t, writeModelProblems(good))
	})

	t.Run("model without a set-zero field is rejected", func(t *testing.T) {
		t.Parallel()

		lazy := goodWireModel{Name: "n", Note: ynab.Set("memo"), Approved: ynab.Set(true)}
		problems := writeModelProblems(lazy)
		require.NotEmpty(t, problems)
		require.Contains(t, problems[0], "at least one Set(zero-of-T)")
	})
}
