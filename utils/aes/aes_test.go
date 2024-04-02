/*
 * Copyright 2024 The RuleGo Authors.
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

package aes

import (
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestAes(t *testing.T) {
	key := "secret"
	plaintext := "Hello, World!"

	encrypted, err := Encrypt(plaintext, []byte(key))
	if err != nil {
		panic(err)
	}

	decrypted, err := Decrypt(encrypted, []byte(key))
	if err != nil {
		panic(err)
	}
	assert.Equal(t, plaintext, decrypted)

}
