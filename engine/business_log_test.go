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

package engine

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// blog/ok、blog/fail、blog/collect 为业务日志集成测试专用组件；
// collect 把收到的事件消息记入包级 collector（engine 包测试串行执行，无共享冲突）。

type blogOkNode struct{}

func (n *blogOkNode) Type() string                                 { return "blog/ok" }
func (n *blogOkNode) New() types.Node                              { return &blogOkNode{} }
func (n *blogOkNode) Init(types.Config, types.Configuration) error { return nil }
func (n *blogOkNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.SetData(strings.ToUpper(msg.GetData()))
	ctx.TellSuccess(msg)
}
func (n *blogOkNode) Destroy() {}

type blogFailNode struct{}

func (n *blogFailNode) Type() string                                 { return "blog/fail" }
func (n *blogFailNode) New() types.Node                              { return &blogFailNode{} }
func (n *blogFailNode) Init(types.Config, types.Configuration) error { return nil }
func (n *blogFailNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellFailure(msg, errors.New("boom"))
}
func (n *blogFailNode) Destroy() {}

var collectorMu sync.Mutex
var collector []types.RuleMsg

type blogCollectNode struct{}

func (n *blogCollectNode) Type() string                                 { return "blog/collect" }
func (n *blogCollectNode) New() types.Node                              { return &blogCollectNode{} }
func (n *blogCollectNode) Init(types.Config, types.Configuration) error { return nil }
func (n *blogCollectNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	collectorMu.Lock()
	collector = append(collector, msg)
	collectorMu.Unlock()
	ctx.TellSuccess(msg)
}
func (n *blogCollectNode) Destroy() {}

var registerBlogOnce sync.Once

func registerBlogNodes(t *testing.T) {
	t.Helper()
	registerBlogOnce.Do(func() {
		for _, n := range []types.Node{&blogOkNode{}, &blogFailNode{}, &blogCollectNode{}} {
			if err := Registry.Register(n); err != nil {
				t.Fatalf("register %s: %v", n.Type(), err)
			}
		}
	})
}

type logEventSink struct {
	mu     sync.Mutex
	events []types.RuleMsg
}

func (s *logEventSink) onDebug(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	if flowType != types.Log {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, msg)
}

func (s *logEventSink) ofNode(nodeId, phase string) []types.RuleMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []types.RuleMsg
	for _, m := range s.events {
		if m.Metadata.GetValue("nodeId") == nodeId && m.Metadata.GetValue("phase") == phase {
			out = append(out, m)
		}
	}
	return out
}

func (s *logEventSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func waitUntil(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

func runChain(t *testing.T, e *RuleEngine, metadata map[string]string) (string, error) {
	t.Helper()
	done := make(chan struct{}, 1)
	var mu sync.Mutex
	var result string
	var err error
	md := types.NewMetadata()
	for k, v := range metadata {
		md.PutValue(k, v)
	}
	e.OnMsg(types.NewMsg(0, "TEST", types.JSON, md, "data"), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, e2 error, relationType string) {
		mu.Lock()
		result = msg.GetData()
		err = e2
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		return result, err
	case <-time.After(5 * time.Second):
		t.Fatal("chain execution timeout")
		return "", nil
	}
}

// TestBusinessLogE2E 覆盖：节点模板事件（in/out）、chainEnd 失败事件、
// 处理链派发、失败模板渲染 errorMsg。
func TestBusinessLogE2E(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug

	handlerDsl := `{
		"ruleChain": {"id": "blog_handler", "name": "handler"},
		"metadata": {"nodes": [{"id": "c1", "type": "blog/collect", "name": "收集"}]}
	}`
	if _, err := DefaultPool.New("blog_handler", []byte(handlerDsl)); err != nil {
		t.Fatalf("create handler: %v", err)
	}
	defer DefaultPool.Del("blog_handler")

	mainDsl := `{
		"ruleChain": {
			"id": "blog_main",
			"name": "main",
			"configuration": {"logHandler": "blog_handler", "logEvents": ["chainEnd"]}
		},
		"metadata": {
			"nodes": [
				{"id": "n1", "type": "blog/ok", "name": "正常节点",
				 "logConfig": {"before": "进入:${metadata.k}", "after": "完成:${data}"}},
				{"id": "n2", "type": "blog/fail", "name": "失败节点"}
			],
			"connections": [{"fromId": "n1", "toId": "n2", "type": "Success"}]
		}
	}`
	e, err := NewRuleEngine("blog_main", []byte(mainDsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_main")

	_, runErr := runChain(t, e, map[string]string{"k": "订单1"})
	assert.NotNil(t, runErr) // n2 失败且无 Failure 下游

	// 3 类事件齐：n1 in / n1 out / chainEnd
	if !waitUntil(3*time.Second, func() bool { return sink.count() >= 3 }) {
		sink.mu.Lock()
		var dump []string
		for _, m := range sink.events {
			dump = append(dump, m.Metadata.GetValue("nodeId")+"/"+m.Metadata.GetValue("phase")+"="+m.GetData())
		}
		sink.mu.Unlock()
		t.Fatalf("expect 3 log events, got %d: %v", sink.count(), dump)
	}

	in := sink.ofNode("n1", "in")
	assert.Equal(t, 1, len(in))
	assert.Equal(t, "进入:订单1", in[0].GetData())
	assert.Equal(t, "node", in[0].Metadata.GetValue("scope"))
	assert.Equal(t, types.MsgTypeLog, in[0].Type)

	out := sink.ofNode("n1", "out")
	assert.Equal(t, 1, len(out))
	assert.Equal(t, "完成:DATA", out[0].GetData())

	// n2 失败无 Failure 下游，链以 Failure 结束，只发结束事件
	var endEvent *types.RuleMsg
	sink.mu.Lock()
	for i := range sink.events {
		if sink.events[i].Metadata.GetValue("phase") == "end" {
			endEvent = &sink.events[i]
		}
	}
	sink.mu.Unlock()
	if endEvent == nil {
		t.Fatal("chainEnd event missing")
	}
	assert.Equal(t, "chain", endEvent.Metadata.GetValue("scope"))
	// Data 为触发点消息数据；error 经 wrapEndErr 包装，含链与节点前缀
	assert.Equal(t, "DATA", endEvent.GetData())
	assert.Equal(t, "Failure", endEvent.Metadata.GetValue("relationType"))
	assert.True(t, strings.Contains(endEvent.Metadata.GetValue("error"), "boom"))

	// 处理链收到全部 3 条派发事件
	if !waitUntil(3*time.Second, func() bool {
		collectorMu.Lock()
		defer collectorMu.Unlock()
		return len(collector) >= 3
	}) {
		collectorMu.Lock()
		t.Fatalf("handler received %d events, expect >=3", len(collector))
	}
}

// TestBusinessLogLoopGuard 处理链节点自身配置模板也不会产生二层事件（_logEvent 防环）。
func TestBusinessLogLoopGuard(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug

	handlerDsl := `{
		"ruleChain": {"id": "blog_handler2", "name": "handler"},
		"metadata": {"nodes": [{"id": "c1", "type": "blog/collect", "name": "收集",
			"logConfig": {"before": "不应出现", "after": "不应出现"}}]}
	}`
	if _, err := DefaultPool.New("blog_handler2", []byte(handlerDsl), WithConfig(config)); err != nil {
		t.Fatalf("create handler: %v", err)
	}
	defer DefaultPool.Del("blog_handler2")

	mainDsl := `{
		"ruleChain": {"id": "blog_main2", "name": "main",
			"configuration": {"logHandler": "blog_handler2"}},
		"metadata": {"nodes": [{"id": "n1", "type": "blog/ok", "name": "n1",
			"logConfig": {"before": "b"}}]}
	}`
	e, err := NewRuleEngine("blog_main2", []byte(mainDsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_main2")

	_, runErr := runChain(t, e, nil)
	assert.Nil(t, runErr)
	if !waitUntil(3*time.Second, func() bool {
		collectorMu.Lock()
		defer collectorMu.Unlock()
		return len(collector) >= 1
	}) {
		t.Fatal("handler received nothing")
	}
	time.Sleep(200 * time.Millisecond)
	// 只有主链 n1 的 in 事件；处理链节点的事件被防环标记拦截
	assert.Equal(t, 1, sink.count())
	assert.Equal(t, 0, len(sink.ofNode("c1", "in")))
}

// TestBusinessLogHandlerMissing 处理链不存在：仅告警丢弃，业务链不受影响。
func TestBusinessLogHandlerMissing(t *testing.T) {
	registerBlogNodes(t)
	config := NewConfig()
	mainDsl := `{
		"ruleChain": {"id": "blog_main3", "name": "main",
			"configuration": {"logHandler": "no_such_handler"}},
		"metadata": {"nodes": [{"id": "n1", "type": "blog/ok", "name": "n1",
			"logConfig": {"after": "done"}}]}
	}`
	e, err := NewRuleEngine("blog_main3", []byte(mainDsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_main3")

	out, runErr := runChain(t, e, nil)
	assert.Nil(t, runErr)
	assert.Equal(t, "DATA", out)
}

// TestBusinessLogReload 热更后模板重新编译生效。
func TestBusinessLogReload(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug

	dsl := func(before string) []byte {
		return []byte(`{
			"ruleChain": {"id": "blog_main4", "name": "main"},
			"metadata": {"nodes": [{"id": "n1", "type": "blog/ok", "name": "n1",
				"logConfig": {"before": "` + before + `"}}]}
		}`)
	}
	e, err := NewRuleEngine("blog_main4", dsl("旧模板"), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_main4")

	_, _ = runChain(t, e, map[string]string{"k": "x"})
	if !waitUntil(2*time.Second, func() bool { return len(sink.ofNode("n1", "in")) >= 1 }) {
		t.Fatal("no event before reload")
	}
	assert.Equal(t, "旧模板", sink.ofNode("n1", "in")[0].GetData())

	if err := e.ReloadSelf(dsl("新模板:${metadata.k}")); err != nil {
		t.Fatalf("reload: %v", err)
	}
	_, _ = runChain(t, e, map[string]string{"k": "y"})
	if !waitUntil(2*time.Second, func() bool { return len(sink.ofNode("n1", "in")) >= 2 }) {
		t.Fatal("no event after reload")
	}
	assert.Equal(t, "新模板:y", sink.ofNode("n1", "in")[1].GetData())
}

// TestBlogJsonField 模板引用 JSON 负荷字段：顶层/嵌套/数组（msg 键挂解析后的 JSON）
func TestBlogJsonField(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug
	dsl := `{
		"ruleChain": {"id": "blog_json", "name": "j"},
		"metadata": {"nodes": [{"id": "n1", "type": "blog/ok", "name": "n1",
			"logConfig": {"before": "订单${msg.order.id}用户${msg.user}规格${msg.items[0].sku}"}}]}
	}`
	e, err := NewRuleEngine("blog_json", []byte(dsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_json")
	md := types.NewMetadata()
	md.PutValue("k", "v")
	e.OnMsg(types.NewMsg(0, "TEST", types.JSON, md, `{"order":{"id":1001},"user":"bob","items":[{"sku":"XL"}]}`), types.WithOnEnd(func(ctx types.RuleContext, m types.RuleMsg, e2 error, rt string) {}))
	if !waitUntil(2*time.Second, func() bool { return len(sink.ofNode("n1", "in")) >= 1 }) {
		t.Fatal("no in event")
	}
	assert.Equal(t, "订单1001用户bob规格XL", sink.ofNode("n1", "in")[0].GetData())
}

// TestBusinessLogMultiBranch 无结束节点时每个终点分支各发一条结束事件。
func TestBusinessLogMultiBranch(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug

	dsl := `{
		"ruleChain": {"id": "blog_fork", "name": "fork", "configuration": {"logEvents": ["chainEnd"]}},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{"id": "n1", "type": "blog/ok", "name": "分叉源"},
				{"id": "n2", "type": "blog/collect", "name": "分支A"},
				{"id": "n3", "type": "blog/collect", "name": "分支B"}
			],
			"connections": [
				{"fromId": "n1", "toId": "n2", "type": "Success"},
				{"fromId": "n1", "toId": "n3", "type": "Success"}
			]
		}
	}`
	e, err := NewRuleEngine("blog_fork", []byte(dsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_fork")

	_, runErr := runChain(t, e, map[string]string{"k": "fork"})
	assert.Nil(t, runErr)

	if !waitUntil(3*time.Second, func() bool { return sink.count() >= 2 }) {
		t.Fatalf("expect 2 end events, got %d", sink.count())
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for _, m := range sink.events {
		assert.Equal(t, "chain", m.Metadata.GetValue("scope"))
		assert.Equal(t, "end", m.Metadata.GetValue("phase"))
		assert.Equal(t, "DATA", m.GetData())
	}
}

// TestBusinessLogEndNodeGating 链内有结束节点时仅结束节点触发结束事件，
// 其余分支终点被引擎门控抑制。
func TestBusinessLogEndNodeGating(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug

	dsl := `{
		"ruleChain": {"id": "blog_fork_end", "name": "forkEnd", "configuration": {"logEvents": ["chainEnd"]}},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{"id": "n1", "type": "blog/ok", "name": "分叉源"},
				{"id": "n2", "type": "blog/collect", "name": "分支A"},
				{"id": "n3", "type": "blog/collect", "name": "分支B"},
				{"id": "n4", "type": "end", "name": "结束"}
			],
			"connections": [
				{"fromId": "n1", "toId": "n2", "type": "Success"},
				{"fromId": "n1", "toId": "n3", "type": "Success"},
				{"fromId": "n2", "toId": "n4", "type": "Success"}
			]
		}
	}`
	e, err := NewRuleEngine("blog_fork_end", []byte(dsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	defer DefaultPool.Del("blog_fork_end")

	_, runErr := runChain(t, e, map[string]string{"k": "fork"})
	assert.Nil(t, runErr)

	time.Sleep(500 * time.Millisecond)
	// n2/n3 终点被抑制，只有 n4（end 节点）发一条。count 内部加锁，不可持锁调用
	assert.Equal(t, 1, sink.count())
	sink.mu.Lock()
	defer sink.mu.Unlock()
	assert.Equal(t, "DATA", sink.events[0].GetData())
	assert.Equal(t, "Success", sink.events[0].Metadata.GetValue("relationType"))
}

// TestBlogSingleJsFilter 单节点链（无下游连接）的事件回归：首节点路径的 in/out 与结束事件都要产生
func TestBlogSingleJsFilter(t *testing.T) {
	registerBlogNodes(t)
	sink := &logEventSink{}
	config := NewConfig()
	config.OnDebug = sink.onDebug
	dsl := `{
		"ruleChain": {"id": "blog_single", "name": "s", "configuration": {"logEvents": ["chainEnd"]}},
		"metadata": {"nodes": [{"id": "n1", "type": "blog/ok", "name": "f1",
			"configuration": {"jsScript": "return true;"},
			"logConfig": {"before": "in:${metadata.k}", "after": "out"}}]}
	}`
	e, err := NewRuleEngine("blog_single", []byte(dsl), WithConfig(config))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer DefaultPool.Del("blog_single")
	md := types.NewMetadata()
	md.PutValue("k", "v1")
	e.OnMsg(types.NewMsg(0, "TEST", types.JSON, md, "{}"))
	if !waitUntil(2*time.Second, func() bool { return sink.count() >= 3 }) {
		sink.mu.Lock()
		var dump []string
		for _, m := range sink.events {
			dump = append(dump, m.Metadata.GetValue("nodeId")+"/"+m.Metadata.GetValue("phase"))
		}
		sink.mu.Unlock()
		t.Fatalf("expect 3 events, got %d: %v", sink.count(), dump)
	}
}
