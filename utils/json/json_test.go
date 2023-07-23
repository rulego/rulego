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
	"encoding/json"
	"fmt"
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
	v, _ := json.Marshal(user)
	fmt.Println(string(v))
	v, _ = Marshal(user)
	fmt.Println(string(v))
}
