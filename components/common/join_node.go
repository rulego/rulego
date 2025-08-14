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

package common

// Example of rule chain node configuration:
//{
//	"id": "s1",
//	"type": "join",
//	"name": "join",
//	"configuration": {
//	 }
//	}
//}
import (
	"context"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init 注册JoinNode组件
// init registers the JoinNode component with the default registry.
func init() {
	Registry.Add(&JoinNode{})
}

// JoinNodeConfiguration JoinNode配置结构
// JoinNodeConfiguration defines the configuration structure for the JoinNode component.
type JoinNodeConfiguration struct {
	// Timeout 执行超时时间（秒），默认值0表示无超时限制
	// Timeout specifies the execution timeout in seconds.
	// Default value of 0 means no timeout limit.
	Timeout int
}

// JoinNode 合并多个异步节点执行结果的动作组件
// JoinNode is an action component that merges results from multiple asynchronous node executions.
//
// 核心算法：
// Core Algorithm:
// 1. 等待所有并行分支完成执行 - Wait for all parallel branches to complete
// 2. 收集来自所有分支的消息 - Collect messages from all branches
// 3. 合并所有分支的元数据 - Merge metadata from all branches
// 4. 将收集的结果合并为JSON数组 - Combine collected results into JSON array
// 5. 通过Success关系发送合并结果 - Send merged results via Success relation
//
// 工作流模式 - Workflow pattern:
//   - Fork -> [BranchA, BranchB, BranchC] -> Join -> 继续 - Fork -> [Parallel Processing] -> Join -> Continue
//
// 超时处理 - Timeout handling:
//   - 可配置超时防止无限等待 - Configurable timeout prevents indefinite waiting
//   - 超时时通过Failure关系路由 - Route via Failure relation on timeout
type JoinNode struct {
	// Config 节点配置
	// Config holds the node configuration including timeout settings
	Config JoinNodeConfiguration
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *JoinNode) Type() string {
	return "join"
}

// New 创建新实例
// New creates a new instance.
func (x *JoinNode) New() types.Node {
	return &JoinNode{Config: JoinNodeConfiguration{}}
}

// Init 初始化组件
// Init initializes the component.
func (x *JoinNode) Init(_ types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, &x.Config)
}

// OnMsg 处理消息，收集并合并来自并行分支的结果
// OnMsg processes incoming messages by collecting results from parallel branches and merging them.
func (x *JoinNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	c := make(chan struct{}, 1)
	var chanCtx context.Context
	var cancel context.CancelFunc
	if x.Config.Timeout > 0 {
		chanCtx, cancel = context.WithTimeout(ctx.GetContext(), time.Duration(x.Config.Timeout)*time.Second)
	} else {
		chanCtx, cancel = context.WithCancel(ctx.GetContext())
	}
	defer cancel()

	var wrapperMsg = msg.Copy()

	ok := ctx.TellCollect(msg, func(msgList []types.WrapperMsg) {
		// 检查context是否已被取消，避免无意义的计算
		select {
		case <-chanCtx.Done():
			return // 提前退出，避免资源浪费
		default:
		}

		wrapperMsg.SetDataType(types.JSON)
		wrapperMsg.SetData(str.ToString(filterEmptyAndRemoveMeta(msgList)))
		mergeMetadata(msgList, &wrapperMsg)
		select {
		case c <- struct{}{}:
		default: // 防止阻塞
		}
	})
	if ok {
		select {
		case <-chanCtx.Done():
			ctx.TellFailure(wrapperMsg, chanCtx.Err())
		case <-c:
			ctx.TellSuccess(wrapperMsg)
		}
	}
}

// Destroy 清理资源
func (x *JoinNode) Destroy() {
}
