// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func TestDateParseAndString(t *testing.T) {
	t.Parallel()

	t.Run("round trips wire form", func(t *testing.T) {
		t.Parallel()

		for _, s := range []string{"2015-12-30", "2016-01-01", "0001-01-01", "9999-12-31"} {
			d, err := ynab.ParseDate(s)
			require.NoError(t, err, s)
			require.Equal(t, s, d.String(), s)
			require.False(t, d.IsZero())
		}
	})

	t.Run("rejects non-wire forms", func(t *testing.T) {
		t.Parallel()

		malformed := []string{
			"", "current", "2015-1-02", "2015-01-2", "2015-13-01",
			"2015-02-30", "2015-12-30T00:00:00Z", "30-12-2015",
		}
		for _, s := range malformed {
			_, err := ynab.ParseDate(s)
			require.Error(t, err, s)
		}
	})

	t.Run("zero renders empty", func(t *testing.T) {
		t.Parallel()

		var d ynab.Date
		require.True(t, d.IsZero())
		require.Empty(t, d.String())
	})
}

func TestDateAccessorsAndArithmetic(t *testing.T) {
	t.Parallel()

	d := ynab.NewDate(2015, time.December, 30)
	require.Equal(t, 2015, d.Year())
	require.Equal(t, time.December, d.Month())
	require.Equal(t, 30, d.Day())

	require.Equal(t, ynab.NewDate(2016, time.January, 2), d.AddDays(3))
	require.Equal(t, ynab.NewDate(2015, time.December, 27), d.AddDays(-3))
	require.Equal(t, ynab.NewDate(2016, time.January, 30), d.AddMonths(1))
	// AddMonths normalizes like time.AddDate: Jan 31 + 1 month rolls over.
	require.Equal(t, ynab.NewDate(2015, time.March, 3), ynab.NewDate(2015, time.January, 31).AddMonths(1))

	// NewDate normalizes out-of-range components the same way.
	require.Equal(t, ynab.NewDate(2016, time.January, 9), ynab.NewDate(2015, time.December, 40))

	a, b := ynab.NewDate(2015, time.May, 1), ynab.NewDate(2015, time.May, 2)
	require.Equal(t, -1, a.Compare(b))
	require.Equal(t, 1, b.Compare(a))
	require.Equal(t, 0, a.Compare(ynab.NewDate(2015, time.May, 1)))
	require.True(t, a.Before(b))
	require.False(t, a.After(b))
	require.True(t, b.After(a))

	// Ordering across month and year boundaries, and the zero value
	// ordering before every real date.
	require.Equal(t, -1, ynab.NewDate(2015, time.December, 31).Compare(ynab.NewDate(2016, time.January, 1)))
	require.Equal(t, 1, ynab.NewDate(2015, time.June, 1).Compare(ynab.NewDate(2015, time.May, 31)))
	require.Equal(t, -1, (ynab.Date{}).Compare(ynab.NewDate(1, time.January, 1)))

	// Arithmetic on the zero value passes through — "no date" never
	// fabricates a wire-visible date.
	require.True(t, (ynab.Date{}).AddDays(5).IsZero())
	require.True(t, (ynab.Date{}).AddMonths(-3).IsZero())
}

func TestDateTimeConversions(t *testing.T) {
	t.Parallel()

	d := ynab.NewDate(2015, time.December, 30)
	require.Equal(t, time.Date(2015, time.December, 30, 0, 0, 0, 0, time.UTC), d.Time())

	loc := time.FixedZone("UTC+7", 7*3600)
	require.Equal(t, time.Date(2015, time.December, 30, 0, 0, 0, 0, loc), d.In(loc))

	require.Equal(t, d, ynab.DateOf(time.Date(2015, time.December, 30, 23, 59, 0, 0, loc)))

	before := ynab.DateOf(time.Now())
	today := ynab.Today()
	after := ynab.DateOf(time.Now())
	require.GreaterOrEqual(t, today.Compare(before), 0)
	require.LessOrEqual(t, today.Compare(after), 0)
}

func TestDateJSON(t *testing.T) {
	t.Parallel()

	t.Run("value receiver marshal through non-pointer field", func(t *testing.T) {
		t.Parallel()

		// The pr-era regression: a pointer-receiver MarshalJSON silently
		// vanishes for non-addressable values. Prove the value receiver.
		in := struct {
			D ynab.Date `json:"d"`
		}{D: ynab.NewDate(2015, time.December, 30)}

		got, err := json.Marshal(in)
		require.NoError(t, err)
		require.JSONEq(t, `{"d":"2015-12-30"}`, string(got))
	})

	t.Run("unmarshal round trip", func(t *testing.T) {
		t.Parallel()

		var out struct {
			D ynab.Date `json:"d"`
		}
		require.NoError(t, json.Unmarshal([]byte(`{"d":"2015-12-30"}`), &out))
		require.Equal(t, ynab.NewDate(2015, time.December, 30), out.D)
	})

	t.Run("zero marshals as null and null unmarshals as zero", func(t *testing.T) {
		t.Parallel()

		got, err := json.Marshal(ynab.Date{})
		require.NoError(t, err)
		require.Equal(t, "null", string(got))

		d := ynab.NewDate(2015, time.May, 1)
		require.NoError(t, json.Unmarshal([]byte("null"), &d))
		require.True(t, d.IsZero())
	})

	t.Run("rejects malformed wire dates", func(t *testing.T) {
		t.Parallel()

		var d ynab.Date
		require.Error(t, json.Unmarshal([]byte(`"2015-13-01"`), &d))
		require.Error(t, json.Unmarshal([]byte(`123`), &d))
	})

	t.Run("Optional Date matches inner encoding", func(t *testing.T) {
		t.Parallel()

		// Extends the Task-4 MarshalJSON table with Optional[Date].
		got, err := json.Marshal(ynab.Set(ynab.NewDate(2015, time.December, 30)))
		require.NoError(t, err)
		require.Equal(t, `"2015-12-30"`, string(got))
	})
}
