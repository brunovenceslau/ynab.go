// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// usd mirrors the wire CurrencyFormat of a $-first two-decimal currency.
func usd() ynab.CurrencyFormat {
	return ynab.CurrencyFormat{
		ISOCode:          "USD",
		ExampleFormat:    "123,456.78",
		DecimalDigits:    2,
		DecimalSeparator: ".",
		SymbolFirst:      true,
		GroupSeparator:   ",",
		CurrencySymbol:   "$",
		DisplaySymbol:    true,
	}
}

func TestMilliunitsString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    ynab.Milliunits
		want string
	}{
		{name: "negative sub-unit", m: -220, want: "-0.220"},
		{name: "positive", m: 123930, want: "123.930"},
		{name: "zero", m: 0, want: "0.000"},
		{name: "one milliunit", m: 1, want: "0.001"},
		{name: "negative one milliunit", m: -1, want: "-0.001"},
		{name: "min int64", m: math.MinInt64, want: "-9223372036854775.808"},
		{name: "max int64", m: math.MaxInt64, want: "9223372036854775.807"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.m.String())
		})
	}
}

func TestMilliunitsParse(t *testing.T) {
	t.Parallel()

	t.Run("round trips", func(t *testing.T) {
		t.Parallel()

		for _, s := range []string{"-0.220", "123.930", "0.000", "-9223372036854775.808", "9223372036854775.807"} {
			m, err := ynab.ParseMilliunits(s)
			require.NoError(t, err, s)
			require.Equal(t, s, m.String(), s)
		}
	})

	t.Run("partial fractions and integers", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			in   string
			want ynab.Milliunits
		}{
			{in: "1", want: 1000},
			{in: "-1", want: -1000},
			{in: "1.5", want: 1500},
			{in: "1.05", want: 1050},
			{in: "+2.25", want: 2250},
			{in: "0.001", want: 1},
		}
		for _, tt := range tests {
			m, err := ynab.ParseMilliunits(tt.in)
			require.NoError(t, err, tt.in)
			require.Equal(t, tt.want, m, tt.in)
		}
	})

	t.Run("rejects malformed input", func(t *testing.T) {
		t.Parallel()

		malformed := []string{
			"", ".", "1.", ".5", "abc", "1.2345", "1,5", "1.5e3", "--1",
			"9223372036854775.808", "-9223372036854775.809",
		}
		for _, s := range malformed {
			_, err := ynab.ParseMilliunits(s)
			require.Error(t, err, s)
		}
	})

	t.Run("Must panics on parse failure", func(t *testing.T) {
		t.Parallel()

		require.Panics(t, func() { ynab.MustParseMilliunits("abc") })
		require.Equal(t, ynab.Milliunits(-220), ynab.MustParseMilliunits("-0.220"))
	})
}

func TestMilliunitsArithmetic(t *testing.T) {
	t.Parallel()

	require.Equal(t, ynab.Milliunits(300), ynab.Milliunits(100).Add(200))
	require.Equal(t, ynab.Milliunits(-100), ynab.Milliunits(100).Sub(200))
	require.Equal(t, ynab.Milliunits(-100), ynab.Milliunits(100).Neg())
	require.Equal(t, ynab.Milliunits(100), ynab.Milliunits(-100).Abs())
	require.Equal(t, ynab.Milliunits(100), ynab.Milliunits(100).Abs())
	require.Equal(t, ynab.Milliunits(-600), ynab.Milliunits(-200).MulInt(3))
	require.Equal(t, ynab.Milliunits(5000), ynab.UnitsToMilliunits(5))
	require.InEpsilon(t, 123.93, ynab.Milliunits(123930).Units(), 1e-9)
}

func TestMilliunitsSplitEven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    ynab.Milliunits
		n    int
		want []ynab.Milliunits
	}{
		{name: "even split", m: 900, n: 3, want: []ynab.Milliunits{300, 300, 300}},
		{name: "indivisible positive", m: 100, n: 3, want: []ynab.Milliunits{34, 33, 33}},
		{name: "indivisible negative", m: -220, n: 3, want: []ynab.Milliunits{-74, -73, -73}},
		{name: "zero amount", m: 0, n: 2, want: []ynab.Milliunits{0, 0}},
		{name: "single part", m: -220, n: 1, want: []ynab.Milliunits{-220}},
		{name: "more parts than units", m: 2, n: 4, want: []ynab.Milliunits{1, 1, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.m.SplitEven(tt.n)
			require.Equal(t, tt.want, got)

			var sum ynab.Milliunits
			for _, p := range got {
				sum = sum.Add(p)
			}
			require.Equal(t, tt.m, sum, "parts must sum exactly to m")
		})
	}

	t.Run("panics on non-positive n", func(t *testing.T) {
		t.Parallel()

		require.Panics(t, func() { ynab.Milliunits(100).SplitEven(0) })
		require.Panics(t, func() { ynab.Milliunits(100).SplitEven(-1) })
	})
}

func TestCurrencyFormatFormat(t *testing.T) {
	t.Parallel()

	eur := ynab.CurrencyFormat{
		ISOCode:          "EUR",
		DecimalDigits:    2,
		DecimalSeparator: ".",
		SymbolFirst:      true,
		GroupSeparator:   ",",
		CurrencySymbol:   "€",
		DisplaySymbol:    true,
	}
	jod := ynab.CurrencyFormat{
		ISOCode:          "JOD",
		DecimalDigits:    3,
		DecimalSeparator: ".",
		SymbolFirst:      false,
		GroupSeparator:   ",",
		CurrencySymbol:   "JD",
		DisplaySymbol:    false,
	}
	sek := ynab.CurrencyFormat{
		ISOCode:          "SEK",
		DecimalDigits:    2,
		DecimalSeparator: ",",
		SymbolFirst:      false,
		GroupSeparator:   " ",
		CurrencySymbol:   "kr",
		DisplaySymbol:    true,
	}

	tests := []struct {
		name string
		m    ynab.Milliunits
		f    ynab.CurrencyFormat
		want string
	}{
		{name: "USD positive", m: 123930, f: usd(), want: "$123.93"},
		{name: "USD negative sub-unit", m: -220, f: usd(), want: "-$0.22"},
		{name: "EUR grouped", m: 4924340, f: eur, want: "€4,924.34"},
		{name: "JOD three decimals no symbol", m: -395032, f: jod, want: "-395.032"},
		{name: "symbol last with locale separators", m: 1234560, f: sek, want: "1 234,56kr"},
		{name: "rounds half away from zero", m: 5, f: usd(), want: "$0.01"},
		{name: "negative rounds half away from zero", m: -5, f: usd(), want: "-$0.01"},
		{name: "zero decimal digits", m: 1500, f: ynab.CurrencyFormat{DecimalDigits: 0, GroupSeparator: ","}, want: "2"},
		{name: "large grouping", m: 1234567890, f: usd(), want: "$1,234,567.89"},
		{
			name: "more decimals than the wire carries",
			m:    1500,
			f:    ynab.CurrencyFormat{DecimalDigits: 4, DecimalSeparator: "."},
			want: "1.5000",
		},
		{
			name: "hostile decimal digits clamp at 10",
			m:    1500,
			f:    ynab.CurrencyFormat{DecimalDigits: 5000, DecimalSeparator: "."},
			want: "1.5000000000",
		},
		{name: "negative decimal digits clamp at 0", m: 1500, f: ynab.CurrencyFormat{DecimalDigits: -3}, want: "2"},
		{name: "rounded-to-zero keeps its sign", m: -1, f: usd(), want: "-$0.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.m.Format(tt.f))
		})
	}
}

func TestCurrencyFormatWire(t *testing.T) {
	t.Parallel()

	// Wire shape from the vendored spec: all eight members required; the
	// object itself is nullable at its use sites (modeled as *CurrencyFormat).
	raw := `{
		"iso_code": "USD",
		"example_format": "123,456.78",
		"decimal_digits": 2,
		"decimal_separator": ".",
		"symbol_first": true,
		"group_separator": ",",
		"currency_symbol": "$",
		"display_symbol": true
	}`

	var got ynab.CurrencyFormat
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, usd(), got)

	// Optional[Milliunits] direct marshal matches the inner int64 encoding
	// (extends the Task-4 MarshalJSON table).
	b, err := json.Marshal(ynab.Set(ynab.Milliunits(-220)))
	require.NoError(t, err)
	require.Equal(t, "-220", string(b))
}
