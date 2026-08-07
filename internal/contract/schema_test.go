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
		name       string
		doc        string
		wantBounds []contract.Bound
		wantEnums  int
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
          enum: [inline]
`,
		wantEnums: 1,
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
}
