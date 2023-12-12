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
	"github.com/rulego/rulego/test/assert"
	"testing"
)

type User struct {
	Username string
	Age      int
	Address  Address
}
type Address struct {
	Detail string
}

func TestMarshal(t *testing.T) {
	var user = User{
		Username: "test",
	}
	v1, _ := json.Marshal(user)

	v2, _ := Marshal(user)
	assert.Equal(t, string(v1), string(v2))
}

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
}

func TestFormat(t *testing.T) {
	var user = User{
		Username: "test",
	}

	v, _ := json.Marshal(user)

	var buf bytes.Buffer
	_ = json.Indent(&buf, v, "", "  ")
	result, _ := Format(v)

	assert.Equal(t, buf.Bytes(), result)
}
