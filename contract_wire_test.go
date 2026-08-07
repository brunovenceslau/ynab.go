// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// The schema half of the spec contract. G1 diffs operation tuples and the
// content pin proves the spec file is the one that was reviewed, but
// neither notices that a wire constant transcribed into Go has stopped
// matching the schema it came from. SPEC's standing rule is that wire
// constraints come from openapi.yaml and never from memory; these tests
// are what make that enforceable rather than aspirational.
//
// Both directions are asserted. A value that drifts fails on comparison; a
// bound or enum that upstream ADDS fails on completeness, because an
// unmapped one is the case a value check can never see.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
	"pkg.venceslau.dev/ynab/internal/contract"
)

// wireBounds maps every maxLength the spec declares — including the ones a
// schema inherits through allOf, which is the set the wire enforces — to the
// Go constant that enforces it. Several schemas share a constant: the
// transaction, subtransaction and scheduled-transaction payloads bound
// payee_name and memo identically, and ScheduledTransactionSpec.validate
// reuses the transaction constants rather than declaring its own.
var wireBounds = map[[2]string]int{
	{"NewTransaction", "import_id"}:                     ynab.ImportIDMax,
	{"NewTransaction", "payee_name"}:                    ynab.TransactionPayeeNameMax,
	{"NewTransaction", "memo"}:                          ynab.MemoMax,
	{"ExistingTransaction", "payee_name"}:               ynab.TransactionPayeeNameMax,
	{"ExistingTransaction", "memo"}:                     ynab.MemoMax,
	{"SaveTransactionWithOptionalFields", "payee_name"}: ynab.TransactionPayeeNameMax,
	{"SaveTransactionWithOptionalFields", "memo"}:       ynab.MemoMax,
	{"SaveTransactionWithIdOrImportId", "payee_name"}:   ynab.TransactionPayeeNameMax,
	{"SaveTransactionWithIdOrImportId", "memo"}:         ynab.MemoMax,
	{"SaveSubTransaction", "payee_name"}:                ynab.TransactionPayeeNameMax,
	{"SaveSubTransaction", "memo"}:                      ynab.MemoMax,
	{"SaveScheduledTransaction", "payee_name"}:          ynab.TransactionPayeeNameMax,
	{"SaveScheduledTransaction", "memo"}:                ynab.MemoMax,
	{"PostPayee", "name"}:                               ynab.PayeeNameMax,
	{"SavePayee", "name"}:                               ynab.PayeeNameMax,
	{"SaveCategoryGroup", "name"}:                       ynab.CategoryGroupNameMax,
}

// wireBoundsUnenforced records the bounds this library does NOT pre-flight,
// with the reason. A bound belongs here rather than in wireBounds only when
// leaving it unchecked is a decision someone made; the completeness
// assertion accepts either table, so the spec can never declare a bound
// that goes entirely unremarked.
var wireBoundsUnenforced = map[[2]string]string{
	{"SaveTransactionWithIdOrImportId", "import_id"}: "PatchByImportID's key reaches the wire " +
		"unchecked: TransactionPatch embeds TransactionUpdate, which has no ImportID field, so " +
		"UpdateBatch validates everything except this. A >36-character key comes back a server " +
		"400 instead of an *ArgumentError. Tracked as a follow-up; enforcing it is a behavior " +
		"change and belongs in its own commit.",
}

// wireEnums maps every enum the spec declares on a NAMED schema to the Go
// enum type that mirrors it. The Go members come from the same AST scan
// TestEnumValidTables uses, so neither side is hand-repeated here.
//
// Scope, stated because the set is not everything transcribed: the spec also
// declares four enums inline on properties inside components: — GoalType,
// GoalFrequency and HybridType, which would pin cleanly today, and
// DebtTransactionType, whose Go set has eight members against the spec's
// four, sourced deliberately from docs/v1/research. Pinning the first three
// and waiving the fourth is a follow-up, not a claim this file already makes.
var wireEnums = map[string]string{
	"AccountType":                   "AccountType",
	"SaveAccountType":               "AccountSpecType",
	"TransactionClearedStatus":      "ClearedStatus",
	"TransactionFlagColor":          "FlagColor",
	"ScheduledTransactionFrequency": "Frequency",
}

// wireNullableEnums pins which of those enums admit a bare null. The Go side
// models that through the type — *FlagColor on reads, Optional[FlagColor] on
// writes — so a change here is a change to what those types must be.
var wireNullableEnums = map[string]bool{
	"AccountType":                   false,
	"SaveAccountType":               false,
	"TransactionClearedStatus":      false,
	"TransactionFlagColor":          true,
	"ScheduledTransactionFrequency": false,
}

// TestContractWireBounds diffs the transcribed maxLength constants against
// the spec, both ways.
func TestContractWireBounds(t *testing.T) {
	t.Parallel()

	schemas, err := contract.ScanSchemas("openapi.yaml")
	require.NoError(t, err)
	require.Len(t, schemas.Bounds, len(wireBounds)+len(wireBoundsUnenforced),
		"every bound the spec declares must be mapped or waived, and nothing else")

	for key, want := range wireBounds {
		got, ok := schemas.BoundFor(key[0], key[1])
		require.True(t, ok,
			"spec no longer declares maxLength on %s.%s; the Go bound is now unanchored",
			key[0], key[1])
		require.Equal(t, want, got,
			"%s.%s: the spec says %d, the Go constant says %d — re-vendoring moved the bound "+
				"and the constant was not updated with it", key[0], key[1], got, want)
	}

	for _, b := range schemas.Bounds {
		key := [2]string{b.Schema, b.Property}
		if _, waived := wireBoundsUnenforced[key]; waived {
			continue
		}
		require.Contains(t, wireBounds, key,
			"spec declares maxLength %d on %s.%s and nothing in this library enforces it; "+
				"map it to a constant, or record why not in wireBoundsUnenforced",
			b.MaxLength, b.Schema, b.Property)
	}

	for key := range wireBoundsUnenforced {
		_, ok := schemas.BoundFor(key[0], key[1])
		require.True(t, ok,
			"%s.%s is waived as unenforced but the spec no longer declares it — drop the waiver",
			key[0], key[1])
	}
}

// TestContractWireEnums diffs the Go enum member sets against the spec's,
// both ways, and pins which admit a null.
func TestContractWireEnums(t *testing.T) {
	t.Parallel()

	schemas, err := contract.ScanSchemas("openapi.yaml")
	require.NoError(t, err)
	require.Len(t, schemas.Enums, len(wireEnums),
		"every schema-level enum must be mapped, and nothing else")

	goEnums := scanEnumConsts(t)

	for specName, goName := range wireEnums {
		specValues, ok := schemas.EnumFor(specName)
		require.True(t, ok, "spec no longer declares an enum on %s", specName)

		goValues, ok := goEnums[goName]
		require.True(t, ok, "no Go enum named %s; the mapping is stale", goName)

		// ElementsMatch, not Equal: the spec's declaration order is
		// upstream's business, and reordering is not drift.
		require.ElementsMatch(t, specValues, goValues,
			"%s does not match spec enum %s — upstream added or removed a wire value",
			goName, specName)

		hasNull, ok := schemas.NullableEnum(specName)
		require.True(t, ok)
		require.Equal(t, wireNullableEnums[specName], hasNull,
			"%s changed nullability upstream — the Go type models that as a pointer or "+
				"Optional, so this is a type change, not a value change", specName)
	}

	for _, e := range schemas.Enums {
		require.Contains(t, wireEnums, e.Schema,
			"spec declares an enum on %s that no Go type mirrors: %q", e.Schema, e.Values)
	}
}
