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

package types

import (
	"context"
	"reflect"
	"testing"

	"github.com/rulego/rulego/test/assert"
)

// TestConfigurationCopy 浅拷贝与 nil 语义
func TestConfigurationCopy(t *testing.T) {
	assert.Nil(t, Configuration(nil).Copy())

	src := Configuration{"timeout": 30, "host": "localhost"}
	dst := src.Copy()
	assert.Equal(t, 2, len(dst))
	assert.Equal(t, 30, dst["timeout"])
	dst["extra"] = true
	_, exists := src["extra"]
	assert.False(t, exists, "copy must not write back to source")
}

// TestNodeRequestBuilders NodeRequest 构造器语义：
// nil RelationTypes 执行节点本身，空/非空切片执行后继节点
func TestNodeRequestBuilders(t *testing.T) {
	req := ExecuteNode("n1")
	assert.Equal(t, "n1", req.NodeId)
	assert.Nil(t, req.RelationTypes)
	assert.Nil(t, req.Msg)

	msg := NewMsg(0, "T", JSON, nil, "{}")
	req = ExecuteNodeWithMsg("n1", msg)
	assert.Equal(t, &msg, req.Msg)

	req = ExecuteNext("n1")
	assert.NotNil(t, req.RelationTypes)
	assert.Equal(t, 0, len(req.RelationTypes))

	req = ExecuteNext("n1", "Success", "Failure")
	assert.Equal(t, []string{"Success", "Failure"}, req.RelationTypes)

	req = ExecuteNextWithMsg("n1", msg, "Success")
	assert.Equal(t, []string{"Success"}, req.RelationTypes)
	assert.Equal(t, &msg, req.Msg)
}

// mockRuleContext 记录被测选项会调用的 setter，其余接口方法由内嵌零值兜底。
type mockRuleContext struct {
	RuleContext
	endFunc     OnEndFunc
	ctx         context.Context
	allDone     func()
	callbacks   map[string]interface{}
	debugMode   bool
	skipNext    bool
	executeNodN []NodeRequest
}

func (m *mockRuleContext) SetEndFunc(f OnEndFunc) RuleContext {
	m.endFunc = f
	return m
}
func (m *mockRuleContext) SetContext(c context.Context) RuleContext {
	m.ctx = c
	return m
}
func (m *mockRuleContext) SetOnAllNodeCompleted(f func()) { m.allDone = f }
func (m *mockRuleContext) SetCallbackFunc(name string, f interface{}) {
	if m.callbacks == nil {
		m.callbacks = make(map[string]interface{})
	}
	m.callbacks[name] = f
}
func (m *mockRuleContext) SetDebugMode(debugMode bool) { m.debugMode = debugMode }
func (m *mockRuleContext) SetSkipTellNext(skip bool)   { m.skipNext = skip }
func (m *mockRuleContext) SetExecuteNodes(nodes ...NodeRequest) {
	m.executeNodN = nodes
}

// funcPointer 函数值无法用 reflect.DeepEqual 比较，取指针地址断言同一回调
func funcPointer(f interface{}) uintptr {
	return reflect.ValueOf(f).Pointer()
}

// TestRuleContextOptions 各 RuleContextOption 对上下文的副作用
func TestRuleContextOptions(t *testing.T) {
	endFunc := func(ctx RuleContext, msg RuleMsg, err error, relationType string) {}
	WithOnEnd(endFunc)(&mockRuleContext{})

	rc := &mockRuleContext{}
	WithOnEnd(endFunc)(rc)
	assert.Equal(t, funcPointer(endFunc), funcPointer(rc.endFunc))

	ctx := context.Background()
	WithContext(ctx)(rc)
	assert.Equal(t, ctx, rc.ctx)

	allDone := func() {}
	WithOnAllNodeCompleted(allDone)(rc)
	assert.Equal(t, funcPointer(allDone), funcPointer(rc.allDone))

	chainCompleted := func(ctx RuleContext, snapshot RuleChainRunSnapshot) {}
	WithOnRuleChainCompleted(chainCompleted)(rc)
	assert.Equal(t, funcPointer(chainCompleted), funcPointer(rc.callbacks[CallbackFuncOnRuleChainCompleted]))

	nodeCompleted := func(ctx RuleContext, log RuleNodeRunLog) {}
	WithOnNodeCompleted(nodeCompleted)(rc)
	assert.Equal(t, funcPointer(nodeCompleted), funcPointer(rc.callbacks[CallbackFuncOnNodeCompleted]))

	onDebug := func(chainId, flowType, nodeId string, msg RuleMsg, relationType string, err error) {}
	WithOnNodeDebug(onDebug)(rc)
	assert.Equal(t, funcPointer(onDebug), funcPointer(rc.callbacks[CallbackFuncDebug]))

	WithDebugMode(true)(rc)
	assert.True(t, rc.debugMode)

	WithSkipTellNext()(rc)
	assert.True(t, rc.skipNext)

	// WithStartNode：空参不触发；有参转 ExecuteNode 请求
	WithStartNode()(rc)
	assert.Equal(t, 0, len(rc.executeNodN))
	WithStartNode("a", "b")(rc)
	assert.Equal(t, 2, len(rc.executeNodN))
	assert.Nil(t, rc.executeNodN[0].RelationTypes)

	// WithTellNext：空 fromNodeId 不触发；正常转 ExecuteNext 请求
	WithTellNext("")(rc)
	assert.Equal(t, 2, len(rc.executeNodN))
	WithTellNext("n1", "Success")(rc)
	assert.Equal(t, 1, len(rc.executeNodN))
	assert.Equal(t, "n1", rc.executeNodN[0].NodeId)
	assert.Equal(t, []string{"Success"}, rc.executeNodN[0].RelationTypes)

	// WithRestoreNodes 原样透传
	WithRestoreNodes(ExecuteNode("x"), ExecuteNext("y"))(rc)
	assert.Equal(t, 2, len(rc.executeNodN))
}
