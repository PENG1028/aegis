package noderuntime

import (
	"encoding/json"
)

// jsonUnmarshal wraps json.Unmarshal for use within the package.
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// jsonMarshal wraps json.Marshal for use within the package.
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
