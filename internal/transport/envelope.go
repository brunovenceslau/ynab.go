package transport

import (
	"encoding/json"
	"errors"
)

// decodeData unwraps the success envelope {"data": ...} into T. The API's
// envelope invariant: a 2xx always carries data, never error — the two
// keys are mutually exclusive, so a missing data key is a loud decode
// error, not a silently zero-valued result.
func decodeData[T any](body []byte) (T, error) {
	var zero T
	var env struct {
		Data *T `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return zero, err
	}
	if env.Data == nil {
		return zero, errors.New("success envelope has no data key")
	}
	return *env.Data, nil
}
