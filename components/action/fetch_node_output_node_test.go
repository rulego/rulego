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

package action

import (
	"context"
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// MockRuleContext 模拟规则上下文，支持节点输出缓存
type MockRuleContext struct {
	*test.NodeTestRuleContext
	nodeOutputs map[string]types.RuleMsg
	mutex       sync.RWMutex
	callback    func(msg types.RuleMsg, relationType string, err error)
}

// NewMockRuleContext 创建模拟规则上下文
func NewMockRuleContext(config types.Config, callback func(msg types.RuleMsg, relationType string, err error)) *MockRuleContext {
	baseCtx := test.NewRuleContext(config, nil).(*test.NodeTestRuleContext)
	return &MockRuleContext{
		NodeTestRuleContext: baseCtx,
		nodeOutputs:         make(map[string]types.RuleMsg),
		callback:            callback,
	}
}

// SetNodeOutput 设置节点输出
func (ctx *MockRuleContext) SetNodeOutput(nodeId string, msg types.RuleMsg) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	ctx.nodeOutputs[nodeId] = msg
}

// GetNodeRuleMsg 获取节点输出
func (ctx *MockRuleContext) GetNodeRuleMsg(nodeId string) (types.RuleMsg, bool) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	msg, exists := ctx.nodeOutputs[nodeId]
	return msg, exists
}

// 实现其他必要的接口方法
func (ctx *MockRuleContext) TellSuccess(msg types.RuleMsg) {
	if ctx.callback != nil {
		ctx.callback(msg, types.Success, nil)
	}
}

func (ctx *MockRuleContext) TellFailure(msg types.RuleMsg, err error) {
	if ctx.callback != nil {
		ctx.callback(msg, types.Failure, err)
	}
}

func (ctx *MockRuleContext) Config() types.Config {
	if ctx.NodeTestRuleContext != nil {
		return ctx.NodeTestRuleContext.Config()
	}
	return types.Config{}
}

func (ctx *MockRuleContext) GetContext() context.Context {
	return context.TODO()
}

// TestFetchNodeOutputNodeType 测试节点类型
func TestFetchNodeOutputNodeType(t *testing.T) {
	var node FetchNodeOutputNode
	assert.Equal(t, "fetchNodeOutput", node.Type())
}

// TestFetchNodeOutputNodeNew 测试创建新实例
func TestFetchNodeOutputNodeNew(t *testing.T) {
	var node FetchNodeOutputNode
	newNode := node.New().(*FetchNodeOutputNode)
	assert.Equal(t, "", newNode.Config.NodeId)
}

// TestFetchNodeOutputNodeOnMsg 测试消息处理
func TestFetchNodeOutputNodeOnMsg(t *testing.T) {
	t.Run("target node exists", func(t *testing.T) {
		var node FetchNodeOutputNode
		config := types.NewConfig()
		configuration := make(types.Configuration)
		configuration["nodeId"] = "targetNode"

		node.Init(config, configuration)

		// 创建模拟测试上下文
		mockCtx := NewMockRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Success, relationType)
			assert.Equal(t, "target data", msg.GetData())
			assert.Equal(t, "target value", msg.Metadata.GetValue("key"))
		})

		// 模拟目标节点输出
		targetMetadata := types.NewMetadata()
		targetMetadata.PutValue("key", "target value")
		targetMsg := types.NewMsg(0, "TELEMETRY", types.JSON, targetMetadata, "target data")
		mockCtx.SetNodeOutput("targetNode", targetMsg)

		// 创建输入消息
		currentMetadata := types.NewMetadata()
		currentMetadata.PutValue("key", "current value")
		msg := types.NewMsg(0, "TELEMETRY", types.JSON, currentMetadata, "current data")

		// 处理消息
		node.OnMsg(mockCtx, msg)
	})

	t.Run("target node not exists", func(t *testing.T) {
		var node FetchNodeOutputNode
		config := types.NewConfig()
		configuration := make(types.Configuration)
		configuration["nodeId"] = "nonExistentNode"

		node.Init(config, configuration)

		// 创建模拟测试上下文
		mockCtx := NewMockRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Failure, relationType)
			assert.Equal(t, "current data", msg.GetData())
			assert.NotNil(t, err)
		})

		// 创建输入消息
		currentMetadata := types.NewMetadata()
		currentMetadata.PutValue("key", "current value")
		msg := types.NewMsg(0, "TELEMETRY", types.JSON, currentMetadata, "current data")

		// 处理消息
		node.OnMsg(mockCtx, msg)
	})

}

// TestFetchNodeOutputNodeDestroy 测试销毁
func TestFetchNodeOutputNodeDestroy(t *testing.T) {
	var node FetchNodeOutputNode
	// 销毁不应该引发panic
	node.Destroy()
}

// BenchmarkFetchNodeOutputNode 性能测试
func BenchmarkFetchNodeOutputNode(b *testing.B) {
	var node FetchNodeOutputNode
	config := types.NewConfig()
	configuration := make(types.Configuration)
	configuration["nodeId"] = "targetNode"

	node.Init(config, configuration)

	// 创建模拟测试上下文
	mockCtx := NewMockRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
		// 空回调
	})

	// 模拟目标节点输出
	targetMetadata := types.NewMetadata()
	targetMsg := types.NewMsg(0, "TELEMETRY", types.JSON, targetMetadata, "target data")
	mockCtx.SetNodeOutput("targetNode", targetMsg)

	// 创建输入消息
	currentMetadata := types.NewMetadata()
	msg := types.NewMsg(0, "TELEMETRY", types.JSON, currentMetadata, "current data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.OnMsg(mockCtx, msg)
	}
}
