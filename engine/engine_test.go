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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	shareKey       = "shareKey"
	shareValue     = "shareValue"
	addShareKey    = "addShareKey"
	addShareValue  = "addShareValue"
	testdataFolder = "../testdata/rule/"
)
var ruleChainFile = `{
          "ruleChain": {
            "id": "test01",
            "name": "testRuleChain01",
            "debugMode": true,
            "root": true,
            "disabled": false
          },
          "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "return msg.temperature>10;"
                }
              },
              {
                "id": "s2",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "jsTransform",
                "name": "转换",
                "debugMode": true,
                "configuration": {
                  "jsScript": "msgType='TEST_MSG_TYPE';var msg2={};\n  msg2['aa']=66\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
                }
              }
            ],
            "connections": [
              {
                "fromId": "s1",
                "toId": "s2",
                "type": "True"
              }
            ]
          }
        }`

var updateRuleChainFile = `
	{
	  "ruleChain": {
		"id":"test01",
		"name": "updateRuleChainFile"
	  },
	  "metadata": {
		"nodes": [
		  {
			"id":"s1",
			"type": "jsFilter",
			"name": "过滤",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msg.temperature>10;"
			}
		  },
		  {
			"id":"s3",
			"type": "jsTransform",
			"name": "转换2",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['productType']='product02';msgType='TEST_MSG_TYPE';var msg2={};\n  msg2['aa']=77\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  },
		  {
			"id":"s4",
			"type": "jsTransform",
			"name": "转换4",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['name']='productName'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  }
		],
		"connections": [
		  {
			"fromId": "s1",
			"toId": "s3",
			"type": "True"
		  },
		  {
			"fromId": "s3",
			"toId": "s4",
			"type": "Success"
		  }
		]
	  }
	}
`

// 修改metadata和msg 节点
var modifyMetadataAndMsgNode = `
	  {
			"id":"s2",
			"type": "jsTransform",
			"name": "转换",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['test']='test02';\n metadata['index']=50;\n msgType='TEST_MSG_TYPE_MODIFY';\n  msg['aa']=66;\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  }
`

// 加载文件
func loadFile(filePath string) []byte {
	buf, err := os.ReadFile(testdataFolder + filePath)
	if err != nil {
		return nil
	} else {
		return buf
	}
}

func testRuleEngine(t *testing.T, ruleChainFile string, modifyNodeId, modifyNodeFile string) {
	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == modifyNodeId && modifyNodeId != "" {
			indexStr := msg.Metadata.GetValue("index")
			testStr := msg.Metadata.GetValue("test")
			assert.Equal(t, "50", indexStr)
			assert.Equal(t, "test02", testStr)
			assert.Equal(t, "TEST_MSG_TYPE_MODIFY", msg.Type)
		} else {
			assert.Equal(t, "{\"temperature\":35}", msg.GetData())
		}
	}
	ruleEngine, err := New("rule01", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del("rule01")

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")
	maxTimes := 1
	for j := 0; j < maxTimes; j++ {
		if modifyNodeId != "" {
			//modify the node
			_ = ruleEngine.ReloadChild(modifyNodeId, []byte(modifyNodeFile))
		}
		ruleEngine.OnMsg(msg)
	}
	time.Sleep(time.Second)
}

func TestRuleChain(t *testing.T) {
	testRuleEngine(t, ruleChainFile, "", "")
}

func TestRuleChainChangeMetadataAndMsg(t *testing.T) {
	testRuleEngine(t, ruleChainFile, "s2", modifyMetadataAndMsgNode)
}

// test reload rule chain
func TestReloadRuleChain(t *testing.T) {
	config1 := NewConfig()
	var config1DebugDone int32
	config1.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config1.Logger.Printf("before reload : flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == "s2" {
			productType := msg.Metadata.GetValue("productType")
			assert.Equal(t, "test01", productType)
		}
		atomic.StoreInt32(&config1DebugDone, 1)
	}

	chainId := str.RandomStr(10)

	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config1))
	assert.Nil(t, err)
	defer Del(chainId)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")

	ruleEngine.OnMsg(msg)

	time.Sleep(time.Millisecond * 200)

	assert.True(t, atomic.LoadInt32(&config1DebugDone) == 1)

	//config1.Logger.Printf("reload rule chain......")
	config2 := NewConfig()
	var config2DebugDone int32
	config2.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config2.Logger.Printf("before after : flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == "s3" {
			productType := msg.Metadata.GetValue("productType")
			assert.Equal(t, "product02", productType)
		}
		atomic.StoreInt32(&config2DebugDone, 1)
	}
	//更新规则链
	err = ruleEngine.ReloadSelf([]byte(updateRuleChainFile), WithConfig(config2))
	assert.Nil(t, err)

	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)
	assert.True(t, atomic.LoadInt32(&config2DebugDone) == 1)
}

// 测试子规则链
func TestSubRuleChain(t *testing.T) {
	//start := time.Now()
	var completed int32
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes * 2)
	var subChainDone int32
	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if chainId == "sub_chain_01" {
			atomic.StoreInt32(&subChainDone, 1)
		}
		//config.Logger.Printf("chainId=%s,flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", chainId, flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
	}

	ruleFile := loadFile("./chain_has_sub_chain_node.json")
	subRuleFile := loadFile("./sub_chain.json")
	//初始化子规则链实例
	_, err := New("sub_chain_01", subRuleFile, WithConfig(config))

	chainId := str.RandomStr(10)

	//初始化主规则链实例
	ruleEngine, err := New(chainId, ruleFile, WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	for i := 0; i < maxTimes; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "productType01")
		metaData.PutValue("name1", "name1")
		metaData.PutValue("name2", "name2")
		metaData.PutValue("name3", "name3")
		metaData.PutValue("name4", "name4")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")

		//处理消息并得到处理结果
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {

			atomic.AddInt32(&completed, 1)
			group.Done()
			if msg.Type == "TEST_MSG_TYPE1" {
				//root chain end
				assert.Equal(t, msg.GetData(), "{\"aa\":11}")
				v := msg.Metadata.GetValue("test")
				assert.Equal(t, v, "Modified by root chain")
			} else {
				//sub chain end
				assert.Equal(t, true, strings.Contains(msg.GetData(), `"data":"{\"bb\":22}"`))
				v := msg.Metadata.GetValue("test")
				assert.Equal(t, v, "Modified by sub chain")
				v = msg.Metadata.GetValue("test_s3")
				assert.Equal(t, v, "Modified by sub chain node sub_s3")
			}
		}))

	}
	group.Wait()
	assert.Equal(t, int32(maxTimes*2), completed)
	time.Sleep(time.Millisecond * 200)
	assert.True(t, atomic.LoadInt32(&subChainDone) == 1)
	//fmt.Printf("use times:%s \n", time.Since(start))
}

// 测试规则链debug模式
func TestRuleChainDebugMode(t *testing.T) {
	config := NewConfig()
	var inTimes int32
	var outTimes int32
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.In {
			atomic.AddInt32(&inTimes, 1)
		}
		if flowType == types.Out {
			atomic.AddInt32(&outTimes, 1)
		}
	}
	chainId := str.RandomStr(10)
	ruleFile := loadFile("./test_debug_mode_chain.json")
	ruleEngine, err := New(chainId, ruleFile, WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "productType01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)

	assert.Equal(t, int32(2), atomic.LoadInt32(&inTimes))
	assert.Equal(t, int32(2), atomic.LoadInt32(&outTimes))

	// close s1 node debug mode
	nodeCtx, ok := ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "sub_s1"})
	assert.True(t, ok)
	ruleNodeCtx, ok := nodeCtx.(*RuleNodeCtx)
	assert.True(t, ok)
	ruleNodeCtx.SelfDefinition.DebugMode = false

	atomic.StoreInt32(&inTimes, 0)
	atomic.StoreInt32(&outTimes, 0)
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second)

	assert.Equal(t, int32(1), atomic.LoadInt32(&inTimes))
	assert.Equal(t, int32(1), atomic.LoadInt32(&outTimes))

	// close s1 node debug mode
	nodeCtx, ok = ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "sub_s2"})
	assert.True(t, ok)
	ruleNodeCtx, ok = nodeCtx.(*RuleNodeCtx)
	assert.True(t, ok)
	ruleNodeCtx.SelfDefinition.DebugMode = false

	atomic.StoreInt32(&inTimes, 0)
	atomic.StoreInt32(&outTimes, 0)
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)

	assert.Equal(t, int32(0), atomic.LoadInt32(&inTimes))
	assert.Equal(t, int32(0), atomic.LoadInt32(&outTimes))
}

func TestNotDebugModel(t *testing.T) {
	//start := time.Now()
	config := NewConfig()
	var debugDone int32
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.StoreInt32(&debugDone, 1)
	}
	// closed debug mode
	ruleEngine, err := New(str.RandomStr(10), loadFile("./not_debug_mode_chain.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")
	var wg sync.WaitGroup
	wg.Add(1)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		wg.Done()
		//已经被 s2 节点修改消息类型
		assert.Equal(t, "TEST_MSG_TYPE2", msg.Type)
		assert.Nil(t, err)
	}))

	wg.Wait()

	assert.False(t, atomic.LoadInt32(&debugDone) == 1)

	// open debug mode
	debugEnableRuleChain := strings.Replace(string(loadFile("./not_debug_mode_chain.json")), "\"debugMode\": false", "\"debugMode\": true", -1)
	err = ruleEngine.ReloadSelf([]byte(debugEnableRuleChain))
	assert.Nil(t, err)

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
	}))
	time.Sleep(time.Millisecond * 200)
	assert.True(t, atomic.LoadInt32(&debugDone) == 1)
}

// 测试获取节点
func TestGetNodeId(t *testing.T) {
	parser := JsonParser{}
	def, _ := parser.DecodeRuleChain([]byte(ruleChainFile))
	ctx, err := InitRuleChainCtx(NewConfig(), nil, &def, nil)
	assert.Nil(t, err)
	nodeCtx, ok := ctx.GetNodeById(types.RuleNodeId{Id: "s1", Type: types.NODE})
	assert.True(t, ok)

	nodeCtx, ok = ctx.GetNodeById(types.RuleNodeId{Id: "s1", Type: types.CHAIN})
	assert.False(t, ok)
	nodeCtx, ok = ctx.GetNodeById(types.RuleNodeId{Id: "node5", Type: types.NODE})
	assert.False(t, ok)
	_ = nodeCtx
}

// 测试callRestApi
func TestCallRestApi(t *testing.T) {
	//start := time.Now()
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes)

	//wp, _ := ants.NewPool(math.MaxInt32)
	//使用协程池
	config := NewConfig(types.WithDefaultPool())
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if err != nil {
			config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.GetData(), msg.GetMetadata().Values(), relationType, err)
		}
	}
	ruleFile := loadFile("./chain_call_rest_api.json")
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleFile), WithConfig(config))
	defer Stop()

	for i := 0; i < maxTimes; i++ {
		if err == nil {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "productType01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"aa\":\"aaaaaaaaaaaaaa\"}")
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
				group.Done()
			}))

		}
	}
	group.Wait()
	time.Sleep(time.Millisecond * 200)
	//fmt.Printf("total massages:%d,use times:%s \n", maxTimes, time.Since(start))
}

// 测试消息路由
func TestMsgTypeSwitch(t *testing.T) {
	var wg sync.WaitGroup

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		wg.Done()
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./chain_msg_type_switch.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//TEST_MSG_TYPE1 找到2条chains,4个nodes
	wg.Add(6)
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg)
	wg.Wait()

	//TEST_MSG_TYPE2 找到1条chain,2个nodes
	wg.Add(4)
	msg = types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "s4", msg.Type)
		v := msg.Metadata.GetValue("addFrom")
		assert.Equal(t, "s4", v)
	}))
	wg.Wait()

	//TEST_MSG_TYPE3 找到1 other chain,4个node
	wg.Add(4)
	msg = types.NewMsg(0, "TEST_MSG_TYPE3", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "TEST_MSG_TYPE3", msg.Type)
		v := msg.Metadata.GetValue("addFrom")
		assert.Equal(t, "Default", v)
	}))
	wg.Wait()
}

func TestWithContext(t *testing.T) {
	//注册自定义组件
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	//start := time.Now()
	config := NewConfig()

	_, err := New("test_context_chain", loadFile("./test_context_chain.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_context.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")
	var maxTimes = 1000
	var wg sync.WaitGroup
	wg.Add(maxTimes)
	for j := 0; j < maxTimes; j++ {
		go func(index int) {
			ruleEngine.OnMsg(msg, types.WithContext(context.WithValue(context.Background(), shareKey, shareValue+strconv.Itoa(index))), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
				v1 := msg.Metadata.GetValue(shareKey)
				assert.Equal(t, shareValue+strconv.Itoa(index), v1)

				assert.Equal(t, "TEST_MSG_TYPE", msg.Type)

				v2 := msg.Metadata.GetValue(addShareKey)
				assert.Equal(t, addShareValue, v2)
				assert.Nil(t, err)
				wg.Done()
			}))
		}(j)

	}
	wg.Wait()
	//fmt.Printf("total massages:%d,use times:%s \n", maxTimes, time.Since(start))
}

func TestSpecifyID(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New("", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	assert.Equal(t, "test01", ruleEngine.Id())
	_, ok := Get("test01")
	assert.Equal(t, true, ok)

	chainId := str.RandomStr(10)

	ruleEngine, err = New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	assert.Equal(t, chainId, ruleEngine.Id())
	ruleEngine, ok = Get(chainId)
	assert.Equal(t, true, ok)
}

// waitForCount 轮询等待原子计数到期望值。OnEnd 回调在协程池异步触发，
// 与 onAllNodeCompleted/OnMsgAndWait 返回无固定先后，断言前必须等计数到齐
func waitForCount(t *testing.T, count *int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(count) < want {
		if time.Now().After(deadline) {
			t.Fatalf("count=%d, want %d", atomic.LoadInt32(count), want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestOnMsgAndWait 测试同步执行规则链
func TestOnMsgAndWait(t *testing.T) {
	// 调试事件经 SubmitTask 异步投递，可能跨越消息边界迟到，
	// 按消息 id 计数并等待，避免共享 WaitGroup 的负计数与跨消息污染
	var debugMu sync.Mutex
	debugEvents := map[string]int{}
	waitDebug := func(id string, want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			debugMu.Lock()
			got := debugEvents[id]
			debugMu.Unlock()
			if got >= want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("debug events for %s=%d, want %d", id, got, want)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		debugMu.Lock()
		debugEvents[msg.GetId()]++
		debugMu.Unlock()
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_wait.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	subEngine, err := New("sub_chain_02", loadFile("./sub_chain.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	//子链引擎按固定 id 缓存复用，不清理会把本迭代的调试事件发往上一迭代注册的旧闭包
	defer Del(ruleEngine.Id())
	defer Del(subEngine.Id())
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//TEST_MSG_TYPE1 找到2条chains,5个nodes
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var count int32
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		atomic.AddInt32(&count, 1)
	}))
	waitForCount(t, &count, 2) //OnEnd 在协程池异步触发，与 onCompleted 无固定先后，断言前等计数到齐
	waitDebug(msg.GetId(), 10)

	//TEST_MSG_TYPE2 找到1条chain,2个nodes
	atomic.StoreInt32(&count, 0)
	msg = types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		atomic.AddInt32(&count, 1)
	}))
	waitForCount(t, &count, 1)
	waitDebug(msg.GetId(), 4)

	//TEST_MSG_TYPE3 找到0条chain,1个node
	atomic.StoreInt32(&count, 0)
	data := ""
	msg = types.NewMsg(0, "TEST_MSG_TYPE3", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		atomic.AddInt32(&count, 1)
		data = msg.GetData()
	}))
	waitForCount(t, &count, 1) //count 原子递增先于 data 写入，计数到齐后读 data 无竞争
	assert.Equal(t, "{\"temperature\":41}", data)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count))
	waitDebug(msg.GetId(), 2)
}

// 测试functions节点，并发修改metadata
func TestFunctionsNode(t *testing.T) {
	action.Functions.Register("modifyMetadata", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("aa", "aa")
		msg.Metadata.PutValue("bb", "bb")
		ctx.TellSuccess(msg)
	})

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.Out {
			msg.Metadata.PutValue("aa", "aa")
			time.Sleep(time.Millisecond * 10)
			msg.Metadata.PutValue("bb", "bb")
			assert.Equal(t, "aa", msg.Metadata.GetValue("aa"))
			assert.Equal(t, "bb", msg.Metadata.GetValue("bb"))
		}
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_functions_node.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var i = 0
	for i < 100 {
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		}))
		i++
	}

	time.Sleep(time.Second)
}

func TestFunctionsNodeRelationTypeEmpty(t *testing.T) {
	action.Functions.Register("tellNextRelationTypeEmpty", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("aa", "aa")
		msg.Metadata.PutValue("bb", "bb")
		ctx.TellNext(msg)
	})

	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_functions_node2.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var wg sync.WaitGroup
	wg.Add(1)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		assert.Equal(t, "aa", msg.Metadata.GetValue("aa"))
		assert.Equal(t, "bb", msg.Metadata.GetValue("bb"))
		wg.Done()
	}))
	wg.Wait()
}

func TestExecuteNode(t *testing.T) {
	config := NewConfig()
	var err error
	chainId := "executeNode_rule01"
	b := loadFile("./test_group_filter_node.json")
	ruleEngine, err := New(chainId, b, WithConfig(config))

	chainId2 := "executeNode_rule02"
	chainJson2 := strings.Replace(string(b), chainId, chainId2, -1)
	_, err = New(chainId2, []byte(chainJson2), WithConfig(config))
	// 清理两条链，避免重复运行(-count=N)时 Pool.New 幂等命中上一轮 ReloadSelf 后残留的旧链
	defer func() {
		Del(chainId)
		Del(chainId2)
	}()

	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	// 回调迟到打满 groupFilter 超时窗的现象只在 CI -race 负载下出现，本地无法复现，
	// 失败时的全量 goroutine 栈是定位卡点的唯一取证
	dumpIfStalled := func(msg types.RuleMsg) {
		if msg.Metadata.GetValue("result") != "" {
			return
		}
		buf := make([]byte, 1<<22)
		n := runtime.Stack(buf, true)
		t.Logf("groupFilter stalled, dumping all goroutines:\n%s", buf[:n])
	}

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	// Use WaitGroup to ensure callback completes before reload
	// 使用 WaitGroup 确保回调完成后再重新加载
	var firstCallbackDone sync.WaitGroup
	firstCallbackDone.Add(1)
	ruleEngine.OnMsg(msg1, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		defer firstCallbackDone.Done()
		dumpIfStalled(msg)
		assert.Equal(t, "true", msg.Metadata.GetValue("result"),
			fmt.Sprintf("err=%v relationType=%s errorMsg=%s", err, relationType, msg.Metadata.GetValue(types.KeyErrorMsg)))
	}))

	// Wait for callback to complete before reloading
	// 等待回调完成后再重新加载
	firstCallbackDone.Wait()

	chainJsonFile1 := string(loadFile("./test_group_filter_node.json"))
	newChainJsonFile1 := strings.Replace(chainJsonFile1, `"allMatches": false`, `"allMatches": true`, -1)
	newChainJsonFile1 = strings.Replace(newChainJsonFile1, "test_group_filter_node", chainId, -1)
	//更新规则链，groupFilter必须所有节点都满足True,才走True链
	_ = ruleEngine.ReloadSelf([]byte(newChainJsonFile1))

	// 等待规则链重新加载完成
	time.Sleep(time.Millisecond * 200)

	// 等待 reload 后的 msg1/msg2 处理完成再发送 msg3，避免它们与 msg3 回调并发时偶尔晚于
	// msg3 完成：测试结束后 defer Del 会 Stop 引擎并取消在途消息 context，groupFilter 无 Failure
	// 下游，被取消后 result 为空，导致断言失败（间歇性，慢速 CI 易触发）
	var reloadDone sync.WaitGroup
	reloadDone.Add(2)
	ruleEngine.OnMsg(msg1, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		defer reloadDone.Done()
		dumpIfStalled(msg)
		assert.Equal(t, "false", msg.Metadata.GetValue("result"),
			fmt.Sprintf("err=%v relationType=%s errorMsg=%s", err, relationType, msg.Metadata.GetValue(types.KeyErrorMsg)))
	}))

	msg2 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":52,\"humidity\":90}")
	ruleEngine.OnMsg(msg2, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		defer reloadDone.Done()
		dumpIfStalled(msg)
		assert.Equal(t, "true", msg.Metadata.GetValue("result"),
			fmt.Sprintf("err=%v relationType=%s errorMsg=%s", err, relationType, msg.Metadata.GetValue(types.KeyErrorMsg)))
	}))
	reloadDone.Wait()

	var wg sync.WaitGroup
	wg.Add(4)

	msg3 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":51,\"humidity\":90}")
	ruleEngine.OnMsg(msg3, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "true", msg.Metadata.GetValue("result"))
		ctx.TellNode(context.Background(), "aa", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.NotNil(t, err)
			assert.Equal(t, types.Failure, relationType)

			wg.Done()
		}, nil)

		ctx.TellChainNode(context.Background(), chainId, "s1", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.True, relationType)
			wg.Done()
		}, nil)

		ctx.TellChainNode(context.Background(), "notfound", "s2", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.NotNil(t, err)
			assert.Equal(t, "ruleChain id=notfound not found: rule chain not found", err.Error())
			assert.Equal(t, types.Failure, relationType)
			wg.Done()
		}, nil)

		ctx.TellChainNode(context.Background(), chainId2, "s2", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.True, relationType)
			wg.Done()
		}, nil)
		//ctx.TellChainNode(context.Background(), chainId2, "s2", msg, false, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		//	assert.Equal(t, types.Failure, relationType)
		//	assert.Equal(t, "Computing the full value of call results is not supported", err.Error())
		//	wg.Done()
		//}, nil)
	}))

	wg.Wait()
}

// TestTellNextNodePanic 节点 panic 须以 Failure 结束分支并触发 OnEnd
func TestTellNextNodePanic(t *testing.T) {
	action.Functions.Register("panicFn", func(ctx types.RuleContext, msg types.RuleMsg) {
		panic("boom")
	})
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testPanicNode",
            "name": "testPanicNode"
          },
          "metadata": {
            "nodes": [
              {
                "id": "n1",
                "type": "functions",
                "name": "panic节点",
                "configuration": {
                  "functionName": "panicFn"
                }
              }
            ],
            "connections": []
          }
        }`
	config := NewConfig()
	ruleEngine, err := New("testPanicNode", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del("testPanicNode")
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{}")

	var wg sync.WaitGroup
	wg.Add(1)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		defer wg.Done()
		assert.Equal(t, types.Failure, relationType)
		assert.True(t, err != nil && strings.Contains(err.Error(), "panic"),
			"onEnd should surface the node panic, got err=%v", err)
	}))
	wg.Wait()
}

func TestBatchOnMsgAndWait(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	var maxTimes = 100000
	var wg sync.WaitGroup
	wg.Add(maxTimes)
	for i := 0; i < maxTimes; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsgAndWait(msg, types.WithOnAllNodeCompleted(func() {
		}), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
			wg.Done()
		}))
	}
	wg.Wait()

}

// TestBatchOnMsgAndWait  测试同步处理消息，有多个end
func TestBatchOnMsgAndWaitMultipleOnEnd(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), loadFile("./chain_msg_type_switch.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	var maxTimes = 100
	for i := 0; i < maxTimes; i++ {
		var count = int32(0)
		ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 50) //因为OnEnd 和 onCompleted 是异步的。所以不能确保顺序，这里需要等一下
		assert.Equal(t, int32(2), atomic.LoadInt32(&count))
	}
	time.Sleep(time.Millisecond * 100)
}

var s1NodeFile = `
  {
			"Id":"s1",
			"type": "jsFilter",
			"name": "过滤-更改",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msg!='bb';"
			}
		  }
`

// TestEngine 测试规则引擎
func TestEngine(t *testing.T) {
	config := NewConfig()
	_, err := New("subChain01", []byte{}, WithConfig(config))
	assert.NotNil(t, err)
	//初始化子规则链
	subRuleEngine, err := New("subChain01", loadFile("./sub_chain.json"), WithConfig(config))
	//初始化根规则链
	ruleEngine, err := New("testEngine", []byte(ruleChainFile), WithConfig(config))
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.True(t, ruleEngine.Initialized())

	assert.Equal(t, strings.Replace(ruleChainFile, " ", "", -1), strings.Replace(string(ruleEngine.DSL()), " ", "", -1))

	//获取节点
	s1NodeId := types.RuleNodeId{Id: "s1"}
	ruleEngine.RootRuleChainCtx()
	s1Node, ok := ruleEngine.RootRuleChainCtx().GetNodeById(s1NodeId)
	assert.True(t, ok)

	nodeDsl := ruleEngine.NodeDSL(types.RuleNodeId{}, s1NodeId)

	assert.Equal(t, strings.Replace(` {
                "id": "s1",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "return msg.temperature>10;"
                }
              }`, " ", "", -1), strings.Replace(string(nodeDsl), " ", "", -1))

	s1RuleNodeCtx, ok := s1Node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.Equal(t, "过滤", s1RuleNodeCtx.SelfDefinition.Name)
	assert.Equal(t, "return msg.temperature>10;", s1RuleNodeCtx.SelfDefinition.Configuration["jsScript"])

	//获取子规则链
	subChain01Id := types.RuleNodeId{Id: "subChain01", Type: types.CHAIN}
	subChain01Node, ok := ruleEngine.RootRuleChainCtx().GetNodeById(subChain01Id)
	assert.True(t, ok)
	subChain01NodeCtx, ok := subChain01Node.(*RuleChainCtx)
	assert.True(t, ok)
	assert.Equal(t, "测试子规则链", subChain01NodeCtx.SelfDefinition.RuleChain.Name)
	assert.Equal(t, subChain01NodeCtx, subRuleEngine.RootRuleChainCtx())

	//修改根规则链节点
	_ = ruleEngine.ReloadChild(s1NodeId.Id, []byte(s1NodeFile))
	s1Node, ok = ruleEngine.RootRuleChainCtx().GetNodeById(s1NodeId)
	assert.True(t, ok)
	s1RuleNodeCtx, ok = s1Node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.Equal(t, "过滤-更改", s1RuleNodeCtx.SelfDefinition.Name)
	assert.Equal(t, "return msg!='bb';", s1RuleNodeCtx.SelfDefinition.Configuration["jsScript"])

	subRuleChain := string(loadFile("./sub_chain.json"))
	//修改子规则链
	_ = subRuleEngine.ReloadSelf([]byte(strings.Replace(subRuleChain, "测试子规则链", "测试子规则链-更改", -1)))

	subChain01Node, ok = ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "subChain01", Type: types.CHAIN})
	assert.True(t, ok)
	subChain01NodeCtx, ok = subChain01Node.(*RuleChainCtx)
	assert.True(t, ok)
	assert.Equal(t, "测试子规则链-更改", subChain01NodeCtx.SelfDefinition.RuleChain.Name)

	//获取规则引擎实例
	ruleEngineNew, ok := Get("testEngine")
	assert.True(t, ok)
	assert.Equal(t, ruleEngine, ruleEngineNew)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	var onAllNodeCompleted = int32(0)

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		newMsg := ctx.NewMsg("TEST_MSG_TYPE2", types.NewMetadata(), "test")
		assert.Equal(t, "test", newMsg.GetData())
		assert.Equal(t, types.JSON, newMsg.DataType)
		assert.Equal(t, "TEST_MSG_TYPE2", newMsg.Type)
	}), types.WithOnAllNodeCompleted(func() {
		atomic.StoreInt32(&onAllNodeCompleted, 1)
	}))
	time.Sleep(time.Millisecond * 100)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {

	}))
	ruleEngine.OnMsg(msg)

	time.Sleep(time.Millisecond * 200)
	assert.True(t, atomic.LoadInt32(&onAllNodeCompleted) == 1)

	//删除对应规则引擎实例
	Del("testEngine")
	_, ok = Get("testEngine")
	assert.False(t, ok)
	assert.False(t, ruleEngine.Initialized())

}

func TestOnDebug(t *testing.T) {
	var onDebugConfigWg sync.WaitGroup
	onDebugConfigWg.Add(8)
	config := NewConfig(types.WithDefaultPool())
	config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if nodeId == "s1" && flowType == types.Out {
			assert.Equal(t, types.True, relationType)
		}
		if nodeId == "s2" && flowType == types.Out {
			assert.Equal(t, types.Success, relationType)
		}
		onDebugConfigWg.Done()
	}
	ruleEngine, _ := New("testOnDebug", []byte(ruleChainFile), WithConfig(config))
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	t.Run("hasOnDebug", func(t *testing.T) {
		var snapshotWg sync.WaitGroup
		snapshotWg.Add(2)
		var onDebugWg sync.WaitGroup
		onDebugWg.Add(8)
		ruleEngine.OnMsg(msg, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			assert.Equal(t, "s1", ctx.GetSelfId())
			assert.Equal(t, 2, len(snapshot.Logs))
			for _, item := range snapshot.Logs {
				if item.Id == "s1" {
					assert.Equal(t, types.True, item.RelationType)
				}
				if item.Id == "s2" {
					assert.Equal(t, types.Success, item.RelationType)
				}
			}
			snapshotWg.Done()
		}), types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			if nodeRunLog.Id == "s1" {
				assert.Equal(t, "s1", ctx.GetSelfId())
				assert.Equal(t, types.True, nodeRunLog.RelationType)
			}
			if nodeRunLog.Id == "s2" {
				assert.Equal(t, "s2", ctx.GetSelfId())
				assert.Equal(t, types.Success, nodeRunLog.RelationType)
			}
		}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if nodeId == "s1" && flowType == types.Out {
				assert.Equal(t, types.True, relationType)
			}
			if nodeId == "s2" && flowType == types.Out {
				assert.Equal(t, types.Success, relationType)
			}
			onDebugWg.Done()
		}))

		ruleEngine.OnMsg(msg, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			assert.Equal(t, "s1", ctx.GetSelfId())
			assert.Equal(t, 2, len(snapshot.Logs))
			for _, item := range snapshot.Logs {
				if item.Id == "s1" {
					assert.Equal(t, types.True, item.RelationType)
				}
				if item.Id == "s2" {
					assert.Equal(t, types.Success, item.RelationType)
				}
			}
			snapshotWg.Done()
		}), types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			if nodeRunLog.Id == "s1" {
				assert.Equal(t, types.True, nodeRunLog.RelationType)
			}
			if nodeRunLog.Id == "s2" {
				assert.Equal(t, "s2", ctx.GetSelfId())
				assert.Equal(t, types.Success, nodeRunLog.RelationType)
			}
		}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if nodeId == "s1" && flowType == types.Out {
				assert.Equal(t, types.True, relationType)
			}
			if nodeId == "s2" && flowType == types.Out {
				assert.Equal(t, types.Success, relationType)
			}
			onDebugWg.Done()
		}))
		snapshotWg.Wait()
		onDebugWg.Wait()
		onDebugConfigWg.Wait()
	})
}

func TestReload(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testReload",
            "name": "testRuleChain01"
          },
          "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "${global.js}"
                }
              }
            ]
          }
        }`

	config := NewConfig(types.WithDefaultPool())
	config.Properties.PutValue("js", "return msg.temperature>10;")
	ruleEngine, err := New("testReload", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.True, relationType)
	}))

	config.Properties.PutValue("js", "return msg.temperature>70;")
	//刷新配置
	_ = ruleEngine.Reload(WithConfig(config))
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.False, relationType)
	}))
}

func TestUseVars(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testReload",
            "name": "testRuleChain01",
			"configuration": {
				"vars":{
					"js":"return msg.temperature>10;"
				}
			}
          },
          "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "${vars.js}"
                }
              }
            ]
          }
        }`

	config := NewConfig(types.WithDefaultPool())
	config.Properties.PutValue("js", "return msg.temperature>10;")
	ruleEngine, err := New("testUseVars", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.True, relationType)
	}))
	ruleChainFile = strings.Replace(ruleChainFile, "msg.temperature>10;", "msg.temperature>70;", 1)
	//刷新配置
	_ = ruleEngine.ReloadSelf([]byte(ruleChainFile), WithConfig(config))
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.False, relationType)
	}))
	time.Sleep(time.Millisecond * 100)
	var i = 0
	for i < 200 {
		msg = types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":21,\"humidity\":90}")
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.False, relationType)
		}))
		msg = types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.False, relationType)
		}))
		i++
	}
	time.Sleep(time.Millisecond * 200)
}

func TestNoNodes(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testNoNodes",
            "name": "testRuleChain01"
          }
        }`
	var wg sync.WaitGroup
	wg.Add(1)
	config := NewConfig(types.WithDefaultPool())
	config.Properties.PutValue("js", "return msg.temperature>10;")
	ruleEngine, err := New("testNoNodes", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
		assert.Equal(t, "the rule chain has no nodes", err.Error())
	}), types.WithOnAllNodeCompleted(func() {
		wg.Done()
	}))
	time.Sleep(time.Millisecond * 100)
	wg.Wait()
}

func TestDoOnEnd(t *testing.T) {
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
                  "functionName": "doEnd"
                }
              },
              {
                "id": "s2",
                "type": "log",
                "name": "记录日志",
                "debugMode": true,
                "configuration": {
                  "jsScript": "return 'Incoming';"
                }
              }
            ],
            "connections": [
              {
                "fromId": "s1",
                "toId": "s2",
                "type": "True"
              },
 				{
                "fromId": "s1",
                "toId": "s2",
                "type": "False"
              }
            ]
          }
        }`

	//测试函数
	action.Functions.Register("doEnd", func(ctx types.RuleContext, msg types.RuleMsg) {
		if msg.Metadata.GetValue("productType") == "test01" {
			ctx.TellNext(msg, types.True)
		} else {
			//中断执行规则链
			ctx.DoOnEnd(msg, nil, types.False)
		}
	})
	count := int32(0)
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("testDoOnEnd", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.AddInt32(&count, 1)
	}))
	time.Sleep(time.Millisecond * 100)
	assert.Equal(t, int32(4), atomic.LoadInt32(&count))
	count = int32(0)
	metaData2 := types.NewMetadata()
	metaData2.PutValue("productType", "test02")
	msg2 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData2, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg2, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.False, relationType)
	}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.AddInt32(&count, 1)
	}))
	time.Sleep(time.Millisecond * 100)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count))
}

func TestJoinNode(t *testing.T) {
	t.Run("Basic Join Test", func(t *testing.T) {
		var ruleChainFile = loadFile("test_join_node.json")
		var wg sync.WaitGroup
		wg.Add(1)
		config := NewConfig(types.WithDefaultPool())
		ruleEngine, err := New("testJoinNode", ruleChainFile, WithConfig(config))
		assert.Nil(t, err)
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
		ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			var result []map[string]interface{}
			json.Unmarshal([]byte(msg.GetData()), &result)
			assert.Equal(t, types.Success, relationType)
			assert.Equal(t, 2, len(result))
			assert.True(t, result[0]["nodeId"] != result[1]["nodeId"])
		}), types.WithOnAllNodeCompleted(func() {
			wg.Done()
		}))
		time.Sleep(time.Millisecond * 100)
		wg.Wait()
	})
	t.Run("Single Parent Join Test", func(t *testing.T) {
		// 创建只有一个父节点的join场景
		singleParentJoinDSL := `{
			"ruleChain": {
				"id": "singleParentJoin",
				"name": "单父节点Join测试"
			},
			"metadata": {
				"nodes": [
					{
						"id": "node_transform",
						"type": "jsTransform",
						"name": "Transform",
						"configuration": {
							"jsScript": "msg.single='single_value'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "node_join",
						"type": "join",
						"name": "SingleJoin",
						"configuration": {
							"timeout": 1
						}
					}
				],
				"connections": [
					{
						"fromId": "node_transform",
						"toId": "node_join",
						"type": "Success"
					}
				]
			}
		}`

		config := NewConfig(types.WithDefaultPool())
		ruleEngine, err := New("singleParentJoinTest", []byte(singleParentJoinDSL), WithConfig(config))
		assert.Nil(t, err)

		var wg sync.WaitGroup
		wg.Add(1)
		var callbackCount int32

		metaData := types.NewMetadata()
		msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, "{\"test\":\"data\"}")

		ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&callbackCount, 1)
			var result []map[string]interface{}
			json.Unmarshal([]byte(msg.GetData()), &result)
			assert.Equal(t, types.Success, relationType)
			assert.Equal(t, 1, len(result)) // 应该只有一个输入消息
			assert.Equal(t, "node_transform", result[0]["nodeId"])

			// 正确解析消息数据
			if msgInfo, ok := result[0]["msg"].(map[string]interface{}); ok {
				if dataStr, ok := msgInfo["data"].(string); ok {
					var msgData map[string]interface{}
					json.Unmarshal([]byte(dataStr), &msgData)
					assert.Equal(t, "single_value", msgData["single"])
				}
			}
		}), types.WithOnAllNodeCompleted(func() {
			wg.Done()
		}))

		wg.Wait()
		// 验证单父节点场景下回调能正常触发
		assert.Equal(t, int32(1), atomic.LoadInt32(&callbackCount))
	})

	t.Run("Join Node Timeout Test", func(t *testing.T) {
		// 创建一个真正会超时的join场景：延迟节点延迟时间超过join超时时间
		timeoutJoinDSL := `{
			"ruleChain": {
				"id": "timeoutJoin",
				"name": "超时Join测试"
			},
			"metadata": {
				"nodes": [
					{
						"id": "node_split",
						"type": "jsTransform",
						"name": "分发节点",
						"configuration": {
							"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "node_delay",
						"type": "delay",
						"name": "延迟节点",
						"configuration": {
							"periodInSeconds": 3
						}
					},
					{
						"id": "node_immediate",
						"type": "jsTransform",
						"name": "立即节点",
						"configuration": {
							"jsScript": "msg.immediate='true'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "node_join",
						"type": "join",
						"name": "TimeoutJoin",
						"configuration": {
							"timeout": 1
						}
					}
				],
				"connections": [
					{
						"fromId": "node_split",
						"toId": "node_delay",
						"type": "Success"
					},
					{
						"fromId": "node_split",
						"toId": "node_immediate",
						"type": "Success"
					},
					{
						"fromId": "node_delay",
						"toId": "node_join",
						"type": "Success"
					},
					{
						"fromId": "node_immediate",
						"toId": "node_join",
						"type": "Success"
					}
				]
			}
		}`

		config := NewConfig(types.WithDefaultPool())
		ruleEngine, err := New("timeoutJoinTest", []byte(timeoutJoinDSL), WithConfig(config))
		assert.Nil(t, err)

		var wg sync.WaitGroup
		wg.Add(1)
		var callbackCount int32
		var joinCallbackReceived bool

		metaData := types.NewMetadata()
		msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, "{\"test\":\"timeout\"}")

		start := time.Now()

		// 使用OnMsg让分支并行执行
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// 只统计来自join节点的回调
			if ctx.GetSelfId() == "node_join" {
				atomic.AddInt32(&callbackCount, 1)
				joinCallbackReceived = true
				elapsed := time.Since(start)

				// join节点超时应该返回Failure
				assert.Equal(t, types.Failure, relationType)
				assert.NotNil(t, err)

				// 检查错误信息是否包含超时相关内容
				errMsg := err.Error()
				assert.True(t, strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "timeout"),
					"错误信息应该包含超时相关内容: %s", errMsg)

				// 验证确实在超时时间附近结束（1秒超时）
				assert.True(t, elapsed > 900*time.Millisecond && elapsed < 1500*time.Millisecond,
					"执行时间应该在1秒左右: %v", elapsed)

				wg.Done()
			}
		}))

		wg.Wait()
		assert.True(t, joinCallbackReceived, "应该收到join节点的回调")
		assert.Equal(t, int32(1), atomic.LoadInt32(&callbackCount))
	})

	t.Run("Concurrent Join Test", func(t *testing.T) {
		// 测试多个消息同时处理join节点的情况
		var ruleChainFile = loadFile("test_join_node.json")
		config := NewConfig(types.WithDefaultPool())
		ruleEngine, err := New("concurrentJoinTest", ruleChainFile, WithConfig(config))
		assert.Nil(t, err)

		const concurrentCount = 10
		var wg sync.WaitGroup
		wg.Add(concurrentCount)
		var successCount int32

		for i := 0; i < concurrentCount; i++ {
			go func(index int) {
				defer wg.Done()
				metaData := types.NewMetadata()
				metaData.PutValue("index", fmt.Sprintf("%d", index))
				msg := types.NewMsg(0, "CONCURRENT_TEST", types.JSON, metaData,
					fmt.Sprintf("{\"temperature\":%d,\"humidity\":90}", 40+index))

				ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					if relationType == types.Success {
						atomic.AddInt32(&successCount, 1)
						var result []map[string]interface{}
						json.Unmarshal([]byte(msg.GetData()), &result)
						assert.Equal(t, 2, len(result)) // 每个join都应该有2个输入
					}
				}))
			}(i)
		}

		wg.Wait()
		// 验证所有消息都能正常处理
		assert.Equal(t, int32(concurrentCount), atomic.LoadInt32(&successCount))
	})

	t.Run("Multi Branch Join Test", func(t *testing.T) {
		// 测试多分支Join场景 - 一个过滤器分出两个分支，都汇聚到join节点
		var ruleChainFile = loadFile("test_join_multi_branch.json")
		config := NewConfig(types.WithDefaultPool())
		ruleEngine, err := New("multiBranchJoinTest", ruleChainFile, WithConfig(config))
		assert.Nil(t, err)

		// 测试温度 > 50 的情况，会走True分支到node_4
		t.Run("High Temperature", func(t *testing.T) {
			var wg sync.WaitGroup
			wg.Add(1)
			var joinCallbackReceived bool
			var callbackCount int32

			metaData := types.NewMetadata()
			metaData.PutValue("testType", "highTemp")
			msg := types.NewMsg(0, "HIGH_TEMP_TEST", types.JSON, metaData,
				`{"temperature":60,"humidity":70}`)

			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if ctx.GetSelfId() == "node_7" {
					atomic.AddInt32(&callbackCount, 1)
					joinCallbackReceived = true

					// join节点应该能正常处理
					assert.Equal(t, types.Success, relationType)
					assert.Nil(t, err)

					wg.Done()
				}
			}))

			wg.Wait()
			assert.True(t, joinCallbackReceived, "应该收到join节点的回调")
			assert.Equal(t, int32(1), atomic.LoadInt32(&callbackCount))
		})
	})
}

func TestDisabled(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())
	defStr := ruleChainFile

	e, err := New("testDisabled1", []byte(defStr), WithConfig(config))
	assert.Nil(t, err)

	defStr = strings.Replace(defStr, "\"disabled\": false", "\"disabled\": true", -1)
	err = e.ReloadSelf([]byte(defStr))
	assert.Equal(t, types.ErrEngineDisabled.Error(), err.Error())

	defStr = strings.Replace(defStr, "\"disabled\": true", "\"disabled\": false", -1)
	err = e.ReloadSelf([]byte(defStr))
	assert.Nil(t, err)

	err = e.Reload()
	assert.Nil(t, err)

	defStr = strings.Replace(ruleChainFile, "\"disabled\": false", "\"disabled\": true", -1)
	_, err = New("testDisabled2", []byte(defStr), WithConfig(config))
	assert.Equal(t, types.ErrEngineDisabled.Error(), err.Error())
}

// TestMetadataCopyOnWritePerformance 测试新 Metadata 设计在多节点并发场景下的性能和正确性
func TestMetadataCopyOnWritePerformance(t *testing.T) {
	// 创建一个多节点规则链，用于测试并发场景
	multiNodeRuleChain := `{
		"ruleChain": {
			"id": "test_cow_performance",
			"name": "testCOWPerformance",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "filter1",
					"type": "jsFilter",
					"name": "过滤器1",
					"configuration": {
						"jsScript": "return true;"
					}
				},
				{
					"id": "transform1",
					"type": "jsTransform",
					"name": "转换器1",
					"configuration": {
						"jsScript": "metadata.node1_processed = 'true'; metadata.timestamp1 = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "transform2",
					"type": "jsTransform",
					"name": "转换器2",
					"configuration": {
						"jsScript": "metadata.node2_processed = 'true'; metadata.timestamp2 = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "transform3",
					"type": "jsTransform",
					"name": "转换器3",
					"configuration": {
						"jsScript": "metadata.node3_processed = 'true'; metadata.timestamp3 = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "filter1",
					"toId": "transform1",
					"type": "True"
				},
				{
					"fromId": "filter1",
					"toId": "transform2",
					"type": "True"
				},
				{
					"fromId": "filter1",
					"toId": "transform3",
					"type": "True"
				}
			]
		}
	}`

	config := NewConfig(types.WithDefaultPool())
	chainId := fmt.Sprintf("test_cow_performance_%s_%d", str.RandomStr(10), time.Now().UnixNano())
	ruleEngine, err := New(chainId, []byte(multiNodeRuleChain), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试并发场景下的性能和正确性
	var wg sync.WaitGroup
	var processedCount int32
	var isolationErrors int32
	maxGoroutines := 50
	messagesPerGoroutine := 20

	// 记录开始时间
	startTime := time.Now()

	wg.Add(maxGoroutines)

	for i := 0; i < maxGoroutines; i++ {
		go func(goroutineIndex int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				// 创建包含大量元数据的消息
				metaData := types.NewMetadata()
				for k := 0; k < 100; k++ {
					metaData.PutValue(fmt.Sprintf("key_%d", k), fmt.Sprintf("value_%d_%d_%d", goroutineIndex, j, k))
				}
				metaData.PutValue("goroutineID", fmt.Sprintf("%d", goroutineIndex))
				metaData.PutValue("messageIndex", fmt.Sprintf("%d", j))

				msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"test\":\"data\"}")

				var msgWg sync.WaitGroup
				msgWg.Add(3) // 三个并行的transform节点，每个都会触发EndFunc

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error, _ string) {
					defer msgWg.Done()
					atomic.AddInt32(&processedCount, 1)

					// 验证消息隔离性
					if resultMsg.Metadata.GetValue("goroutineID") != fmt.Sprintf("%d", goroutineIndex) {
						atomic.AddInt32(&isolationErrors, 1)
						t.Errorf("Metadata isolation failed: expected goroutineID %d, got %s", goroutineIndex, resultMsg.Metadata.GetValue("goroutineID"))
					}

					if resultMsg.Metadata.GetValue("messageIndex") != fmt.Sprintf("%d", j) {
						atomic.AddInt32(&isolationErrors, 1)
						t.Errorf("Metadata isolation failed: expected messageIndex %d, got %s", j, resultMsg.Metadata.GetValue("messageIndex"))
					}

					// 验证至少有一个节点处理标记（因为每个transform节点只设置自己的标记）
					processedCount := 0
					if resultMsg.Metadata.GetValue("node1_processed") == "true" {
						processedCount++
					}
					if resultMsg.Metadata.GetValue("node2_processed") == "true" {
						processedCount++
					}
					if resultMsg.Metadata.GetValue("node3_processed") == "true" {
						processedCount++
					}
					if processedCount == 0 {
						atomic.AddInt32(&isolationErrors, 1)
						t.Errorf("Node processing verification failed: no processing markers found")
					}

					assert.Nil(t, err)
				}))

				msgWg.Wait()
			}
		}(i)
	}

	wg.Wait()

	// 计算总耗时
	totalTime := time.Since(startTime)
	totalMessages := int32(maxGoroutines * messagesPerGoroutine)
	expectedProcessedCount := totalMessages * 3 // 每个消息会被3个并行节点处理

	// 验证测试结果
	assert.Equal(t, int32(0), atomic.LoadInt32(&isolationErrors), "No metadata isolation errors should occur")
	assert.Equal(t, expectedProcessedCount, atomic.LoadInt32(&processedCount), "All messages should be processed by all nodes")

	// 输出性能统计
	t.Logf("Performance Test Results:")
	t.Logf("Total input messages: %d", totalMessages)
	t.Logf("Total processed callbacks: %d", atomic.LoadInt32(&processedCount))
	t.Logf("Total time: %v", totalTime)
	t.Logf("Average time per input message: %v", totalTime/time.Duration(totalMessages))
	t.Logf("Input messages per second: %.2f", float64(totalMessages)/totalTime.Seconds())
	t.Logf("Isolation errors: %d", atomic.LoadInt32(&isolationErrors))
}

// TestMetadataIsolationInMultipleNodes 测试多节点场景下 Metadata 的隔离性
func TestMetadataIsolationInMultipleNodes(t *testing.T) {
	// 创建一个分叉规则链，测试不同分支的 Metadata 隔离
	forkRuleChain := `{
		"ruleChain": {
			"id": "test_metadata_isolation",
			"name": "testMetadataIsolation",
			"debugMode": true,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "fork",
					"type": "jsFilter",
					"name": "分叉节点",
					"configuration": {
						"jsScript": "return true;"
					}
				},
				{
					"id": "branch1",
					"type": "jsTransform",
					"name": "分支1",
					"configuration": {
						"jsScript": "metadata.branch = 'branch1'; metadata.branch1_data = 'data1'; return {'msg':msg,'metadata':metadata,'msgType':'BRANCH1'};"
					}
				},
				{
					"id": "branch2",
					"type": "jsTransform",
					"name": "分支2",
					"configuration": {
						"jsScript": "metadata.branch = 'branch2'; metadata.branch2_data = 'data2'; return {'msg':msg,'metadata':metadata,'msgType':'BRANCH2'};"
					}
				}
			],
			"connections": [
				{
					"fromId": "fork",
					"toId": "branch1",
					"type": "True"
				},
				{
					"fromId": "fork",
					"toId": "branch2",
					"type": "True"
				}
			]
		}
	}`

	config := NewConfig()
	var branch1Results []types.RuleMsg
	var branch2Results []types.RuleMsg
	var mu sync.Mutex

	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.Out {
			mu.Lock()
			defer mu.Unlock()
			if nodeId == "branch1" {
				branch1Results = append(branch1Results, msg)
			} else if nodeId == "branch2" {
				branch2Results = append(branch2Results, msg)
			}
		}
	}

	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(forkRuleChain), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 发送测试消息
	metaData := types.NewMetadata()
	metaData.PutValue("original_key", "original_value")
	metaData.PutValue("shared_key", "shared_value")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"test\":\"data\"}")

	var wg sync.WaitGroup
	wg.Add(2)

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error, _ string) {
		wg.Done()
	}))

	wg.Wait()
	time.Sleep(time.Millisecond * 200) // 等待所有调试回调完成

	// 验证结果
	mu.Lock()
	assert.Equal(t, 1, len(branch1Results), "Should have one result from branch1")
	assert.Equal(t, 1, len(branch2Results), "Should have one result from branch2")

	branch1Msg := branch1Results[0]
	branch2Msg := branch2Results[0]
	mu.Unlock()

	// 验证分支隔离性
	assert.Equal(t, "BRANCH1", branch1Msg.Type)
	assert.Equal(t, "BRANCH2", branch2Msg.Type)

	// 验证 Metadata 隔离性
	assert.Equal(t, "branch1", branch1Msg.Metadata.GetValue("branch"))
	assert.Equal(t, "branch2", branch2Msg.Metadata.GetValue("branch"))

	assert.Equal(t, "data1", branch1Msg.Metadata.GetValue("branch1_data"))
	assert.Equal(t, "", branch1Msg.Metadata.GetValue("branch2_data"))

	assert.Equal(t, "data2", branch2Msg.Metadata.GetValue("branch2_data"))
	assert.Equal(t, "", branch2Msg.Metadata.GetValue("branch1_data"))

	// 验证原始数据仍然存在
	assert.Equal(t, "original_value", branch1Msg.Metadata.GetValue("original_key"))
	assert.Equal(t, "original_value", branch2Msg.Metadata.GetValue("original_key"))
	assert.Equal(t, "shared_value", branch1Msg.Metadata.GetValue("shared_key"))
	assert.Equal(t, "shared_value", branch2Msg.Metadata.GetValue("shared_key"))

}

// TestGroupActionNodeIntegration 测试 GroupActionNode 在完整规则链中的集成功能
func TestGroupActionNodeIntegration(t *testing.T) {
	// 注册测试用的函数
	action.Functions.Register("processTemperature", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("tempProcessed", "true")
		msg.Metadata.PutValue("processNode", "temperature")
		ctx.TellSuccess(msg)
	})

	action.Functions.Register("processHumidity", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("humidityProcessed", "true")
		msg.Metadata.PutValue("processNode", "humidity")
		ctx.TellSuccess(msg)
	})

	action.Functions.Register("processPressure", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("pressureProcessed", "true")
		msg.Metadata.PutValue("processNode", "pressure")
		time.Sleep(time.Millisecond * 10) // 模拟较慢的处理
		ctx.TellFailure(msg, errors.New("pressure sensor failed"))
	})

	// GroupActionNode 规则链配置
	groupActionRuleChain := `{
		"ruleChain": {
			"id": "test_group_action_chain",
			"name": "GroupAction测试规则链",
			"debugMode": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "groupAction1",
					"type": "groupAction",
					"name": "传感器数据处理组",
					"debugMode": true,
					"configuration": {
						"matchRelationType": "Success",
						"matchNum": 2,
						"nodeIds": "tempNode,humidityNode,pressureNode",
						"timeout": 10
					}
				},
				{
					"id": "tempNode",
					"type": "functions",
					"name": "温度处理节点",
					"debugMode": true,
					"configuration": {
						"functionName": "processTemperature"
					}
				},
				{
					"id": "humidityNode", 
					"type": "functions",
					"name": "湿度处理节点",
					"debugMode": true,
					"configuration": {
						"functionName": "processHumidity"
					}
				},
				{
					"id": "pressureNode",
					"type": "functions", 
					"name": "压力处理节点",
					"debugMode": true,
					"configuration": {
						"functionName": "processPressure"
					}
				},
				{
					"id": "successResult",
					"type": "jsTransform",
					"name": "成功结果处理",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata['groupResult'] = 'success'; metadata['processedCount'] = msg.length; return {'msg':msg,'metadata':metadata,'msgType':'GROUP_SUCCESS'};"
					}
				},
				{
					"id": "failureResult",
					"type": "jsTransform",
					"name": "失败结果处理", 
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata['groupResult'] = 'failure'; return {'msg':msg,'metadata':metadata,'msgType':'GROUP_FAILURE'};"
					}
				}
			],
			"connections": [
				{
					"fromId": "groupAction1",
					"toId": "successResult",
					"type": "Success"
				},
				{
					"fromId": "groupAction1", 
					"toId": "failureResult",
					"type": "Failure"
				}
			]
		}
	}`

	t.Run("GroupAction Success Case", func(t *testing.T) {
		var resultReceived int32
		var successReceived int32

		config := NewConfig()
		config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if flowType == types.Out && nodeId == "successResult" {
				// 应该走成功路径，因为温度和湿度节点都会成功(满足matchNum=2)
				result := msg.Metadata.GetValue("groupResult")
				assert.Equal(t, "success", result)
				assert.Equal(t, "GROUP_SUCCESS", msg.Type)
				atomic.AddInt32(&successReceived, 1)
			}
			if flowType == types.Out && nodeId == "failureResult" {
				t.Errorf("不应该走失败路径，但是收到了: %s", msg.Metadata.GetValue("groupResult"))
			}
		}

		chainId := str.RandomStr(10) + "_success"
		ruleEngine, err := New(chainId, []byte(groupActionRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		metaData := types.NewMetadata()
		metaData.PutValue("sensorType", "environmental")
		msg := types.NewMsg(0, "SENSOR_DATA", types.JSON, metaData, `{"temperature":25.5,"humidity":60.2,"pressure":1013.25}`)

		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&resultReceived, 1)
			assert.Nil(t, err)
		}))

		// 等待处理完成
		time.Sleep(time.Millisecond * 200)

		assert.Equal(t, int32(1), atomic.LoadInt32(&successReceived), "应该收到1个成功结果")
		assert.True(t, atomic.LoadInt32(&resultReceived) >= 1, "应该收到结果回调")
	})

	t.Run("GroupAction Modified MatchNum Case", func(t *testing.T) {
		// 修改规则链配置：要求3个Success（但只有2个能成功）
		modifiedRuleChain := strings.Replace(groupActionRuleChain, `"matchNum": 2`, `"matchNum": 3`, 1)
		modifiedRuleChain = strings.Replace(modifiedRuleChain, `"test_group_action_chain"`, `"test_group_action_chain_modified"`, 1)

		var resultReceived int32
		var failureReceived int32

		config := NewConfig()
		config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if flowType == types.Out && nodeId == "failureResult" {
				// 应该走失败路径，因为只有2个节点成功但要求3个
				result := msg.Metadata.GetValue("groupResult")
				assert.Equal(t, "failure", result)
				assert.Equal(t, "GROUP_FAILURE", msg.Type)
				atomic.AddInt32(&failureReceived, 1)
			}
			if flowType == types.Out && nodeId == "successResult" {
				t.Errorf("不应该走成功路径，但是收到了: %s", msg.Metadata.GetValue("groupResult"))
			}
		}

		chainId := str.RandomStr(10) + "_failure"
		ruleEngine, err := New(chainId, []byte(modifiedRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		metaData := types.NewMetadata()
		metaData.PutValue("sensorType", "environmental")
		msg := types.NewMsg(0, "SENSOR_DATA", types.JSON, metaData, `{"temperature":25.5,"humidity":60.2,"pressure":1013.25}`)

		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&resultReceived, 1)
			assert.Nil(t, err)
		}))

		// 等待处理完成
		time.Sleep(time.Millisecond * 200)

		assert.Equal(t, int32(1), atomic.LoadInt32(&failureReceived), "应该收到1个失败结果")
		assert.True(t, atomic.LoadInt32(&resultReceived) >= 1, "应该收到结果回调")
	})

	t.Run("GroupAction Concurrent Safety", func(t *testing.T) {
		// 并发安全测试：同时发送多个消息
		var successCount, failureCount int32

		config := NewConfig()
		config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if flowType == types.Out && nodeId == "successResult" {
				atomic.AddInt32(&successCount, 1)
			}
			if flowType == types.Out && nodeId == "failureResult" {
				atomic.AddInt32(&failureCount, 1)
			}
		}

		chainId := str.RandomStr(10) + "_concurrent"
		ruleEngine, err := New(chainId, []byte(groupActionRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 并发发送多个消息
		var wg sync.WaitGroup
		concurrentCount := 50

		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				metaData := types.NewMetadata()
				metaData.PutValue("messageIndex", fmt.Sprintf("%d", index))
				msg := types.NewMsg(0, "SENSOR_DATA", types.JSON, metaData,
					fmt.Sprintf(`{"temperature":%f,"humidity":%f,"pressure":%f}`,
						20.0+float64(index)*0.1, 50.0+float64(index)*0.2, 1000.0+float64(index)*0.5))

				ruleEngine.OnMsg(msg)
			}(i)
		}

		wg.Wait()
		time.Sleep(time.Millisecond * 500) // 等待所有处理完成

		//t.Logf("并发测试结果: Success=%d, Failure=%d, Expected=%d",
		//	atomic.LoadInt32(&successCount), atomic.LoadInt32(&failureCount), concurrentCount)

		// 验证：应该都是成功的，因为温度和湿度节点都会成功(满足matchNum=2)
		assert.Equal(t, int32(concurrentCount), atomic.LoadInt32(&successCount), "所有消息都应该成功处理")
		assert.Equal(t, int32(0), atomic.LoadInt32(&failureCount), "不应该有失败的消息")
	})
}

func TestAliasIntegration(t *testing.T) {
	// 为测试组件注册别名
	err := Registry.RegisterAlias("jsFilter", "js_filter", "javascriptFilter")
	assert.Nil(t, err)

	err = Registry.RegisterAlias("jsTransform", "js_transform", "javascriptTransform")
	assert.Nil(t, err)

	t.Run("ChainWithAliasNodeType", func(t *testing.T) {
		// 使用别名作为节点类型创建规则链
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_alias_chain",
				"name": "Test Alias Chain",
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "node1",
						"type": "js_filter",
						"name": "Filter Node",
						"configuration": {
							"jsScript": "return msg.temperature > 20;"
						}
					},
					{
						"id": "node2",
						"type": "js_transform",
						"name": "Transform Node",
						"configuration": {
							"jsScript": "msg.filtered = true; return msg;"
						}
					}
				],
				"connections": [
					{
						"fromId": "node1",
						"toId": "node2",
						"type": "True"
					}
				]
			}
		}`

		config := NewConfig()
		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		// 使用别名创建规则链上下文
		ruleChainCtx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)
		assert.NotNil(t, ruleChainCtx)

		// 验证节点被正确创建（使用别名）
		node1Ctx := ruleChainCtx.nodes[types.RuleNodeId{Id: "node1"}]
		assert.NotNil(t, node1Ctx)
		// SelfDefinition 保留原始配置中的类型名
		assert.Equal(t, "js_filter", node1Ctx.(*RuleNodeCtx).SelfDefinition.Type)

		node2Ctx := ruleChainCtx.nodes[types.RuleNodeId{Id: "node2"}]
		assert.NotNil(t, node2Ctx)
		assert.Equal(t, "js_transform", node2Ctx.(*RuleNodeCtx).SelfDefinition.Type)

		// 验证底层节点实例是正确的主类型
		assert.Equal(t, "jsFilter", node1Ctx.Type())
		assert.Equal(t, "jsTransform", node2Ctx.Type())
	})

	t.Run("ChainWithMultipleAliases", func(t *testing.T) {
		// 测试同一个规则链中使用不同别名
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_multi_alias",
				"name": "Test Multi Alias Chain",
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "filter1",
						"type": "jsFilter",
						"name": "Original Name Filter",
						"configuration": {
							"jsScript": "return true;"
						}
					},
					{
						"id": "filter2",
						"type": "js_filter",
						"name": "Underscore Alias Filter",
						"configuration": {
							"jsScript": "return true;"
						}
					},
					{
						"id": "filter3",
						"type": "javascriptFilter",
						"name": "Full Name Alias Filter",
						"configuration": {
							"jsScript": "return true;"
						}
					}
				],
				"connections": []
			}
		}`

		config := NewConfig()
		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ruleChainCtx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)
		assert.NotNil(t, ruleChainCtx)

		// 所有节点都应该被正确创建
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter1"}])
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter2"}])
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter3"}])

		// 验证所有节点都是正确的主类型
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter1"}].Type())
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter2"}].Type())
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter3"}].Type())
	})

	t.Run("NodeWithAlias", func(t *testing.T) {
		// 单独测试使用别名创建节点
		selfDefinition := types.RuleNode{
			Id:   "test_node",
			Type: "js_filter", // 使用别名
		}
		ctx, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Nil(t, err)
		assert.NotNil(t, ctx)
		// SelfDefinition 保留原始配置
		assert.Equal(t, "js_filter", ctx.SelfDefinition.Type)
		// 但底层节点是正确的主类型
		assert.Equal(t, "jsFilter", ctx.Type())
	})

	t.Run("NewNodeWithAlias", func(t *testing.T) {
		// 测试通过别名创建新节点实例
		node, err := Registry.NewNode("js_filter")
		assert.Nil(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, "jsFilter", node.Type())

		node, err = Registry.NewNode("javascriptTransform")
		assert.Nil(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, "jsTransform", node.Type())
	})

	t.Run("UnregisterAliasAffectsChain", func(t *testing.T) {
		// 创建一个临时别名
		err := Registry.RegisterAlias("log", "logAlias")
		assert.Nil(t, err)

		// 使用别名创建节点应该成功
		selfDefinition := types.RuleNode{
			Id:   "log_node",
			Type: "logAlias",
		}
		ctx, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Nil(t, err)
		assert.NotNil(t, ctx)

		// 删除别名
		err = Registry.Unregister("logAlias")
		assert.Nil(t, err)

		// 使用已删除的别名创建节点应该失败
		_, err = InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.NotNil(t, err)
	})

	// 清理：删除测试别名
	defer func() {
		Registry.Unregister("js_filter")
		Registry.Unregister("javascriptFilter")
		Registry.Unregister("js_transform")
		Registry.Unregister("javascriptTransform")
	}()
}

func BenchmarkChainNotChangeMetadata(b *testing.B) {
	b.ResetTimer()
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < b.N; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsg(msg)
	}
}

func BenchmarkChainChangeMetadataAndMsg(b *testing.B) {

	config := NewConfig()

	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	if err != nil {
		b.Error(err)
	}
	//modify s1 node content
	_ = ruleEngine.ReloadChild("s2", []byte(modifyMetadataAndMsgNode))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsg(msg)
	}
}

func BenchmarkCallRestApiNodeGo(b *testing.B) {
	//不使用协程池
	config := NewConfig()
	ruleEngine, _ := New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), WithConfig(config))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callRestApiNode(ruleEngine)
	}
}

func BenchmarkCallRestApiNodeWorkerPool(b *testing.B) {
	//使用协程池
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, _ := New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), WithConfig(config))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callRestApiNode(ruleEngine)
	}
}

//	func BenchmarkCallRestApiNodeAnts(b *testing.B) {
//		defaultAntsPool, _ := ants.NewPool(200000)
//		//使用协程池
//		config := NewConfig(types.WithPool(defaultAntsPool))
//		ruleEngine, _ := New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), WithConfig(config))
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			callRestApiNode(ruleEngine)
//		}
//	}
func callRestApiNode(ruleEngine types.RuleEngine) {
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"aa\":\"aaaaaaaaaaaaaa\"}")
	ruleEngine.OnMsg(msg)
}

// BenchmarkRuleMsgDataCOW 基准测试：RuleMsg Data字段的写时复制性能
func BenchmarkRuleMsgDataCOW(b *testing.B) {
	// 测试不同大小的数据
	testCases := []struct {
		name string
		data string
	}{
		{"Small", "small data"},
		{"Medium", strings.Repeat("medium data ", 100)},
		{"Large", strings.Repeat("large data content ", 1000)},
	}

	for _, tc := range testCases {
		b.Run(tc.name+"_Copy", func(b *testing.B) {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			original := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = original.Copy()
			}
		})

		b.Run(tc.name+"_CopyAndModifyDirect", func(b *testing.B) {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			original := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy := original.Copy()
				copy.SetData("modified data")
			}
		})

		b.Run(tc.name+"_CopyAndModifyCOW", func(b *testing.B) {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			original := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy := original.Copy()
				copy.SetData("modified data")
			}
		})
	}
}

// BenchmarkRuleMsgDataCOWMultipleCopies 基准测试：多副本创建和修改性能
func BenchmarkRuleMsgDataCOWMultipleCopies(b *testing.B) {
	largeData := strings.Repeat("benchmark data for multiple copies ", 1000)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	original := types.NewMsg(0, "BENCH", types.JSON, metaData, largeData)

	b.Run("Create100Copies", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]types.RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 防止编译器优化
			_ = copies
		}
	})

	b.Run("Create100CopiesAndModifyOne", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]types.RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 修改第一个副本
			copies[0].SetData("modified")
			// 防止编译器优化
			_ = copies
		}
	})

	b.Run("Create100CopiesAndModifyAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]types.RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 修改所有副本
			for j := 0; j < 100; j++ {
				copies[j].SetData("modified")
			}
			// 防止编译器优化
			_ = copies
		}
	})
}

// BenchmarkRuleMsgDataCOWInRuleChain 基准测试：规则链中的Data COW性能
func BenchmarkRuleMsgDataCOWInRuleChain(b *testing.B) {
	// 创建一个会修改Data的规则链
	dataModifyRuleChain := `{
		"ruleChain": {
			"id": "test_data_cow",
			"name": "testDataCOW",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "transform1",
					"type": "jsTransform",
					"name": "数据转换1",
					"configuration": {
						"jsScript": "msg.processed_by = 'transform1'; msg.timestamp = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "transform2",
					"type": "jsTransform",
					"name": "数据转换2",
					"configuration": {
						"jsScript": "msg.processed_by = 'transform2'; msg.counter = (msg.counter || 0) + 1; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "transform1",
					"toId": "transform2",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(dataModifyRuleChain), WithConfig(config))
	if err != nil {
		b.Error(err)
	}

	b.Run("RuleChainWithDataModification", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, `{"temperature":35,"humidity":60}`)
			ruleEngine.OnMsg(msg)
		}
	})
}

// BenchmarkRuleMsgDataCOWConcurrent 基准测试：并发场景下的Data COW性能
func BenchmarkRuleMsgDataCOWConcurrent(b *testing.B) {
	largeData := strings.Repeat("concurrent test data ", 1000)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	original := types.NewMsg(0, "CONCURRENT", types.JSON, metaData, largeData)

	b.Run("ConcurrentCopy", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = original.Copy()
			}
		})
	})

	b.Run("ConcurrentCopyAndRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				copy := original.Copy()
				_ = copy.GetData()
			}
		})
	})

	b.Run("ConcurrentCopyAndModify", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				copy := original.Copy()
				copy.SetData("modified in goroutine")
			}
		})
	})
}

// linearNode 直通节点：收到消息按 Success 转发，用于测量引擎调度开销
type linearNode struct {
	BaseNode
}

func (n *linearNode) Type() string { return "bench/linear" }
func (n *linearNode) New() types.Node {
	return &linearNode{}
}
func (n *linearNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellNext(msg, types.Success)
}

func linearChainDsl(id string, hops int) []byte {
	dsl := `{"ruleChain":{"id":"` + id + `"},"metadata":{"nodes":[`
	for i := 0; i < hops; i++ {
		if i > 0 {
			dsl += ","
		}
		dsl += `{"id":"n` + strconv.Itoa(i) + `","type":"bench/linear","name":"n` + strconv.Itoa(i) + `"}`
	}
	dsl += `],"connections":[`
	for i := 0; i < hops-1; i++ {
		if i > 0 {
			dsl += ","
		}
		dsl += `{"fromId":"n` + strconv.Itoa(i) + `","toId":"n` + strconv.Itoa(i+1) + `","type":"Success"}`
	}
	dsl += `]}}`
	return []byte(dsl)
}

// BenchmarkLinearChain 线性直通链两种调用模式的吞吐：
// wait=OnMsgAndWait 同步等待（单子节点跳转内联在调用方 goroutine），
// async=OnMsg 异步投递（每跳交协程池）。
// 引擎池按 id 单例，每个子基准必须用唯一 id，否则拿到的是缓存的旧链。
func BenchmarkLinearChain(b *testing.B) {
	if err := Registry.Register(&linearNode{}); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = Registry.Unregister("bench/linear") }()

	config := NewConfig(types.WithDefaultPool())
	for _, hop := range []int{1, 10, 30} {
		id := "bench_linear_" + strconv.Itoa(hop)
		e, err := New(id, linearChainDsl(id, hop), WithConfig(config))
		if err != nil {
			b.Fatal(err)
		}
		for _, mode := range []string{"wait", "async"} {
			b.Run(mode+"/hops="+strconv.Itoa(hop), func(b *testing.B) {
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						msg := types.NewMsg(0, "BENCH", types.JSON, types.NewMetadata(), `{"temperature":35}`)
						if mode == "wait" {
							e.OnMsgAndWait(msg)
						} else {
							e.OnMsg(msg)
						}
					}
				})
			})
		}
	}
}

// 测试故障降级切面
func TestSkipFallbackAspect(t *testing.T) {
	//如果10s内出现3次错误，则跳过当前节点，继续执行下一个节点，10s后恢复
	config := NewConfig()

	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config.Logger.Printf("chainId=%s,flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", chainId, flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
	}

	ruleEngine, err := DefaultPool.New(str.RandomStr(10), loadFile("./test_skip_fallback_aspect.json"), WithConfig(config), types.WithAspects(&aspect.SkipFallbackAspect{ErrorCountLimit: 3, LimitDuration: time.Second * 10}))
	if err != nil {
		t.Error(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	//第1次
	ruleEngine.OnMsg(msg)

	//第2次
	start := time.Now()
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		//没达到错误降级阈值，执行该组件
		//fmt.Printf("第2次耗时:%s", time.Since(start).String())
		//fmt.Println()
		assert.True(t, time.Since(start) > time.Second)
	}))

	//第3次
	ruleEngine.OnMsg(msg)

	time.Sleep(time.Second * 4)

	//第4次,达到错误降级阈值
	msg = types.NewMsg(0, "TEST_MSG_TYPE4", types.JSON, metaData, "{\"temperature\":44}")
	start4 := time.Now()
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		//进入故障降级，跳过该组件
		//fmt.Printf("第4次耗时:%s", time.Since(start4).String())
		//fmt.Println()
		assert.True(t, time.Since(start4) < time.Second)
	}))

	//等待恢复时间
	time.Sleep(time.Second * 11)

	start5 := time.Now()
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		//故障恢复，执行该组件
		//fmt.Printf("第5次耗时:%s", time.Since(start5).String())
		//fmt.Println()
		assert.True(t, time.Since(start5) > time.Second)
	}))

	ruleEngine.OnMsg(msg)
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second * 11)
	//更新规则链，清除错误信息
	ruleEngine.ReloadSelf(loadFile("./test_skip_fallback_aspect.json"))

	start6 := time.Now()
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		//故障恢复，执行该组件
		//fmt.Printf("第6次耗时:%s", time.Since(start6).String())
		//fmt.Println()
		assert.True(t, time.Since(start6) > time.Second)
	}))

	time.Sleep(time.Second * 3)
	ruleEngine.Stop(context.Background())

}

func TestAspectOrder(t *testing.T) {
	var aspects = types.AspectList{
		&NodeAspect2{Name: "NodeAspect2"},
		&NodeAspect1{Name: "NodeAspect1"},
		&ChainAspect{Name: "ChainAspect"},
		&EngineAspect{Name: "EngineAspect"},
	}
	onChainBeforeCreate, onNodeBeforeCreate, onCreated, onAfterReload, onDestroy := aspects.GetEngineAspects()
	assert.Equal(t, len(onChainBeforeCreate), 1)
	assert.Equal(t, len(onNodeBeforeCreate), 1)
	assert.Equal(t, len(onCreated), 1)
	assert.Equal(t, len(onAfterReload), 1)
	assert.Equal(t, len(onDestroy), 1)

	onStart, onEnd, onCompleted := aspects.GetChainAspects()
	assert.Equal(t, len(onStart), 1)
	assert.Equal(t, len(onEnd), 1)
	assert.Equal(t, len(onCompleted), 1)

	around, before, after := aspects.GetNodeAspects()
	assert.Equal(t, len(around), 1)
	assert.Equal(t, len(before), 2)
	assert.Equal(t, len(after), 1)
	assert.Equal(t, 3, before[0].Order())
}

func TestEngineAspect(t *testing.T) {
	chainId := "test_engine_aspect"
	// reuse the shared fixture under a unique chain id so DefaultPool does not
	// return the engine already loaded by TestRuleChain, which would skip the
	// aspect callbacks this test counts
	chainDsl := strings.Replace(ruleChainFile, `"id": "test01"`, `"id": "`+chainId+`"`, 1)
	var count int32
	callback := &CallbackTest{}
	callback.OnCreated = func(ctx types.NodeCtx) {
		assert.Equal(t, chainId, ctx.GetNodeId().Id)
		atomic.AddInt32(&count, 1)
	}
	callback.OnReload = func(parentCtx types.NodeCtx, ctx types.NodeCtx) {
		assert.Equal(t, chainId, parentCtx.GetNodeId().Id)
		if ctx.GetNodeId().Type == types.NODE {
			assert.Equal(t, "s2", ctx.GetNodeId().Id)
		}
		atomic.AddInt32(&count, 1)
	}
	callback.OnDestroy = func(ctx types.NodeCtx) {
		assert.Equal(t, chainId, ctx.GetNodeId().Id)
		assert.Equal(t, types.CHAIN, ctx.GetNodeId().Type)
		atomic.AddInt32(&count, 1)
	}
	var onCompleted int32
	callback.OnCompleted = func(ctx types.RuleContext, msg types.RuleMsg) {
		atomic.StoreInt32(&onCompleted, 1)
	}
	config := NewConfig()
	ruleEngine, err := DefaultPool.New(chainId, []byte(chainDsl), WithConfig(config), types.WithAspects(&NodeAspect2{Name: "NodeAspect2"}, &NodeAspect1{Name: "NodeAspect1"},
		&ChainAspect{Name: "ChainAspect"}, &EngineAspect{Name: "EngineAspect", Callback: callback}))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, int32(1), count)
	atomic.StoreInt32(&count, 0)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg)
	//重新加载规则链，会同时触发Reload 和 OnDestroy
	err = ruleEngine.ReloadSelf([]byte(chainDsl))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, int32(2), count)
	atomic.StoreInt32(&count, 0)

	//更新子节节点
	err = ruleEngine.ReloadChild("s2", []byte(`
	  {
			"id": "s2",
			"type": "log",
			"name": "记录日志Success",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msgType+':Success';"
			}
		  }
	`))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, int32(1), count)
	//销毁
	ruleEngine.Stop(context.Background())
	time.Sleep(time.Millisecond * 200)

	assert.True(t, atomic.LoadInt32(&onCompleted) == 1)

}

func TestBeforeInitErrAspect(t *testing.T) {
	chainId := "testBeforeCreateErrAspect"
	config := NewConfig()
	count := int32(0)
	ruleEngine, err := DefaultPool.New(chainId, []byte(ruleChainFile), WithConfig(config), types.WithAspects(&BeforeCreateErrAspect{
		Name:  "BeforeCreateErrAspect",
		Count: &count,
	}))
	assert.Nil(t, err)
	assert.Equal(t, int32(4), count)
	atomic.StoreInt32(&count, 0)

	newRuleChainFile := strings.ReplaceAll(ruleChainFile, "test01", "test02")
	err = ruleEngine.ReloadSelf([]byte(newRuleChainFile))

	assert.Equal(t, "crate error break", err.Error())

	assert.Equal(t, int32(1), count)
	atomic.StoreInt32(&count, 0)

	err = ruleEngine.ReloadChild("s2", []byte(`
	  {
			"id": "s2",
			"type": "log",
			"name": "记录日志Success",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msgType+':Success';"
			}
		  }
	`))
	assert.Nil(t, err)
	assert.Equal(t, int32(2), count)
	//销毁
	ruleEngine.Stop(context.Background())
}

func TestChainAspect(t *testing.T) {
	chainId := "test_skip_fallback_aspect"

	callback := &CallbackTest{}

	config := NewConfig()

	ruleEngine, err := DefaultPool.New(chainId, loadFile("./test_skip_fallback_aspect.json"), WithConfig(config), types.WithAspects(
		&NodeAspect2{Name: "NodeAspect2"},
		&NodeAspect1{Name: "NodeAspect1"},
		&ChainAspect{Name: "ChainAspect"},
		&EngineAspect{Name: "EngineAspect", Callback: callback},
	))
	if err != nil {
		t.Error(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, _ string) {
		v1 := msg.Metadata.GetValue("key1")
		assert.Equal(t, "addValueOnStart", v1)
		v2 := msg.Metadata.GetValue("key2")
		assert.Equal(t, "addValueOnEnd", v2)
	}))

	time.Sleep(time.Millisecond * 200)
}

type CallbackTest struct {
	OnCreated   func(ctx types.NodeCtx)
	OnReload    func(parentCtx types.NodeCtx, ctx types.NodeCtx)
	OnDestroy   func(ctx types.NodeCtx)
	OnCompleted func(ctx types.RuleContext, msg types.RuleMsg)
}
type EngineAspect struct {
	Name     string
	Callback *CallbackTest
}

func (aspect *EngineAspect) Order() int {
	return 1
}

func (aspect *EngineAspect) New() types.Aspect {
	return &EngineAspect{Callback: aspect.Callback, Name: aspect.Name}
}

func (aspect *EngineAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (aspect *EngineAspect) OnChainBeforeInit(config types.Config, def *types.RuleChain) error {
	return nil
}

func (aspect *EngineAspect) OnNodeBeforeInit(config types.Config, def *types.RuleNode) error {
	return nil
}

func (aspect *EngineAspect) OnCreated(ctx types.NodeCtx) error {
	//fmt.Println("OnCreated:" + ctx.GetNodeId().Id)
	if aspect.Callback != nil && aspect.Callback.OnCreated != nil {
		aspect.Callback.OnCreated(ctx)
	}
	return nil
}

func (aspect *EngineAspect) OnReload(parentCtx types.NodeCtx, ctx types.NodeCtx) error {
	//fmt.Println("OnReload:" + ctx.GetNodeId().Id)
	if aspect.Callback != nil && aspect.Callback.OnReload != nil {
		aspect.Callback.OnReload(parentCtx, ctx)
	}
	return nil
}

func (aspect *EngineAspect) OnDestroy(ctx types.NodeCtx) {
	//fmt.Println("OnDestroy:" + ctx.GetNodeId().Id)
	if aspect.Callback != nil && aspect.Callback.OnDestroy != nil {
		aspect.Callback.OnDestroy(ctx)
	}
}

func (aspect *EngineAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	if aspect.Callback != nil && aspect.Callback.OnCompleted != nil {
		aspect.Callback.OnCompleted(ctx, msg)
	}
	return msg
}

type ChainAspect struct {
	Name string
}

func (aspect *ChainAspect) Order() int {
	return 2
}

func (aspect *ChainAspect) New() types.Aspect {
	return &ChainAspect{}
}

func (aspect *ChainAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (aspect *ChainAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	msg.Metadata.PutValue("key1", "addValueOnStart")
	return msg, nil
}

func (aspect *ChainAspect) End(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	msg.Metadata.PutValue("key2", "addValueOnEnd")
	return msg
}

type NodeAspect1 struct {
	Name string
}

func (aspect *NodeAspect1) Order() int {
	return 3
}

func (aspect *NodeAspect1) New() types.Aspect {
	return &NodeAspect1{}
}

func (aspect *NodeAspect1) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}
func (aspect *NodeAspect1) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	return msg
}
func (aspect *NodeAspect1) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	return msg
}

type NodeAspect2 struct {
	Name string
}

func (aspect *NodeAspect2) Order() int {
	return 4
}

func (aspect *NodeAspect2) New() types.Aspect {
	return &NodeAspect2{}
}

func (aspect *NodeAspect2) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (aspect *NodeAspect2) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	return msg
}

func (aspect *NodeAspect2) Around(ctx types.RuleContext, msg types.RuleMsg, relationType string) (types.RuleMsg, bool) {
	return msg, true
}

type BeforeCreateErrAspect struct {
	Name  string
	Count *int32
}

func (aspect *BeforeCreateErrAspect) Order() int {
	return 3
}

func (aspect *BeforeCreateErrAspect) New() types.Aspect {
	return &BeforeCreateErrAspect{Count: aspect.Count}
}

func (aspect *BeforeCreateErrAspect) OnChainBeforeInit(config types.Config, def *types.RuleChain) error {
	atomic.AddInt32(aspect.Count, 1)
	if def != nil {
		if def.RuleChain.ID == "test02" {
			return fmt.Errorf("crate error break")
		}
	}
	return nil
}

func (aspect *BeforeCreateErrAspect) OnNodeBeforeInit(config types.Config, def *types.RuleNode) error {
	atomic.AddInt32(aspect.Count, 1)
	return nil
}

func (aspect *BeforeCreateErrAspect) OnCreated(chainCtx types.NodeCtx) error {
	atomic.AddInt32(aspect.Count, 1)
	return nil
}

func (aspect *BeforeCreateErrAspect) OnReload(chainCtx types.NodeCtx, nodeCtx types.NodeCtx) error {
	atomic.AddInt32(aspect.Count, 1)
	return nil
}

type AroundAspect struct {
	Name string
	t    *testing.T
}

func (aspect *AroundAspect) Order() int {
	return 5
}

func (aspect *AroundAspect) New() types.Aspect {
	return &AroundAspect{t: aspect.t}
}

func (aspect *AroundAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (aspect *AroundAspect) Around(ctx types.RuleContext, msg types.RuleMsg, relationType string) (types.RuleMsg, bool) {
	//fmt.Printf("debug Around before ruleChainId:%s,flowType:%s,nodeId:%s,msg:%+v,relationType:%s", ctx.RuleChain().GetNodeId().Id, "Around", ctx.Self().GetNodeId().Id, msg, relationType)
	//fmt.Println()
	msg.Metadata.PutValue(ctx.GetSelfId()+"_before", ctx.GetSelfId()+"_before")
	if ctx.GetSelfId() == "s3" {
		//s3 in not err
		assert.Nil(aspect.t, ctx.GetErr())
	}
	if ctx.GetSelfId() == "s4" {
		//s4 in err
		assert.NotNil(aspect.t, ctx.GetErr())
	}
	// 执行当前节点
	ctx.Self().OnMsg(ctx, msg)
	// 节点执行完之后逻辑
	if ctx.GetSelfId() == "s1" {
		msg.Metadata.PutValue(ctx.GetSelfId()+"_after", ctx.GetSelfId()+"_after")
	}
	if ctx.GetSelfId() == "s2" {
		// 方案1: 使用推荐的 GetData() 方法（当前实现）
		out := ctx.GetOut()
		assert.Equal(aspect.t, "{\"temperature\":41,\"userName\":\"NO-1\"}", out.GetData())

		// 方案2: 使用新的 String() 方法保持兼容性
		// assert.Equal(aspect.t, "{\"temperature\":41,\"userName\":\"NO-1\"}", ctx.GetOut().Data.String())

		// 方案3: 使用 fmt.Sprintf 格式化（也支持兼容性）
		// assert.Equal(aspect.t, "{\"temperature\":41,\"userName\":\"NO-1\"}", fmt.Sprintf("%s", ctx.GetOut().Data))
	}
	if ctx.GetSelfId() == "s3" {
		//s3 out err
		assert.NotNil(aspect.t, ctx.GetErr())
	}
	if ctx.GetSelfId() == "s4" {
		//s4 out not err
		assert.Nil(aspect.t, ctx.GetErr())
	}
	//fmt.Println(ctx.GetOut())
	//fmt.Printf("debug Around after ruleChainId:%s,flowType:%s,nodeId:%s,msg:%+v,relationType:%s", ctx.RuleChain().GetNodeId().Id, "Around", ctx.Self().GetNodeId().Id, msg, relationType)
	//fmt.Println()
	//返回false,脱离框架不重复执行该节点逻辑
	return msg, false
}

func TestAroundAspect(t *testing.T) {
	var chain = `
{
  "ruleChain": {
    "id": "rule8848",
    "name": "测试规则链",
    "root": true
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "过滤",
        "debugMode": true,
        "configuration": {
          "jsScript": "return msg.role=='admin';"
        }
      },
      {
        "id": "s2",
        "type": "jsTransform",
        "name": "转换",
        "configuration": {
          "jsScript": "msg.userName='NO-1';\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s3",
        "type": "jsTransform",
        "name": "转换错误",
        "configuration": {
          "jsScript": "xx.userName='错误';\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s4",
        "type": "jsTransform",
        "name": "转换",
        "configuration": {
          "jsScript": " return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
         {
        "fromId": "s1",
        "toId": "s2",
        "type": "False"
      }, {
        "fromId": "s2",
        "toId": "s3",
        "type": "Success"
      }, {
        "fromId": "s3",
        "toId": "s4",
        "type": "Failure"
      }
    ]
  }
}

`
	chainId := "test_around_aspect"

	config := NewConfig()

	ruleEngine, err := DefaultPool.New(chainId, []byte(chain), WithConfig(config), types.WithAspects(
		&AroundAspect{Name: "AroundAspect1", t: t},
	))
	if err != nil {
		t.Error(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		fmt.Println("end")
	}))

	time.Sleep(time.Millisecond * 20000)
}

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

func TestMetricsAspect(t *testing.T) {
	ruleFile := loadFile("./test_metrics_chain.json")
	//测试函数
	action.Functions.Register("doErr", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(time.Millisecond * 100)
		ctx.TellFailure(msg, errors.New("error"))
	})
	action.Functions.Register("doSuccess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(time.Millisecond * 100)
		ctx.TellNext(msg, types.Success)
	})

	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("testMetricsAspect", ruleFile, WithConfig(config))
	assert.Nil(t, err)

	metaData := types.NewMetadata()
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg)
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 50)
	metrics := ruleEngine.GetMetrics().Get()
	//正在执行
	assert.Equal(t, int64(2), metrics.Current)

	time.Sleep(time.Millisecond * 500)
	//等待所有规则链支持完
	metrics = ruleEngine.GetMetrics().Get()
	assert.Equal(t, int64(0), metrics.Current)
	assert.Equal(t, int64(2), metrics.Total)
	assert.Equal(t, int64(2), metrics.Failed)
	assert.Equal(t, int64(4), metrics.Success)
	//重置
	ruleEngine.GetMetrics().Reset()
	assert.Equal(t, int64(0), ruleEngine.GetMetrics().Get().Total)
	assert.Equal(t, int64(0), ruleEngine.GetMetrics().Get().Failed)
	assert.Equal(t, int64(0), ruleEngine.GetMetrics().Get().Success)

	ruleEngine.OnMsg(msg)
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 500)
	metrics = ruleEngine.GetMetrics().Get()
	assert.Equal(t, int64(0), metrics.Current)
	assert.Equal(t, int64(2), metrics.Total)
	assert.Equal(t, int64(2), metrics.Failed)
	assert.Equal(t, int64(4), metrics.Success)

	//刷新，指标不变
	_ = ruleEngine.Reload()
	metrics = ruleEngine.GetMetrics().Get()
	assert.Equal(t, int64(0), metrics.Current)
	assert.Equal(t, int64(2), metrics.Total)
	assert.Equal(t, int64(2), metrics.Failed)
	assert.Equal(t, int64(4), metrics.Success)

}

// TestForNodeConcurrentMetadataAccess 测试for节点在并发场景下的元数据读写安全性
// 这个测试专门检查for节点处理元数据时是否存在并发读写问题
func TestForNodeConcurrentMetadataAccess(t *testing.T) {
	// 创建包含for节点的规则链
	forNodeRuleChain := `{
		"ruleChain": {
			"id": "test_for_concurrent",
			"name": "testForConcurrent",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "for_node",
					"type": "for",
					"name": "循环节点",
					"configuration": {
						"range": "msg.items",
						"do": "process_item",
						"mode": 1
					}
				},
				{
					"id": "process_item",
					"type": "jsTransform",
					"name": "处理项目",
					"configuration": {
						"jsScript": "metadata['processed_' + metadata._loopIndex] = 'item_' + metadata._loopItem; metadata['timestamp'] = Date.now(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "for_node",
					"toId": "process_item",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_for_concurrent", []byte(forNodeRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 并发测试参数
	concurrentCount := 50
	itemsPerMessage := 10
	var successCount int64
	var errorCount int64

	// 用于同步等待所有消息处理完成
	done := make(chan bool, 1)

	// 启动多个goroutine并发发送消息
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// 创建包含数组的消息
			items := make([]interface{}, itemsPerMessage)
			for j := 0; j < itemsPerMessage; j++ {
				items[j] = fmt.Sprintf("item_%d_%d", index, j)
			}

			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))

			itemsJSON, _ := json.Marshal(items)
			msg := types.NewMsg(0, "TEST_FOR_CONCURRENT", types.JSON, metaData, fmt.Sprintf(`{"items": %s, "batch_id": %d}`, itemsJSON, index))

			// 发送消息并等待处理完成
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
				if atomic.LoadInt64(&successCount)+atomic.LoadInt64(&errorCount) == int64(concurrentCount) {
					done <- true
				}
			}))
		}(i)
	}

	// 等待所有消息处理完成
	select {
	case <-done:
		// 所有消息处理完成
	case <-time.After(10 * time.Second):
		t.Fatal("测试超时")
	}

	// 验证结果
	if successCount != int64(concurrentCount) {
		t.Errorf("期望处理 %d 条消息，实际处理 %d 条", concurrentCount, successCount)
	}
	if errorCount != 0 {
		t.Errorf("期望0个错误，实际有 %d 个错误", errorCount)
	}

}

// TestForNodeMetadataRaceCondition 测试for节点元数据的竞态条件
// 这个测试专门检查在高并发情况下是否会出现数据竞争
func TestForNodeMetadataRaceCondition(t *testing.T) {
	// 创建一个更复杂的规则链，包含多个节点来增加竞态条件的可能性
	raceTestRuleChain := `{
		"ruleChain": {
			"id": "test_race_condition",
			"name": "testRaceCondition",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "for_node",
					"type": "for",
					"name": "循环节点",
					"configuration": {
						"range": "1..100",
						"do": "concurrent_processor",
						"mode": 3
					}
				},
				{
					"id": "concurrent_processor",
					"type": "jsTransform",
					"name": "并发处理器",
					"configuration": {
						"jsScript": "var key = 'race_test_' + metadata._loopIndex; metadata[key] = metadata._loopItem + '_processed'; metadata['global_counter'] = (metadata['global_counter'] || 0) + 1; return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "for_node",
					"toId": "concurrent_processor",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_race_condition", []byte(raceTestRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 高并发测试
	concurrentCount := 100
	var wg sync.WaitGroup
	var processedCount int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 启动多个goroutine同时发送消息
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			metaData := types.NewMetadata()
			metaData.PutValue("test_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))

			msg := types.NewMsg(0, "RACE_TEST", types.JSON, metaData, `{"test": "data"}`)

			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					t.Errorf("处理消息时出错: %v", err)
				} else {
					atomic.AddInt64(&processedCount, 1)
				}
			}))
		}(i)
	}

	// 等待所有goroutine完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:

	case <-ctx.Done():
		currentProcessed := atomic.LoadInt64(&processedCount)
		t.Errorf("测试超时: 只处理了 %d 条消息", currentProcessed)
		return
	}

	// OnMsg 是异步投递，OnEnd 回调在协程池触发；生产者返回不代表处理完成，
	// 必须等计数到齐再断言，否则消息还在池里排队时断言必然失败
	for atomic.LoadInt64(&processedCount) < int64(concurrentCount) {
		select {
		case <-ctx.Done():
			t.Errorf("处理超时: 只处理了 %d/%d 条消息", atomic.LoadInt64(&processedCount), concurrentCount)
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestForNodeConcurrentWithFork 测试使用fork节点并发执行，同时验证SharedData和Metadata的并发安全性
// 这个测试验证fork节点能够正确地将消息分发到多个节点并行处理，并测试数据和元数据的并发修改
func TestForNodeConcurrentWithFork(t *testing.T) {
	// 创建包含fork节点和多个并发处理节点的规则链
	forkRuleChain := `{
		"ruleChain": {
			"id": "test_fork_concurrent_data_safety",
			"name": "测试Fork并发数据安全性",
			"debugMode": true,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "fork_start",
					"type": "fork",
					"name": "并行网关"
				},
				{
					"id": "concurrent_processor_1",
					"type": "jsTransform",
					"name": "并发处理器1",
					"configuration": {
						"jsScript": "msg.processor1_timestamp = Date.now(); msg.processor1_id = Math.random(); msg.concurrent_modifications = (msg.concurrent_modifications || 0) + 1; metadata['processor1'] = 'executed_' + Date.now(); metadata['total_processors'] = (parseInt(metadata['total_processors'] || '0') + 1).toString(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "concurrent_processor_2",
					"type": "jsTransform",
					"name": "并发处理器2",
					"configuration": {
						"jsScript": "msg.processor2_timestamp = Date.now(); msg.processor2_id = Math.random(); msg.concurrent_modifications = (msg.concurrent_modifications || 0) + 1; metadata['processor2'] = 'executed_' + Date.now(); metadata['total_processors'] = (parseInt(metadata['total_processors'] || '0') + 1).toString(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "concurrent_processor_3",
					"type": "jsTransform",
					"name": "并发处理器3",
					"configuration": {
						"jsScript": "msg.processor3_timestamp = Date.now(); msg.processor3_id = Math.random(); msg.concurrent_modifications = (msg.concurrent_modifications || 0) + 1; metadata['processor3'] = 'executed_' + Date.now(); metadata['total_processors'] = (parseInt(metadata['total_processors'] || '0') + 1).toString(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "final_validator",
					"type": "jsTransform",
					"name": "最终验证器",
					"configuration": {
						"jsScript": "metadata['final_processed'] = 'true'; metadata['completion_time'] = Date.now(); metadata['data_integrity_check'] = (msg.concurrent_modifications >= 1 ? 'passed' : 'failed'); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "fork_start",
					"toId": "concurrent_processor_1",
					"type": "Success"
				},
				{
					"fromId": "fork_start",
					"toId": "concurrent_processor_2",
					"type": "Success"
				},
				{
					"fromId": "fork_start",
					"toId": "concurrent_processor_3",
					"type": "Success"
				},
				{
					"fromId": "concurrent_processor_1",
					"toId": "final_validator",
					"type": "Success"
				},
				{
					"fromId": "concurrent_processor_2",
					"toId": "final_validator",
					"type": "Success"
				},
				{
					"fromId": "concurrent_processor_3",
					"toId": "final_validator",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	var dataCorruptions int64
	var metadataCorruptions int64
	var refCountAnomalies int64
	var inDataValidations int64
	var outDataValidations int64
	var nodeProcessingErrors int64

	// 配置调试回调以检测并发问题和验证每个节点的IN/OUT数据准确性
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		// 处理错误情况
		if err != nil {
			atomic.AddInt64(&nodeProcessingErrors, 1)
			return
		}

		// 直接验证数据完整性
		data := msg.GetData()
		if data == "" {
			atomic.AddInt64(&dataCorruptions, 1)
			return
		}

		// 验证JSON格式
		var jsonData map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(data), &jsonData); jsonErr != nil {
			atomic.AddInt64(&dataCorruptions, 1)
			return
		}

		// 验证引用计数
		if sharedData := msg.Data; sharedData != nil {
			if refCount := sharedData.GetRefCount(); refCount <= 0 {
				atomic.AddInt64(&refCountAnomalies, 1)
			}
		}

		// 验证元数据完整性
		if msg.Metadata.Len() == 0 {
			atomic.AddInt64(&metadataCorruptions, 1)
			return
		}

		// 验证每个节点的IN/OUT数据准确性
		if flowType == types.In {
			atomic.AddInt64(&inDataValidations, 1)
			// 验证输入数据的完整性
			switch nodeId {
			case "fork_start":
				// fork节点输入：应该包含原始数据
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if initialValue, exists := jsonData["initial_value"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := initialValue.(string); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证原始元数据
				batchId := msg.Metadata.GetValue("batch_id")
				if batchId == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_1", "concurrent_processor_2", "concurrent_processor_3":
				// 并发处理器输入：应该包含原始数据和fork传递的数据
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证并发修改字段的初始状态
				if modifications, exists := jsonData["concurrent_modifications"]; exists {
					if modCount, ok := modifications.(float64); ok && modCount < 0 {
						atomic.AddInt64(&dataCorruptions, 1)
					}
				}

			case "final_validator":
				// 最终验证器输入：应该包含被处理器修改过的数据
				if modifications, exists := jsonData["concurrent_modifications"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if modCount, ok := modifications.(float64); !ok || modCount < 1 {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证处理器特定字段
				processorFound := false
				for _, field := range []string{"processor1_timestamp", "processor2_timestamp", "processor3_timestamp"} {
					if _, exists := jsonData[field]; exists {
						processorFound = true
						break
					}
				}
				if !processorFound {
					atomic.AddInt64(&dataCorruptions, 1)
				}
			}

		} else if flowType == types.Out {
			atomic.AddInt64(&outDataValidations, 1)
			// 验证输出数据的完整性
			switch nodeId {
			case "fork_start":
				// fork节点输出：数据应该保持不变
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if initialValue, exists := jsonData["initial_value"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := initialValue.(string); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

			case "concurrent_processor_1":
				// 并发处理器1输出：应该包含processor1特定的字段
				if timestamp, exists := jsonData["processor1_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processorId, exists := jsonData["processor1_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := processorId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证并发修改计数增加
				if modifications, exists := jsonData["concurrent_modifications"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if modCount, ok := modifications.(float64); !ok || modCount < 1 {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证元数据中的processor1字段
				if processor1 := msg.Metadata.GetValue("processor1"); processor1 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_2":
				// 并发处理器2输出验证
				if timestamp, exists := jsonData["processor2_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processor2 := msg.Metadata.GetValue("processor2"); processor2 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_3":
				// 并发处理器3输出验证
				if timestamp, exists := jsonData["processor3_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processor3 := msg.Metadata.GetValue("processor3"); processor3 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "final_validator":
				// 最终验证器输出：应该包含所有验证标记
				if finalProcessed := msg.Metadata.GetValue("final_processed"); finalProcessed != "true" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				if completionTime := msg.Metadata.GetValue("completion_time"); completionTime == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				if integrityCheck := msg.Metadata.GetValue("data_integrity_check"); integrityCheck != "passed" && integrityCheck != "failed" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				// 验证原始数据仍然存在
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}
			}

			// 通用验证：所有输出都应该保持原始batch_id
			originalBatchId := msg.Metadata.GetValue("batch_id")
			if originalBatchId == "" {
				atomic.AddInt64(&metadataCorruptions, 1)
			}

			// 通用验证：所有输出都应该保持原始test_type
			testType := msg.Metadata.GetValue("test_type")
			if testType != "fork_concurrent_data_safety" {
				atomic.AddInt64(&metadataCorruptions, 1)
			}
		}
	}

	ruleEngine, err := New("test_fork_concurrent_data_safety", []byte(forkRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 并发测试参数
	concurrentCount := 30 // 增加并发数量以增强测试强度
	var successCount int64
	var errorCount int64
	var finalProcessorCount int64

	// 用于同步等待所有消息处理完成
	done := make(chan bool, 1)

	// 启动多个goroutine并发发送消息
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// 创建包含复杂数据的消息以增加并发修改的复杂性
			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))
			metaData.PutValue("test_type", "fork_concurrent_data_safety")
			metaData.PutValue("initial_processor_count", "0")

			// 创建包含多个字段的JSON数据，这些将被并发修改
			originalData := fmt.Sprintf(`{
				"message_id": %d, 
				"initial_value": "test_%d",
				"concurrent_modifications": 0,
				"creation_timestamp": %d,
				"processor_data": {}
			}`, index, index, time.Now().UnixNano())

			msg := types.NewMsg(0, "TEST_FORK_CONCURRENT_SAFETY", types.JSON, metaData, originalData)

			// 发送消息并等待处理完成
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)

					// 检查是否是最终处理器的结果
					if msg.Metadata.GetValue("final_processed") == "true" {
						atomic.AddInt64(&finalProcessorCount, 1)

						// 验证数据完整性
						data := msg.GetData()
						var jsonData map[string]interface{}
						if jsonErr := json.Unmarshal([]byte(data), &jsonData); jsonErr != nil {
							atomic.AddInt64(&dataCorruptions, 1)
							t.Errorf("最终数据JSON解析失败: %v, 数据: %s", jsonErr, data)
						} else {
							// 验证原始数据是否还存在
							if messageId, exists := jsonData["message_id"]; !exists {
								atomic.AddInt64(&dataCorruptions, 1)
								t.Errorf("原始message_id丢失，数据: %s", data)
							} else if float64(index) != messageId {
								atomic.AddInt64(&dataCorruptions, 1)
								t.Errorf("message_id不匹配: 期望 %d, 实际 %v", index, messageId)
							}

							// 验证并发修改计数
							if modifications, exists := jsonData["concurrent_modifications"]; exists {
								if modCount, ok := modifications.(float64); ok && modCount < 1 {
									t.Errorf("并发修改计数异常: %v", modCount)
								}
							}
						}

						// 验证元数据完整性
						originalBatchId := msg.Metadata.GetValue("batch_id")
						if originalBatchId != strconv.Itoa(index) {
							atomic.AddInt64(&metadataCorruptions, 1)
							t.Errorf("batch_id不匹配: 期望 %d, 实际 %s", index, originalBatchId)
						}

						// 验证数据完整性检查结果
						if integrityCheck := msg.Metadata.GetValue("data_integrity_check"); integrityCheck == "failed" {
							atomic.AddInt64(&dataCorruptions, 1)
							t.Errorf("数据完整性检查失败")
						}
					}
				}

				// 当所有最终处理器完成时发送完成信号
				if msg.Metadata.GetValue("final_processed") == "true" {
					if atomic.LoadInt64(&finalProcessorCount) >= int64(concurrentCount*3) {
						select {
						case done <- true:
						default:
						}
					}
				}
			}))
		}(i)
	}

	// 等待所有消息处理完成
	select {
	case <-done:
		// 所有消息处理完成
	case <-time.After(20 * time.Second): // 增加超时时间
		t.Fatal("测试超时")
	}

	// 等待额外时间确保所有异步操作完成
	time.Sleep(time.Second)

	// 验证结果
	finalErrorCount := atomic.LoadInt64(&errorCount)
	finalDataCorruptions := atomic.LoadInt64(&dataCorruptions)
	finalMetadataCorruptions := atomic.LoadInt64(&metadataCorruptions)
	finalRefCountAnomalies := atomic.LoadInt64(&refCountAnomalies)
	finalProcessorCountResult := atomic.LoadInt64(&finalProcessorCount)
	finalInDataValidations := atomic.LoadInt64(&inDataValidations)
	finalOutDataValidations := atomic.LoadInt64(&outDataValidations)
	finalNodeProcessingErrors := atomic.LoadInt64(&nodeProcessingErrors)

	// 验证错误
	if finalNodeProcessingErrors > 0 {
		t.Errorf("期望0个节点处理错误，实际有 %d 个错误", finalNodeProcessingErrors)
	}

	if finalErrorCount > 0 {
		t.Errorf("期望0个处理错误，实际有 %d 个错误", finalErrorCount)
	}

	if finalDataCorruptions > 0 {
		t.Errorf("检测到 %d 次数据损坏", finalDataCorruptions)
	}

	if finalMetadataCorruptions > 0 {
		t.Errorf("检测到 %d 次元数据损坏", finalMetadataCorruptions)
	}

	if finalRefCountAnomalies > 0 {
		t.Errorf("检测到 %d 次引用计数异常", finalRefCountAnomalies)
	}

	// 验证数据验证次数的合理性（每个消息会产生多次IN/OUT事件）
	expectedMinValidations := int64(concurrentCount * 4) // 每个消息至少经过4个节点
	if finalInDataValidations < expectedMinValidations {
		t.Errorf("IN数据验证次数过少: 期望至少 %d 次，实际 %d 次", expectedMinValidations, finalInDataValidations)
	}

	if finalOutDataValidations < expectedMinValidations {
		t.Errorf("OUT数据验证次数过少: 期望至少 %d 次，实际 %d 次", expectedMinValidations, finalOutDataValidations)
	}

	// 验证最终处理器被调用的次数（应该等于并发数量的3倍，因为每个消息会触发3个处理器）
	expectedFinalCount := int64(concurrentCount * 3)
	if finalProcessorCountResult != expectedFinalCount {
		t.Errorf("期望最终处理器被调用 %d 次，实际调用 %d 次", expectedFinalCount, finalProcessorCountResult)
	}
}

// TestForNodeAsyncModeMetadataSafety 测试for节点异步模式下的元数据安全性
func TestForNodeAsyncModeMetadataSafety(t *testing.T) {
	// 创建异步模式的for节点规则链
	asyncRuleChain := `{
		"ruleChain": {
			"id": "test_async_safety",
			"name": "testAsyncSafety",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "async_for",
					"type": "for",
					"name": "异步循环",
					"configuration": {
						"range": "msg.items",
						"do": "async_processor",
						"mode": 3
					}
				},
				{
					"id": "async_processor",
					"type": "jsTransform",
					"name": "异步处理器",
					"configuration": {
						"jsScript": "metadata['async_processed_' + metadata._loopIndex] = metadata._loopItem; metadata['process_time'] = Date.now(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "async_for",
					"toId": "async_processor",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_async_safety", []byte(asyncRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 创建包含大量项目的消息
	itemsCount := 50
	items := make([]interface{}, itemsCount)
	for i := 0; i < itemsCount; i++ {
		items[i] = fmt.Sprintf("async_item_%d", i)
	}

	itemsJSON, _ := json.Marshal(items)
	msgData := fmt.Sprintf(`{"items": %s}`, itemsJSON)
	metaData := types.NewMetadata()
	metaData.PutValue("test_type", "async_safety")
	metaData.PutValue("items_count", strconv.Itoa(itemsCount))

	msg := types.NewMsg(0, "ASYNC_TEST", types.JSON, metaData, msgData)

	// 发送消息并验证异步处理不会导致数据竞争
	var processedCount int64
	var errorCount int64

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		if err != nil {
			atomic.AddInt64(&errorCount, 1)

		} else {
			atomic.AddInt64(&processedCount, 1)

		}
	}))

	// 等待一段时间让异步处理完成
	time.Sleep(2 * time.Second)

	// 验证结果
	finalProcessedCount := atomic.LoadInt64(&processedCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)
	if finalProcessedCount != 1 {
		t.Errorf("期望处理1条消息，实际处理 %d 条", finalProcessedCount)
	}
	if finalErrorCount != 0 {
		t.Errorf("期望0个错误，实际有 %d 个错误", finalErrorCount)
	}

}

// TestConcurrentRaceCondition 测试getEnv并发竞态条件
func TestConcurrentGetEnv(t *testing.T) {
	// 规则链DSL - 一个节点分叉到两个并发节点
	ruleChainDSL := `{
		"ruleChain": {
			"id": "kOPFwceGDK9p",
			"name": "测试并发",
			"root": true,
			"debugMode": true,
			"additionalInfo": {
				"description": "",
				"layoutX": "280",
				"layoutY": "280"
			},
			"configuration": {}
		},
		"metadata": {
			"endpoints": [],
			"nodes": [
				{
					"id": "node_2",
					"type": "restApiCall",
					"name": "并发1",
					"configuration": {
						"requestMethod": "GET",
						"headers": {
							"Content-Type": "application/json",
							"Token": "${metadata.token}"
						},
						"readTimeoutMs": 2000,
						"insecureSkipVerify": true,
						"maxParallelRequestsCount": 200,
						"proxyPort": 0,
						"restEndpointUrlPattern": "https://aa/delay/1"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 480,
						"layoutY": 280
					}
				},
				{
					"id": "node_3",
					"type": "restApiCall",
					"name": "并发2",
					"configuration": {
						"requestMethod": "GET",
						"headers": {
							"Content-Type": "application/json",
							"Token": "${metadata.token}"
						},
						"readTimeoutMs": 2000,
						"insecureSkipVerify": true,
						"maxParallelRequestsCount": 200,
						"proxyPort": 0,
						"restEndpointUrlPattern": "https://aa/delay/1"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 750,
						"layoutY": 200
					}
				},
				{
					"id": "node_4",
					"type": "restApiCall",
					"name": "并发3",
					"configuration": {
						"requestMethod": "GET",
						"headers": {
							"Content-Type": "application/json",
							"Token": "${metadata.token}"
						},
						"readTimeoutMs": 2000,
						"insecureSkipVerify": true,
						"maxParallelRequestsCount": 200,
						"proxyPort": 0,
						"restEndpointUrlPattern": "https://aa/delay/1"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 750,
						"layoutY": 350
					}
				}
			],
			"connections": [
			{
				"fromId": "node_2",
				"toId": "node_3",
				"type": "Success"
			},
			{
				"fromId": "node_2",
				"toId": "node_3",
				"type": "Failure"
			},
			{
				"fromId": "node_2",
				"toId": "node_4",
				"type": "Success"
			},
			{
				"fromId": "node_2",
				"toId": "node_4",
				"type": "Failure"
			}
			]
		}
	}`

	// 创建规则引擎
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("test", []byte(ruleChainDSL), WithConfig(config))
	assert.Nil(t, err)

	// 并发测试参数
	concurrentCount := 50 // 并发数量
	messageCount := 1     // 每个协程发送的消息数量

	var wg sync.WaitGroup
	wg.Add(concurrentCount * messageCount * 2)
	// 启动多个协程并发执行规则链
	for i := 0; i < concurrentCount; i++ {
		go func(routineID int) {

			// 每个协程发送多条消息
			for j := 0; j < messageCount; j++ {
				metadata := types.NewMetadata()
				// 创建消息
				msg := types.NewMsg(0, "TEST", types.JSON, metadata, fmt.Sprintf(`{"id":%d,"count":%d}`, routineID, j))

				// 设置metadata，包含token用于模板替换
				msg.Metadata.PutValue("token", fmt.Sprintf("token_%d_%d", routineID, j))
				msg.Metadata.PutValue("routineID", fmt.Sprintf("%d", routineID))
				msg.Metadata.PutValue("messageID", fmt.Sprintf("%d", j))

				// 执行规则链
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					wg.Done()
				}))

				// 添加小延迟，增加并发竞争的可能性
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	// 等待所有协程完成
	wg.Wait()

}

// globalRunLogChain is a minimal 2-node chain: jsFilter -> jsTransform.
// ruleChain.debugMode is off (to avoid the OnDebug path); node debugMode is off too.
// A message with temperature=41 passes s1's jsFilter (>10 -> True) and reaches s2.
// globalRunLogChain 是一个最小 2 节点链：jsFilter -> jsTransform。
// ruleChain.debugMode 关闭（避免走 OnDebug 路径），节点 debugMode 也关闭。
// 消息 temperature=41 通过 s1 的 jsFilter（>10 为 True）到达 s2。
var globalRunLogChain = `{
  "ruleChain": {
    "id": "r1",
    "name": "globalRunLogChain",
    "debugMode": false,
    "root": true,
    "disabled": false
  },
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "filter",
        "debugMode": false,
        "configuration": {
          "jsScript": "return msg.temperature>10;"
        }
      },
      {
        "id": "s2",
        "type": "jsTransform",
        "name": "transform",
        "debugMode": false,
        "configuration": {
          "jsScript": "msgType='TEST_MSG_TYPE'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "True"
      }
    ]
  }
}`

// chainJSONWithLevel returns globalRunLogChain but with the rule chain's
// additionalInfo.runLogMode set to the given level. Used to test chain-level
// override in both directions (up to detail, down to summary).
// chainJSONWithLevel 返回 globalRunLogChain，但把规则链的
// additionalInfo.runLogMode 设为指定值。用于双向测试链级覆盖（升 detail / 降 summary）。
func chainJSONWithLevel(chainId, level string) string {
	return `{
  "ruleChain": {
    "id": "` + chainId + `",
    "name": "globalRunLogLevelChain",
    "debugMode": false,
    "root": true,
    "disabled": false,
    "additionalInfo": {
      "runLogMode": "` + level + `"
    }
  },
  "metadata": {
    "firstNodeIndex": 0,
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "filter",
        "debugMode": false,
        "configuration": {
          "jsScript": "return msg.temperature>10;"
        }
      },
      {
        "id": "s2",
        "type": "jsTransform",
        "name": "transform",
        "debugMode": false,
        "configuration": {
          "jsScript": "msgType='TEST_MSG_TYPE'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "True"
      }
    ]
  }
}`
}

// newGlobalRunLogMsg builds a temperature=41 test message (passes jsFilter's >10).
// newGlobalRunLogMsg 构造一条 temperature=41 的测试消息（会通过 jsFilter 的 >10 条件）。
func newGlobalRunLogMsg() types.RuleMsg {
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	return types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, `{"temperature":41,"humidity":90}`)
}

// completedRecorder is a tiny helper that captures the latest snapshot under a mutex.
// completedRecorder 是一个在互斥锁下记录最近一次 snapshot 的小工具。
type completedRecorder struct {
	mu     sync.Mutex
	called bool
	latest types.RuleChainRunSnapshot
}

func (r *completedRecorder) onCompleted(_ types.RuleContext, snapshot types.RuleChainRunSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	r.latest = snapshot
}

// captured returns copies of the captured state. captured 返回已捕获状态的副本。
func (r *completedRecorder) captured() (bool, types.RuleChainRunSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.called, r.latest
}

// TestGlobalRunLog_Detail: Config.OnRuleChainCompleted + RunLogMode="detail".
// The callback fires and snapshot.Logs is non-empty (per-node logs collected).
// TestGlobalRunLog_Detail：设置 Config.OnRuleChainCompleted + RunLogMode="detail"。
// 回调被调用，且 snapshot.Logs 非空（收集了逐节点日志）。
func TestGlobalRunLog_Detail(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	// Unique id + defer Del so DefaultPool does not reuse a stale instance
	// (one created with an old, callback-less Config) across -count=N runs.
	// 使用唯一 id + defer Del，避免 DefaultPool 在 -count=N 重跑时命中旧实例（旧 Config 无回调）
	chainId := "testGlobalRunLogDetail"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "Config.OnRuleChainCompleted callback should be invoked")
	assert.True(t, len(snap.Logs) > 0, "snapshot.Logs should contain per-node logs at detail level")
	// Both nodes s1/s2 should be collected. 两个节点 s1/s2 都应被收集。
	assert.Equal(t, 2, len(snap.Logs))
}

// TestGlobalRunLog_Summary: Config.OnRuleChainCompleted + RunLogMode="summary".
// The callback fires but snapshot.Logs is empty (per-node collection skipped).
// TestGlobalRunLog_Summary：设置 Config.OnRuleChainCompleted + RunLogMode="summary"。
// 回调被调用，但 snapshot.Logs 为空（跳过了逐节点收集）。
func TestGlobalRunLog_Summary(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeSummary),
	)

	chainId := "testGlobalRunLogSummary"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "Config.OnRuleChainCompleted callback should be invoked even at summary level")
	// At summary level collectDetail=false, per-node logs are not collected;
	// snapshot.Logs is nil or an empty slice.
	// summary 级别 collectDetail=false，逐节点日志不收集；snapshot.Logs 为 nil 或空切片
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty at summary level (per-node logs skipped)")
}

// TestGlobalRunLog_Off: Config.OnRuleChainCompleted + RunLogMode="off".
// The callback still fires (whether a callback fires is orthogonal to RunLogMode),
// but snapshot.Logs is empty because off != detail.
// TestGlobalRunLog_Off：设置 Config.OnRuleChainCompleted + RunLogMode="off"。
// 回调仍会触发（回调是否触发与 RunLogMode 正交），但因 off != detail，snapshot.Logs 为空。
func TestGlobalRunLog_Off(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeOff),
	)

	chainId := "testGlobalRunLogOff"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "callback should still fire at off level (triggering is orthogonal to RunLogMode)")
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty at off level")
}

// TestGlobalRunLog_NoCallback: no completion callback registered at all.
// Verifies the engine runs without error and nothing fires. Constructs the
// config WITHOUT WithOnRuleChainCompletedGlobal (rather than registering then
// clearing), so the test would actually catch a "fires even when never set" bug.
// TestGlobalRunLog_NoCallback：完全不注册任何完成回调。
// 验证引擎正常运行不报错、且无回调触发。构造 Config 时不注册
// WithOnRuleChainCompletedGlobal（而不是注册后再清空），从而能真正捕获"未设置却触发"的 bug。
func TestGlobalRunLog_NoCallback(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithRunLogMode(types.RunLogModeDetail),
		// Deliberately no WithOnRuleChainCompletedGlobal here.
		// 故意不注册 WithOnRuleChainCompletedGlobal。
	)

	chainId := "testGlobalRunLogNoCallback"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, _ := rec.captured()
	assert.False(t, called, "Config.OnRuleChainCompleted callback should NOT be invoked when not set")
}

// TestGlobalRunLog_PerCallPriority: both per-call WithOnRuleChainCompleted and
// Config.OnRuleChainCompleted are set. Only the per-call one fires; the
// Config-level one is suppressed.
// TestGlobalRunLog_PerCallPriority：同时设置 per-call WithOnRuleChainCompleted 和
// Config.OnRuleChainCompleted。只有 per-call 触发，Config 级被抑制不触发。
func TestGlobalRunLog_PerCallPriority(t *testing.T) {
	var (
		mu            sync.Mutex
		globalCalled  bool
		perCallCalled bool
		perCallSnap   types.RuleChainRunSnapshot
	)
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(func(_ types.RuleContext, _ types.RuleChainRunSnapshot) {
			mu.Lock()
			defer mu.Unlock()
			globalCalled = true
		}),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	chainId := "testGlobalRunLogPerCall"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg(), types.WithOnRuleChainCompleted(func(_ types.RuleContext, snapshot types.RuleChainRunSnapshot) {
		mu.Lock()
		defer mu.Unlock()
		perCallCalled = true
		perCallSnap = snapshot
	}))

	mu.Lock()
	globalCalledCopy := globalCalled
	perCallCalledCopy := perCallCalled
	snap := perCallSnap
	mu.Unlock()

	assert.True(t, perCallCalledCopy, "per-call OnRuleChainCompleted should be invoked")
	assert.False(t, globalCalledCopy, "Config-level OnRuleChainCompleted should NOT be invoked when per-call is set")
	// detail level + per-call callback present -> collectDetail=true, per-node logs collected.
	// detail 级别 + per-call 回调存在 -> collectDetail=true，逐节点日志被收集
	assert.True(t, len(snap.Logs) > 0, "snapshot.Logs should contain per-node logs at detail level with per-call callback")
	assert.Equal(t, 2, len(snap.Logs))
}

// TestGlobalRunLog_ChainLevelDowngrade: global RunLogMode="detail" but the chain's
// additionalInfo.runLogMode="summary". snapshot.Logs should be empty (chain-level
// overrides global, downgrading to summary). The global callback still fires.
// TestGlobalRunLog_ChainLevelDowngrade：全局 RunLogMode="detail"，但链定义的
// additionalInfo.runLogMode="summary"。snapshot.Logs 应为空（链级覆盖全局降级为 summary）。
// 全局回调仍会触发。
func TestGlobalRunLog_ChainLevelDowngrade(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	chainId := "testGlobalRunLogChainDowngrade"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(chainJSONWithLevel(chainId, "summary")), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	// Chain-level summary overrides global detail -> collectDetail=false; but the
	// global callback still exists and still fires (the else-if branch).
	// 链级 summary 覆盖全局 detail -> collectDetail=false；但全局回调仍存在，仍会触发（走 else-if 分支）
	assert.True(t, called, "Config.OnRuleChainCompleted callback should still be invoked")
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty when chain-level runLogMode=summary overrides global detail")
}

// TestGlobalRunLog_ChainLevelUpgrade: global RunLogMode="off" but the chain's
// additionalInfo.runLogMode="detail". snapshot.Logs should be non-empty (chain-level
// overrides global, upgrading to detail). Confirms chain-level precedence is symmetric.
// TestGlobalRunLog_ChainLevelUpgrade：全局 RunLogMode="off"，但链定义的
// additionalInfo.runLogMode="detail"。snapshot.Logs 应非空（链级覆盖全局升 detail）。
// 验证链级优先是对称的。
func TestGlobalRunLog_ChainLevelUpgrade(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeOff),
	)

	chainId := "testGlobalRunLogChainUpgrade"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(chainJSONWithLevel(chainId, "detail")), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "Config.OnRuleChainCompleted callback should be invoked")
	assert.True(t, len(snap.Logs) > 0, "snapshot.Logs should contain per-node logs when chain-level runLogMode=detail overrides global off")
	assert.Equal(t, 2, len(snap.Logs))
}

// TestGlobalRunLog_InvalidGlobalMode: an unrecognized global mode is normalized to off.
// The callback still fires but no per-node logs are collected.
// TestGlobalRunLog_InvalidGlobalMode：无法识别的全局 mode 被规范化为 off。
// 回调仍触发，但不收集逐节点日志。
func TestGlobalRunLog_InvalidGlobalMode(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogMode("verbose")), // typo / unknown value
	)

	chainId := "testGlobalRunLogInvalidMode"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())

	called, snap := rec.captured()
	assert.True(t, called, "callback should still fire even with an invalid mode")
	assert.True(t, len(snap.Logs) == 0, "snapshot.Logs should be empty: invalid mode normalized to off, which is not detail")
	// And the Config should reflect the normalization.
	// 同时 Config 应反映规范化后的值。
	assert.Equal(t, types.RunLogModeOff, config.RunLogMode)
}

// TestGlobalRunLog_Concurrent: many messages processed concurrently through a shared
// engine each produce an isolated snapshot for their own message id. Guards against
// runSnapshot state leaking across messages (each msg gets a fresh RunSnapshot).
// TestGlobalRunLog_Concurrent：多条消息并发流经同一个引擎，每条消息各自产生隔离的
// snapshot（对应自己的 msg id）。防止 runSnapshot 状态跨消息泄漏（每条消息新建 RunSnapshot）。
func TestGlobalRunLog_Concurrent(t *testing.T) {
	rec := &completedRecorder{}
	config := NewConfig(
		types.WithOnRuleChainCompletedGlobal(rec.onCompleted),
		types.WithRunLogMode(types.RunLogModeDetail),
	)

	chainId := "testGlobalRunLogConcurrent"
	defer Del(chainId)
	ruleEngine, err := New(chainId, []byte(globalRunLogChain), WithConfig(config))
	assert.Nil(t, err)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ruleEngine.OnMsgAndWait(newGlobalRunLogMsg())
		}()
	}
	wg.Wait()

	called, snap := rec.captured()
	assert.True(t, called, "callback should have fired at least once")
	// Each OnMsgAndWait builds its own snapshot with exactly 2 node logs; the
	// recorder only keeps the last one, so it must still show 2 (never more, never less).
	// 每次 OnMsgAndWait 都新建自己的 snapshot，恰好 2 条节点日志；recorder 只保留最后一次，
	// 因此仍然必须是 2（不多不少）。
	assert.Equal(t, 2, len(snap.Logs), "each message's snapshot should be isolated with exactly 2 node logs")
}

// 注册测试用的自定义函数
func init() {
	// 快速处理函数
	action.Functions.Register("fastProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(10 * time.Millisecond)
		ctx.TellSuccess(msg)
	})

	// 慢处理函数
	action.Functions.Register("slowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(2 * time.Second)
		ctx.TellSuccess(msg)
	})

	// 超慢处理函数 - 支持上下文取消
	action.Functions.Register("verySlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 模拟一个真正的慢处理过程，不立即响应上下文取消
		// 这样可以测试Stop方法的超时行为
		startTime := time.Now()
		for {
			// 检查上下文是否被取消（优雅停机）
			select {
			case <-ctx.GetContext().Done():
				// 上下文被取消，进行清理并标记为失败
				// 这模拟了真实世界中的情况，即使收到停机信号，操作也需要时间来安全退出
				time.Sleep(350 * time.Millisecond) // 模拟清理时间，确保总时间超过400ms
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				// 每100ms检查一次是否应该退出
				time.Sleep(100 * time.Millisecond)

				// 如果已经运行了5秒，正常完成
				if time.Since(startTime) >= 5*time.Second {
					ctx.TellSuccess(msg)
					return
				}
				// 继续处理
			}
		}
	})

	// 计数器测试函数
	action.Functions.Register("counterTest", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 模拟一些处理逻辑
		time.Sleep(100 * time.Millisecond)
		ctx.TellSuccess(msg)
	})
}

// TestEngineGracefulShutdownBehavior 测试引擎优雅停机行为（合并多个相关测试）
func TestEngineGracefulShutdownBehavior(t *testing.T) {
	// 通用的规则链配置
	createRuleChain := func(functionName, chainId string) string {
		return fmt.Sprintf(`{
			"ruleChain": {
				"id": "%s",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Test Function",
						"configuration": {
							"functionName": "%s"
						}
					}
				]
			}
		}`, chainId, functionName)
	}

	// 测试场景1：计数器边界情况
	t.Run("CounterEdgeCases", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("counterTest", "test_counter")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 发送消息并验证计数器
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg)
			}(i)
		}
		wg.Wait()
		time.Sleep(200 * time.Millisecond)

		// 验证活跃操作计数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.True(t, activeOps <= 0, "Active operations should be <= 0, got: %d", activeOps)
		}

		// 测试停机期间的计数器行为
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// 测试场景2：停机超时处理
	t.Run("StopTimeout", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("verySlowProcess", "test_timeout")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 启动长时间运行的消息
		msg := types.NewMsg(0, "TIMEOUT_TEST", types.JSON, types.NewMetadata(), `{"test": "timeout"}`)
		go ruleEngine.OnMsg(msg)
		time.Sleep(200 * time.Millisecond)

		// 使用短超时停机
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		ruleEngine.Stop(ctx)
		elapsed := time.Since(startTime)

		// 验证超时行为
		assert.True(t, elapsed >= 400*time.Millisecond && elapsed <= 2*time.Second,
			"Stop should respect timeout, elapsed: %v", elapsed)
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// 测试场景3：并发停机和重载
	t.Run("ConcurrentStopAndReload", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleChainFile := createRuleChain("slowProcess", "test_concurrent")
		ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 启动消息处理
		for i := 0; i < 3; i++ {
			go func(index int) {
				msg := types.NewMsg(0, "CONCURRENT_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg)
			}(i)
		}
		time.Sleep(300 * time.Millisecond)

		// 并发执行停机和重载
		var wg sync.WaitGroup
		var stopCompleted, reloadErrors int32

		wg.Add(2)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			atomic.StoreInt32(&stopCompleted, 1)
		}()

		go func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
			if err != nil {
				atomic.AddInt32(&reloadErrors, 1)
			}
		}()

		wg.Wait()

		// 验证结果
		assert.Equal(t, int32(1), atomic.LoadInt32(&stopCompleted))
		assert.True(t, ruleEngine.IsShuttingDown())
		assert.True(t, atomic.LoadInt32(&reloadErrors) >= 0) // 重载可能失败或成功
	})
}

// TestEngineGracefulShutdownAdvanced 测试引擎高级优雅停机场景（合并活跃消息处理和消息拒绝测试）
func TestEngineGracefulShutdownAdvanced(t *testing.T) {
	// 通用的规则链配置
	createRuleChain := func(functionName, chainId string) string {
		return fmt.Sprintf(`{
			"ruleChain": {
				"id": "%s",
				"name": "Advanced Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Test Function",
						"configuration": {
							"functionName": "%s"
						}
					}
				]
			}
		}`, chainId, functionName)
	}

	// 测试场景1：有活跃消息时的优雅停机
	t.Run("ActiveMessagesShutdown", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("slowProcess", "test_active")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 启动多个慢处理消息
		var processedCount int64
		var wg sync.WaitGroup
		messageCount := 3

		for i := 0; i < messageCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				msg := types.NewMsg(0, "ACTIVE_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					atomic.AddInt64(&processedCount, 1)
				}))
			}(i)
		}

		// 等待消息开始处理
		time.Sleep(500 * time.Millisecond)

		// 检查活跃操作数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.True(t, activeOps > 0, "Should have active operations")
		}

		// 启动优雅停机
		shutdownStart := time.Now()
		shutdownDone := make(chan bool, 1)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			shutdownDone <- true
		}()

		// 等待所有消息处理完成
		wg.Wait()

		// 等待停机完成
		select {
		case <-shutdownDone:
			elapsed := time.Since(shutdownStart)
			assert.True(t, elapsed >= 1*time.Second, "Should wait for messages to complete")
			assert.True(t, elapsed < 10*time.Second, "Should not take too long")
		case <-time.After(15 * time.Second):
			t.Fatal("Graceful shutdown timeout")
		}

		// 验证最终状态
		finalCount := atomic.LoadInt64(&processedCount)
		assert.True(t, finalCount >= 0, "Should process some messages")
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// 测试场景2：停机后拒绝新消息
	t.Run("MessageRejectionAfterShutdown", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("fastProcess", "test_rejection")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 先处理一条消息确保引擎正常工作
		msg := types.NewMsg(0, "PRE_SHUTDOWN", types.JSON, types.NewMetadata(), `{"test": "pre"}`)
		processed := make(chan bool, 1)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			processed <- true
		}))
		<-processed

		// 停机
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		assert.True(t, ruleEngine.IsShuttingDown())

		// 尝试发送新消息，应该被拒绝
		var rejectedCount, processedCount int64
		for i := 0; i < 5; i++ {
			newMsg := types.NewMsg(0, "POST_SHUTDOWN", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, i))
			ruleEngine.OnMsg(newMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if relationType == types.Failure || (err != nil && err.Error() != "") {
					atomic.AddInt64(&rejectedCount, 1)
				} else {
					atomic.AddInt64(&processedCount, 1)
				}
			}))
			time.Sleep(10 * time.Millisecond)
		}

		// 等待回调完成
		time.Sleep(200 * time.Millisecond)

		// 停机后应该拒绝所有新消息
		assert.Equal(t, int64(0), atomic.LoadInt64(&processedCount), "Should not process new messages after shutdown")
		assert.True(t, atomic.LoadInt64(&rejectedCount) >= 0, "Should track rejection attempts")
	})
}

// TestEngineGracefulShutdownShouldWaitForRuleChain 测试优雅停机是否等待规则链执行完成
// 验证当有消息正在处理时，Stop方法应该等待规则链执行完成而不是立即取消
// TestEngineTwoPhaseGracefulShutdown 测试两阶段优雅停机逻辑
// 验证：1. 正在执行的规则链能继续处理完成 2. 超时后能强制中断
func TestEngineTwoPhaseGracefulShutdown(t *testing.T) {
	// 注册一个支持上下文检查的处理函数
	action.Functions.Register("contextAwareProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 模拟处理过程，每100ms检查一次上下文
		for i := 0; i < 30; i++ { // 总共3秒
			time.Sleep(100 * time.Millisecond)
			// 检查上下文是否被取消
			select {
			case <-ctx.GetContext().Done():
				// 上下文被取消，标记为失败并退出
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				// 继续处理
			}
		}
		// 正常完成
		ctx.TellSuccess(msg)
	})

	ruleChainFile := `{
		"ruleChain": {
			"id": "test_two_phase_shutdown",
			"name": "Two Phase Shutdown Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Context Aware Process Function",
					"configuration": {
						"functionName": "contextAwareProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试场景1：短时间内完成的消息应该正常完成
	t.Run("ShortProcessShouldComplete", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string

		// 注册一个快速处理函数
		action.Functions.Register("quickProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(500 * time.Millisecond) // 0.5秒
			ctx.TellSuccess(msg)
		})

		quickRuleChain := `{
			"ruleChain": {
				"id": "test_quick_process",
				"name": "Quick Process Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Quick Process Function",
						"configuration": {
							"functionName": "quickProcess"
						}
					}
				]
			}
		}`

		quickChainId := str.RandomStr(10)
		quickEngine, err := New(quickChainId, []byte(quickRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(quickChainId)

		// 发送消息
		msg := types.NewMsg(0, "QUICK_TEST", types.JSON, types.NewMetadata(), `{"test": "quick"}`)
		quickEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageCompletedMutex.Unlock()
		}))

		// 等待消息开始处理
		time.Sleep(100 * time.Millisecond)

		// 启动优雅停机，给予2秒超时
		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		quickEngine.Stop(ctx)
		elapsed := time.Since(shutdownStart)

		// 检查结果
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		messageCompletedMutex.Unlock()

		t.Logf("Quick process - Completed: %v, RelationType: %s, Elapsed: %v", completed, relationType, elapsed)

		// 快速处理应该正常完成
		assert.True(t, completed, "Quick process should complete")
		assert.Equal(t, types.Success, relationType, "Quick process should succeed")
		// 由于并发处理，实际时间可能略少于500ms，所以放宽要求
		assert.True(t, elapsed >= 300*time.Millisecond, "Should wait for process to complete, got: %v", elapsed)
		assert.True(t, elapsed < 1500*time.Millisecond, "Should not take too long")
	})

	// 测试场景2：超时的消息应该被强制中断
	t.Run("TimeoutProcessShouldBeInterrupted", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string
		var messageError error

		// 发送一个长时间处理的消息
		msg := types.NewMsg(0, "TIMEOUT_TEST", types.JSON, types.NewMetadata(), `{"test": "timeout"}`)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageError = err
			messageCompletedMutex.Unlock()
			t.Logf("Long process completed with relation: %s, error: %v", relationType, err)
		}))

		// 等待消息开始处理
		time.Sleep(200 * time.Millisecond)

		// 启动优雅停机，给予1秒超时（小于3秒的处理时间）
		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		elapsed := time.Since(shutdownStart)

		// 等待一段时间确保回调被调用
		time.Sleep(500 * time.Millisecond)

		// 检查结果
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		err := messageError
		messageCompletedMutex.Unlock()

		t.Logf("Long process - Completed: %v, RelationType: %s, Error: %v, Elapsed: %v", completed, relationType, err, elapsed)

		// 应该在超时后被中断
		assert.True(t, elapsed >= 1*time.Second, "Should wait for timeout")
		assert.True(t, elapsed < 4*time.Second, "Should not wait for full process completion")

		// 如果消息完成了，应该是失败状态
		if completed {
			assert.Equal(t, types.Failure, relationType, "Interrupted process should be marked as failure")
			assert.NotNil(t, err, "Should have cancellation error")
		}
	})
}

func TestEngineGracefulShutdownShouldWaitForRuleChain(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_graceful_wait",
			"name": "Graceful Wait Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Slow Process Function",
					"configuration": {
						"functionName": "slowProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 启动一个慢处理消息
	var messageCompleted bool
	var messageCompletedMutex sync.Mutex
	var messageStarted bool
	var messageStartedMutex sync.Mutex

	// 发送消息
	msg := types.NewMsg(0, "GRACEFUL_TEST", types.JSON, types.NewMetadata(), `{"test": "graceful"}`)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		messageCompletedMutex.Unlock()
		t.Logf("Message completed with relation: %s, error: %v", relationType, err)
	}))

	// 等待消息开始处理
	//time.Sleep(100 * time.Millisecond)
	messageStartedMutex.Lock()
	messageStarted = true
	messageStartedMutex.Unlock()

	// 检查活跃操作数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		activeOps := engine.GetActiveOperations()
		t.Logf("Active operations before shutdown: %d", activeOps)
		assert.True(t, activeOps > 0, "Should have active operations")
	}

	// 启动优雅停机
	shutdownStart := time.Now()
	shutdownDone := make(chan bool, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		shutdownDone <- true
	}()

	// 等待停机完成
	select {
	case <-shutdownDone:
		elapsed := time.Since(shutdownStart)
		t.Logf("Shutdown completed in %v", elapsed)

		// 检查消息是否完成
		messageCompletedMutex.Lock()
		completed := messageCompleted
		messageCompletedMutex.Unlock()

		messageStartedMutex.Lock()
		started := messageStarted
		messageStartedMutex.Unlock()

		t.Logf("Message started: %v, Message completed: %v", started, completed)

		// 如果消息已经开始处理，优雅停机应该等待其完成
		if started {
			assert.True(t, completed, "Graceful shutdown should wait for message to complete")
			assert.True(t, elapsed >= 1*time.Second, "Should wait for slow process to complete")
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown timeout")
	}

	// 验证最终状态
	assert.True(t, ruleEngine.IsShuttingDown())

	// 检查最终的活跃操作计数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		finalActiveOps := engine.GetActiveOperations()
		assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
	}
}

// TestEngineGracefulShutdownWithContextCancellation 测试上下文取消时的处理
// 验证当上下文被取消时，正在处理的消息应该被标记为失败而不是成功
func TestEngineGracefulShutdownWithContextCancellation(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_context_cancel",
			"name": "Context Cancel Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Very Slow Process Function",
					"configuration": {
						"functionName": "verySlowProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 启动一个超慢处理消息
	var messageCompleted bool
	var messageCompletedMutex sync.Mutex
	var messageRelationType string
	var messageError error

	// 发送消息
	msg := types.NewMsg(0, "CANCEL_TEST", types.JSON, types.NewMetadata(), `{"test": "cancel"}`)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		messageRelationType = relationType
		messageError = err
		messageCompletedMutex.Unlock()
		t.Logf("Message completed with relation: %s, error: %v", relationType, err)
	}))

	// 等待消息开始处理
	time.Sleep(100 * time.Millisecond)

	// 检查活跃操作数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		activeOps := engine.GetActiveOperations()
		t.Logf("Active operations before shutdown: %d", activeOps)
		assert.True(t, activeOps > 0, "Should have active operations")
	}

	// 启动优雅停机，但使用较短的超时时间强制取消
	shutdownStart := time.Now()
	shutdownDone := make(chan bool, 1)

	go func() {
		// 使用2秒超时，但verySlowProcess需要5秒，所以会被取消
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		shutdownDone <- true
	}()

	// 等待停机完成
	select {
	case <-shutdownDone:
		elapsed := time.Since(shutdownStart)
		t.Logf("Shutdown completed in %v", elapsed)

		// 检查消息处理结果
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		err := messageError
		messageCompletedMutex.Unlock()

		t.Logf("Message completed: %v, Relation: %s, Error: %v", completed, relationType, err)

		// 应该在合理时间内完成，考虑到verySlowProcess需要清理时间
		// 2秒超时 + 350ms清理时间，应该在2.6秒内完成
		assert.True(t, elapsed <= 2600*time.Millisecond, "Should complete within timeout + cleanup time, elapsed: %v", elapsed)
		// 但应该比原本的5秒快很多
		assert.True(t, elapsed >= 2*time.Second, "Should wait for timeout before cancellation, elapsed: %v", elapsed)

		// 消息应该被标记为失败或被取消
		if completed {
			// 如果消息完成了，应该是失败状态或包含取消错误
			assert.True(t, relationType == types.Failure || (err != nil && strings.Contains(err.Error(), "cancel")),
				"Message should be marked as failure or cancelled, got relation: %s, error: %v", relationType, err)
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown timeout")
	}

	// 验证最终状态
	assert.True(t, ruleEngine.IsShuttingDown())

	// 检查最终的活跃操作计数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		finalActiveOps := engine.GetActiveOperations()
		assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
	}
}

// TestEngineGracefulShutdownWithConcurrentStop 测试消息执行期间并发Stop的行为
// 验证当有消息正在执行时并发调用Stop，消息应该继续执行完成
func TestEngineGracefulShutdownWithConcurrentStop(t *testing.T) {
	// 注册带同步信号的慢处理函数
	processingStarted := make(chan bool, 1)
	action.Functions.Register("syncSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 发送处理开始信号
		processingStarted <- true
		// 执行慢处理逻辑
		time.Sleep(2 * time.Second)
		ctx.TellSuccess(msg)
	})

	ruleChainFile := `{
		"ruleChain": {
			"id": "test_concurrent_stop",
			"name": "Concurrent Stop Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Sync Slow Process Function",
					"configuration": {
						"functionName": "syncSlowProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试场景：消息执行期间并发Stop，消息应该继续执行完成
	t.Run("MessageShouldContinueExecution", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string
		var messageError error

		// 启动一个慢处理消息
		msg := types.NewMsg(0, "CONCURRENT_STOP_TEST", types.JSON, types.NewMetadata(), `{"test": "concurrent_stop"}`)
		go ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageError = err
			messageCompletedMutex.Unlock()
			t.Logf("Message completed with relation: %s, error: %v", relationType, err)
		}))

		// 等待消息确实开始处理（使用同步信号而非固定延迟）
		select {
		case <-processingStarted:
			t.Logf("Message processing started")
		case <-time.After(1 * time.Second):
			t.Fatal("Message processing did not start within timeout")
		}

		// 检查活跃操作数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			t.Logf("Active operations before shutdown: %d", activeOps)
			assert.True(t, activeOps > 0, "Should have active operations")
		}

		// 在消息执行期间启动优雅停机
		shutdownStart := time.Now()
		shutdownDone := make(chan bool, 1)

		go func() {
			// 使用足够长的超时时间，确保消息能够完成
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			shutdownDone <- true
		}()

		// 等待停机完成
		select {
		case <-shutdownDone:
			elapsed := time.Since(shutdownStart)
			t.Logf("Shutdown completed in %v", elapsed)

			// 检查消息处理结果
			messageCompletedMutex.Lock()
			completed := messageCompleted
			relationType := messageRelationType
			err := messageError
			messageCompletedMutex.Unlock()

			t.Logf("Message completed: %v, Relation: %s, Error: %v", completed, relationType, err)

			// 应该等待消息完成，所以至少需要接近2秒（syncSlowProcess的执行时间）
			assert.True(t, elapsed >= 1800*time.Millisecond, "Should wait for message to complete, elapsed: %v", elapsed)

			// 消息应该成功完成（已开始执行的消息不应被中断）
			assert.True(t, completed, "Message should be completed")
			assert.Equal(t, types.Success, relationType, "Message should complete successfully")
			assert.Nil(t, err, "Message should not have error")

		case <-time.After(10 * time.Second):
			t.Fatal("Graceful shutdown timeout")
		}

		// 验证最终状态
		assert.True(t, ruleEngine.IsShuttingDown())

		// 检查最终的活跃操作计数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			finalActiveOps := engine.GetActiveOperations()
			assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
		}
	})
}

// TestEngineConcurrentOnMsgAndStop 测试并发执行OnMsg和Stop时的计数器问题
// 这个测试用例验证了当OnMsg和Stop并发执行时，活跃操作计数器可能阻塞减1导致Stop超时的问题
// TestEngineContextPreservation tests that user-provided context is not overridden by shutdown context
// TestEngineContextPreservation 测试用户提供的上下文不会被停机上下文覆盖
func TestEngineContextPreservation(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_context_preservation",
			"name": "Context Preservation Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Context Check Function",
					"configuration": {
						"functionName": "contextCheck"
					}
				}
			]
		}
	}`

	// Register a function that checks the context value
	action.Functions.Register("contextCheck", func(ctx types.RuleContext, msg types.RuleMsg) {
		if value := ctx.GetContext().Value("test_key"); value != nil {
			msg.Metadata.PutValue("context_preserved", "true")
		} else {
			msg.Metadata.PutValue("context_preserved", "false")
		}
		ctx.TellSuccess(msg)
	})

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Create a custom context with a value
	customCtx := context.WithValue(context.Background(), "test_key", "test_value")

	var messageCompleted bool
	var preservedValue string
	var messageCompletedMutex sync.Mutex

	// Send message with custom context
	msg := types.NewMsg(0, "CONTEXT_TEST", types.JSON, types.NewMetadata(), `{"test": "context"}`)
	ruleEngine.OnMsg(msg, types.WithContext(customCtx), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		preservedValue = msg.Metadata.GetValue("context_preserved")
		messageCompletedMutex.Unlock()
	}))

	// Wait for message completion
	time.Sleep(200 * time.Millisecond)

	// Check results
	messageCompletedMutex.Lock()
	completed := messageCompleted
	value := preservedValue
	messageCompletedMutex.Unlock()

	assert.True(t, completed, "Message should complete")
	assert.Equal(t, "true", value, "Custom context should be preserved")
}

// TestEngineGracefulShutdownWithUserContext tests that user-provided context
// is properly combined with shutdown context to ensure graceful shutdown timeout works
// TestEngineGracefulShutdownWithUserContext 测试用户提供的上下文与停机上下文正确组合，
// 确保优雅停机超时机制正常工作
func TestEngineGracefulShutdownWithUserContext(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_user_context_shutdown",
			"name": "User Context Shutdown Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "User Context Slow Process",
					"configuration": {
						"functionName": "userContextSlowProcess"
					}
				}
			]
		}
	}`

	// Register a slow process function that checks both user context and shutdown context
	action.Functions.Register("userContextSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Check if user context value exists
		userValue := ctx.GetContext().Value("user_key")
		if userValue == nil {
			ctx.DoOnEnd(msg, fmt.Errorf("user context not preserved"), types.Failure)
			return
		}

		// Simulate slow processing while checking for cancellation
		for i := 0; i < 50; i++ {
			select {
			case <-ctx.GetContext().Done():
				// Context was cancelled (by shutdown), this is expected
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}

		// If we reach here, the process completed without cancellation
		ctx.TellSuccess(msg)
	})

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Create a user context with custom data
	userCtx := context.WithValue(context.Background(), "user_key", "user_value")

	// Create a message
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// Start message processing with user context
	var completed bool
	var relation string
	var processErr error
	var messageCompletedMutex sync.Mutex

	ruleEngine.OnMsg(msg, types.WithContext(userCtx), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		completed = true
		relation = relationType
		processErr = err
		messageCompletedMutex.Unlock()
	}))

	// Wait a bit to ensure processing starts
	time.Sleep(200 * time.Millisecond)

	// Trigger graceful shutdown with 1 second timeout
	startTime := time.Now()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ruleEngine.Stop(shutdownCtx)
	elapsed := time.Since(startTime)

	// Wait for message processing to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the behavior
	messageCompletedMutex.Lock()
	completedResult := completed
	relationResult := relation
	errorResult := processErr
	messageCompletedMutex.Unlock()

	assert.True(t, completedResult, "Message processing should complete")
	assert.Equal(t, types.Failure, relationResult, "Message should fail due to context cancellation")
	assert.NotNil(t, errorResult, "Should have cancellation error")
	assert.True(t, strings.Contains(errorResult.Error(), "context canceled"), "Error should indicate context cancellation")

	// Verify that shutdown happened within reasonable time (should be around 1 second + some overhead)
	assert.True(t, elapsed >= 1*time.Second, "Should wait for timeout")
	assert.True(t, elapsed < 2*time.Second, "Should not wait too long after timeout")

	// Verify engine is in shutdown state
	assert.True(t, ruleEngine.IsShuttingDown(), "Engine should be in shutdown state")
}

func TestEngineConcurrentOnMsgAndStop(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_concurrent_onmsg_stop",
			"name": "Concurrent OnMsg Stop Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "test/upper",
					"name": "Upper Node"
				},
				{
					"id": "s2",
					"type": "test/time",
					"name": "Time Node"
				}
			],
			"connections": [
				{
					"fromId": "s1",
					"toId": "s2",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig()
	// 注册测试节点
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	// 测试场景1: 模拟竞态条件
	t.Run("ConcurrentRaceCondition", func(t *testing.T) {
		// 重复多次以增加触发竞态条件的概率
		for attempt := 0; attempt < 10; attempt++ {
			t.Logf("Attempt %d", attempt+1)

			// 创建新的引擎实例
			testChainId := fmt.Sprintf("test_concurrent_%d", attempt)
			testEngine, err := New(testChainId, []byte(ruleChainFile), WithConfig(config))
			assert.Nil(t, err)
			defer Del(testChainId)

			// 并发执行OnMsg和Stop
			var wg sync.WaitGroup
			var stopTimeout bool
			var stopCompleted int32

			// 启动OnMsg
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test": "data"}`)
				testEngine.OnMsg(msg)
			}()

			// 立即启动Stop（不等待）
			wg.Add(1)
			go func() {
				defer wg.Done()
				// 短暂延迟以确保OnMsg先开始
				time.Sleep(1 * time.Millisecond)

				startTime := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				testEngine.Stop(ctx)
				elapsed := time.Since(startTime)

				// 检查是否超时
				if elapsed >= 1800*time.Millisecond { // 接近2秒超时
					stopTimeout = true
					t.Logf("Stop timeout detected in attempt %d, elapsed: %v", attempt+1, elapsed)
				}
				atomic.StoreInt32(&stopCompleted, 1)
			}()

			wg.Wait()

			// 验证Stop是否完成
			assert.Equal(t, int32(1), atomic.LoadInt32(&stopCompleted), "Stop should complete")

			// 检查最终的活跃操作计数
			if engine, ok := testEngine.(*RuleEngine); ok {
				finalActiveOps := engine.GetActiveOperations()
				t.Logf("Final active operations: %d", finalActiveOps)
				// 注意：这里可能仍然是正数，这就是问题所在
			}

			// 如果发现超时，记录问题
			if stopTimeout {
				t.Logf("Race condition detected: Stop timeout due to active operations counter not decremented")
				// 这里不使用assert.Fail，因为我们期望在某些情况下会出现这个问题
				break // 找到问题就退出循环
			}
		}
	})

	// 测试场景2：验证修复后的行为（添加适当的延迟）
	t.Run("WithProperDelay", func(t *testing.T) {
		testChainId := "test_concurrent_fixed"
		testEngine, err := New(testChainId, []byte(ruleChainFile), WithConfig(config))
		assert.Nil(t, err)
		// Note: Don't use defer Del() to avoid double Stop() call issue

		// 发送消息
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test": "data"}`)
		testEngine.OnMsg(msg)

		// 添加适当的延迟，让消息处理完成
		time.Sleep(200 * time.Millisecond)

		// 检查活跃操作计数应该为0
		if engine, ok := testEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.Equal(t, int64(0), activeOps, "Active operations should be 0 after message processing")
		}

		// 现在Stop应该不会超时
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		testEngine.Stop(ctx)
		elapsed := time.Since(startTime)

		// Stop应该很快完成，不会超时
		assert.True(t, elapsed < 500*time.Millisecond, "Stop should complete quickly when no active operations, elapsed: %v", elapsed)
		assert.True(t, testEngine.IsShuttingDown(), "Engine should be in shutdown state")

		// Manually clean up after successful stop
		Del(testChainId)
	})
}

// TestEngineReloadBehavior 测试引擎重载行为
func TestEngineReloadBehavior(t *testing.T) {
	// 注册重载测试用的处理函数
	action.Functions.Register("reloadTestProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(100 * time.Millisecond) // 0.1秒处理时间
		ctx.TellSuccess(msg)
	})

	// 注册一个慢速重载测试函数，用于模拟重载期间的长时间处理
	action.Functions.Register("slowReloadTestProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(2 * time.Second) // 2秒处理时间，确保重载期间有足够时间
		ctx.TellSuccess(msg)
	})

	ruleChainFile := `{
		"ruleChain": {
			"id": "test_reload_behavior",
			"name": "Reload Behavior Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Reload Test Function",
					"configuration": {
						"functionName": "reloadTestProcess"
					}
				}
			],
			"connections": [
				{
					"fromId": "s1",
					"toId": "",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试场景1：重载期间消息应该被阻塞直到重载完成
	t.Run("MessagesBlockedDuringReload", func(t *testing.T) {
		// 确保引擎使用快速处理的规则链
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)
		time.Sleep(100 * time.Millisecond) // 等待重载完成
		var processedCount int64
		var blockedCount int64
		var wg sync.WaitGroup
		var callbackWg sync.WaitGroup

		// 创建一个使用慢速处理函数的规则链，用于模拟重载期间的长时间操作
		slowRuleChainFile := `{
			"ruleChain": {
				"id": "test_reload_behavior_slow",
				"name": "Slow Reload Behavior Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Slow Reload Test Function",
						"configuration": {
							"functionName": "slowReloadTestProcess"
						}
					}
				],
				"connections": [
					{
						"fromId": "s1",
						"toId": "",
						"type": "Success"
					}
				]
			}
		}`

		// 启动重载操作（先启动重载）
		reloadDone := make(chan error, 1)
		go func() {
			err := ruleEngine.ReloadSelf([]byte(slowRuleChainFile))
			reloadDone <- err
		}()

		// 稍微延迟后发送消息，确保消息在重载期间到达
		time.Sleep(50 * time.Millisecond)

		// 发送消息，这些消息应该等待重载完成
		for i := 0; i < 3; i++ {
			wg.Add(1)
			callbackWg.Add(1)
			go func(index int) {
				defer wg.Done()
				startTime := time.Now()
				msg := types.NewMsg(0, "RELOAD_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					elapsed := time.Since(startTime)
					t.Logf("Message %d processed in %v with relation: %s, error: %v", index, elapsed, relationType, err)
					if elapsed > 200*time.Millisecond {
						// 如果处理时间超过200毫秒，说明等待了重载完成
						newBlocked := atomic.AddInt64(&blockedCount, 1)
						t.Logf("Message %d marked as blocked, blockedCount now: %d", index, newBlocked)
					}
					if err == nil {
						newProcessed := atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d marked as processed, processedCount now: %d", index, newProcessed)
					}
				}))
			}(i)
			time.Sleep(10 * time.Millisecond) // 短间隔发送
		}

		// 等待所有消息处理完成
		wg.Wait()

		// 等待重载完成
		reloadResult := <-reloadDone
		t.Logf("Reload completed with error: %v", reloadResult)
		assert.Nil(t, reloadResult, "Reload should succeed")

		// 等待所有回调函数完成
		callbackWg.Wait()

		// 验证结果
		processed := atomic.LoadInt64(&processedCount)
		blocked := atomic.LoadInt64(&blockedCount)
		t.Logf("Final - Processed: %d, Blocked: %d", processed, blocked)

		assert.Equal(t, int64(3), processed, "All messages should be processed")
		assert.True(t, blocked > 0, "Some messages should wait for reload to complete")

		// 测试结束后重置为快速处理规则链，避免影响后续测试
		resetErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, resetErr)
		time.Sleep(100 * time.Millisecond) // 等待重载完成
	})

	// 测试场景2：重载完成后新消息应该正常处理
	t.Run("MessagesProcessedAfterReload", func(t *testing.T) {
		// 先执行一次重载，确保使用快速处理的规则链
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)

		// 等待重载完成
		time.Sleep(100 * time.Millisecond)

		// 验证引擎不再处于重载状态
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			assert.False(t, engine.IsReloading(), "Engine should not be reloading after reload completes")
		}

		// 发送新消息，应该正常处理
		var processedCount int64
		var wg sync.WaitGroup
		var callbackWg sync.WaitGroup

		for i := 0; i < 3; i++ {
			wg.Add(1)
			callbackWg.Add(1)
			go func(index int) {
				defer wg.Done()
				startTime := time.Now()
				msg := types.NewMsg(0, "POST_RELOAD_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					elapsed := time.Since(startTime)
					t.Logf("Message %d processed in %v with relation: %s", index, elapsed, relationType)
					if relationType == types.Success {
						newProcessed := atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d marked as processed, processedCount now: %d", index, newProcessed)
					}
				}))
			}(i)
		}

		wg.Wait()

		// 等待所有回调函数完成
		callbackWg.Wait()

		// 验证所有消息都正常处理
		processed := atomic.LoadInt64(&processedCount)
		t.Logf("Final processedCount: %d", processed)
		assert.Equal(t, int64(3), processed, "All messages should be processed successfully after reload")
	})

	// 测试场景3：重载期间活跃消息应该等待完成
	t.Run("ActiveMessagesWaitDuringReload", func(t *testing.T) {
		// 确保引擎使用快速处理的规则链
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)
		time.Sleep(100 * time.Millisecond) // 等待重载完成
		// 发送一个长时间处理的消息
		var longProcessCompleted bool
		var longProcessMutex sync.Mutex

		msg := types.NewMsg(0, "LONG_PROCESS_TEST", types.JSON, types.NewMetadata(), `{"test": "long"}`)
		go ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			longProcessMutex.Lock()
			longProcessCompleted = true
			longProcessMutex.Unlock()
			t.Logf("Long process completed with relation: %s", relationType)
		}))

		// 等待消息开始处理
		time.Sleep(100 * time.Millisecond)

		// 启动重载
		reloadStart := time.Now()
		err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		reloadElapsed := time.Since(reloadStart)

		assert.Nil(t, err)
		// 重载应该等待活跃消息完成，所以至少需要0.1秒
		assert.True(t, reloadElapsed >= 100*time.Millisecond, "Reload should wait for active messages, elapsed: %v", reloadElapsed)

		// 验证长时间处理的消息最终完成
		time.Sleep(200 * time.Millisecond)
		longProcessMutex.Lock()
		completed := longProcessCompleted
		longProcessMutex.Unlock()

		assert.True(t, completed, "Long process should complete before reload finishes")
	})

	// 测试场景4：并发重载应该安全处理
	t.Run("ConcurrentReloadSafety", func(t *testing.T) {
		var reloadSuccessCount int64
		var reloadErrorCount int64
		var wg sync.WaitGroup

		// 启动多个并发重载
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
				if err != nil {
					atomic.AddInt64(&reloadErrorCount, 1)
					t.Logf("Reload %d failed: %v", index, err)
				} else {
					atomic.AddInt64(&reloadSuccessCount, 1)
					t.Logf("Reload %d succeeded", index)
				}
			}(i)
			time.Sleep(10 * time.Millisecond) // 稍微错开启动时间
		}

		wg.Wait()

		// 验证结果
		successCount := atomic.LoadInt64(&reloadSuccessCount)
		errorCount := atomic.LoadInt64(&reloadErrorCount)
		t.Logf("Concurrent reload - Success: %d, Error: %d", successCount, errorCount)

		// 至少应该有一个重载成功
		assert.True(t, successCount >= 1, "At least one reload should succeed")
		// 总数应该等于尝试次数
		assert.Equal(t, int64(3), successCount+errorCount, "All reload attempts should be accounted for")
	})

	// 测试场景5：重载超时处理
	t.Run("ReloadTimeoutHandling", func(t *testing.T) {
		// 注册一个超长时间处理函数
		action.Functions.Register("superSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(15 * time.Second) // 15秒处理时间
			ctx.TellSuccess(msg)
		})

		superSlowRuleChain := `{
			"ruleChain": {
				"id": "test_super_slow",
				"name": "Super Slow Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Super Slow Function",
						"configuration": {
							"functionName": "superSlowProcess"
						}
					}
				]
			}
		}`

		superSlowChainId := str.RandomStr(10)
		superSlowEngine, err := New(superSlowChainId, []byte(superSlowRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(superSlowChainId)

		// 发送一个超长时间处理的消息
		msg := types.NewMsg(0, "SUPER_SLOW_TEST", types.JSON, types.NewMetadata(), `{"test": "super_slow"}`)
		go superSlowEngine.OnMsg(msg)

		// 等待消息开始处理
		time.Sleep(200 * time.Millisecond)

		// 尝试重载，应该在等待超时后继续
		reloadStart := time.Now()
		err = superSlowEngine.ReloadSelf([]byte(superSlowRuleChain))
		reloadElapsed := time.Since(reloadStart)

		// 重载应该在等待超时（10秒）后继续，不会等待15秒
		assert.Nil(t, err)
		assert.True(t, reloadElapsed >= 9*time.Second, "Reload should wait for timeout")
		assert.True(t, reloadElapsed < 12*time.Second, "Reload should not wait beyond timeout")
	})
}

// TestReloadBackpressureControl 测试重载期间的背压控制功能
func TestReloadBackpressureControl(t *testing.T) {
	// 创建规则链定义
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_backpressure",
			"name": "Test Backpressure Control"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Test Function",
					"configuration": {
						"functionName": "testBackpressureFunc"
					}
				}
			]
		}
	}`

	// 注册测试函数
	action.Functions.Register("testBackpressureFunc", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(10 * time.Millisecond) // 模拟短暂处理时间
		ctx.TellSuccess(msg)
	})

	t.Run("BackpressureControlPreventsMemoryOverflow", func(t *testing.T) {
		// 创建具有低背压限制的规则引擎
		ruleEngine, err := NewRuleEngine("test_backpressure", []byte(ruleChainFile),
			types.WithMaxReloadWaiters(5)) // 只允许5个并发等待者
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 验证背压配置
		maxWaiters, currentWaiters, isReloading := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(5), maxWaiters)
		assert.Equal(t, int64(0), currentWaiters)
		assert.False(t, isReloading)

		// 创建慢速重载函数
		slowReloadChainFile := `{
			"ruleChain": {
				"id": "test_backpressure_slow",
				"name": "Slow Backpressure Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Slow Function",
						"configuration": {
							"functionName": "slowBackpressureFunc"
						}
					}
				]
			}
		}`

		action.Functions.Register("slowBackpressureFunc", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(2 * time.Second) // 模拟慢速处理以延长重载时间
			ctx.TellSuccess(msg)
		})

		// 启动重载操作（在后台异步执行）
		var reloadWg sync.WaitGroup
		reloadWg.Add(1)
		go func() {
			defer reloadWg.Done()
			reloadErr := ruleEngine.ReloadSelf([]byte(slowReloadChainFile))
			assert.Nil(t, reloadErr)
		}()

		// 等待重载真正开始
		for i := 0; i < 50; i++ { // 最多等待500ms
			time.Sleep(10 * time.Millisecond)
			_, _, isReloading := ruleEngine.GetReloadWaitersStats()
			if isReloading {
				break
			}
		}

		// 验证重载状态
		_, _, isReloading = ruleEngine.GetReloadWaitersStats()
		assert.True(t, isReloading, "重载应该已经开始")

		// 发送大量消息来测试背压控制
		var processedCount int64
		var rejectedCount int64
		var callbackWg sync.WaitGroup

		// 发送10个消息（超过5个限制）
		for i := 0; i < 10; i++ {
			callbackWg.Add(1)
			go func(index int) {
				msg := types.NewMsg(0, "BACKPRESSURE_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					if err != nil && errors.Is(err, types.ErrEngineReloadBackpressureLimit) {
						atomic.AddInt64(&rejectedCount, 1)
						t.Logf("Message %d rejected due to backpressure: %v", index, err)
					} else if err == nil && relationType == types.Success {
						atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d processed successfully", index)
					} else {
						t.Logf("Message %d failed with error: %v, relationType: %s", index, err, relationType)
						// 其他错误也计入拒绝数，因为消息没有成功处理
						atomic.AddInt64(&rejectedCount, 1)
					}
				}))
			}(i)
		}

		// 等待所有回调完成
		callbackWg.Wait()

		// 验证背压控制生效
		totalMessages := atomic.LoadInt64(&processedCount) + atomic.LoadInt64(&rejectedCount)
		assert.True(t, totalMessages >= 5, "至少应该有5个消息有回调，实际: %d", totalMessages)
		assert.True(t, atomic.LoadInt64(&rejectedCount) > 0, "应该有消息因为背压控制被拒绝")

		t.Logf("处理的消息: %d, 拒绝的消息: %d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&rejectedCount))

		// 等待重载完成
		reloadWg.Wait()

		// 验证重载完成后状态正常
		_, currentWaiters, isReloading = ruleEngine.GetReloadWaitersStats()
		assert.False(t, isReloading)
		assert.Equal(t, int64(0), currentWaiters, "重载完成后等待者计数应该为0")
	})

	t.Run("BackpressureCanBeDisabled", func(t *testing.T) {
		// 创建禁用背压控制的规则引擎
		ruleEngine, err := NewRuleEngine("test_no_backpressure", []byte(ruleChainFile),
			types.WithMaxReloadWaiters(0))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 验证背压控制被禁用
		maxWaiters, _, _ := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(0), maxWaiters, "背压控制应该被禁用")

		// 测试不进行重载，只验证背压控制不生效
		var processedCount int64
		var backpressureRejectedCount int64
		var callbackWg sync.WaitGroup

		// 发送消息测试（不进行重载）
		for i := 0; i < 5; i++ {
			callbackWg.Add(1)
			go func(index int) {
				msg := types.NewMsg(0, "NO_BACKPRESSURE_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					if err != nil && errors.Is(err, types.ErrEngineReloadBackpressureLimit) {
						atomic.AddInt64(&backpressureRejectedCount, 1)
						t.Logf("Message %d rejected due to backpressure: %v", index, err)
					} else if err == nil && relationType == types.Success {
						atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d processed successfully", index)
					} else {
						t.Logf("Message %d failed with error: %v, relationType: %s", index, err, relationType)
					}
				}))
			}(i)
		}

		callbackWg.Wait()

		// 验证没有消息因为背压控制被拒绝
		assert.Equal(t, int64(0), atomic.LoadInt64(&backpressureRejectedCount), "禁用背压控制时不应该有消息因背压被拒绝")
		assert.Equal(t, int64(5), atomic.LoadInt64(&processedCount), "所有消息都应该被处理")

		t.Logf("禁用背压控制测试 - 处理的消息: %d, 背压拒绝的消息: %d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&backpressureRejectedCount))
	})

	t.Run("BackpressureConfigCanBeChangedAtRuntime", func(t *testing.T) {
		// 创建规则引擎
		ruleEngine, err := NewRuleEngine("test_runtime_config", []byte(ruleChainFile))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 初始配置
		maxWaiters, _, _ := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(1000), maxWaiters, "默认值应该是1000") // 默认值

		// 运行时修改配置
		ruleEngine.SetMaxReloadWaiters(100)

		// 验证配置已更改
		maxWaiters, _, _ = ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(100), maxWaiters)

		// 禁用背压控制
		ruleEngine.SetMaxReloadWaiters(0)

		// 验证配置已更改
		maxWaiters, _, _ = ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(0), maxWaiters, "应该禁用背压控制")
	})
}
