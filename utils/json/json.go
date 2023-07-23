package json

import (
	"bytes"
	"encoding/json"
)

// Marshal marshals the struct to json data.
//escapeHTML=false
//disables this behavior.escape &, <, and > to \u0026, \u003c, and \u003e
func Marshal(v interface{}) ([]byte, error) {
	return Marshal2(v, false)
}

func Marshal2(v interface{}, escapeHTML bool) ([]byte, error) {
	var byteBuf bytes.Buffer
	encoder := json.NewEncoder(&byteBuf)
	encoder.SetEscapeHTML(escapeHTML)
	err := encoder.Encode(v)
	if err == nil && byteBuf.Len() > 0 {
		return byteBuf.Bytes()[:byteBuf.Len()-1], err
	} else {
		return byteBuf.Bytes(), err
	}
}

// Unmarshal json data to struct
func Unmarshal(b []byte, m interface{}) error {
	return json.Unmarshal(b, m)
}
