package ynab_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func TestMonthParseAndString(t *testing.T) {
	t.Parallel()

	t.Run("round trips wire form", func(t *testing.T) {
		t.Parallel()

		m, err := ynab.ParseMonth("2016-12-01")
		require.NoError(t, err)
		require.Equal(t, "2016-12-01", m.String())
		require.Equal(t, ynab.NewMonth(2016, time.December), m)
		require.False(t, m.IsZero())
		require.False(t, m.IsCurrent())
	})

	t.Run("accepts the current literal", func(t *testing.T) {
		t.Parallel()

		m, err := ynab.ParseMonth("current")
		require.NoError(t, err)
		require.Equal(t, ynab.CurrentMonth(), m)
		require.True(t, m.IsCurrent())
		require.False(t, m.IsZero())
		require.Equal(t, "current", m.String())
	})

	t.Run("rejects non-first-of-month and malformed input", func(t *testing.T) {
		t.Parallel()

		for _, s := range []string{"", "2016-12-05", "2016-12", "2016-12-1", "2016-13-01", "garbage", "2016-12-01T00:00:00Z"} {
			_, err := ynab.ParseMonth(s)
			require.Error(t, err, s)
		}
	})

	t.Run("zero month", func(t *testing.T) {
		t.Parallel()

		var m ynab.Month
		require.True(t, m.IsZero())
		require.False(t, m.IsCurrent())
		require.Empty(t, m.String())
	})
}

func TestMonthAccessorsAndArithmetic(t *testing.T) {
	t.Parallel()

	m := ynab.NewMonth(2016, time.December)
	require.Equal(t, 2016, m.Year())
	require.Equal(t, time.December, m.Mon())

	require.Equal(t, ynab.NewMonth(2017, time.January), m.Next())
	require.Equal(t, ynab.NewMonth(2016, time.November), m.Prev())
	require.Equal(t, ynab.NewMonth(2018, time.March), m.AddMonths(15))
	require.Equal(t, ynab.NewMonth(2015, time.September), m.AddMonths(-15))

	// NewMonth normalizes out-of-range months.
	require.Equal(t, ynab.NewMonth(2017, time.January), ynab.NewMonth(2016, time.Month(13)))

	require.Equal(t, ynab.NewDate(2016, time.December, 1), m.FirstDay())
	require.Equal(t, ynab.NewMonth(2016, time.December), ynab.MonthOf(time.Date(2016, time.December, 25, 10, 0, 0, 0, time.UTC)))

	a, b := ynab.NewMonth(2016, time.May), ynab.NewMonth(2016, time.June)
	require.Equal(t, -1, a.Compare(b))
	require.Equal(t, 1, b.Compare(a))
	require.Equal(t, 0, a.Compare(ynab.NewMonth(2016, time.May)))
	require.Equal(t, -1, ynab.NewMonth(2015, time.December).Compare(ynab.NewMonth(2016, time.January)), "year boundary")

	// Sentinels carry zero components: they order before every concrete
	// month and equal to each other.
	require.Equal(t, -1, ynab.CurrentMonth().Compare(a))
	require.Equal(t, 0, ynab.CurrentMonth().Compare(ynab.Month{}))
}

func TestMonthSentinelArithmetic(t *testing.T) {
	t.Parallel()

	// Arithmetic needs a concrete month; the sentinels pass through
	// unchanged and FirstDay yields the zero Date.
	require.Equal(t, ynab.CurrentMonth(), ynab.CurrentMonth().Next())
	require.Equal(t, ynab.CurrentMonth(), ynab.CurrentMonth().AddMonths(5))
	require.True(t, ynab.CurrentMonth().FirstDay().IsZero())

	var zero ynab.Month
	require.Equal(t, zero, zero.Next())
	require.True(t, zero.FirstDay().IsZero())
}

func TestMonthJSON(t *testing.T) {
	t.Parallel()

	t.Run("value receiver marshal through non-pointer field", func(t *testing.T) {
		t.Parallel()

		in := struct {
			M ynab.Month `json:"m"`
		}{M: ynab.NewMonth(2016, time.December)}

		got, err := json.Marshal(in)
		require.NoError(t, err)
		require.JSONEq(t, `{"m":"2016-12-01"}`, string(got))
	})

	t.Run("unmarshal round trip incl current", func(t *testing.T) {
		t.Parallel()

		var m ynab.Month
		require.NoError(t, json.Unmarshal([]byte(`"2016-12-01"`), &m))
		require.Equal(t, ynab.NewMonth(2016, time.December), m)

		require.NoError(t, json.Unmarshal([]byte(`"current"`), &m))
		require.True(t, m.IsCurrent())
	})

	t.Run("zero marshals null, current marshals the literal", func(t *testing.T) {
		t.Parallel()

		got, err := json.Marshal(ynab.Month{})
		require.NoError(t, err)
		require.Equal(t, "null", string(got))

		got, err = json.Marshal(ynab.CurrentMonth())
		require.NoError(t, err)
		require.Equal(t, `"current"`, string(got))

		m := ynab.NewMonth(2016, time.May)
		require.NoError(t, json.Unmarshal([]byte("null"), &m))
		require.True(t, m.IsZero())
	})

	t.Run("rejects non-first-of-month wire values", func(t *testing.T) {
		t.Parallel()

		var m ynab.Month
		require.Error(t, json.Unmarshal([]byte(`"2016-12-05"`), &m))
	})

	t.Run("rejects non-string wire values", func(t *testing.T) {
		t.Parallel()

		var m ynab.Month
		require.Error(t, json.Unmarshal([]byte(`123`), &m))
	})
}
