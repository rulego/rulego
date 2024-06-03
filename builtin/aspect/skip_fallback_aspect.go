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
	"github.com/rulego/rulego/api/types"
	"sync"
	"sync/atomic"
	"time"
)

// FallbackErr 降级错误
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

// SkipFallbackAspect 组件故障降级切面
// 降级逻辑：
// 1. 节点执行错误次数达到 ErrorCountLimit 后，执行跳过降级
// 2. 节点执行时间超过 LimitDuration 后，恢复执行
type SkipFallbackAspect struct {
	// ErrorCountLimit LimitDuration 错误次数达到多少后，执行跳过降级
	ErrorCountLimit int64
	// LimitDuration 限制降级的时长
	LimitDuration time.Duration
	// PointCutFunc 切入点，默认所有
	// 用于判断是否需要执行降级逻辑
	// 若返回true，则执行降级逻辑
	// 若返回false，则不执行降级逻辑
	PointCutFunc func(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool

	// 规则链错误信息缓存 chainId:chainNodeErrorCache
	chainNodeErrorCache sync.Map
	lock                sync.Mutex
}

func (aspect *SkipFallbackAspect) Order() int {
	return 10
}

func (aspect *SkipFallbackAspect) New() types.Aspect {
	return &SkipFallbackAspect{ErrorCountLimit: 3, LimitDuration: time.Second * 10}
}

func (aspect *SkipFallbackAspect) Type() string {
	return "fallback"
}

// PointCut 判断是否执行降级逻辑 可以指定某类型的节点执行降级逻辑，默认所有节点执行降级逻辑。可以被 PointCutFunc 覆盖
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
func (aspect *SkipFallbackAspect) OnReload(parentCtx types.NodeCtx, ctx types.NodeCtx, err error) error {
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

// chainNodeErrorCache 规则链节点错误记录缓存
type chainNodeErrorCache struct {
	nodeErrorCache sync.Map
}

// NodeError 节点错误记录
type NodeError struct {
	//错误次数
	errorCount int64
	//上次错误时间
	lastErrorTime int64
}
