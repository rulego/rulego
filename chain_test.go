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

func TestChainCtx(t *testing.T) {

	ruleChainDef := RuleChain{}

	t.Run("New", func(t *testing.T) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				assert.Equal(t, "not support this func", fmt.Sprintf("%s", e))
			}
		}()
		ruleChainDef.Metadata.RuleChainConnections = []RuleChainConnection{
			{
				FromId: "s1",
				ToId:   "s2",
				Type:   types.True,
			},
		}
		ctx, _ := InitRuleChainCtx(NewConfig(), &ruleChainDef)
		ctx.New()
	})

	t.Run("Init", func(t *testing.T) {
		ctx, _ := InitRuleChainCtx(NewConfig(), &ruleChainDef)
		newRuleChainDef := RuleChain{}
		err := ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Nil(t, err)

		newRuleChainDef = RuleChain{}
		ruleNode := RuleNode{Type: "notFound"}
		newRuleChainDef.Metadata.Nodes = append(newRuleChainDef.Metadata.Nodes, &ruleNode)
		err = ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Equal(t, "component not found.componentType=notFound", err.Error())
	})

	t.Run("ReloadChildNotFound", func(t *testing.T) {
		ctx, _ := InitRuleChainCtx(NewConfig(), &ruleChainDef)
		newRuleChainDef := RuleChain{}
		err := ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Nil(t, err)
		err = ctx.ReloadChild(types.RuleNodeId{}, []byte(""))
		assert.Nil(t, err)
	})

}
