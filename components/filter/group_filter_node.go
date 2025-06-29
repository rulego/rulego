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

package filter

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init 注册GroupFilterNode组件
// init registers the GroupFilterNode component with the default registry.
func init() {
	Registry.Add(&GroupFilterNode{})
}

// GroupFilterNodeConfiguration GroupFilterNode配置结构
// GroupFilterNodeConfiguration defines the configuration structure for the GroupFilterNode component.
type GroupFilterNodeConfiguration struct {
	// AllMatches 决定组评估逻辑
	// AllMatches determines the group evaluation logic:
	//   - true: All nodes must return True for message to route to "True" chain
	//   - false: Any node returning True will route message to "True" chain
	AllMatches bool

	// NodeIds 指定要包含在组中的过滤器节点列表
	// NodeIds specifies the list of filter nodes to include in the group.
	// Can be provided as:
	//   - string: Comma-separated node IDs "node1,node2,node3"
	//   - []string: Array of node ID strings
	//   - []interface{}: Array of node ID values
	NodeIds interface{}

	// Timeout 指定执行超时时间（秒），默认值0表示无超时限制
	// Timeout specifies the execution timeout in seconds.
	// Default value 0 means no timeout limit.
	Timeout int
}

// GroupFilterNode 将多个过滤器节点分组并集体评估的过滤组件
// GroupFilterNode groups multiple filter nodes and evaluates them collectively.
//
// 核心算法：
// Core Algorithm:
// 1. 并发执行所有配置的过滤器节点 - Execute all configured filter nodes concurrently
// 2. 使用原子操作聚合True/False结果 - Aggregate True/False results using atomic operations
// 3. 根据AllMatches配置应用AND/OR逻辑 - Apply AND/OR logic based on AllMatches configuration
// 4. 实现早期终止优化减少不必要计算 - Implement early termination optimization
//
// 评估逻辑 - Evaluation logic:
//   - AllMatches=true (AND逻辑): 所有节点都必须返回True - All nodes must return True
//   - AllMatches=false (OR逻辑): 任何节点返回True就成功 - Any node returning True is success
//
// 超时处理 - Timeout handling:
//   - 可配置超时防止无限等待 - Configurable timeout prevents indefinite waiting
//   - 超时时路由到Failure关系 - Route to Failure relation on timeout
type GroupFilterNode struct {
	// Config 组过滤器配置
	// Config holds the group filter configuration
	Config GroupFilterNodeConfiguration

	// NodeIdList 要执行的节点ID列表
	// NodeIdList contains the parsed list of node IDs to execute
	NodeIdList []string

	// Length 组中节点总数
	// Length is the total number of nodes in the group
	Length int32
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *GroupFilterNode) Type() string {
	return "groupFilter"
}

// New 创建新实例
// New creates a new instance.
func (x *GroupFilterNode) New() types.Node {
	return &GroupFilterNode{Config: GroupFilterNodeConfiguration{AllMatches: false}}
}

// Init 初始化组件
// Init initializes the component.
func (x *GroupFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	var nodeIds []string
	if v, ok := x.Config.NodeIds.(string); ok {
		nodeIds = strings.Split(v, ",")
	} else if v, ok := x.Config.NodeIds.([]string); ok {
		nodeIds = v
	} else if v, ok := x.Config.NodeIds.([]interface{}); ok {
		for _, item := range v {
			nodeIds = append(nodeIds, str.ToString(item))
		}
	}
	for _, nodeId := range nodeIds {
		if v := strings.Trim(nodeId, ""); v != "" {
			x.NodeIdList = append(x.NodeIdList, v)
		}
	}
	x.Length = int32(len(x.NodeIdList))
	return err
}

// OnMsg 处理消息，并发执行所有配置的过滤器节点并根据配置的逻辑聚合结果
// OnMsg processes incoming messages by executing all configured filter nodes concurrently.
func (x *GroupFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.Length == 0 {
		ctx.TellFailure(msg, errors.New("nodeIds is empty"))
		return
	}
	var endCount int32
	var trueCount int32 // 新增：跟踪True结果数量
	var completed int32
	c := make(chan bool, 1)
	var chanCtx context.Context
	var cancel context.CancelFunc
	if x.Config.Timeout > 0 {
		chanCtx, cancel = context.WithTimeout(ctx.GetContext(), time.Duration(x.Config.Timeout)*time.Second)
	} else {
		chanCtx, cancel = context.WithCancel(ctx.GetContext())
	}

	defer cancel()

	//执行节点列表逻辑
	for _, nodeId := range x.NodeIdList {
		ctx.TellNode(chanCtx, nodeId, msg, true, func(callbackCtx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// 检查context是否已被取消，避免无意义的计算
			select {
			case <-chanCtx.Done():
				return // 提前退出，避免资源浪费
			default:
			}

			// 直接使用原子操作获取当前计数，避免竞态窗口
			currentEndCount := atomic.AddInt32(&endCount, 1)
			var currentTrueCount int32
			if relationType == types.True {
				currentTrueCount = atomic.AddInt32(&trueCount, 1)
			} else {
				currentTrueCount = atomic.LoadInt32(&trueCount)
			}

			// 判断是否应该结束并发送结果
			var shouldComplete bool
			var result bool

			if x.Config.AllMatches {
				// AllMatches=true: 有任何False就立即返回False，所有都是True才返回True
				if relationType != types.True {
					shouldComplete = true
					result = false
				} else if currentEndCount >= x.Length && currentTrueCount >= x.Length {
					shouldComplete = true
					result = true
				}
			} else {
				// AllMatches=false: 有任何True就立即返回True，所有都完成且无True才返回False
				if relationType == types.True {
					shouldComplete = true
					result = true
				} else if currentEndCount >= x.Length && currentTrueCount == 0 {
					shouldComplete = true
					result = false
				}
			}

			// 使用CAS确保只有一个goroutine能发送结果
			if shouldComplete && atomic.CompareAndSwapInt32(&completed, 0, 1) {
				// 使用非阻塞发送，防止在超时情况下channel阻塞
				select {
				case c <- result:
					// 发送成功
				default:
					// Channel已满或无接收者（可能主函数已超时退出），放弃发送
				}
			}
		}, nil)
	}

	// 等待执行结束或者超时
	select {
	case <-chanCtx.Done():
		ctx.TellFailure(msg, chanCtx.Err())
	case r := <-c:
		if r {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *GroupFilterNode) Destroy() {
}
