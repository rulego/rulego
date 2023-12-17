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

package rulego

import (
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestNodeCtx(t *testing.T) {
	t.Run("NoFoundNodeType", func(t *testing.T) {
		selfDefinition := RuleNode{
			Id:   "s1",
			Type: "notFound",
		}
		_, err := InitRuleNodeCtx(NewConfig(), &selfDefinition)
		assert.Equal(t, "component not found.componentType=notFound", err.Error())
	})

	t.Run("notSupportThisFunc", func(t *testing.T) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				assert.Equal(t, "not support this func", fmt.Sprintf("%v", e))
			}
		}()
		selfDefinition := RuleNode{
			Id:   "s1",
			Type: "log",
		}
		ctx, _ := InitRuleNodeCtx(NewConfig(), &selfDefinition)
		ctx.ReloadChild(types.RuleNodeId{}, nil)
	})

	t.Run("initErr", func(t *testing.T) {
		selfDefinition := RuleNode{
			Id:            "s1",
			Type:          "dbClient",
			Configuration: types.Configuration{"sql": "xx"},
		}
		_, err := InitRuleNodeCtx(NewConfig(), &selfDefinition)
		assert.NotNil(t, err)
	})
	t.Run("reloadSelfErr", func(t *testing.T) {
		selfDefinition := RuleNode{
			Id:   "s1",
			Type: "log",
		}
		ctx, _ := InitRuleNodeCtx(NewConfig(), &selfDefinition)
		err := ctx.ReloadSelf([]byte("{"))
		assert.NotNil(t, err)
	})

	t.Run("ProcessGlobalPlaceholders", func(t *testing.T) {
		config := NewConfig()
		result := processGlobalPlaceholders(config, types.Configuration{"name": "${global.name}"})
		assert.Equal(t, "${global.name}", result["name"])

		config.Properties.PutValue("name", "lala")
		result = processGlobalPlaceholders(config, types.Configuration{"name": "${global.name}", "age": 18})
		assert.Equal(t, "lala", result["name"])
		assert.Equal(t, 18, result["age"])

		config.Properties = nil
		result = processGlobalPlaceholders(config, types.Configuration{"name": "${global.name}", "age": 18})
		assert.Equal(t, "${global.name}", result["name"])
	})

}
