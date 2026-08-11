// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package contract_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab/internal/contract"
)

// TestContractScanSchemas pins the resolved view against the real vendored
// spec. The counts are exact: this is the input two gates diff against, and
// a change in what the spec declares must be seen, not averaged away.
func TestContractScanSchemas(t *testing.T) {
	t.Parallel()

	s, err := contract.ScanSchemas(specPath)
	require.NoError(t, err)

	require.Len(t, s.Bounds, 17, "11 declared inline, 6 more reached through allOf")
	require.Len(t, s.Enums, 5, "the spec declares 5 schema-level enums")
	require.Len(t, s.PropEnums, 13,
		"5 enums declared inline on properties, reached by 13 (schema, property) pairs through allOf")

	// allOf resolution is the reason this parses rather than scans lines:
	// NewTransaction declares only import_id and inherits the other two.
	for _, want := range []contract.Bound{
		{Schema: "NewTransaction", Property: "import_id", MaxLength: 36},
		{Schema: "NewTransaction", Property: "payee_name", MaxLength: 200},
		{Schema: "NewTransaction", Property: "memo", MaxLength: 500},
	} {
		got, ok := s.BoundFor(want.Schema, want.Property)
		require.True(t, ok, "%s.%s must resolve through allOf", want.Schema, want.Property)
		require.Equal(t, want.MaxLength, got)
	}

	// An ordinary directly-declared property, for contrast.
	got, ok := s.BoundFor("SaveCategoryGroup", "name")
	require.True(t, ok)
	require.Equal(t, 50, got)

	_, ok = s.BoundFor("SaveCategoryGroup", "no_such_property")
	require.False(t, ok, "a miss reports false, not a zero bound")

	// FlagColor carries both spellings the scanner separates: the empty
	// string is a member, a bare null is not.
	values, ok := s.EnumFor("TransactionFlagColor")
	require.True(t, ok)
	require.ElementsMatch(t,
		[]string{"red", "orange", "yellow", "green", "blue", "purple", ""}, values)

	hasNull, ok := s.NullableEnum("TransactionFlagColor")
	require.True(t, ok)
	require.True(t, hasNull, "a bare null member is recorded apart from the scalars")

	hasNull, ok = s.NullableEnum("TransactionClearedStatus")
	require.True(t, ok)
	require.False(t, hasNull)

	_, ok = s.EnumFor("NoSuchSchema")
	require.False(t, ok)

	_, ok = s.NullableEnum("NoSuchSchema")
	require.False(t, ok, "a nullability miss reports false, not a zero value")

	// Property-level enums: declared inline on CategoryBase, inherited by
	// Category through allOf — both spellings must resolve.
	for _, schema := range []string{"CategoryBase", "Category"} {
		values, ok := s.PropEnumFor(schema, "goal_type")
		require.True(t, ok, "%s.goal_type must resolve", schema)
		require.ElementsMatch(t, []string{"TB", "TBD", "MF", "NEED", "DEBT"}, values)

		hasNull, ok := s.NullablePropEnum(schema, "goal_type")
		require.True(t, ok)
		require.True(t, hasNull, "goal_type admits a bare null")
	}

	// A property that $refs a named enum schema is NOT a property enum:
	// cleared resolves to TransactionClearedStatus's members, but that set
	// is pinned once at schema level.
	_, ok = s.PropEnumFor("NewTransaction", "cleared")
	require.False(t, ok, "a $ref to a named enum is pinned at schema level, not per property")

	_, ok = s.NullablePropEnum("NewTransaction", "cleared")
	require.False(t, ok)
}

// TestContractScanSchemasShapes drives the parser over hand-written
// documents. These are the shapes a hand-rolled line scanner got wrong —
// flow style, a nested properties: block, an inline object, an allOf chain —
// kept as regression cases so a future change back to scanning cannot pass
// quietly. Written as fixtures rather than mutations of the vendored spec so
// a re-vendor cannot change what they prove.
func TestContractScanSchemasShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		doc           string
		wantBounds    []contract.Bound
		wantEnums     int
		wantPropEnums []contract.PropEnum
	}{{
		name: "block style",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    SavePayee:
      type: object
      properties:
        name:
          type: string
          maxLength: 500
`,
		wantBounds: []contract.Bound{{Schema: "SavePayee", Property: "name", MaxLength: 500}},
	}, {
		name: "flow style — invisible to a line scanner",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    SavePayee:
      type: object
      properties: {name: {type: string, maxLength: 500}}
`,
		wantBounds: []contract.Bound{{Schema: "SavePayee", Property: "name", MaxLength: 500}},
	}, {
		name: "a sibling after a nested object is not dropped",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        nested:
          type: object
          properties:
            inner:
              type: string
              maxLength: 5
        name:
          type: string
          maxLength: 10
`,
		// inner belongs to Thing.nested, not to Thing: only the top-level
		// property is a bound on Thing.
		wantBounds: []contract.Bound{{Schema: "Thing", Property: "name", MaxLength: 10}},
	}, {
		name: "an array item's property is not attributed to the outer schema",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        legs:
          type: array
          items:
            type: object
            properties:
              memo:
                type: string
                maxLength: 500
`,
	}, {
		name: "allOf contributes the branch's bounds to the composing schema",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Base:
      type: object
      properties:
        memo:
          type: string
          maxLength: 500
    Derived:
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            import_id:
              type: string
              maxLength: 36
`,
		wantBounds: []contract.Bound{
			{Schema: "Base", Property: "memo", MaxLength: 500},
			{Schema: "Derived", Property: "import_id", MaxLength: 36},
			{Schema: "Derived", Property: "memo", MaxLength: 500},
		},
	}, {
		name: "a property-level enum is not a schema-level one",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Colors:
      type: string
      enum: [red, ""]
    Thing:
      type: object
      properties:
        kind:
          type: string
          enum: [inline, null]
`,
		wantEnums: 1,
		wantPropEnums: []contract.PropEnum{
			{Schema: "Thing", Property: "kind", Values: []string{"inline"}, HasNull: true},
		},
	}, {
		name: "a property that $refs a named enum is not re-recorded per property",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Colors:
      type: string
      enum: [red, ""]
    Thing:
      type: object
      properties:
        color:
          $ref: "#/components/schemas/Colors"
`,
		wantEnums: 1,
	}, {
		name: "allOf contributes the branch's inline enums to the composing schema",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Base:
      type: object
      properties:
        kind:
          type: string
          enum: [a, b]
    Derived:
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            extra:
              type: string
              enum: [x]
`,
		wantPropEnums: []contract.PropEnum{
			{Schema: "Base", Property: "kind", Values: []string{"a", "b"}},
			{Schema: "Derived", Property: "extra", Values: []string{"x"}},
			{Schema: "Derived", Property: "kind", Values: []string{"a", "b"}},
		},
	}, {
		name: "a diamond allOf records the shared base once per reaching schema",
		// Derived composes Base through two middle schemas. The seen set
		// must dedupe the second arrival, so Derived carries ONE kind
		// entry — and the two-hop chain itself is proven here, in a
		// fixture a re-vendor cannot change, not only through the real
		// spec's TransactionDetail.
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Base:
      type: object
      properties:
        kind:
          type: string
          enum: [a, b]
    Mid1:
      allOf:
        - $ref: "#/components/schemas/Base"
    Mid2:
      allOf:
        - $ref: "#/components/schemas/Base"
    Derived:
      allOf:
        - $ref: "#/components/schemas/Mid1"
        - $ref: "#/components/schemas/Mid2"
`,
		wantPropEnums: []contract.PropEnum{
			{Schema: "Base", Property: "kind", Values: []string{"a", "b"}},
			{Schema: "Derived", Property: "kind", Values: []string{"a", "b"}},
			{Schema: "Mid1", Property: "kind", Values: []string{"a", "b"}},
			{Schema: "Mid2", Property: "kind", Values: []string{"a", "b"}},
		},
	}, {
		name: "an items enum is out of scope, like an items bound",
		// The walk reads one property level, for enums exactly as for
		// bounds. This pins the cut as a decision: if upstream ever
		// declares an enum on an array's items, extending the walk is
		// the fix — this fixture failing is how that conversation starts.
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        legs:
          type: array
          items:
            type: string
            enum: [x, y]
`,
	}, {
		name: "a nested inline object's enum is out of scope, one level deep only",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        nested:
          type: object
          properties:
            inner:
              type: string
              enum: [x, y]
`,
	}, {
		name: "an enum or bound under a property's own oneOf is out of scope",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        kind:
          oneOf:
            - type: string
              enum: [x, y]
              maxLength: 5
            - type: string
`,
	}, {
		name: "an enum of only null has no scalar members",
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        kind:
          type:
            - string
            - "null"
          enum: [null]
`,
		wantPropEnums: []contract.PropEnum{
			{Schema: "Thing", Property: "kind", HasNull: true},
		},
	}, {
		name: "the same property declared by two branches records twice",
		// allOf means BOTH constraint sets apply; recording both is the
		// honest reading, and the completeness Len in the wire test is
		// what turns the duplicate into a loud failure there.
		doc: `openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      allOf:
        - type: object
          properties:
            kind:
              type: string
              enum: [a]
        - type: object
          properties:
            kind:
              type: string
              enum: [b]
`,
		wantPropEnums: []contract.PropEnum{
			{Schema: "Thing", Property: "kind", Values: []string{"a"}},
			{Schema: "Thing", Property: "kind", Values: []string{"b"}},
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "spec.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.doc), 0o600))

			s, err := contract.ScanSchemas(path)
			require.NoError(t, err)
			require.Equal(t, tc.wantBounds, s.Bounds)
			require.Len(t, s.Enums, tc.wantEnums)
			require.Equal(t, tc.wantPropEnums, s.PropEnums)
		})
	}
}

// TestContractScanSchemasErrors covers the failure paths.
func TestContractScanSchemasErrors(t *testing.T) {
	t.Parallel()

	_, err := contract.ScanSchemas(filepath.Join(t.TempDir(), "absent.yaml"))
	require.Error(t, err, "a missing spec must fail loudly")

	path := filepath.Join(t.TempDir(), "broken.yaml")
	require.NoError(t, os.WriteFile(path, []byte("this: is: not: a spec\n"), 0o600))
	_, err = contract.ScanSchemas(path)
	require.Error(t, err, "an unparseable document must fail loudly")

	path = filepath.Join(t.TempDir(), "empty.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
`), 0o600))
	_, err = contract.ScanSchemas(path)
	require.ErrorContains(t, err, "no component schemas",
		"a spec with no schemas must fail closed, not report an empty set")

	// A non-string member is refused, not rendered: fmt.Sprint would
	// collapse 1, 1.0 and "1" into one pinned spelling, and two specs
	// that differ on the wire would pin identically.
	path = filepath.Join(t.TempDir(), "numeric-prop.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        kind:
          type: integer
          enum: [1, 2]
`), 0o600))
	_, err = contract.ScanSchemas(path)
	require.ErrorContains(t, err, "non-string enum member")
	require.ErrorContains(t, err, "Thing.kind", "the failure must name the pair")

	path = filepath.Join(t.TempDir(), "numeric-schema.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Level:
      type: integer
      enum: [1, 2]
`), 0o600))
	_, err = contract.ScanSchemas(path)
	require.ErrorContains(t, err, "non-string enum member")
	require.ErrorContains(t, err, "Level", "the failure must name the schema")

	// The same refusal must survive the allOf recursion: a bad member
	// reached only through a composed branch is attributed to the
	// composing schema, not silently dropped from the pinned set — a
	// mutation swallowing the recursive error passed the suite before
	// this fixture existed.
	path = filepath.Join(t.TempDir(), "numeric-allof.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Derived:
      allOf:
        - type: object
          properties:
            level:
              type: integer
              enum: [1, 2]
`), 0o600))
	_, err = contract.ScanSchemas(path)
	require.ErrorContains(t, err, "non-string enum member")
	require.ErrorContains(t, err, "Derived.level",
		"the failure must name the composing schema's pair")
}
