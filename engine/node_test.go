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

package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

func TestNodeCtx(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "test01",
            "name": "testRuleChain01",
            "debugMode": true,
            "root": true,
             "configuration": {
                 "vars": {
						"ip":"127.0.0.1"
					},
  				  "decryptSecrets": {
						"bb":"xx"
					}
                }
          }
        }`
	t.Run("NoFoundNodeType", func(t *testing.T) {
		selfDefinition := types.RuleNode{
			Id:   "s1",
			Type: "notFound",
		}
		_, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Equal(t, "nodeType:notFound for id:s1 new error:component not found. componentType=notFound", err.Error())
	})

	t.Run("notSupportThisFunc", func(t *testing.T) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				assert.Equal(t, "not support this func", fmt.Sprintf("%v", e))
			}
		}()
		selfDefinition := types.RuleNode{
			Id:   "s1",
			Type: "log",
		}
		ctx, _ := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		ctx.ReloadChild(types.RuleNodeId{}, nil)
	})

	t.Run("reloadSelf", func(t *testing.T) {
		selfDefinition := types.RuleNode{
			Id:   "s1",
			Type: "log",
		}
		ctx, _ := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		err := ctx.ReloadSelf([]byte(`{"id":"s2","type":"jsFilter"}`))
		assert.Nil(t, err)
		assert.Equal(t, "jsFilter", ctx.SelfDefinition.Type)
		assert.Equal(t, "s2", ctx.SelfDefinition.Id)
	})

	t.Run("initErr", func(t *testing.T) {
		selfDefinition := types.RuleNode{
			Id:            "s1",
			Type:          "dbClient",
			Configuration: types.Configuration{"sql": "xx"},
		}
		_, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.NotNil(t, err)
	})
	t.Run("reloadSelfErr", func(t *testing.T) {
		selfDefinition := types.RuleNode{
			Id:   "s1",
			Type: "log",
		}
		ctx, _ := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		err := ctx.ReloadSelf([]byte("{"))
		assert.NotNil(t, err)
	})

	t.Run("ProcessVariables", func(t *testing.T) {
		config := NewConfig()
		result, _ := processVariables(config, nil, types.Configuration{"name": "${global.name}"})
		assert.Equal(t, "${global.name}", result["name"])

		config.Properties.PutValue("name", "lala")
		result, _ = processVariables(config, nil, types.Configuration{"name": "${global.name}", "age": 18})
		assert.Equal(t, "lala", result["name"])
		assert.Equal(t, 18, result["age"])

		jsonParser := JsonParser{}
		def, _ := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		chainCtx, err := InitRuleChainCtx(config, nil, &def)
		assert.Nil(t, err)
		result, _ = processVariables(config, chainCtx, types.Configuration{"name": "${global.name}", "ip": "${vars.ip}"})
		assert.Equal(t, "lala", result["name"])
		assert.Equal(t, "127.0.0.1", result["ip"])

		config.Properties = types.NewProperties()
		result, _ = processVariables(config, nil, types.Configuration{"name": "${global.name}", "age": 18})
		assert.Equal(t, "${global.name}", result["name"])

	})

}

// TestNodeConcurrentAccess 测试节点并发访问和重载的安全性
func TestNodeConcurrentAccess(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())

	// 创建测试节点定义
	nodeDef := &types.RuleNode{
		Id:            "test_node",
		Type:          "log",
		Name:          "测试节点",
		Configuration: make(types.Configuration),
	}

	// 初始化节点
	nodeCtx, err := InitRuleNodeCtx(config, nil, nil, nodeDef)
	assert.Nil(t, err)
	assert.NotNil(t, nodeCtx)

	var wg sync.WaitGroup
	var errors int64

	// 模拟高频的OnMsg调用
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				func() {
					defer func() {
						if recover() != nil {
							atomic.AddInt64(&errors, 1)
						}
					}()

					// 模拟频繁的节点访问
					nodeCtx.Config()
					nodeCtx.IsDebugMode()
					nodeCtx.GetNodeId()
					nodeCtx.DSL()
				}()
			}
		}()
	}

	// 模拟偶尔的重载操作
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			time.Sleep(time.Millisecond * 10) // 稍微延迟一下

			// 创建新的节点定义
			newDef := types.RuleNode{
				Id:            "test_node",
				Type:          "log",
				Name:          "重载测试节点" + string(rune('A'+index)),
				Configuration: make(types.Configuration),
			}

			err := nodeCtx.ReloadSelfFromDef(newDef)
			if err != nil {
				atomic.AddInt64(&errors, 1)
			}
		}(i)
	}

	wg.Wait()

	// 验证没有出现错误
	assert.Equal(t, int64(0), atomic.LoadInt64(&errors))
}
