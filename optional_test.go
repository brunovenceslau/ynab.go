package ynab_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// writeModel mirrors the shape every write model uses: Optional fields tagged
// omitzero, so the unset state disappears from the wire entirely.
type writeModel struct {
	Name     ynab.Optional[string] `json:"name,omitzero"`
	Approved ynab.Optional[bool]   `json:"approved,omitzero"`
	Count    ynab.Optional[int64]  `json:"count,omitzero"`
}

func TestOptionalTriState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   writeModel
		want string
	}{
		{
			name: "unset is omitted",
			in:   writeModel{},
			want: `{}`,
		},
		{
			name: "null is emitted as null",
			in:   writeModel{Name: ynab.SetNull[string]()},
			want: `{"name":null}`,
		},
		{
			name: "value is emitted as value",
			in:   writeModel{Name: ynab.Set("v")},
			want: `{"name":"v"}`,
		},
		{
			name: "set zero values are emitted, not omitted",
			in: writeModel{
				Name:     ynab.Set(""),
				Approved: ynab.Set(false),
				Count:    ynab.Set[int64](0),
			},
			want: `{"name":"","approved":false,"count":0}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tt.in)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestOptionalStates(t *testing.T) {
	t.Parallel()

	t.Run("unset", func(t *testing.T) {
		t.Parallel()

		var o ynab.Optional[string]
		v, ok := o.Get()
		require.False(t, ok)
		require.Empty(t, v)
		require.False(t, o.IsNull())
		require.True(t, o.IsZero())
	})

	t.Run("null", func(t *testing.T) {
		t.Parallel()

		o := ynab.SetNull[string]()
		v, ok := o.Get()
		require.False(t, ok)
		require.Empty(t, v)
		require.True(t, o.IsNull())
		require.False(t, o.IsZero(), "null is a deliberate state — omitzero must emit it")
	})

	t.Run("value", func(t *testing.T) {
		t.Parallel()

		o := ynab.Set("v")
		v, ok := o.Get()
		require.True(t, ok)
		require.Equal(t, "v", v)
		require.False(t, o.IsNull())
		require.False(t, o.IsZero())
	})

	t.Run("set zero value is not IsZero", func(t *testing.T) {
		t.Parallel()

		// The issue#24 failure mode: Set(false) silently dropped by omitzero.
		// IsZero is state-based, never value-based.
		require.False(t, ynab.Set(false).IsZero())
		require.False(t, ynab.Set(0).IsZero())
		require.False(t, ynab.Set("").IsZero())
	})
}

// TestOptionalMarshalJSON pins direct MarshalJSON output to the inner type's
// own encoding/json encoding. The table grows Optional[Date] and
// Optional[Milliunits] rows when those types land.
func TestOptionalMarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("value matches inner encoding", func(t *testing.T) {
		t.Parallel()

		inner := struct {
			A int
			B string
		}{A: 7, B: "x"}
		want, err := json.Marshal(inner)
		require.NoError(t, err)

		got, err := json.Marshal(ynab.Set(inner))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got))
	})

	t.Run("null encodes as null", func(t *testing.T) {
		t.Parallel()

		got, err := json.Marshal(ynab.SetNull[int]())
		require.NoError(t, err)
		require.Equal(t, "null", string(got))
	})

	t.Run("unset encodes as null when marshaled directly", func(t *testing.T) {
		t.Parallel()

		// Outside an omitzero struct field there is no way to omit; null is
		// the only faithful JSON spelling of "no value".
		got, err := json.Marshal(ynab.Optional[int]{})
		require.NoError(t, err)
		require.Equal(t, "null", string(got))
	})
}
