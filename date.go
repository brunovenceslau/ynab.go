package ynab

import (
	"fmt"
	"time"
)

// Date is a calendar day in the YNAB wire form YYYY-MM-DD. All wire dates
// are UTC. The zero value means "no date": IsZero reports true, String
// renders the empty string, and JSON encodes null.
//
// All methods use value receivers except UnmarshalJSON, so the type is
// safe to copy and compare.
type Date struct {
	year  int
	month time.Month
	day   int
}

// NewDate returns the Date for the given components, normalizing
// out-of-range values the way time.Date does (December 40 → January 9).
func NewDate(year int, month time.Month, day int) Date {
	return DateOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// DateOf returns the calendar day t shows in its own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{year: y, month: m, day: d}
}

// Today returns the current day in the local time zone.
func Today() Date {
	return DateOf(time.Now())
}

// ParseDate parses the strict wire form YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("ynab: parse date %q: %w", s, err)
	}
	d := DateOf(t)
	if d.String() != s {
		return Date{}, fmt.Errorf("ynab: parse date %q: want strict YYYY-MM-DD form", s)
	}
	return d, nil
}

// Year returns the calendar year.
func (d Date) Year() int { return d.year }

// Month returns the calendar month.
func (d Date) Month() time.Month { return d.month }

// Day returns the day of the month.
func (d Date) Day() int { return d.day }

// AddDays returns the date n days after d (before, for negative n).
// The zero value passes through unchanged — arithmetic on "no date" must
// never fabricate a wire-visible date.
func (d Date) AddDays(n int) Date {
	if d.IsZero() {
		return d
	}
	return DateOf(d.Time().AddDate(0, 0, n))
}

// AddMonths returns the date n months after d, normalizing the way
// time.AddDate does: January 31 plus one month rolls over into March.
// The zero value passes through unchanged.
func (d Date) AddMonths(n int) Date {
	if d.IsZero() {
		return d
	}
	return DateOf(d.Time().AddDate(0, n, 0))
}

// Compare orders two dates chronologically: -1 if d is before o, 0 if
// equal, +1 if after. The zero value orders before every real date.
func (d Date) Compare(o Date) int {
	switch {
	case d == o:
		return 0
	case d.year != o.year:
		return compareInt(d.year, o.year)
	case d.month != o.month:
		return compareInt(int(d.month), int(o.month))
	default:
		return compareInt(d.day, o.day)
	}
}

// Before reports whether d is chronologically before o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is chronologically after o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// IsZero reports whether d is the zero "no date" value.
func (d Date) IsZero() bool { return d == Date{} }

// In returns midnight at the start of d in loc.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, loc)
}

// Time returns midnight at the start of d in UTC — the wire's time zone.
func (d Date) Time() time.Time { return d.In(time.UTC) }

// String renders the wire form YYYY-MM-DD, or the empty string for the
// zero value.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, int(d.month), d.day)
}

// MarshalJSON encodes the wire form "YYYY-MM-DD", and null for the zero
// value.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON decodes the strict wire form "YYYY-MM-DD"; JSON null
// yields the zero value.
func (d *Date) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*d = Date{}
		return nil
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("ynab: unmarshal date %s: want a JSON string", s)
	}
	parsed, err := ParseDate(s[1 : len(s)-1])
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// compareInt is the three-way integer comparison Compare builds on.
func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
