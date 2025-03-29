/*
 * Copyright 2025 The RuleGo Authors.
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

package dsl

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"strings"
	"testing"
)

func TestParseVars(t *testing.T) {
	inputData := `{
		"ruleChain": {
			"id": "fahrenheit",
			"name": "华氏温度转换",
			"debugMode": true,
			"root": true,
			"disabled": false,
			"additionalInfo": {
				"createTime": "2025/03/12 10:08:57",
				"description": "华氏温度转换",
				"height": 40,
				"layoutX": "280",
				"layoutY": "280",
				"updateTime": "2025/03/28 23:53:31",
				"username": "admin",
				"width": 240
			}
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s2",
					"additionalInfo": {
						"layoutX": 590,
						"layoutY": 270
					},
					"type": "jsTransform",
					"name": "摄氏温度转华氏温度",
					"debugMode": true,
					"configuration": {
						"jsScript": "var newMsg={'temperature': msg.temperature*msg.scaleFactor+32};\n return {'msg':newMsg,'metadata':metadata,'msgType':msgType};"
					}
				},
                {
					"id": "s2",
					"type": "dbClient",
					"name": "db query",
					"debugMode": true,
					"configuration": {
						"sql": "${msg.sql}"
					}
				}

			],
			"connections": []
		}
	}`

	var ruleChain types.RuleChain
	err := json.Unmarshal([]byte(inputData), &ruleChain)
	if err != nil {
		t.Fatalf("Failed to unmarshal input data: %v", err)
	}

	actualOutput := ParseVars(types.MsgKey, ruleChain)
	assert.True(t, strings.Contains(strings.Join(actualOutput, ","), "scaleFactor"))
	assert.True(t, strings.Contains(strings.Join(actualOutput, ","), "sql"))
}
