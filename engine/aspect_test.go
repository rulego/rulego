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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
	start = time.Now()
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		//进入故障降级，跳过该组件
		//fmt.Printf("第4次耗时:%s", time.Since(start).String())
		//fmt.Println()
		assert.True(t, time.Since(start) < time.Second)
	}))

	//等待恢复时间
	time.Sleep(time.Second * 11)

	start = time.Now()
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		//故障恢复，执行该组件
		//fmt.Printf("第5次耗时:%s", time.Since(start).String())
		//fmt.Println()
		assert.True(t, time.Since(start) > time.Second)
	}))

	ruleEngine.OnMsg(msg)
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second * 11)
	//更新规则链，清除错误信息
	ruleEngine.ReloadSelf(loadFile("./test_skip_fallback_aspect.json"))

	start = time.Now()
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		//故障恢复，执行该组件
		//fmt.Printf("第6次耗时:%s", time.Since(start).String())
		//fmt.Println()
		assert.True(t, time.Since(start) > time.Second)
	}))

	time.Sleep(time.Second * 3)
	ruleEngine.Stop()

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
	var onCompleted = false
	callback.OnCompleted = func(ctx types.RuleContext, msg types.RuleMsg) {
		onCompleted = true
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
	ruleEngine.Stop()
	time.Sleep(time.Millisecond * 200)

	assert.True(t, onCompleted)

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
	ruleEngine.Stop()
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

func (aspect *EngineAspect) OnChainBeforeInit(def *types.RuleChain) error {
	return nil
}

func (aspect *EngineAspect) OnNodeBeforeInit(def *types.RuleNode) error {
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
func (aspect *ChainAspect) Start(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	msg.Metadata.PutValue("key1", "addValueOnStart")
	return msg
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

func (aspect *BeforeCreateErrAspect) OnChainBeforeInit(def *types.RuleChain) error {
	atomic.AddInt32(aspect.Count, 1)
	if def != nil {
		if def.RuleChain.ID == "test02" {
			return fmt.Errorf("crate error break")
		}
	}
	return nil
}

func (aspect *BeforeCreateErrAspect) OnNodeBeforeInit(def *types.RuleNode) error {
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
