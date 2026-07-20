// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"fmt"
	"time"
)

// Month is a plan month in the YNAB wire form YYYY-MM-01 — deliberately
// distinct from Date so a day-precision value cannot compile into a month
// slot. Two sentinels exist besides concrete months: the zero value ("no
// month"; IsZero, renders empty, JSON null) and [CurrentMonth] (the
// server-resolved "current" literal; IsCurrent, renders "current").
//
// All methods use value receivers except UnmarshalJSON, so the type is
// safe to copy and compare.
type Month struct {
	year    int
	month   time.Month
	current bool
}

// NewMonth returns the Month for the given year and month, normalizing
// out-of-range values the way [time.Date] does (month 13 → January next year).
func NewMonth(year int, month time.Month) Month {
	y, m, _ := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Date()
	return Month{year: y, month: m}
}

// MonthOf returns the plan month containing the calendar day t shows in
// its own location.
func MonthOf(t time.Time) Month {
	y, m, _ := t.Date()
	return Month{year: y, month: m}
}

// ParseMonth parses the strict wire form YYYY-MM-01 or the literal
// "current". Any day other than 01 is an error.
func ParseMonth(s string) (Month, error) {
	if s == "current" {
		return CurrentMonth(), nil
	}
	d, err := ParseDate(s)
	if err != nil {
		return Month{}, fmt.Errorf("ynab: parse month %q: want YYYY-MM-01 or \"current\"", s)
	}
	if d.Day() != 1 {
		return Month{}, fmt.Errorf("ynab: parse month %q: day must be 01", s)
	}
	return Month{year: d.Year(), month: d.Month()}, nil
}

// CurrentMonth returns the "current" sentinel — the literal the server
// resolves to its own current plan month. It is not a clock read: use
// [MonthOf](time.Now()) for the client's local notion of this month.
func CurrentMonth() Month {
	return Month{current: true}
}

// zeroMonthError is the shared pre-flight failure every month-taking
// operation returns for the zero Month — the request is never sent.
func zeroMonthError(op string) *ArgumentError {
	return &ArgumentError{Op: op, Field: "month", Reason: "month must not be zero"}
}

// Year returns the calendar year, or 0 for the sentinels.
func (m Month) Year() int { return m.year }

// Month returns the calendar month, or 0 for the sentinels.
func (m Month) Month() time.Month { return m.month }

// Next returns the month after m. Sentinels pass through unchanged —
// only a concrete month supports arithmetic.
func (m Month) Next() Month { return m.AddMonths(1) }

// Prev returns the month before m. Sentinels pass through unchanged —
// only a concrete month supports arithmetic.
func (m Month) Prev() Month { return m.AddMonths(-1) }

// AddMonths returns the month n months after m (before, for negative n).
// Sentinels pass through unchanged — only a concrete month supports
// arithmetic.
func (m Month) AddMonths(n int) Month {
	if m.IsZero() || m.current {
		return m
	}
	return NewMonth(m.year, m.month+time.Month(n))
}

// Compare orders two months chronologically by (year, month): -1, 0, or
// +1. Both sentinels carry zero components and therefore order before
// every concrete month (and equal to each other).
func (m Month) Compare(o Month) int {
	if m.year != o.year {
		return compareInt(m.year, o.year)
	}
	return compareInt(int(m.month), int(o.month))
}

// FirstDay returns the first calendar day of m, or the zero Date for the
// sentinels.
func (m Month) FirstDay() Date {
	if m.IsZero() || m.current {
		return Date{}
	}
	return Date{year: m.year, month: m.month, day: 1}
}

// IsCurrent reports whether m is the [CurrentMonth] sentinel.
func (m Month) IsCurrent() bool { return m.current }

// IsZero reports whether m is the zero "no month" value.
func (m Month) IsZero() bool { return m == Month{} }

// String renders the wire form YYYY-MM-01, the literal "current" for the
// [CurrentMonth] sentinel, or the empty string for the zero value.
func (m Month) String() string {
	switch {
	case m.current:
		return "current"
	case m.IsZero():
		return ""
	default:
		return fmt.Sprintf("%04d-%02d-01", m.year, int(m.month))
	}
}

// MarshalJSON encodes the wire form "YYYY-MM-01", "current" for the
// [CurrentMonth] sentinel, and null for the zero value.
func (m Month) MarshalJSON() ([]byte, error) {
	if m.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + m.String() + `"`), nil
}

// UnmarshalJSON decodes the strict wire form "YYYY-MM-01" or "current";
// JSON null yields the zero value.
func (m *Month) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*m = Month{}
		return nil
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("ynab: unmarshal month %s: want a JSON string", s)
	}
	parsed, err := ParseMonth(s[1 : len(s)-1])
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
