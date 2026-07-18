// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

// Fuzzing for the hand-written parsers. Two properties each: no input
// may panic, and every accepted input must survive a String→Parse
// round-trip unchanged. The seed corpora double as regression tests on
// every ordinary `go test` run.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func FuzzParseDate(f *testing.F) {
	for _, seed := range []string{
		"2026-07-18", "0001-01-01", "9999-12-31", "2026-02-29", "2027-02-29",
		"", "current", "2026-7-18", "2026-07-18T00:00:00Z", "-2026-07-18",
		"2026-13-01", "2026-00-10", "2026-01-32", "20260718", "☃️-07-18",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := ynab.ParseDate(s)
		if err != nil {
			return
		}
		back, err := ynab.ParseDate(d.String())
		require.NoError(t, err, "accepted %q but rejects its own rendering %q", s, d)
		require.Equal(t, d, back, "round-trip drift for %q", s)
	})
}

func FuzzParseMonth(f *testing.F) {
	for _, seed := range []string{
		"2026-07-01", "current", "0001-01-01", "9999-12-01",
		"", "2026-07-18", "2026-07-00", "2026-13-01", "Current", "null",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		m, err := ynab.ParseMonth(s)
		if err != nil {
			return
		}
		back, err := ynab.ParseMonth(m.String())
		require.NoError(t, err, "accepted %q but rejects its own rendering %q", s, m)
		require.Equal(t, m, back, "round-trip drift for %q", s)
	})
}

func FuzzParseMilliunits(f *testing.F) {
	for _, seed := range []string{
		"0", "-0.220", "57.502", "1", "-1", "0.001", "-0.001",
		"9223372036854775.807", "-9223372036854775.808", // int64 edges in units
		"", ".", "-", "1.", ".5", "1.0000", "1e3", "+1", "١٢٣", "0x10", "NaN",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		mu, err := ynab.ParseMilliunits(s)
		if err != nil {
			return
		}
		back, err := ynab.ParseMilliunits(mu.String())
		require.NoError(t, err, "accepted %q but rejects its own rendering %q", s, mu)
		require.Equal(t, mu, back, "round-trip drift for %q", s)
	})
}

func FuzzDateUnmarshalJSON(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`"2026-07-18"`), []byte(`null`), []byte(`""`), []byte(`"current"`),
		[]byte(`"2026-07-18`), []byte(`2026-07-18`), []byte(`{"a":1}`), []byte(`" "`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		var d ynab.Date
		_ = d.UnmarshalJSON(data) // must never panic; errors are fine
		var m ynab.Month
		_ = m.UnmarshalJSON(data)
	})
}
