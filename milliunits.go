package ynab

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Milliunits is a currency amount in thousandths of the currency unit —
// the only amount representation the YNAB API uses. 1000 milliunits = 1.00
// unit; the wire carries exact integers, never floats.
type Milliunits int64

// UnitsToMilliunits converts whole currency units to Milliunits.
// The multiplication wraps on int64 overflow, like the underlying integer.
func UnitsToMilliunits(units int64) Milliunits {
	return Milliunits(units * 1000)
}

// ParseMilliunits parses a decimal amount string ("123.930", "-0.22", "1")
// into Milliunits. It accepts an optional sign, an integer part, and at most
// three fraction digits — the exact 3-decimal form String produces always
// round-trips.
func ParseMilliunits(s string) (Milliunits, error) {
	num := s
	var sign string
	if len(num) > 0 && (num[0] == '+' || num[0] == '-') {
		sign, num = num[:1], num[1:]
	}

	intPart, frac, hasFrac := strings.Cut(num, ".")
	if intPart == "" || (hasFrac && (frac == "" || len(frac) > 3)) {
		return 0, fmt.Errorf("ynab: parse milliunits %q: want [+-]digits with at most 3 fraction digits", s)
	}

	// Normalizing to a plain milliunit integer string makes strconv do all
	// digit validation and both overflow bounds (incl. math.MinInt64).
	v, err := strconv.ParseInt(sign+intPart+frac+strings.Repeat("0", 3-len(frac)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ynab: parse milliunits %q: %w", s, err)
	}
	return Milliunits(v), nil
}

// MustParseMilliunits is ParseMilliunits that panics on parse failure.
// Use it for literals in tests and examples, never on external input.
func MustParseMilliunits(s string) Milliunits {
	m, err := ParseMilliunits(s)
	if err != nil {
		panic(err)
	}
	return m
}

// Add returns m + o. Wraps on int64 overflow.
func (m Milliunits) Add(o Milliunits) Milliunits { return m + o }

// Sub returns m - o. Wraps on int64 overflow.
func (m Milliunits) Sub(o Milliunits) Milliunits { return m - o }

// Neg returns -m. Wraps for math.MinInt64.
func (m Milliunits) Neg() Milliunits { return -m }

// Abs returns the absolute value of m. Wraps for math.MinInt64.
func (m Milliunits) Abs() Milliunits {
	if m < 0 {
		return -m
	}
	return m
}

// MulInt returns m * k. The multiplication wraps on int64 overflow, like the
// underlying integer — callers doing untrusted arithmetic must bound inputs.
func (m Milliunits) MulInt(k int64) Milliunits { return m * Milliunits(k) }

// SplitEven divides m into n parts that differ by at most one milliunit and
// sum exactly to m — the remainder is distributed one milliunit at a time
// over the first parts. Use it to build split transactions that always pass
// the sum-equality validation. It panics if n <= 0.
func (m Milliunits) SplitEven(n int) []Milliunits {
	if n <= 0 {
		panic(fmt.Sprintf("ynab: SplitEven(%d): n must be positive", n))
	}

	quo := m / Milliunits(n)
	rem := m % Milliunits(n)
	extra := rem.Abs()
	step := Milliunits(1)
	if rem < 0 {
		step = -1
	}

	parts := make([]Milliunits, n)
	for i := range parts {
		parts[i] = quo
		if Milliunits(i) < extra {
			parts[i] += step
		}
	}
	return parts
}

// Units returns m as floating-point currency units. Display only: float64
// cannot represent every amount exactly — do money math on Milliunits.
func (m Milliunits) Units() float64 { return float64(m) / 1000 }

// String renders m with exactly three decimals ("-0.220", "123.930") — the
// unambiguous debugging/log form, independent of any CurrencyFormat.
func (m Milliunits) String() string {
	u := magnitude(m)
	s := fmt.Sprintf("%d.%03d", u/1000, u%1000)
	if m < 0 {
		return "-" + s
	}
	return s
}

// Format renders m per the plan's currency format: decimal_digits decimals
// (rounded half away from zero), the format's separators, and the currency
// symbol when display_symbol is set (before the number and after the sign
// when symbol_first, appended otherwise). DecimalDigits is clamped to
// 0..10 — the wire never exceeds 3, and an out-of-range value from a
// hostile payload must not drive huge allocations. Amounts that round to
// zero keep their sign ("-$0.00").
func (m Milliunits) Format(f CurrencyFormat) string {
	digits := min(max(int(f.DecimalDigits), 0), 10)

	// Rescale the thousandths magnitude to the requested decimal count.
	u := magnitude(m)
	if digits <= 3 {
		div := pow10(3 - digits)
		u = (u + div/2) / div
	} else {
		u *= pow10(digits - 3)
	}

	scale := pow10(digits)
	number := groupDigits(strconv.FormatUint(u/scale, 10), f.GroupSeparator)
	if digits > 0 {
		frac := strconv.FormatUint(u%scale, 10)
		number += f.DecimalSeparator + strings.Repeat("0", digits-len(frac)) + frac
	}

	var b strings.Builder
	if m < 0 {
		b.WriteByte('-')
	}
	if f.DisplaySymbol && f.SymbolFirst {
		b.WriteString(f.CurrencySymbol)
	}
	b.WriteString(number)
	if f.DisplaySymbol && !f.SymbolFirst {
		b.WriteString(f.CurrencySymbol)
	}
	return b.String()
}

// magnitude returns |m| as a uint64, exact for math.MinInt64 too.
func magnitude(m Milliunits) uint64 {
	if m >= 0 {
		return uint64(m)
	}
	if m == math.MinInt64 {
		return 1 << 63
	}
	return uint64(-m)
}

// pow10 returns 10^n for the small exponents Format needs.
func pow10(n int) uint64 {
	p := uint64(1)
	for range n {
		p *= 10
	}
	return p
}

// groupDigits inserts sep between every three digits of s, right to left.
func groupDigits(s, sep string) string {
	if sep == "" || len(s) <= 3 {
		return s
	}
	var b strings.Builder
	lead := len(s) % 3
	if lead > 0 {
		b.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteString(sep)
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// CurrencyFormat is the plan-level currency rendering metadata. All eight
// members are always present on the wire; the object itself can be null at
// its use sites, which model it as *CurrencyFormat.
type CurrencyFormat struct {
	ISOCode          string `json:"iso_code"`
	ExampleFormat    string `json:"example_format"`
	DecimalDigits    int32  `json:"decimal_digits"`
	DecimalSeparator string `json:"decimal_separator"`
	SymbolFirst      bool   `json:"symbol_first"`
	GroupSeparator   string `json:"group_separator"`
	CurrencySymbol   string `json:"currency_symbol"`
	DisplaySymbol    bool   `json:"display_symbol"`
}
