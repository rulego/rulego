/*
 * Copyright 2026 The RuleGo Authors.
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

package el

import (
	"testing"

	"github.com/rulego/rulego/test/assert"
)

func TestExecuteTemplate(t *testing.T) {
	dict := map[string]interface{}{
		"name": "Alice",
		"age":  "18",
		"info": map[string]interface{}{
			"job": map[string]interface{}{
				"title": "Engineer",
			},
			"location": map[string]interface{}{
				"city": "GZ",
				"addr": "",
			},
		},
	}

	s := ExecuteTemplate("Hello, ${name}. You are ${age} years old. I am an ${info.job.title} from ${info.location.city} ${info.location.addr}. ${unknown}", dict)
	assert.Equal(t, "Hello, Alice. You are 18 years old. I am an Engineer from GZ . ${unknown}", s)

	s = ExecuteTemplate("Hello, Alice.", dict)
	assert.Equal(t, "Hello, Alice.", s)
}
