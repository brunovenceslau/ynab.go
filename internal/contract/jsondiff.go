package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

// DiffJSON compares an expected JSON document against the actually emitted
// one: full value equality (numbers kept exact via json.Number) plus the
// emitted key set at every nesting level, both directions — an
// accidentally omitted Optional field and an accidentally added key are
// both discrepancies. Empty result means byte-equivalent JSON.
func DiffJSON(expected, actual []byte) []string {
	exp, err := decodeExact(expected)
	if err != nil {
		return []string{"expected JSON invalid: " + err.Error()}
	}
	act, err := decodeExact(actual)
	if err != nil {
		return []string{"emitted JSON invalid: " + err.Error()}
	}

	var problems []string
	expKeys, actKeys := keyPaths(exp, "$"), keyPaths(act, "$")
	for _, k := range expKeys {
		if !slices.Contains(actKeys, k) {
			problems = append(problems, "missing key "+k+" (an Optional silently omitted?)")
		}
	}
	for _, k := range actKeys {
		if !slices.Contains(expKeys, k) {
			problems = append(problems, "unexpected key "+k)
		}
	}
	if len(problems) == 0 && !reflect.DeepEqual(exp, act) {
		problems = append(problems, fmt.Sprintf("values differ: want %s, got %s", expected, actual))
	}
	return problems
}

// decodeExact unmarshals keeping numbers as json.Number, so int64
// milliunits never lose precision through float64.
func decodeExact(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// keyPaths flattens every object key (and array index) into a sorted list
// of paths like $.transaction.approved.
func keyPaths(v any, prefix string) []string {
	var out []string
	switch tv := v.(type) {
	case map[string]any:
		for k, child := range tv {
			p := prefix + "." + k
			out = append(out, p)
			out = append(out, keyPaths(child, p)...)
		}
	case []any:
		for i, child := range tv {
			out = append(out, keyPaths(child, prefix+"["+strconv.Itoa(i)+"]")...)
		}
	}
	slices.Sort(out)
	return out
}
