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
	"github.com/rulego/rulego/builtin/aspect"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test/assert"
)

func TestConcurrencyLimiterAspect(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testDoOnEnd",
            "name": "TestDoOnEnd"
          },
          "metadata": {
            "nodes": [
              {
                "id": "s1",
                "type": "functions",
                "name": "结束函数",
                "debugMode": true,
                "configuration": {
                  "functionName": "doSleep"
                }
              }
            ],
            "connections": [
            ]
          }
        }`

	//测试函数
	action.Functions.Register("doSleep", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(time.Millisecond * 200)
		ctx.TellNext(msg, types.Success)
	})
	config := NewConfig(types.WithDefaultPool())
	//限制并发1
	ruleEngine, err := New("testLimiterAspect", []byte(ruleChainFile), WithConfig(config),
		types.WithAspects(&aspect.Debug{}, aspect.NewConcurrencyLimiterAspect(1)))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 100)
	//上一条没执行完，并发超过限制
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 200)
	//都已经执行完，解除并发限制
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 250)
	//修改并发2
	_ = ruleEngine.Reload(types.WithAspects(&aspect.Debug{}, aspect.NewConcurrencyLimiterAspect(2)))
	msg = types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 100)
	//触发并发限制
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 250)
	//取消限制并发
	_ = ruleEngine.Reload(types.WithAspects(&aspect.Debug{}))
	var i = 0
	for i < 10 {
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		i++
	}
	time.Sleep(time.Millisecond * 400)

	//重新设置并发
	_ = ruleEngine.Reload(types.WithAspects(&aspect.Debug{}, aspect.NewConcurrencyLimiterAspect(1)))
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 100)
	//上一条没执行完，并发超过限制
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 100)
}
