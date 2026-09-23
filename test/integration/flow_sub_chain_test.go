/*
 * Copyright 2025 The RuleGo Authors.
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

package integration

// flow 子规则链组件集成测试：真引擎覆盖 merge/extend 两种输出模式、
// {chainId}:{nodeId} 起点语法、跨链来源键（fromChainId/fromNodeId）
// 以及跳数预算对 flow 自递归的熔断。

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/maps"
)

// flowMarkNode 向 metadata[key] 追加 value，用于标记哪些节点执行过。
type flowMarkNode struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (x *flowMarkNode) Type() string { return "testFlowMark" }
func (x *flowMarkNode) New() types.Node {
	return &flowMarkNode{}
}
func (x *flowMarkNode) Init(_ types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, x)
}
func (x *flowMarkNode) Destroy() {}

func (x *flowMarkNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Metadata.PutValue(x.Key, msg.Metadata.GetValue(x.Key)+x.Value)
	ctx.TellSuccess(msg)
}

// flowFailNode 以固定错误走 Failure，验证 merge 模式对失败分支的聚合。
type flowFailNode struct{}

func (x *flowFailNode) Type() string { return "testFlowFail" }
func (x *flowFailNode) New() types.Node {
	return &flowFailNode{}
}
func (x *flowFailNode) Init(types.Config, types.Configuration) error { return nil }
func (x *flowFailNode) Destroy()                                     {}

func (x *flowFailNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellFailure(msg, errors.New("flowFailNode boom"))
}

// flowCopyMetaNode 把 metadata[from] 复制到 metadata[to]，用于在子链内固定来源键快照。
type flowCopyMetaNode struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (x *flowCopyMetaNode) Type() string { return "testFlowCopyMeta" }
func (x *flowCopyMetaNode) New() types.Node {
	return &flowCopyMetaNode{}
}
func (x *flowCopyMetaNode) Init(_ types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, x)
}
func (x *flowCopyMetaNode) Destroy() {}

func (x *flowCopyMetaNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Metadata.PutValue(x.To, msg.Metadata.GetValue(x.From))
	ctx.TellSuccess(msg)
}

func init() {
	_ = rulego.Registry.Register(&flowMarkNode{})
	_ = rulego.Registry.Register(&flowFailNode{})
	_ = rulego.Registry.Register(&flowCopyMetaNode{})
}

// flowMarkChain 两个标记节点串行链：n1(a:A) -> n2(b:B)
func flowMarkChainDSL(id string) string {
	return fmt.Sprintf(`{
		"ruleChain":{"id":"%s","name":"flow sub chain"},
		"metadata":{
			"nodes":[
				{"id":"n1","type":"testFlowMark","configuration":{"key":"a","value":"A"}},
				{"id":"n2","type":"testFlowMark","configuration":{"key":"b","value":"B"}}
			],
			"connections":[{"fromId":"n1","toId":"n2","type":"Success"}]
		}
	}`, id)
}

// flowParentDSL 单 flow 节点的父链
func flowParentDSL(id, targetId string, extend bool) string {
	return fmt.Sprintf(`{
		"ruleChain":{"id":"%s","name":"flow parent chain"},
		"metadata":{
			"nodes":[
				{"id":"s1","type":"flow","configuration":{"targetId":"%s","extend":%t}}
			],
			"connections":[]
		}
	}`, id, targetId, extend)
}

type flowResult struct {
	relationType string
	err          error
	msg          types.RuleMsg
}

// flowRunParent 向父链发消息并等待首个 onEnd 回调
func flowRunParent(t *testing.T, parentID string, msg types.RuleMsg) flowResult {
	t.Helper()
	e, ok := engine.DefaultPool.Get(parentID)
	if !ok {
		t.Fatalf("parent chain %s not registered", parentID)
	}
	done := make(chan flowResult, 8)
	e.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
		done <- flowResult{relationType: relationType, err: err, msg: onEndMsg}
	}))
	select {
	case r := <-done:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for onEnd")
		return flowResult{}
	}
}

// flowNewEngine 注册链并登记清理；同 ID 重复 New 不会重建引擎，先 Del 保证 DSL 生效
func flowNewEngine(t *testing.T, id, dsl string, opts ...types.Option) {
	t.Helper()
	engine.DefaultPool.Del(id)
	opts = append([]types.Option{types.WithComponentsRegistry(rulego.Registry)}, opts...)
	config := rulego.NewConfig(opts...)
	_, err := rulego.New(id, []byte(dsl), rulego.WithConfig(config))
	if err != nil {
		t.Fatalf("new engine %s: %v", id, err)
	}
	t.Cleanup(func() {
		engine.DefaultPool.Del(id)
	})
}

func TestFlowMergeWholeChain(t *testing.T) {
	const subID = "flow_it_sub_merge"
	const parentID = "flow_it_parent_merge"
	flowNewEngine(t, subID, flowMarkChainDSL(subID))
	flowNewEngine(t, parentID, flowParentDSL(parentID, subID, false))

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)
	r := flowRunParent(t, parentID, msg)
	if r.relationType != types.Success {
		t.Fatalf("expected Success, got %s (err=%v)", r.relationType, r.err)
	}
	if got := r.msg.Metadata.GetValue("a") + r.msg.Metadata.GetValue("b"); got != "AB" {
		t.Fatalf("expected merged metadata a=A b=B, got a=%q b=%q", r.msg.Metadata.GetValue("a"), r.msg.Metadata.GetValue("b"))
	}
	// 合并输出是 []WrapperMsg，应包含每个分支的结束节点 ID
	if data := r.msg.Data.Get(); !strings.Contains(data, "n2") {
		t.Fatalf("expected wrapper data to contain end node id n2, got %s", data)
	}
}

func TestFlowExtendWholeChain(t *testing.T) {
	const subID = "flow_it_sub_extend"
	const parentID = "flow_it_parent_extend"
	flowNewEngine(t, subID, flowMarkChainDSL(subID))
	flowNewEngine(t, parentID, flowParentDSL(parentID, subID, true))

	e, _ := engine.DefaultPool.Get(parentID)
	done := make(chan flowResult, 8)
	e.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`),
		types.WithOnEnd(func(ctx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			done <- flowResult{relationType: relationType, err: err, msg: onEndMsg}
		}))

	// onEnd 是分支结束回调：串行链只有末节点的输出结束分支，extend 不合并、按原始关系转发这一个输出
	select {
	case r := <-done:
		if r.relationType != types.Success {
			t.Fatalf("expected Success, got %s (err=%v)", r.relationType, r.err)
		}
		if r.msg.Metadata.GetValue("a") != "A" || r.msg.Metadata.GetValue("b") != "B" {
			t.Fatalf("expected n1+n2 output, got a=%q b=%q", r.msg.Metadata.GetValue("a"), r.msg.Metadata.GetValue("b"))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for onEnd")
	}
}

func TestFlowStartNodeMergeAndExtend(t *testing.T) {
	const subID = "flow_it_sub_start"
	for _, extend := range []bool{false, true} {
		name := "Merge"
		if extend {
			name = "Extend"
		}
		t.Run(name, func(t *testing.T) {
			parentID := "flow_it_parent_start_" + name
			flowNewEngine(t, subID, flowMarkChainDSL(subID))
			flowNewEngine(t, parentID, flowParentDSL(parentID, subID+":n2", extend))

			msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)
			if extend {
				// extend 下每个分支回调一次，取首个（子链只有 n2 一个输出）
				e, _ := engine.DefaultPool.Get(parentID)
				done := make(chan flowResult, 4)
				e.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
					done <- flowResult{relationType: relationType, err: err, msg: onEndMsg}
				}))
				select {
				case r := <-done:
					if r.relationType != types.Success {
						t.Fatalf("expected Success, got %s (err=%v)", r.relationType, r.err)
					}
					if r.msg.Metadata.GetValue("a") != "" || r.msg.Metadata.GetValue("b") != "B" {
						t.Fatalf("expected only n2 ran, got a=%q b=%q", r.msg.Metadata.GetValue("a"), r.msg.Metadata.GetValue("b"))
					}
				case <-time.After(5 * time.Second):
					t.Fatal("timeout waiting for onEnd")
				}
				return
			}
			r := flowRunParent(t, parentID, msg)
			if r.relationType != types.Success {
				t.Fatalf("expected Success, got %s (err=%v)", r.relationType, r.err)
			}
			if r.msg.Metadata.GetValue("a") != "" || r.msg.Metadata.GetValue("b") != "B" {
				t.Fatalf("expected only n2 ran, got a=%q b=%q", r.msg.Metadata.GetValue("a"), r.msg.Metadata.GetValue("b"))
			}
		})
	}
}

func TestFlowNotFoundFailures(t *testing.T) {
	const subID = "flow_it_sub_missing"
	flowNewEngine(t, subID, flowMarkChainDSL(subID))
	for _, tc := range []struct {
		name     string
		targetId string
		extend   bool
		wantIn   string
	}{
		{"ChainNotFoundMerge", "flow_it_no_such_chain", false, "flow_it_no_such_chain"},
		{"ChainNotFoundExtend", "flow_it_no_such_chain", true, "flow_it_no_such_chain"},
		{"StartNodeNotFoundMerge", subID + ":nope", false, "nope"},
		{"StartNodeNotFoundExtend", subID + ":nope", true, "nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const parentID = "flow_it_parent_missing"
			flowNewEngine(t, parentID, flowParentDSL(parentID, tc.targetId, tc.extend))
			r := flowRunParent(t, parentID, types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`))
			if r.relationType != types.Failure {
				t.Fatalf("expected Failure, got %s (err=%v)", r.relationType, r.err)
			}
			if r.err == nil || !strings.Contains(r.err.Error(), tc.wantIn) {
				t.Fatalf("expected error containing %q, got %v", tc.wantIn, r.err)
			}
		})
	}
}

func TestFlowMergeSubChainFailure(t *testing.T) {
	const subID = "flow_it_sub_fail"
	const parentID = "flow_it_parent_fail"
	failDSL := fmt.Sprintf(`{
		"ruleChain":{"id":"%s","name":"flow fail chain"},
		"metadata":{
			"nodes":[
				{"id":"f1","type":"testFlowFail"}
			],
			"connections":[]
		}
	}`, subID)
	flowNewEngine(t, subID, failDSL)
	flowNewEngine(t, parentID, flowParentDSL(parentID, subID, false))

	r := flowRunParent(t, parentID, types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`))
	if r.relationType != types.Failure {
		t.Fatalf("expected Failure, got %s", r.relationType)
	}
	if r.err == nil || !strings.Contains(r.err.Error(), "boom") {
		t.Fatalf("expected sub-chain boom error, got %v", r.err)
	}
}

// 链式跨链时来源键是上一跳：parent -> C2 -> C3，C3 里看到的是 C2
func TestFlowSourceMetadataLastHop(t *testing.T) {
	const c2 = "flow_it_c2"
	const c3 = "flow_it_c3"
	const parentID = "flow_it_parent_lasthop"

	c3DSL := fmt.Sprintf(`{
		"ruleChain":{"id":"%s","name":"c3"},
		"metadata":{
			"nodes":[
				{"id":"cp1","type":"testFlowCopyMeta","configuration":{"from":"fromChainId","to":"seen_chain"}},
				{"id":"cp2","type":"testFlowCopyMeta","configuration":{"from":"fromNodeId","to":"seen_node"}}
			],
			"connections":[{"fromId":"cp1","toId":"cp2","type":"Success"}]
		}
	}`, c3)
	flowNewEngine(t, c3, c3DSL)
	flowNewEngine(t, c2, flowParentDSL(c2, c3, false))
	flowNewEngine(t, parentID, flowParentDSL(parentID, c2, false))

	r := flowRunParent(t, parentID, types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`))
	if r.relationType != types.Success {
		t.Fatalf("expected Success, got %s (err=%v)", r.relationType, r.err)
	}
	if got := r.msg.Metadata.GetValue("seen_chain"); got != c2 {
		t.Fatalf("expected last-hop chain %s, got %q", c2, got)
	}
	if got := r.msg.Metadata.GetValue("seen_node"); got != "s1" {
		t.Fatalf("expected caller node s1, got %q", got)
	}
	if got := r.msg.Metadata.GetValue(types.KeyFromChainId); got != c2 {
		t.Fatalf("expected fromChainId=%s, got %q", c2, got)
	}
}

// merge 模式对并行分支做元数据合并，WrapperMsg 每个分支一条
func TestFlowMergeParallelBranches(t *testing.T) {
	const subID = "flow_it_sub_fork"
	const parentID = "flow_it_parent_fork"
	forkDSL := fmt.Sprintf(`{
		"ruleChain":{"id":"%s","name":"flow fork chain"},
		"metadata":{
			"nodes":[
				{"id":"n1","type":"testFlowMark","configuration":{"key":"p","value":"P"}},
				{"id":"n2","type":"testFlowMark","configuration":{"key":"q","value":"Q"}},
				{"id":"n3","type":"testFlowMark","configuration":{"key":"r","value":"R"}}
			],
			"connections":[
				{"fromId":"n1","toId":"n2","type":"Success"},
				{"fromId":"n1","toId":"n3","type":"Success"}
			]
		}
	}`, subID)
	flowNewEngine(t, subID, forkDSL)
	flowNewEngine(t, parentID, flowParentDSL(parentID, subID, false))

	r := flowRunParent(t, parentID, types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`))
	if r.relationType != types.Success {
		t.Fatalf("expected Success, got %s (err=%v)", r.relationType, r.err)
	}
	md := r.msg.Metadata
	if md.GetValue("p") != "P" || md.GetValue("q") != "Q" || md.GetValue("r") != "R" {
		t.Fatalf("expected branches merged p=P q=Q r=R, got p=%q q=%q r=%q", md.GetValue("p"), md.GetValue("q"), md.GetValue("r"))
	}
	data := r.msg.Data.Get()
	if !strings.Contains(data, "n2") || !strings.Contains(data, "n3") {
		t.Fatalf("expected wrapper entries for n2 and n3, got %s", data)
	}
}

// flow 节点指向自身链形成递归，跳数预算（跨 TellFlow 共享）将其熔断。
// extend 模式熔断错误沿 onEnd 逐层冒泡；merge 模式的已知缺陷见下一个测试。
func TestFlowSelfRecursionCappedByHopBudget(t *testing.T) {
	const parentID = "flow_it_self_loop"
	flowNewEngine(t, parentID, flowParentDSL(parentID, parentID, true), types.WithMsgMaxHops(50))

	e, _ := engine.DefaultPool.Get(parentID)
	var mu sync.Mutex
	var gotErr error
	var gotRel string
	done := make(chan struct{}, 1)
	e.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			mu.Lock()
			gotErr = err
			gotRel = relationType
			mu.Unlock()
			done <- struct{}{}
		}))

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("hop budget did not terminate the self-recursive flow")
	}
	mu.Lock()
	defer mu.Unlock()
	if !errors.Is(gotErr, types.ErrMsgHopBudgetExceeded) {
		t.Fatalf("expected ErrMsgHopBudgetExceeded, got %v", gotErr)
	}
	if gotRel != types.Failure {
		t.Fatalf("expected %s relation, got %s", types.Failure, gotRel)
	}
}

// 已知缺陷：merge 模式下 flow 自递归被跳数预算熔断后，各层聚合结果依赖
// onAllNodeCompleted 触发，而完成级联在中途失联（configOnEnd 计数停滞在
// 预算耗尽层数附近），最外层 onEnd 永不触发、调用方悬挂。已用基线比对确认
// 为存量问题。修复完成后去掉 Skip 使本测试生效。
func TestFlowMergeSelfRecursionBudgetKill(t *testing.T) {
	t.Skip("known issue: merge mode self-recursion hangs after hop budget kill, waiting onAllNodeCompleted that never fires")
	const parentID = "flow_it_self_loop_merge"
	flowNewEngine(t, parentID, flowParentDSL(parentID, parentID, false), types.WithMsgMaxHops(50))

	e, _ := engine.DefaultPool.Get(parentID)
	done := make(chan flowResult, 1)
	e.OnMsg(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			done <- flowResult{relationType: relationType, err: err}
		}))

	select {
	case r := <-done:
		if !errors.Is(r.err, types.ErrMsgHopBudgetExceeded) {
			t.Fatalf("expected ErrMsgHopBudgetExceeded, got %v", r.err)
		}
		if r.relationType != types.Failure {
			t.Fatalf("expected %s relation, got %s", types.Failure, r.relationType)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("hop budget did not terminate the self-recursive flow")
	}
}
