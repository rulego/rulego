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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
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
		ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {

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
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
	}))
	time.Sleep(time.Millisecond * 200)
	assert.True(t, atomic.LoadInt32(&debugDone) == 1)
}

// 测试获取节点
func TestGetNodeId(t *testing.T) {
	parser := JsonParser{}
	def, _ := parser.DecodeRuleChain([]byte(ruleChainFile))
	ctx, err := InitRuleChainCtx(NewConfig(), nil, &def)
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
			ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
			ruleEngine.OnMsg(msg, types.WithContext(context.WithValue(context.Background(), shareKey, shareValue+strconv.Itoa(index))), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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

// TestOnMsgAndWait 测试同步执行规则链
func TestOnMsgAndWait(t *testing.T) {
	var wg sync.WaitGroup

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		wg.Done()
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_wait.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	_, err = New("sub_chain_02", loadFile("./sub_chain.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//TEST_MSG_TYPE1 找到2条chains,5个nodes
	wg.Add(10)
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var count int32
	ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		atomic.AddInt32(&count, 1)
	}))
	assert.Equal(t, int32(2), count)
	wg.Wait()

	//TEST_MSG_TYPE2 找到1条chain,2个nodes
	wg.Add(4)
	count = 0
	msg = types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		atomic.AddInt32(&count, 1)
	}))

	assert.Equal(t, int32(1), count)
	wg.Wait()

	//TEST_MSG_TYPE3 找到0条chain,1个node
	wg.Add(2)
	count = 0
	msg = types.NewMsg(0, "TEST_MSG_TYPE3", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		atomic.AddInt32(&count, 1)
	}))
	assert.Equal(t, int32(1), count)
	wg.Wait()
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
		ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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

	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	ruleEngine.OnMsg(msg1, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "true", msg.Metadata.GetValue("result"))
	}))

	time.Sleep(time.Millisecond * 200)

	chainJsonFile1 := string(loadFile("./test_group_filter_node.json"))
	newChainJsonFile1 := strings.Replace(chainJsonFile1, `"allMatches": false`, `"allMatches": true`, -1)
	newChainJsonFile1 = strings.Replace(newChainJsonFile1, "test_group_filter_node", chainId, -1)
	//更新规则链，groupFilter必须所有节点都满足True,才走True链
	_ = ruleEngine.ReloadSelf([]byte(newChainJsonFile1))

	ruleEngine.OnMsg(msg1, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "false", msg.Metadata.GetValue("result"))
	}))

	msg2 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":52,\"humidity\":90}")
	ruleEngine.OnMsg(msg2, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "true", msg.Metadata.GetValue("result"))
	}))

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
			assert.Equal(t, "ruleChain id=notfound not found", err.Error())
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
		}), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
		ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
			atomic.AddInt32(&count, 1)
		}))
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

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
}

func TestDisabled(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())
	defStr := ruleChainFile

	e, err := New("testDisabled1", []byte(defStr), WithConfig(config))
	assert.Nil(t, err)

	defStr = strings.Replace(defStr, "\"disabled\": false", "\"disabled\": true", -1)
	err = e.ReloadSelf([]byte(defStr))
	assert.Equal(t, ErrDisabled.Error(), err.Error())

	defStr = strings.Replace(defStr, "\"disabled\": true", "\"disabled\": false", -1)
	err = e.ReloadSelf([]byte(defStr))
	assert.Nil(t, err)

	err = e.Reload()
	assert.Nil(t, err)

	defStr = strings.Replace(ruleChainFile, "\"disabled\": false", "\"disabled\": true", -1)
	_, err = New("testDisabled2", []byte(defStr), WithConfig(config))
	assert.Equal(t, ErrDisabled.Error(), err.Error())
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

				ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error) {
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

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error) {
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
