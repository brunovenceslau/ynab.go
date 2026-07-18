package ynab

import "encoding/json"

// optionalState distinguishes the three wire meanings a write field can carry.
type optionalState uint8

const (
	optionalUnset optionalState = iota // field omitted from JSON
	optionalNull                       // field emitted as null ("clear this")
	optionalValue                      // field emitted with its value
)

// Optional models the tri-state every YNAB write needs:
// unset (field omitted) | null (field cleared) | value.
//
// Write models tag Optional fields with omitzero (Go 1.24+), and the
// unset state — and only the unset state — is what json omits. The zero value
// of Optional is unset.
//
// Optional appears in write models only; it has no UnmarshalJSON. Nullable
// read fields are plain pointers instead.
type Optional[T any] struct {
	value T
	state optionalState
}

// Set returns an Optional holding v. The field is emitted even when v is the
// zero value of T: Set(false), Set(0), and Set("") all reach the wire.
func Set[T any](v T) Optional[T] {
	return Optional[T]{value: v, state: optionalValue}
}

// SetNull returns an Optional that serializes as JSON null — "clear this
// field" on the server. Omitting the field (the unset zero value) leaves it
// unchanged instead; the two are different operations.
func SetNull[T any]() Optional[T] {
	return Optional[T]{state: optionalNull}
}

// Get returns the held value and whether one was set. It reports false for
// both the unset and null states.
func (o Optional[T]) Get() (T, bool) {
	return o.value, o.state == optionalValue
}

// IsNull reports whether the Optional was built with SetNull.
func (o Optional[T]) IsNull() bool {
	return o.state == optionalNull
}

// IsZero reports whether the Optional is unset. It is state-based, never
// value-based: Set(zero-of-T) reports false, so omitzero emits deliberately
// set zero values instead of silently dropping them.
func (o Optional[T]) IsZero() bool {
	return o.state == optionalUnset
}

// MarshalJSON encodes the held value using its own encoding/json encoding,
// and null for the null state. An unset Optional also encodes as null: only
// an omitzero struct field can express omission, and unset fields never
// reach MarshalJSON there.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.state != optionalValue {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}
