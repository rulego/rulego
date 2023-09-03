/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

//MarshalIndent see json.MarshalIndent
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	b, err := Marshal2(v, false)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = json.Indent(&buf, b, prefix, indent)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
