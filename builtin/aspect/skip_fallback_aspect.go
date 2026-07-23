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
// FallbackErr returns when node execution is skipped due to fuse activation.
// This error indicates that the node has been temporarily disabled due to repeated failures.
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
// SkipFallbackAspect implements the fuse mode for node fault handling.
// When the error count exceeds the threshold, it automatically skips node execution, providing system resilience and preventing cascading failures.
//
// Circuit Breaker Logic:
// Fuse logic:
//  1. Track error count per node in each rule chain
//  2. Skip execution when error count >= ErrorCountLimit
//  3. Automatically recover after LimitDuration expires
//  4. Reset error count on successful recovery
//
// Features:
// Features:
//   - Per-node error tracking
//   - Configurable error threshold
//   - Time-based automatic recovery
//   - Customizable point-cut function
//   - Thread-safe concurrent access
//
// Usage:
// How to use:
//
//	// Create with custom configuration
//	Created using custom configurations
//	fallback := &SkipFallbackAspect{
//		ErrorCountLimit: 5,
//		LimitDuration:   time.Minute * 2,
//	}
//
//	// Apply to rule engine
//	Applied to the rule engine
//	config := types.NewConfig().WithAspects(fallback)
//	engine := rulego.NewRuleEngine(config)
type SkipFallbackAspect struct {
	// ErrorCountLimit is the maximum number of consecutive errors before
	// triggering circuit breaker. Default is 3 if not specified.
	//
	// ErrorCountLimit is the maximum number of consecutive errors before the fuse is triggered.
	// If not specified, the default is 3.
	ErrorCountLimit int64

	// LimitDuration is the time period for which the circuit breaker remains
	// active. After this duration, the node will be retried. Default is 10 seconds.
	//
	// LimitDuration is the period during which the fuse remains active.
	// After this duration, the node will retry. The default is 10 seconds.
	LimitDuration time.Duration

	// PointCutFunc is an optional function to determine which nodes should
	// have circuit breaker applied. If nil, applies to all nodes.
	//
	// PointCutFunc is an optional function used to determine which nodes should be fused.
	// If nil, it applies to all nodes.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule context for the current execution
	//     ctx: The context of the currently executed rule
	//   - msg: Rule message being processed
	//     msg: The rule message being processed
	//   - relationType: Type of relation triggering the execution
	//     relationType: The type of relationship that triggers execution
	//
	// Returns:
	// Returns:
	//   - bool: true to apply circuit breaker, false to skip
	//     bool:true Apply the fuse, false skips
	PointCutFunc func(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool

	// chainNodeErrorCache stores error information for each rule chain
	// Key: chainId, Value: chainNodeErrorCache
	//
	// chainNodeErrorCache stores error information for each rule chain
	// Key: chainId, Value: chainNodeErrorCache
	chainNodeErrorCache sync.Map

	// lock provides synchronization for cache operations
	// lock provides synchronization for cache operations
	lock sync.Mutex
}

// Order returns the execution order of this aspect. Lower values execute earlier.
// SkipFallbackAspect has order 10, ensuring it operates before most other aspects.
//
// Order returns the execution order of this aspect. The lower the value, the earlier it is executed.
// SkipFallbackAspect has order 10, ensuring it runs before most other aspects.
func (aspect *SkipFallbackAspect) Order() int {
	return 10
}

// New creates a new instance of the circuit breaker aspect with validated configuration.
// It applies default values if ErrorCountLimit or LimitDuration are not specified.
//
// New creates a new instance of the fuse cross-section with a verified configuration.
// If ErrorCountLimit or LimitDuration is not specified, the default value will be applied.
//
// Default Values:
// Default values:
//   - ErrorCountLimit: 3 consecutive errors
//   - LimitDuration: 10 seconds 10 seconds
//
// Returns:
// Returns:
//   - types.Aspect: Configured circuit breaker aspect instance
//     types.Aspect: Example of the configured fuse cross-section
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
// Type returns a unique identifier for this facet type.
func (aspect *SkipFallbackAspect) Type() string {
	return "fallback"
}

// PointCut determines which nodes should have circuit breaker logic applied.
// It can be customized using PointCutFunc to target specific node types.
// If PointCutFunc is nil, circuit breaker applies to all nodes by default.
//
// PointCut determines which nodes should apply fuse logic.
// You can customize it with PointCutFunc for specific node types.
// If PointCutFunc is nil, the fuse is applied to all nodes by default.
//
// Parameters:
// Parameters:
//   - ctx: Rule context for the current execution
//     ctx: The context of the currently executed rule
//   - msg: Rule message being processed
//     msg: The rule message being processed
//   - relationType: Type of relation triggering the execution
//     relationType: The type of relationship that triggers execution
//
// Returns:
// Returns:
//   - bool: true to apply circuit breaker, false to skip
//     bool:true Apply the fuse, false skips
func (aspect *SkipFallbackAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	if aspect.PointCutFunc != nil {
		return aspect.PointCutFunc(ctx, msg, relationType)
	}
	return true
}

// Around determines whether to execute the demotion logic
func (aspect *SkipFallbackAspect) Around(ctx types.RuleContext, msg types.RuleMsg, relationType string) (types.RuleMsg, bool) {
	chainId := ctx.RuleChain().GetNodeId().Id
	if chainError, ok := aspect.getChainError(chainId); ok {
		if nodeError, ok := aspect.getNodeError(chainError, ctx.GetSelfId()); ok &&
			nodeError.errorCount >= aspect.ErrorCountLimit {
			if nodeError.lastErrorTime+aspect.LimitDuration.Milliseconds() < time.Now().UnixMilli() {
				//If the time is over, the error records will be cleared
				chainError.nodeErrorCache.Delete(ctx.GetSelfId())
			} else {
				//If the number of errors reaches the threshold, the downgrade is executed
				ctx.TellFailure(msg, FallbackErr)
				return msg, false
			}

		}
	}

	return msg, true
}

// After If an error occurs, record the number of errors
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

// OnReload node update clears error cache
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
// chainNodeErrorCache is a thread-safe cache that stores error information for all nodes within a specific rule chain.
// It uses sync.Map to concurrently record node errors.
type chainNodeErrorCache struct {
	nodeErrorCache sync.Map // Map[nodeId]*NodeError - stores error data per node
}

// NodeError represents the error tracking information for a specific node.
// It maintains both the count of consecutive errors and the timestamp of
// the last error occurrence for circuit breaker decision making.
//
// NodeError represents error tracking information for a specific node.
// It maintains the count of consecutive errors and the timestamp of the last error occurred, used for fuse decision-making.
type NodeError struct {
	// errorCount tracks the number of consecutive errors for this node.
	// Reset to 0 when node execution succeeds or circuit breaker recovers.
	//
	// errorCount tracks the number of consecutive errors at this node.
	// Resets to 0 when the node executes successfully or the fuse recovers.
	errorCount int64

	// lastErrorTime stores the timestamp (in milliseconds) of the most recent error.
	// Used to determine when the circuit breaker should attempt recovery.
	//
	// lastErrorTime stores the most recent error timestamp (in milliseconds).
	// Used to determine when the fuse should attempt restoration.
	lastErrorTime int64
}
