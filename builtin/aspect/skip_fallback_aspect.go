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
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
)

// FallbackErr is returned when a node execution is skipped due to circuit breaker activation.
// This error indicates that the node has been temporarily disabled due to repeated failures.
//
// FallbackErr 当由于熔断器激活而跳过节点执行时返回。
// 此错误表示节点由于重复失败已被临时禁用。
var FallbackErr = errors.New("skip fallback error")

var (
	// Compile-time check SkipFallbackAspect implements types.AroundAspect.
	_ types.AroundAspect = (*SkipFallbackAspect)(nil)
	// Compile-time check SkipFallbackAspect implements types.AfterAspect.
	_ types.AfterAspect = (*SkipFallbackAspect)(nil)
	// Compile-time check SkipFallbackAspect implements types.OnReloadAspect.
	_ types.OnReloadAspect = (*SkipFallbackAspect)(nil)
	// Compile-time check SkipFallbackAspect implements types.OnDestroyAspect.
	_ types.OnDestroyAspect = (*SkipFallbackAspect)(nil)
)

// SkipFallbackAspect implements a circuit breaker pattern for node failure handling.
// It automatically skips node execution when error count exceeds the threshold,
// providing system resilience and preventing cascade failures.
//
// SkipFallbackAspect 实现节点故障处理的熔断器模式。
// 当错误计数超过阈值时，它自动跳过节点执行，提供系统弹性并防止级联故障。
//
// Circuit Breaker Logic:
// 熔断器逻辑：
//  1. Track error count per node in each rule chain  跟踪每个规则链中每个节点的错误计数
//  2. Skip execution when error count >= ErrorCountLimit  错误计数 >= ErrorCountLimit 时跳过执行
//  3. Automatically recover after LimitDuration expires  LimitDuration 过期后自动恢复
//  4. Reset error count on successful recovery  成功恢复时重置错误计数
//
// Features:
// 功能特性：
//   - Per-node error tracking  每节点错误跟踪
//   - Configurable error threshold  可配置的错误阈值
//   - Time-based automatic recovery  基于时间的自动恢复
//   - Customizable point-cut function  可自定义的切入点函数
//   - Thread-safe concurrent access  线程安全的并发访问
//
// Usage:
// 使用方法：
//
//	// Create with custom configuration
//	// 使用自定义配置创建
//	fallback := &SkipFallbackAspect{
//		ErrorCountLimit: 5,
//		LimitDuration:   time.Minute * 2,
//	}
//
//	// Apply to rule engine
//	// 应用到规则引擎
//	config := types.NewConfig().WithAspects(fallback)
//	engine := rulego.NewRuleEngine(config)
type SkipFallbackAspect struct {
	// ErrorCountLimit is the maximum number of consecutive errors before
	// triggering circuit breaker. Default is 3 if not specified.
	//
	// ErrorCountLimit 是触发熔断器之前的最大连续错误数。
	// 如果未指定，默认为 3。
	ErrorCountLimit int64

	// LimitDuration is the time period for which the circuit breaker remains
	// active. After this duration, the node will be retried. Default is 10 seconds.
	//
	// LimitDuration 是熔断器保持活跃的时间周期。
	// 在此持续时间后，节点将重试。默认为 10 秒。
	LimitDuration time.Duration

	// PointCutFunc is an optional function to determine which nodes should
	// have circuit breaker applied. If nil, applies to all nodes.
	//
	// PointCutFunc 是一个可选函数，用于确定哪些节点应该应用熔断器。
	// 如果为 nil，则应用于所有节点。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule context for the current execution
	//     ctx：当前执行的规则上下文
	//   - msg: Rule message being processed
	//     msg：正在处理的规则消息
	//   - relationType: Type of relation triggering the execution
	//     relationType：触发执行的关系类型
	//
	// Returns:
	// 返回：
	//   - bool: true to apply circuit breaker, false to skip
	//     bool：true 应用熔断器，false 跳过
	PointCutFunc func(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool

	// chainNodeErrorCache stores error information for each rule chain
	// Key: chainId, Value: chainNodeErrorCache
	//
	// chainNodeErrorCache 存储每个规则链的错误信息
	// 键：chainId，值：chainNodeErrorCache
	chainNodeErrorCache sync.Map

	// lock provides synchronization for cache operations
	// lock 为缓存操作提供同步
	lock sync.Mutex
}

// Order returns the execution order of this aspect. Lower values execute earlier.
// SkipFallbackAspect has order 10, ensuring it operates before most other aspects.
//
// Order 返回此切面的执行顺序。值越低，执行越早。
// SkipFallbackAspect 的顺序为 10，确保它在大多数其他切面之前运行。
func (aspect *SkipFallbackAspect) Order() int {
	return 10
}

// New creates a new instance of the circuit breaker aspect with validated configuration.
// It applies default values if ErrorCountLimit or LimitDuration are not specified.
//
// New 创建具有验证配置的熔断器切面新实例。
// 如果未指定 ErrorCountLimit 或 LimitDuration，它会应用默认值。
//
// Default Values:
// 默认值：
//   - ErrorCountLimit: 3 consecutive errors  连续 3 次错误
//   - LimitDuration: 10 seconds  10 秒
//
// Returns:
// 返回：
//   - types.Aspect: Configured circuit breaker aspect instance
//     types.Aspect：配置好的熔断器切面实例
func (aspect *SkipFallbackAspect) New() types.Aspect {
	var errorCountLimit = aspect.ErrorCountLimit
	var limitDuration = aspect.LimitDuration
	if errorCountLimit == 0 {
		errorCountLimit = 3
	}
	if limitDuration == 0 {
		limitDuration = time.Second * 10
	}
	return &SkipFallbackAspect{ErrorCountLimit: errorCountLimit, LimitDuration: limitDuration}
}

// Type returns the unique identifier for this aspect type.
//
// Type 返回此切面类型的唯一标识符。
func (aspect *SkipFallbackAspect) Type() string {
	return "fallback"
}

// PointCut determines which nodes should have circuit breaker logic applied.
// It can be customized using PointCutFunc to target specific node types.
// If PointCutFunc is nil, circuit breaker applies to all nodes by default.
//
// PointCut 确定哪些节点应该应用熔断器逻辑。
// 可以使用 PointCutFunc 自定义以针对特定节点类型。
// 如果 PointCutFunc 为 nil，熔断器默认应用于所有节点。
//
// Parameters:
// 参数：
//   - ctx: Rule context for the current execution
//     ctx：当前执行的规则上下文
//   - msg: Rule message being processed
//     msg：正在处理的规则消息
//   - relationType: Type of relation triggering the execution
//     relationType：触发执行的关系类型
//
// Returns:
// 返回：
//   - bool: true to apply circuit breaker, false to skip
//     bool：true 应用熔断器，false 跳过
func (aspect *SkipFallbackAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	if aspect.PointCutFunc != nil {
		return aspect.PointCutFunc(ctx, msg, relationType)
	}
	return true
}

// Around 判断是否执行降级逻辑
func (aspect *SkipFallbackAspect) Around(ctx types.RuleContext, msg types.RuleMsg, relationType string) (types.RuleMsg, bool) {
	chainId := ctx.RuleChain().GetNodeId().Id
	if chainError, ok := aspect.getChainError(chainId); ok {
		if nodeError, ok := aspect.getNodeError(chainError, ctx.GetSelfId()); ok &&
			nodeError.errorCount >= aspect.ErrorCountLimit {
			if nodeError.lastErrorTime+aspect.LimitDuration.Milliseconds() < time.Now().UnixMilli() {
				//超过时间，清除错误记录
				chainError.nodeErrorCache.Delete(ctx.GetSelfId())
			} else {
				//出错次数达到阈值，执行降级
				ctx.TellFailure(msg, FallbackErr)
				return msg, false
			}

		}
	}

	return msg, true
}

// After 如果出现错误，则记录错误次数
func (aspect *SkipFallbackAspect) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	if relationType == types.Failure {
		chainId := ctx.RuleChain().GetNodeId().Id
		var ok bool
		var chainError *chainNodeErrorCache
		if chainError, ok = aspect.getChainError(chainId); !ok {
			aspect.lock.Lock()
			if chainError, ok = aspect.getChainError(chainId); !ok {
				chainError = &chainNodeErrorCache{}
				aspect.chainNodeErrorCache.Store(chainId, chainError)
			}
			aspect.lock.Unlock()
		}

		var nodeError *NodeError
		if nodeError, ok = aspect.getNodeError(chainError, ctx.GetSelfId()); !ok {
			aspect.lock.Lock()
			if nodeError, ok = aspect.getNodeError(chainError, ctx.GetSelfId()); !ok {
				nodeError = &NodeError{
					errorCount:    1,
					lastErrorTime: time.Now().UnixMilli(),
				}
				chainError.nodeErrorCache.Store(ctx.GetSelfId(), nodeError)
			} else {
				atomic.AddInt64(&nodeError.errorCount, 1)
				atomic.StoreInt64(&nodeError.lastErrorTime, time.Now().UnixMilli())
			}
			aspect.lock.Unlock()

		} else {
			atomic.AddInt64(&nodeError.errorCount, 1)
			atomic.StoreInt64(&nodeError.lastErrorTime, time.Now().UnixMilli())
		}

	}
	return msg
}

// OnReload 节点更新清除错误缓存
func (aspect *SkipFallbackAspect) OnReload(parentCtx types.NodeCtx, ctx types.NodeCtx) error {
	nodeId := ctx.GetNodeId()
	if nodeId.Type == types.CHAIN {
		aspect.chainNodeErrorCache.Delete(nodeId.Id)
	} else {
		if chainCache, ok := aspect.chainNodeErrorCache.Load(parentCtx.GetNodeId().Id); ok {
			if chainError, ok := chainCache.(*chainNodeErrorCache); ok {
				chainError.nodeErrorCache.Delete(nodeId.Id)
			}
		}
	}
	return nil
}

func (aspect *SkipFallbackAspect) OnDestroy(ctx types.NodeCtx) {
	nodeId := ctx.GetNodeId()
	if nodeId.Type == types.CHAIN {
		aspect.chainNodeErrorCache.Delete(nodeId.Id)
	}
}

func (aspect *SkipFallbackAspect) getChainError(chainId string) (*chainNodeErrorCache, bool) {
	if chainCache, ok := aspect.chainNodeErrorCache.Load(chainId); ok {
		if chainError, ok := chainCache.(*chainNodeErrorCache); ok {
			return chainError, true
		}
	}
	return nil, false
}

func (aspect *SkipFallbackAspect) getNodeError(chainCache *chainNodeErrorCache, nodeId string) (*NodeError, bool) {
	if nodeCache, ok := chainCache.nodeErrorCache.Load(nodeId); ok {
		if nodeError, ok := nodeCache.(*NodeError); ok {
			return nodeError, true
		}
	}
	return nil, false
}

// chainNodeErrorCache is a thread-safe cache that stores error information
// for all nodes within a specific rule chain. It uses sync.Map for concurrent
// access to node error records.
//
// chainNodeErrorCache 是一个线程安全的缓存，存储特定规则链内所有节点的错误信息。
// 它使用 sync.Map 来并发访问节点错误记录。
type chainNodeErrorCache struct {
	nodeErrorCache sync.Map // Map[nodeId]*NodeError - stores error data per node  每个节点的错误数据映射
}

// NodeError represents the error tracking information for a specific node.
// It maintains both the count of consecutive errors and the timestamp of
// the last error occurrence for circuit breaker decision making.
//
// NodeError 表示特定节点的错误跟踪信息。
// 它维护连续错误的计数和最后一次错误发生的时间戳，用于熔断器决策。
type NodeError struct {
	// errorCount tracks the number of consecutive errors for this node.
	// Reset to 0 when node execution succeeds or circuit breaker recovers.
	//
	// errorCount 跟踪此节点的连续错误数量。
	// 当节点执行成功或熔断器恢复时重置为 0。
	errorCount int64

	// lastErrorTime stores the timestamp (in milliseconds) of the most recent error.
	// Used to determine when the circuit breaker should attempt recovery.
	//
	// lastErrorTime 存储最近错误的时间戳（毫秒）。
	// 用于确定熔断器何时应该尝试恢复。
	lastErrorTime int64
}
