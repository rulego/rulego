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
	"context"
	"math"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestDefaultRuleGo(t *testing.T) {
	ruleDsl, err := os.ReadFile("testdata/rule/filter_node.json")
	assert.Nil(t, err)
	err = Load("./api/")

	_, err = New("aa", ruleDsl)
	assert.Nil(t, err)

	_, err = New("aa", ruleDsl)
	assert.Nil(t, err)

	_, ok := Get("aa")
	assert.True(t, ok)

	j := 0
	Range(func(key, value any) bool {
		j++
		return true
	})
	assert.True(t, j > 0)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	OnMsg(msg)

	Reload()

	Del("aa")

	_, ok = Get("aa")
	assert.False(t, ok)

	Stop()
}

// TestRuleGo 测试加载规则链文件夹
func TestRuleGo(t *testing.T) {
	//注册自定义组件
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	myRuleGo := &RuleGo{}

	p := engine.NewPool()
	myRuleGo = &RuleGo{
		pool: p,
	}

	assert.True(t, p == myRuleGo.Pool())
	assert.False(t, NewRuleGo() == NewRuleGo())
	config := NewConfig()
	var chainHasSubChainNodeDone int32
	var chainMsgTypeSwitchDone int32
	config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if ruleChainId == "chain_has_sub_chain_node" {
			atomic.StoreInt32(&chainHasSubChainNodeDone, 1)
		}
		if ruleChainId == "chain_msg_type_switch" {
			atomic.StoreInt32(&chainMsgTypeSwitchDone, 1)
		}
	}

	err := myRuleGo.Load("./testdata/aa.txt", WithConfig(config))
	assert.NotNil(t, err)

	err = myRuleGo.Load("./testdata/aa", WithConfig(config))
	assert.NotNil(t, err)

	err = myRuleGo.Load("./testdata/*.json", WithConfig(config))
	assert.Nil(t, err)

	var i = 0
	myRuleGo.Range(func(key, value any) bool {
		i++
		return true
	})
	assert.True(t, i > 0)

	_, ok := myRuleGo.Get("chain_call_rest_api")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("chain_has_sub_chain_node")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("chain_msg_type_switch")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("not_debug_mode_chain")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("sub_chain")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("test_context_chain")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("aa")
	assert.Equal(t, false, ok)

	myRuleGo.Del("sub_chain")

	_, ok = myRuleGo.Get("sub_chain")
	assert.Equal(t, false, ok)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	myRuleGo.OnMsg(msg)

	time.Sleep(time.Millisecond * 500)

	assert.True(t, atomic.LoadInt32(&chainHasSubChainNodeDone) == 1)
	assert.True(t, atomic.LoadInt32(&chainMsgTypeSwitchDone) == 1)

	myRuleGo.Reload()

	myRuleGo.OnMsg(msg)

	ruleEngine, _ := myRuleGo.Get("test_context_chain")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	ruleEngine.Stop(ctx) //故意在OnMsg之前停止

	ruleEngine.OnMsg(msg)

	myRuleGo.Stop()

	_, ok = myRuleGo.Get("test_context_chain")
	assert.Equal(t, false, ok)
}

func TestHttpEndpointAspect(t *testing.T) {
	ruleDsl, err := os.ReadFile("testdata/rule/with_http_endpoint.json")
	assert.Nil(t, err)

	id := "withHttpEndpoint"
	ruleEngine, err := New(id, ruleDsl)
	assert.Nil(t, err)

	//端口已经占用错误
	_, err = New("withHttpEndpoint2", ruleDsl)
	assert.NotNil(t, err)

	config := engine.NewConfig(types.WithDefaultPool())
	metaData := types.BuildMetadata(make(map[string]string))
	msg := types.NewMsg(0, "TEST_MSG_TYPE_AA", types.JSON, metaData, "{\"name\":\"lala\"}")

	sendMsg(t, "http://127.0.0.1:9090/api/v1/test/"+id, "POST", msg, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
		assert.Equal(t, "{\"name\":\"lala\"}", msg.GetData())
	}))
	time.Sleep(time.Millisecond * 500)

	newRuleDsl := strings.Replace(string(ruleDsl), "9090", "8080", -1)
	err = ruleEngine.ReloadSelf([]byte(newRuleDsl))
	assert.Nil(t, err)

	sendMsg(t, "http://127.0.0.1:9090/api/v1/test/"+id, "POST", msg, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Failure, relationType)
	}))

	sendMsg(t, "http://127.0.0.1:8080/api/v1/test/"+id, "POST", msg, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
		assert.Equal(t, "{\"name\":\"lala\"}", msg.GetData())
	}))

	time.Sleep(time.Millisecond * 500)
	Del(id)
}

func TestScheduleEndpointAspect(t *testing.T) {
	var count = int64(0)

	// 使用唯一的处理器名称避免冲突
	processorName := "testPrint_" + t.Name()
	processor.InBuiltins.Register(processorName, func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//fmt.Printf("testPrint:%s \n", time.Now().Format("2006-01-02 15:04:05"))
		atomic.AddInt64(&count, 1)
		return true
	})
	defer processor.InBuiltins.Unregister(processorName)

	// 修改规则定义以使用唯一的处理器名称
	ruleDsl, err := os.ReadFile("testdata/rule/with_schedule_endpoint.json")
	assert.Nil(t, err)

	// 替换处理器名称
	modifiedRuleDsl := strings.Replace(string(ruleDsl), "testPrint", processorName, -1)

	id := "withScheduleEndpoint_" + t.Name()
	ruleEngine, err := New(id, []byte(modifiedRuleDsl))
	assert.Nil(t, err)
	defer Del(id)

	time.Sleep(time.Second * 6)
	countValue := atomic.LoadInt64(&count)
	assert.True(t, math.Abs(float64(countValue)-float64(6)) <= float64(1))

	atomic.StoreInt64(&count, 0)

	oldAspects := ruleEngine.(*engine.RuleEngine).GetAspects()
	currentAspects := ruleEngine.(*engine.RuleEngine).GetAspects()
	assert.Equal(t, len(engine.BuiltinsAspects), len(currentAspects))
	assert.False(t, reflect.DeepEqual(engine.BuiltinsAspects, currentAspects))

	newRuleDsl := strings.Replace(modifiedRuleDsl, "*/1 * * * * *", "*/3 * * * * *", -1)
	err = ruleEngine.ReloadSelf([]byte(newRuleDsl), types.WithConfig(engine.NewConfig(types.WithDefaultPool())))
	assert.Nil(t, err)

	assert.True(t, reflect.DeepEqual(oldAspects, ruleEngine.(*engine.RuleEngine).GetAspects()))
	time.Sleep(time.Second * 9) // 延长等待时间从6秒到9秒，确保至少执行3次

	// 使用原子操作安全地读取count值
	countValue = atomic.LoadInt64(&count)
	// 9秒内每3秒执行一次，期望3次，允许±1误差（即2-4次都算通过）
	assert.True(t, countValue >= 2 && countValue <= 4, "Expected 2-4 executions in 9 seconds with 3-second interval, but got %d", countValue)

	err = ruleEngine.ReloadChild("s1", []byte(` {
        "id":"s1",
        "type": "jsFilter",
        "name": "过滤",
        "debugMode": true,
        "configuration": {
          "jsScript": "return msg.temperature>10;"
        }
      }`))
	assert.Nil(t, err)
}

// 发送消息到rest服务器
func sendMsg(t *testing.T, url, method string, msg types.RuleMsg, ctx types.RuleContext) types.Node {
	node, _ := engine.Registry.NewNode("restApiCall")
	var configuration = make(types.Configuration)
	configuration["restEndpointUrlPattern"] = url
	configuration["requestMethod"] = method
	config := types.NewConfig()

	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	//发送消息
	node.OnMsg(ctx, msg)
	return node
}
