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
	"testing"

	"github.com/rulego/rulego/test/assert"
)

type User struct {
	Username string
	Age      int
	Address  Address
}
type Address struct {
	Detail string
}

func TestMarshalAndMarshal2(t *testing.T) {
	var user = User{
		Username: "test & < >", // Add characters that would be escaped
		Age:      30,
		Address:  Address{Detail: "Some Street"},
	}

	// Test Marshal (escapeHTML=false by default in our Marshal)
	v1, err1 := Marshal(user)
	assert.Nil(t, err1)
	assert.Equal(t, `{"Username":"test & < >","Age":30,"Address":{"Detail":"Some Street"}}`, string(v1))

	// Test Marshal2 with escapeHTML=false
	v2, err2 := Marshal2(user, false)
	assert.Nil(t, err2)
	assert.Equal(t, `{"Username":"test & < >","Age":30,"Address":{"Detail":"Some Street"}}`, string(v2))

	// Test Marshal2 with escapeHTML=true
	v3, err3 := Marshal2(user, true)
	assert.Nil(t, err3)
	assert.Equal(t, `{"Username":"test \u0026 \u003c \u003e","Age":30,"Address":{"Detail":"Some Street"}}`, string(v3))

	// Test Marshal with a type that cannot be marshaled (e.g., a channel)
	chanValue := make(chan int)
	_, errMarshalChan := Marshal(chanValue)
	assert.NotNil(t, errMarshalChan, "Expected error when marshaling a channel")

	_, errMarshal2Chan := Marshal2(chanValue, false)
	assert.NotNil(t, errMarshal2Chan, "Expected error when marshaling a channel with Marshal2")
}

func TestOldMarshal(t *testing.T) {
	var user = User{
		Username: "test",
	}
	v1, _ := json.Marshal(user)

	v2, _ := Marshal(user)
	assert.Equal(t, string(v1), string(v2))
}

// Renaming original TestMarshal to TestOldMarshal to avoid conflict, will be replaced by TestMarshalAndMarshal2
// func TestMarshal(t *testing.T) { ... }

func TestUnMarshal(t *testing.T) {
	var user = User{
		Username: "test",
	}

	v, _ := json.Marshal(user)

	var user1 = User{}
	_ = json.Unmarshal(v, &user1)
	var user2 = User{}
	err := Unmarshal(v, &user2)
	assert.Nil(t, err)
	assert.Equal(t, user1.Username, user2.Username)

	// Test Unmarshal with invalid JSON
	invalidJson := []byte(`{"Username":"test", invalid}`)
	var user3 User
	errUnmarshal := Unmarshal(invalidJson, &user3)
	assert.NotNil(t, errUnmarshal, "Expected error when unmarshaling invalid JSON")

	// Test Unmarshal with nil data
	errNilData := Unmarshal(nil, &user3)
	assert.NotNil(t, errNilData, "Expected error when unmarshaling nil data")

	// This would cause a compile-time error if directly passing user4.
	// The standard json.Unmarshal would panic. Our Unmarshal wraps it.
	// We can't directly test passing a non-pointer in a way that compiles and then fails at runtime
	// in the same way `json.Unmarshal(data, val)` would if `val` isn't a pointer.
	// However, if `m` is `nil` interface, `json.Unmarshal` will return an error.
	errNilInterface := Unmarshal(v, nil)
	assert.NotNil(t, errNilInterface, "Expected error when unmarshaling to a nil interface")
	assert.Nil(t, err)
	assert.Equal(t, user1.Username, user2.Username)
}

func TestFormat(t *testing.T) {
	var user = User{
		Username: "test",
	}

	v, _ := json.Marshal(user)

	var buf bytes.Buffer
	_ = json.Indent(&buf, v, "", "  ")
	result, errFormat := Format(v)
	assert.Nil(t, errFormat)
	assert.Equal(t, buf.Bytes(), result)

	// Test Format with invalid JSON
	invalidJson := []byte(`{"Username":"test", invalid}`)
	_, errFormatInvalid := Format(invalidJson)
	assert.NotNil(t, errFormatInvalid, "Expected error when formatting invalid JSON")

	// Test Format with empty JSON
	emptyJson := []byte(`{}`)
	formattedEmpty, errFormatEmpty := Format(emptyJson)
	assert.Nil(t, errFormatEmpty)
	assert.Equal(t, "{}", string(formattedEmpty)) // or check for specific formatting

	// Test Format with null JSON
	nullJson := []byte(`null`)
	formattedNull, errFormatNull := Format(nullJson)
	assert.Nil(t, errFormatNull)
	assert.Equal(t, "null", string(formattedNull))

	assert.Equal(t, buf.Bytes(), result)
}
