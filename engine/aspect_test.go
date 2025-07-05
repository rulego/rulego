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
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
)

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
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		//进入故障降级，跳过该组件
		//fmt.Printf("第4次耗时:%s", time.Since(start4).String())
		//fmt.Println()
		assert.True(t, time.Since(start4) < time.Second)
	}))

	//等待恢复时间
	time.Sleep(time.Second * 11)

	start5 := time.Now()
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
	chainId := "test01"
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
	ruleEngine, err := DefaultPool.New(chainId, []byte(ruleChainFile), WithConfig(config), types.WithAspects(&NodeAspect2{Name: "NodeAspect2"}, &NodeAspect1{Name: "NodeAspect1"},
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
	err = ruleEngine.ReloadSelf([]byte(ruleChainFile))
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

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
