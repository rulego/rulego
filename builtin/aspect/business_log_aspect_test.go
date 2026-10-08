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

package aspect

import (
	"errors"
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

type fakeTemplate struct {
	err error
	val string
}

func (f *fakeTemplate) Parse() error                                         { return nil }
func (f *fakeTemplate) Execute(map[string]any) (interface{}, error)          { return f.val, f.err }
func (f *fakeTemplate) ExecuteFn(func() map[string]any) (interface{}, error) { return f.val, f.err }
func (f *fakeTemplate) ExecuteAsString(map[string]any) string                { return f.val }
func (f *fakeTemplate) HasVar() bool                                         { return true }

type fakeEngine struct {
	types.RuleEngine
	mu       sync.Mutex
	received []types.RuleMsg
}

func (f *fakeEngine) OnMsg(msg types.RuleMsg, opts ...types.RuleContextOption) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, msg)
}

type fakePool struct {
	types.RuleEnginePool
	engine *fakeEngine
	found  bool
}

func (f *fakePool) Get(string) (types.RuleEngine, bool) {
	if f.found {
		return f.engine, true
	}
	return nil, false
}

// logTestCtx 提供 Before/After/Completed 所需的最小上下文：OnDebug 记录事件、
// SubmitTask 同步执行（测试确定性）、RuleChain 实现 ChainCtx 以驱动处理链出口。
type logEventRecord struct {
	chainId, flowType, nodeId, relationType string
	err                                     error
	msg                                     types.RuleMsg
}

type logTestCtx struct {
	types.RuleContext
	chain *stubChainCtx
	pool  *fakePool
	mu    sync.Mutex
	logs  []logEventRecord
}

type stubChainCtx struct {
	types.ChainCtx
	pool types.RuleEnginePool
}

func (s *stubChainCtx) GetNodeId() types.RuleNodeId {
	return types.RuleNodeId{Id: "chain1", Type: types.CHAIN}
}
func (s *stubChainCtx) GetRuleEnginePool() types.RuleEnginePool { return s.pool }

func (c *logTestCtx) RuleChain() types.NodeCtx { return c.chain }
func (c *logTestCtx) Self() types.NodeCtx {
	return &stubNodeCtx{nodeId: types.RuleNodeId{Id: "node1", Type: types.NODE}}
}
func (c *logTestCtx) OnDebug(chainId, flowType, nodeId string, msg types.RuleMsg, relationType string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, logEventRecord{chainId: chainId, flowType: flowType, nodeId: nodeId, relationType: relationType, err: err, msg: msg})
}
func (c *logTestCtx) SubmitTask(task func()) { task() }
func (c *logTestCtx) GetEnv(msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	md := map[string]interface{}{}
	if msg.Metadata != nil {
		for k, v := range msg.Metadata.Values() {
			md[k] = v
		}
	}
	return map[string]interface{}{
		"metadata": md,
		"global":   map[string]string{"apiKey": "sk-secret-123", "env": "prod"},
	}
}
func (c *logTestCtx) NewMsg(msgType string, metaData *types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}

func newLogTestCtx() *logTestCtx {
	return &logTestCtx{
		RuleContext: test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {}),
		chain:       &stubChainCtx{},
	}
}

func (c *logTestCtx) logCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.logs)
}

func businessLogChainDef(handler string, logEvents interface{}) *types.RuleChain {
	def := &types.RuleChain{}
	def.RuleChain.Configuration = types.Configuration{}
	if handler != "" {
		def.RuleChain.Configuration[types.LogHandler] = handler
	}
	if logEvents != nil {
		def.RuleChain.Configuration[types.LogEvents] = logEvents
	}
	return def
}

func TestBusinessLogInit(t *testing.T) {
	a := (&BusinessLog{}).New().(*BusinessLog)
	assert.Equal(t, 950, a.Order())
	assert.Equal(t, "businessLog", a.Type())

	// logEvents 的 JSON 解码形态 []interface{} 与代码构造形态 []string 都要识别
	assert.Nil(t, a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("h1", []interface{}{"chainEnd"})))
	st := a.loadState()
	assert.Equal(t, "h1", st.handlerId)
	assert.True(t, st.chainEnd)

	// 未知事件值忽略
	a2 := (&BusinessLog{}).New().(*BusinessLog)
	assert.Nil(t, a2.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", []string{"nodeError"})))
	st2 := a2.loadState()
	assert.Equal(t, "", st2.handlerId)
	assert.False(t, st2.chainEnd)

	// 节点模板编译进快照；无模板的节点不进 nodes/names
	assert.Nil(t, a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
		LogConfig: &types.NodeLogConfig{Before: "enter ${metadata.k}", After: "done"}}))
	assert.Nil(t, a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node2", Name: "N2"}))
	st = a.loadState()
	assert.NotNil(t, st.nodes["node1"])
	assert.NotNil(t, st.nodes["node1"].before)
	assert.Equal(t, "enter ${metadata.k}", st.nodes["node1"].beforeRaw)
	assert.Equal(t, "", st.names["node2"])
}

func TestBusinessLogPointCut(t *testing.T) {
	a := (&BusinessLog{}).New().(*BusinessLog)
	assert.Nil(t, a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil)))
	ctx := newLogTestCtx()
	// 无任何配置：不命中
	assert.False(t, a.PointCut(ctx, types.RuleMsg{}, ""))

	assert.Nil(t, a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
		LogConfig: &types.NodeLogConfig{Before: "b"}}))
	// 配置节点命中
	assert.True(t, a.PointCut(ctx, types.RuleMsg{}, ""))

	// chainEnd 开启时全节点命中
	a3 := (&BusinessLog{}).New().(*BusinessLog)
	_ = a3.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", []string{"chainEnd"}))
	assert.True(t, a3.PointCut(ctx, types.RuleMsg{}, ""))
}

func TestBusinessLogBeforeAfter(t *testing.T) {
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "hello")
	msg.Metadata.PutValue("k", "v")

	t.Run("before/after render", func(t *testing.T) {
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil))
		_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
			LogConfig: &types.NodeLogConfig{Before: "enter ${metadata.k}", After: "leave"}})
		ctx := newLogTestCtx()
		a.Before(ctx, msg, "")
		a.After(ctx, msg, nil, types.Success)
		assert.Equal(t, 2, ctx.logCount())
		assert.Equal(t, "enter v", ctx.logs[0].msg.GetData())
		assert.Equal(t, phaseIn, ctx.logs[0].msg.Metadata.GetValue(MetaLogPhase))
		assert.Equal(t, "node1", ctx.logs[0].msg.Metadata.GetValue(MetaLogNodeId))
		assert.Equal(t, "N1", ctx.logs[0].msg.Metadata.GetValue(MetaLogNodeName))
		assert.Equal(t, "1", ctx.logs[0].msg.Metadata.GetValue(MetaLogEventFlag))
		assert.Equal(t, types.MsgTypeLog, ctx.logs[0].msg.Type)
		assert.Equal(t, types.Log, ctx.logs[0].flowType)
		assert.Equal(t, "leave", ctx.logs[1].msg.GetData())
		assert.Equal(t, phaseOut, ctx.logs[1].msg.Metadata.GetValue(MetaLogPhase))
		assert.Equal(t, types.Success, ctx.logs[1].msg.Metadata.GetValue(MetaLogRelation))
	})

	t.Run("only before", func(t *testing.T) {
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil))
		_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
			LogConfig: &types.NodeLogConfig{Before: "b"}})
		ctx := newLogTestCtx()
		a.Before(ctx, msg, "")
		a.After(ctx, msg, nil, types.Success)
		assert.Equal(t, 1, ctx.logCount())
	})

	t.Run("failure keeps after event with error", func(t *testing.T) {
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil))
		_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
			LogConfig: &types.NodeLogConfig{After: "fail:${metadata.errorMsg}"}})
		ctx := newLogTestCtx()
		failMsg := msg.Copy()
		failMsg.Metadata.PutValue("errorMsg", "boom")
		failMsg.Metadata.PutValue("k", "v")
		a.After(ctx, failMsg, errors.New("boom"), types.Failure)
		assert.Equal(t, 1, ctx.logCount())
		assert.Equal(t, "fail:boom", ctx.logs[0].msg.GetData())
		assert.Equal(t, "boom", ctx.logs[0].msg.Metadata.GetValue(MetaLogError))
		assert.Equal(t, types.Failure, ctx.logs[0].msg.Metadata.GetValue(MetaLogRelation))
	})

	t.Run("log event messages never emit again", func(t *testing.T) {
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", []string{"chainEnd"}))
		_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
			LogConfig: &types.NodeLogConfig{Before: "b", After: "a"}})
		ctx := newLogTestCtx()
		eventMsg := msg.Copy()
		eventMsg.Metadata.PutValue(MetaLogEventFlag, "1")
		a.Before(ctx, eventMsg, "")
		a.After(ctx, eventMsg, errors.New("e"), types.Failure)
		assert.Equal(t, 0, ctx.logCount())
	})
}

func TestBusinessLogChainEnd(t *testing.T) {
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "x")

	t.Run("data is terminal message data, relation marks outcome", func(t *testing.T) {
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", []string{"chainEnd"}))
		ctx := newLogTestCtx()
		a.End(ctx, msg, nil, types.Success)
		assert.Equal(t, 1, ctx.logCount())
		// Data 为触发点消息数据
		assert.Equal(t, "x", ctx.logs[0].msg.GetData())
		assert.Equal(t, scopeChain, ctx.logs[0].msg.Metadata.GetValue(MetaLogScope))
		assert.Equal(t, phaseEnd, ctx.logs[0].msg.Metadata.GetValue(MetaLogPhase))
		assert.Equal(t, types.Success, ctx.logs[0].msg.Metadata.GetValue(MetaLogRelation))
		assert.Equal(t, "", ctx.logs[0].msg.Metadata.GetValue(MetaLogError))

		a2 := (&BusinessLog{}).New().(*BusinessLog)
		_ = a2.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", []string{"chainEnd"}))
		ctx2 := newLogTestCtx()
		a2.End(ctx2, msg, errors.New("boom"), types.Failure)
		assert.Equal(t, 1, ctx2.logCount())
		assert.Equal(t, "x", ctx2.logs[0].msg.GetData())
		assert.Equal(t, "boom", ctx2.logs[0].msg.Metadata.GetValue(MetaLogError))
		assert.Equal(t, types.Failure, ctx2.logs[0].msg.Metadata.GetValue(MetaLogRelation))
	})

	t.Run("each terminal branch emits its own event", func(t *testing.T) {
		// 每个终点各发一条，不去重
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", []string{"chainEnd"}))
		ctx := newLogTestCtx()
		a.End(ctx, msg, nil, types.Success)
		a.End(ctx, msg, nil, types.Success)
		assert.Equal(t, 2, ctx.logCount())
	})

	t.Run("chainEnd off emits nothing", func(t *testing.T) {
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil))
		ctx := newLogTestCtx()
		a.End(ctx, msg, nil, types.Success)
		assert.Equal(t, 0, ctx.logCount())
	})
}

func TestBusinessLogRenderFallback(t *testing.T) {
	// 渲染失败：Data 回退模板原文 + renderError
	nl := &nodeLog{before: &fakeTemplate{err: errors.New("bad expr")}, beforeRaw: "raw ${x}"}
	a := (&BusinessLog{}).New().(*BusinessLog)
	_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil))
	cur := a.loadState()
	ns := cur.clone()
	ns.nodes["node1"] = nl
	a.state.Store(ns)

	ctx := newLogTestCtx()
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "d")
	a.Before(ctx, msg, "")
	assert.Equal(t, 1, ctx.logCount())
	assert.Equal(t, "raw ${x}", ctx.logs[0].msg.GetData())
	assert.Equal(t, "bad expr", ctx.logs[0].msg.Metadata.GetValue(MetaLogRenderError))
}

func TestBusinessLogHandlerDispatch(t *testing.T) {
	a := (&BusinessLog{}).New().(*BusinessLog)
	_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("handler1", nil))
	_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
		LogConfig: &types.NodeLogConfig{Before: "b"}})

	t.Run("dispatch to pool", func(t *testing.T) {
		engine := &fakeEngine{}
		ctx := newLogTestCtx()
		ctx.chain.pool = &fakePool{engine: engine, found: true}
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "d")
		a.Before(ctx, msg, "")
		assert.Equal(t, 1, len(engine.received))
		assert.Equal(t, types.MsgTypeLog, engine.received[0].Type)
	})

	t.Run("handler missing drops silently", func(t *testing.T) {
		ctx := newLogTestCtx()
		ctx.chain.pool = &fakePool{found: false}
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "d")
		a.Before(ctx, msg, "") // 不应 panic
		assert.Equal(t, 1, ctx.logCount())
	})

	t.Run("no chain ctx skips dispatch", func(t *testing.T) {
		// RuleChain 不是 ChainCtx（如单测桩）时静默跳过出口②
		a := (&BusinessLog{}).New().(*BusinessLog)
		_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("handler1", nil))
		_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
			LogConfig: &types.NodeLogConfig{Before: "b"}})
		base := test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {})
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "d")
		a.Before(&noChainCtx{RuleContext: base}, msg, "")
	})
}

// noChainCtx 的 RuleChain 返回普通 NodeCtx，触发出口②的类型断言失败分支
type noChainCtx struct {
	types.RuleContext
}

func (c *noChainCtx) RuleChain() types.NodeCtx {
	return &stubNodeCtx{nodeId: types.RuleNodeId{Id: "chain1", Type: types.CHAIN}}
}

func TestBusinessLogGlobalMasked(t *testing.T) {
	a := (&BusinessLog{}).New().(*BusinessLog)
	_ = a.OnChainBeforeInit(types.NewConfig(), businessLogChainDef("", nil))
	_ = a.OnNodeBeforeInit(types.NewConfig(), &types.RuleNode{Id: "node1", Name: "N1",
		LogConfig: &types.NodeLogConfig{Before: "env=${global.env} key=${global.apiKey} k=${metadata.k}"}})
	ctx := newLogTestCtx()
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "d")
	msg.Metadata.PutValue("k", "v")
	a.Before(ctx, msg, "")
	assert.Equal(t, 1, ctx.logCount())
	assert.Equal(t, "env=*** key=*** k=v", ctx.logs[0].msg.GetData())
}
