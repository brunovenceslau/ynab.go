// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package contract

import (
	"fmt"
	"math"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// Bound is one maxLength the spec declares on a schema property, including
// the ones a schema inherits through allOf.
type Bound struct {
	Schema    string
	Property  string
	MaxLength int
}

// Enum is one enum set the spec declares directly on a named schema.
// HasNull records a bare null member separately: the Go enums model
// nullability through the type, not through a sentinel value, so the two
// are compared on their scalar members alone.
type Enum struct {
	Schema  string
	Values  []string
	HasNull bool
}

// Schemas is the resolved view of the components: section.
type Schemas struct {
	Bounds []Bound
	Enums  []Enum
}

// ScanSchemas reads the maxLength bounds and schema-level enum sets out of
// the vendored spec's components: section — the half ScanSpec is
// deliberately blind to.
//
// Unlike ScanSpec this parses rather than scans lines. ScanSpec can afford a
// line scanner because the paths: layout it reads is strictly regular and it
// fails closed on an empty result. Neither holds here: components: mixes
// six-space properties with allOf branches four columns deeper, and a spec
// with no bounds is indistinguishable from a scanner that stopped matching.
// A hand-rolled version of this was written first and measured wrong four
// ways — a property in flow style vanished silently, a nested properties:
// dropped its enclosing block's later siblings, an inline object attached
// its inner property to the outer schema, and a comment inside an enum list
// truncated the members. kin-openapi is already a direct test dependency, so
// the parser costs no new module edge and removes all four classes rather
// than narrowing them.
//
// Bounds include those reached through allOf, which is the set the wire
// actually enforces: NewTransaction declares only import_id itself and
// inherits payee_name and memo from SaveTransactionWithOptionalFields.
func ScanSchemas(path string) (*Schemas, error) {
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("contract: load spec: %w", err)
	}
	if doc.Components == nil || len(doc.Components.Schemas) == 0 {
		return nil, fmt.Errorf("contract: %s declares no component schemas", path)
	}

	var out Schemas
	for name, ref := range doc.Components.Schemas {
		if ref.Value == nil {
			continue
		}
		collectBounds(name, ref.Value, &out, map[*openapi3.Schema]bool{})
		if e, ok := enumOf(name, ref.Value); ok {
			out.Enums = append(out.Enums, e)
		}
	}

	// Map iteration is unordered; callers compare sets, but a stable order
	// keeps failure output diffable.
	sort.Slice(out.Bounds, func(i, j int) bool {
		if out.Bounds[i].Schema != out.Bounds[j].Schema {
			return out.Bounds[i].Schema < out.Bounds[j].Schema
		}
		return out.Bounds[i].Property < out.Bounds[j].Property
	})
	sort.Slice(out.Enums, func(i, j int) bool { return out.Enums[i].Schema < out.Enums[j].Schema })
	return &out, nil
}

// collectBounds records every maxLength on s and on the schemas it composes
// through allOf, attributing each to the named schema that reaches it. The
// seen set guards against a cyclic allOf.
func collectBounds(name string, s *openapi3.Schema, out *Schemas, seen map[*openapi3.Schema]bool) {
	if s == nil || seen[s] {
		return
	}
	seen[s] = true

	for prop, ref := range s.Properties {
		if ref.Value == nil || ref.Value.MaxLength == nil {
			continue
		}
		// The spec's bounds are small (36..500). Anything past MaxInt is
		// not a length this library could enforce anyway, and narrowing it
		// silently would turn an absurd bound into a plausible one.
		if n := *ref.Value.MaxLength; n <= math.MaxInt {
			out.Bounds = append(out.Bounds, Bound{name, prop, int(n)})
		}
	}
	for _, branch := range s.AllOf {
		collectBounds(name, branch.Value, out, seen)
	}
}

// enumOf returns the enum declared directly on a named schema, splitting a
// bare null out of the scalar members. Property-level enums are not
// collected: the Go types this pins are named wire enums, and the four
// inline ones under components: are a separate, deliberate scope decision
// recorded by the caller.
func enumOf(name string, s *openapi3.Schema) (Enum, bool) {
	if len(s.Enum) == 0 {
		return Enum{}, false
	}
	e := Enum{Schema: name}
	for _, v := range s.Enum {
		if v == nil {
			e.HasNull = true
			continue
		}
		e.Values = append(e.Values, fmt.Sprint(v))
	}
	return e, true
}

// BoundFor returns the maxLength the spec declares for schema.property.
func (s *Schemas) BoundFor(schema, property string) (int, bool) {
	for _, b := range s.Bounds {
		if b.Schema == schema && b.Property == property {
			return b.MaxLength, true
		}
	}
	return 0, false
}

// EnumFor returns the scalar enum members the spec declares on schema.
func (s *Schemas) EnumFor(schema string) ([]string, bool) {
	for _, e := range s.Enums {
		if e.Schema == schema {
			return e.Values, true
		}
	}
	return nil, false
}

// NullableEnum reports whether the spec's enum on schema admits a bare null.
func (s *Schemas) NullableEnum(schema string) (bool, bool) {
	for _, e := range s.Enums {
		if e.Schema == schema {
			return e.HasNull, true
		}
	}
	return false, false
}
