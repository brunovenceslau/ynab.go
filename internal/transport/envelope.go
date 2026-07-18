package transport

import "encoding/json"

// decodeData unwraps the success envelope {"data": ...} into T. The API's
// envelope invariant: a 2xx always carries data, never error — the two
// keys are mutually exclusive.
func decodeData[T any](body []byte) (T, error) {
	var env struct {
		Data T `json:"data"`
	}
	err := json.Unmarshal(body, &env)
	return env.Data, err
}
