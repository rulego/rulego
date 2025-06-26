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

package aspect

import (
	"sync"
	"time"

	"github.com/rulego/rulego/api/types"
)

var (
	// 编译时检查RunSnapshotAspect实现的接口
	_ types.StartAspect                = (*RunSnapshotAspect)(nil)
	_ types.BeforeAspect               = (*RunSnapshotAspect)(nil)
	_ types.AfterAspect                = (*RunSnapshotAspect)(nil)
	_ types.CompletedAspect            = (*RunSnapshotAspect)(nil)
	_ types.RuleChainCompletedListener = (*RunSnapshotAspect)(nil)
	_ types.NodeCompletedListener      = (*RunSnapshotAspect)(nil)
	_ types.DebugListener              = (*RunSnapshotAspect)(nil)
	_ types.RuleContextInitAspect      = (*RunSnapshotAspect)(nil)
)

// RunSnapshotAspect 规则链运行快照收集切面
// 新架构设计：
// 1. 利用 SharedContextState 中预分类的监听器，避免重复遍历
// 2. 每个 RuleContext 通过 New() 获得独立实例，确保状态隔离
type RunSnapshotAspect struct {
	// RuleContext 级别的状态字段
	msgId                string                                                                                                      // 当前处理的消息ID
	startTs              int64                                                                                                       // 开始时间
	onRuleChainCompleted func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot)                                            // 规则链完成回调
	onNodeCompleted      func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog)                                                // 节点完成回调
	onDebugCustom        func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) // 调试回调
	logs                 map[string]*types.RuleNodeRunLog                                                                            // 节点执行日志
	mutex                sync.RWMutex                                                                                                // 保护日志的并发访问
	initialized          bool                                                                                                        // 是否已初始化

	hasCallbacks bool // 是否有任何回调函数，避免重复检查
}

func (aspect *RunSnapshotAspect) Order() int {
	return 950
}

func (aspect *RunSnapshotAspect) New() types.Aspect {
	// 🔥 关键：每次创建新实例，确保 RuleContext 级别的状态隔离
	return &RunSnapshotAspect{
		logs: make(map[string]*types.RuleNodeRunLog),
	}
}

func (aspect *RunSnapshotAspect) Type() string {
	return "runSnapshot"
}

// InitWithContext 实现 RuleContextInitAspect 接口
// 为每次规则执行创建一个新的 RunSnapshotAspect 实例，确保执行之间的状态隔离
func (aspect *RunSnapshotAspect) InitWithContext(ctx types.RuleContext) types.Aspect {
	// 创建执行特定的新实例，确保每次执行都有独立的状态
	// 这解决了并发执行时的状态混乱问题
	newInstance := &RunSnapshotAspect{
		logs: make(map[string]*types.RuleNodeRunLog), // 每次执行独立的日志容器
		// 继承引擎级实例的回调函数配置（如果有的话）
		onRuleChainCompleted: aspect.onRuleChainCompleted,
		onNodeCompleted:      aspect.onNodeCompleted,
		onDebugCustom:        aspect.onDebugCustom,
	}

	// 从当前执行上下文中提取配置
	config := ctx.Config()

	// 根据当前执行的 RuleContext 进行实例特定的初始化
	// 可以基于执行上下文设置不同的行为
	if config.Logger != nil {
		var chainId string
		if ctx.RuleChain() != nil {
			chainId = ctx.RuleChain().GetNodeId().Id
		} else {
			chainId = "unknown"
		}
		config.Logger.Printf("RunSnapshotAspect: Created execution-specific instance for chainId=%s", chainId)
	}

	// 初始化新实例的 hasCallbacks 缓存
	newInstance.updateCallbackCache()

	return newInstance
}

// PointCut 切入点：高效检查是否需要收集快照
func (aspect *RunSnapshotAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	// 使用缓存的判断结果，避免重复检查
	return aspect.hasCallbacks
}

// Start 规则链开始执行时初始化快照
func (aspect *RunSnapshotAspect) Start(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	// 初始化当前实例的状态
	aspect.initSnapshot(ctx, msg)
	return msg, nil
}

// Before 节点执行前记录入口信息
func (aspect *RunSnapshotAspect) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	if !aspect.hasCallbacks {
		return msg // 快速路径：没有回调则直接返回
	}

	// 收集快照数据
	aspect.collectLog(ctx, types.In, msg, relationType, nil)

	// 优化调试信息处理：直接调用而不通过ctx.OnDebug
	if aspect.onDebugCustom != nil {
		var chainId string
		if ctx.RuleChain() != nil {
			chainId = ctx.RuleChain().GetNodeId().Id
		}
		nodeId := ""
		if ctx.Self() != nil {
			nodeId = ctx.Self().GetNodeId().Id
		}
		aspect.onDebugCustom(chainId, types.In, nodeId, msg, relationType, nil)
	}

	return msg
}

// After 节点执行后记录出口信息
func (aspect *RunSnapshotAspect) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	if !aspect.hasCallbacks {
		return msg // 快速路径：没有回调则直接返回
	}

	// 收集快照数据
	aspect.collectLog(ctx, types.Out, msg, relationType, err)

	// 优化调试信息处理：直接调用而不通过ctx.OnDebug
	if aspect.onDebugCustom != nil {
		var chainId string
		if ctx.RuleChain() != nil {
			chainId = ctx.RuleChain().GetNodeId().Id
		}
		nodeId := ""
		if ctx.Self() != nil {
			nodeId = ctx.Self().GetNodeId().Id
		}
		aspect.onDebugCustom(chainId, types.Out, nodeId, msg, relationType, err)
	}

	return msg
}

// Completed 规则链所有分支执行完成时生成最终快照
func (aspect *RunSnapshotAspect) Completed(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	if aspect.hasCallbacks {
		// 触发规则链完成回调
		aspect.completeSnapshot(ctx)
	}
	return msg
}

// SetOnRuleChainCompleted 实现 RuleChainCompletedListener 接口
func (aspect *RunSnapshotAspect) SetOnRuleChainCompleted(onCallback func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot)) {
	aspect.onRuleChainCompleted = onCallback
	aspect.updateCallbackCache() // 更新缓存
}

// SetOnNodeCompleted 实现 NodeCompletedListener 接口
func (aspect *RunSnapshotAspect) SetOnNodeCompleted(onCallback func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog)) {
	aspect.onNodeCompleted = onCallback
	aspect.updateCallbackCache() // 更新缓存
}

// SetOnDebug 实现 DebugListener 接口
func (aspect *RunSnapshotAspect) SetOnDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error)) {
	aspect.onDebugCustom = onDebug
	aspect.updateCallbackCache() // 更新缓存
}

// ============ 私有方法实现 ============

// updateCallbackCache 更新回调缓存，提高PointCut性能
func (aspect *RunSnapshotAspect) updateCallbackCache() {
	aspect.hasCallbacks = aspect.onRuleChainCompleted != nil ||
		aspect.onNodeCompleted != nil ||
		aspect.onDebugCustom != nil
}

// initSnapshot 初始化快照
func (aspect *RunSnapshotAspect) initSnapshot(ctx types.RuleContext, msg types.RuleMsg) {
	if aspect.initialized {
		return // 避免重复初始化
	}

	aspect.msgId = msg.Id
	aspect.startTs = time.Now().UnixMilli()
	aspect.initialized = true
}

// collectLog 收集节点执行日志
func (aspect *RunSnapshotAspect) collectLog(ctx types.RuleContext, flowType string, msg types.RuleMsg, relationType string, err error) {
	if !aspect.initialized || ctx.Self() == nil {
		return
	}

	nodeId := ctx.Self().GetNodeId().Id

	aspect.mutex.Lock()
	defer aspect.mutex.Unlock()

	// 获取或创建节点日志
	nodeLog, exists := aspect.logs[nodeId]
	if !exists {
		nodeLog = &types.RuleNodeRunLog{
			Id: nodeId,
		}
		aspect.logs[nodeId] = nodeLog
	}

	// 记录日志数据
	if flowType == types.In {
		nodeLog.InMsg = msg
		nodeLog.StartTs = time.Now().UnixMilli()
	} else if flowType == types.Out {
		nodeLog.OutMsg = msg
		nodeLog.RelationType = relationType
		if err != nil {
			nodeLog.Err = err.Error()
		}
		nodeLog.EndTs = time.Now().UnixMilli()

		if aspect.onNodeCompleted != nil {
			aspect.onNodeCompleted(ctx, *nodeLog)
		}
	}
}

// completeSnapshot 完成快照并触发回调
func (aspect *RunSnapshotAspect) completeSnapshot(ctx types.RuleContext) {
	if !aspect.initialized || aspect.onRuleChainCompleted == nil {
		return
	}

	snapshot := aspect.buildRuleChainSnapshot(ctx)
	aspect.onRuleChainCompleted(ctx, snapshot)
}

// buildRuleChainSnapshot 构建规则链运行快照
func (aspect *RunSnapshotAspect) buildRuleChainSnapshot(ctx types.RuleContext) types.RuleChainRunSnapshot {
	aspect.mutex.RLock()
	defer aspect.mutex.RUnlock()

	// 预分配容量，减少slice扩容开销
	logs := make([]types.RuleNodeRunLog, 0, len(aspect.logs))
	for _, log := range aspect.logs {
		logs = append(logs, *log)
	}

	snapshot := types.RuleChainRunSnapshot{
		Id:      aspect.msgId,
		StartTs: aspect.startTs,
		EndTs:   time.Now().UnixMilli(),
		Logs:    logs,
	}
	//snapshot.RuleChain 通过回调函数按需获取，方法：*(ctx.RuleChain().(types.ChainCtx)).Definition()
	return snapshot
}
