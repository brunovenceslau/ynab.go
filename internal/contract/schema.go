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

// PropEnum is one enum the spec declares inline on a schema property,
// including the ones a schema inherits through allOf. It splits a bare
// null out of the scalar members exactly as Enum does, and for the same
// reason.
type PropEnum struct {
	Schema   string
	Property string
	Values   []string
	HasNull  bool
}

// Schemas is the resolved view of the components: section.
type Schemas struct {
	Bounds    []Bound
	Enums     []Enum
	PropEnums []PropEnum
}

// ScanSchemas reads the maxLength bounds and enum sets — schema-level and
// property-level — out of the vendored spec's components: section, the half
// ScanSpec is deliberately blind to.
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
// Bounds and property enums include those reached through allOf, which is
// the set the wire actually enforces: NewTransaction declares only import_id
// itself and inherits payee_name and memo from
// SaveTransactionWithOptionalFields, and TransactionDetail inherits
// debt_transaction_type from TransactionSummaryBase two hops up.
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
		if err := collectProps(name, ref.Value, &out, map[*openapi3.Schema]bool{}); err != nil {
			return nil, err
		}
		e, ok, err := enumOf(name, ref.Value)
		if err != nil {
			return nil, err
		}
		if ok {
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
	// Stable, because (Schema, Property) can tie: two allOf branches may
	// declare the same property, and both records are kept. Stability
	// preserves the deterministic branch order, so equal keys cannot
	// reshuffle between runs.
	sort.SliceStable(out.PropEnums, func(i, j int) bool {
		if out.PropEnums[i].Schema != out.PropEnums[j].Schema {
			return out.PropEnums[i].Schema < out.PropEnums[j].Schema
		}
		return out.PropEnums[i].Property < out.PropEnums[j].Property
	})
	return &out, nil
}

// collectProps records every maxLength and every property-level enum on s
// and on the schemas it composes through allOf, attributing each to the
// named schema that reaches it. The seen set guards against a cyclic allOf
// and keeps a diamond (two branches composing the same base) from recording
// the base's properties twice.
//
// The walk reads one property level deep, matching the bounds' scope: an
// enum (or maxLength) on an array's items, inside a nested inline object,
// or under a property's own composition keywords is deliberately out of
// scope, and a fixture pins each of the three shapes. The vendored spec
// declares nothing there today; if upstream starts to, extending the walk
// is the fix — silently widening the claim is not.
func collectProps(name string, s *openapi3.Schema, out *Schemas, seen map[*openapi3.Schema]bool) error {
	if s == nil || seen[s] {
		return nil
	}
	seen[s] = true

	for prop, ref := range s.Properties {
		if ref.Value == nil {
			continue
		}
		if ref.Value.MaxLength != nil {
			// The spec's bounds are small (36..500). Anything past MaxInt is
			// not a length this library could enforce anyway, and narrowing
			// it silently would turn an absurd bound into a plausible one.
			if n := *ref.Value.MaxLength; n <= math.MaxInt {
				out.Bounds = append(out.Bounds, Bound{name, prop, int(n)})
			}
		}
		// Only enums declared inline on the property. A property that
		// $refs a named enum schema (cleared, flag_color, …) resolves to
		// that schema's members here, and those MEMBER SETS are pinned
		// once at schema level — re-recording them per property would
		// assert the same fact dozens of times and drown the inline ones,
		// which are the sets upstream chose not to name and Go names
		// anyway. What the cut does NOT preserve is anything else the
		// $ref property carries: a $ref retargeted from one named enum to
		// another is invisible because THIS guard skips every $ref-bearing
		// property, and so is a 3.1-style sibling keyword beside the $ref
		// (this spec declares OpenAPI 3.1, where a sibling enum is
		// meaningful; the loader overlays it onto ref.Value — measured —
		// so closing that gap is local to this walk, not a loader
		// problem). Both shapes are tracked as one follow-up rather than
		// implied covered.
		if ref.Ref == "" && len(ref.Value.Enum) > 0 {
			values, hasNull, err := splitEnum(ref.Value.Enum)
			if err != nil {
				return fmt.Errorf("contract: %s.%s: %w", name, prop, err)
			}
			out.PropEnums = append(out.PropEnums, PropEnum{name, prop, values, hasNull})
		}
	}
	for _, branch := range s.AllOf {
		if err := collectProps(name, branch.Value, out, seen); err != nil {
			return err
		}
	}
	return nil
}

// enumOf returns the enum declared directly on a named schema, splitting a
// bare null out of the scalar members. Enums on properties are PropEnums,
// collected by collectProps.
func enumOf(name string, s *openapi3.Schema) (Enum, bool, error) {
	if len(s.Enum) == 0 {
		return Enum{}, false, nil
	}
	values, hasNull, err := splitEnum(s.Enum)
	if err != nil {
		return Enum{}, false, fmt.Errorf("contract: %s: %w", name, err)
	}
	return Enum{Schema: name, Values: values, HasNull: hasNull}, true, nil
}

// splitEnum separates an enum's scalar members from a bare null member.
// The Go enums model nullability through the type, not through a sentinel
// value, so the two are compared apart.
//
// Anything else is refused rather than rendered: every Go mirror is a
// string enum, and fmt.Sprint would collapse a bare true and a quoted
// "true" — or 1, 1.0 and "1" — into one pinned spelling, letting two specs
// that differ on the wire produce identical pins. A gate that coerces is a
// gate that can lie.
func splitEnum(members []any) (values []string, hasNull bool, err error) {
	for _, v := range members {
		if v == nil {
			hasNull = true
			continue
		}
		s, ok := v.(string)
		if !ok {
			return nil, false, fmt.Errorf("non-string enum member %v (%T)", v, v)
		}
		values = append(values, s)
	}
	return values, hasNull, nil
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

// PropEnumFor returns the scalar enum members the spec declares on
// schema.property, directly or through allOf. When two allOf branches
// declare the same pair, the first record in branch order wins here — the
// wire test's completeness count is what makes such a duplicate loud.
func (s *Schemas) PropEnumFor(schema, property string) ([]string, bool) {
	for _, e := range s.PropEnums {
		if e.Schema == schema && e.Property == property {
			return e.Values, true
		}
	}
	return nil, false
}

// NullablePropEnum reports whether the spec's enum on schema.property
// admits a bare null. On a duplicate pair the first record in branch
// order wins, exactly as in PropEnumFor.
func (s *Schemas) NullablePropEnum(schema, property string) (bool, bool) {
	for _, e := range s.PropEnums {
		if e.Schema == schema && e.Property == property {
			return e.HasNull, true
		}
	}
	return false, false
}
