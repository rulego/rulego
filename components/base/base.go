/*
 * Copyright 2024 The RuleGo Authors.
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

// Package base provides foundational components and utilities for the RuleGo rule engine.
package base

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
)

var (
	ErrNetPoolNil    = errors.New("node pool is nil")
	ErrClientNotInit = errors.New("client not init")
)

var NodeUtils = &nodeUtils{}

type nodeUtils struct {
}

func (n *nodeUtils) GetChainCtx(configuration types.Configuration) types.ChainCtx {
	if v, ok := configuration[types.NodeConfigurationKeyChainCtx]; ok {
		if chainCtx, ok := v.(types.ChainCtx); ok {
			return chainCtx
		}
	}
	return nil
}
func (n *nodeUtils) GetSelfDefinition(configuration types.Configuration) types.RuleNode {
	if v, ok := configuration[types.NodeConfigurationKeySelfDefinition]; ok {
		if ruleNode, ok := v.(types.RuleNode); ok {
			return ruleNode
		}
	}
	return types.RuleNode{}
}

func (n *nodeUtils) GetVars(configuration types.Configuration) map[string]interface{} {
	if v, ok := configuration[types.Vars]; ok {
		fromVars := make(map[string]interface{})
		fromVars[types.Vars] = v
		return fromVars
	} else {
		return nil
	}
}

func (n *nodeUtils) GetEvn(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return n.getEvnAndMetadata(ctx, msg, false)
}

// GetEvnAndMetadata 和Metadata key合并
func (n *nodeUtils) GetEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return n.getEvnAndMetadata(ctx, msg, true)
}

func (n *nodeUtils) IsNetPool(config types.Config, server string) bool {
	return strings.HasPrefix(server, types.NodeConfigurationPrefixInstanceId)
}

func (n *nodeUtils) GetInstanceId(config types.Config, server string) string {
	if n.IsNetPool(config, server) {
		//截取资源ID
		return server[len(types.NodeConfigurationPrefixInstanceId):]
	}
	return ""
}

func (n *nodeUtils) IsInitNetResource(_ types.Config, configuration types.Configuration) bool {
	_, ok := configuration[types.NodeConfigurationKeyIsInitNetResource]
	return ok
}

func (n *nodeUtils) getEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	// 直接调用ctx的GetEvnAndMetadata方法
	return ctx.GetEnv(msg, useMetadata)
}

// ReleaseEvn 释放环境变量map到对象池，减少GC压力
// 注意：调用此方法后不应再使用传入的map
// 现在由 RuleContext 自动管理，此方法保留用于向后兼容
func (n *nodeUtils) ReleaseEvn(evn map[string]interface{}) {
	// 环境变量的释放现在由 RuleContext 自动管理
	// 此方法保留用于向后兼容，实际上不需要手动释放
}

// PrepareJsData 准备传递给JavaScript脚本的数据
// 根据消息的数据类型进行不同的处理：
// - JSON类型：解析为map以便JavaScript处理
// - BINARY类型：转换为字节数组，JavaScript将其视为Uint8Array
// - 其他类型：使用原始字符串数据
func (n *nodeUtils) PrepareJsData(msg types.RuleMsg) interface{} {
	var data interface{}

	// 根据数据类型进行不同的处理
	switch msg.DataType {
	case types.JSON:
		// JSON类型：尝试解析为map以便JavaScript处理
		if dataMap, err := msg.GetJsonData(); err == nil {
			data = dataMap
		} else {
			data = msg.GetData()
		}

	case types.BINARY:
		// 二进制类型：直接传递字节数组，JavaScript会将其视为Uint8Array
		data = msg.GetBytes()
	default:
		// 其他类型：使用原始字符串数据
		data = msg.GetData()
	}

	return data
}

// SharedNode 共享资源组件，通过 Get 获取共享实例，多个节点可以在共享池中获取相同的实例
// 例如：mqtt 客户端、数据库客户端，也可以http server以及是可复用的节点。
type SharedNode[T any] struct {
	//节点类型
	NodeType string
	//配置
	RuleConfig types.Config
	//资源ID
	InstanceId string
	//初始化实例资源函数
	InitInstanceFunc func() (T, error)
	////初始化资源资源，防止并发初始化
	//lock int32
	//是否从资源池获取
	isFromPool bool
	Locker     sync.Mutex
	// 优雅关闭相关字段
	isShuttingDown int64 // 使用原子操作
	activeOps      int64 // 活跃操作计数器
}

// Init 初始化，如果 resourcePath 为 ref:// 开头，则从网络资源池获取，否则调用 initInstanceFunc 初始化
// initNow=true，会在立刻初始化，否则在 GetInstance() 时候初始化
func (x *SharedNode[T]) Init(ruleConfig types.Config, nodeType, resourcePath string, initNow bool, initInstanceFunc func() (T, error)) error {
	x.RuleConfig = ruleConfig
	x.NodeType = nodeType

	if instanceId := NodeUtils.GetInstanceId(ruleConfig, resourcePath); instanceId == "" {
		x.InitInstanceFunc = initInstanceFunc
		if initNow {
			//非资源池方式，初始化
			_, err := x.InitInstanceFunc()
			return err
		}
	} else {
		x.isFromPool = true
		x.InstanceId = instanceId
	}
	return nil
}

// IsInit 是否初始化过
func (x *SharedNode[T]) IsInit() bool {
	return x.NodeType != ""
}

// GetInstance 获取共享实例
func (x *SharedNode[T]) GetInstance() (interface{}, error) {
	return x.Get()
}

// Get 获取共享实例，并返回具体类型
func (x *SharedNode[T]) Get() (T, error) {
	if x.InstanceId != "" {
		//从网络资源池获取
		if x.RuleConfig.NetPool == nil {
			return zeroValue[T](), ErrNetPoolNil
		}
		if p, err := x.RuleConfig.NetPool.GetInstance(x.InstanceId); err == nil {
			return p.(T), nil
		} else {
			return zeroValue[T](), err
		}
	} else if x.InitInstanceFunc != nil {
		//根据当前组件配置初始化一个客户端
		return x.InitInstanceFunc()
	} else {
		return zeroValue[T](), ErrClientNotInit
	}
}

//// TryLock 获取锁，如果获取不到则返回false
//func (x *SharedNode[T]) TryLock() bool {
//	return atomic.CompareAndSwapInt32(&x.lock, 0, 1)
//}
//
//// ReleaseLock 释放锁
//func (x *SharedNode[T]) ReleaseLock() {
//	atomic.StoreInt32(&x.lock, 0)
//}

// IsFromPool 是否从资源池获取
func (x *SharedNode[T]) IsFromPool() bool {
	return x.isFromPool
}

// BeginOp 开始一个操作，增加活跃操作计数
func (x *SharedNode[T]) BeginOp() {
	atomic.AddInt64(&x.activeOps, 1)
}

// EndOp 结束一个操作，减少活跃操作计数
func (x *SharedNode[T]) EndOp() {
	atomic.AddInt64(&x.activeOps, -1)
}

// IsShuttingDown 检查是否正在关闭
func (x *SharedNode[T]) IsShuttingDown() bool {
	return atomic.LoadInt64(&x.isShuttingDown) == 1
}

// BeginShutdown 开始优雅关闭过程，设置关闭状态
func (x *SharedNode[T]) BeginShutdown() {
	atomic.StoreInt64(&x.isShuttingDown, 1)
}

// GracefulShutdown 优雅关闭，等待活跃操作完成
// timeout: 等待超时时间，如果为0则使用默认10秒
// closeFunc: 可选的关闭函数，用于关闭非资源池管理的资源
func (x *SharedNode[T]) GracefulShutdown(timeout time.Duration, closeFunc func()) {
	// 设置默认超时时间
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	if x.isFromPool {
		// 共享资源模式：只等待当前活跃操作完成，不设置关闭状态
		// 共享资源的生命周期由资源池管理，不应被单个节点影响
		x.waitForActiveOpsComplete(timeout)

		// 执行自定义关闭函数，让具体节点决定是否需要清理本地资源
		// 注意：不调用 BeginShutdown()，因为其他节点可能还在使用共享资源
		if closeFunc != nil {
			closeFunc()
		}
	} else {
		// 非共享资源模式：立即设置关闭状态，等待操作完成，然后关闭资源
		x.BeginShutdown()
		x.waitForActiveOpsComplete(timeout)

		// 执行自定义关闭函数
		if closeFunc != nil {
			closeFunc()
		}
	}
}

// waitForActiveOpsComplete 等待所有活跃操作完成
// 返回 true 表示正常完成，false 表示超时
func (x *SharedNode[T]) waitForActiveOpsComplete(timeout time.Duration) bool {
	// 如果没有活跃操作，立即返回
	if atomic.LoadInt64(&x.activeOps) == 0 {
		return true
	}

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 使用自适应的检查间隔
	checkInterval := x.calculateCheckInterval(timeout)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 超时
			return false
		case <-ticker.C:
			if atomic.LoadInt64(&x.activeOps) == 0 {
				// 所有操作完成
				return true
			}
		}
	}
}

// calculateCheckInterval 根据超时时间计算合适的检查间隔
// 优化性能：短超时使用更频繁的检查，长超时使用较少的检查
func (x *SharedNode[T]) calculateCheckInterval(timeout time.Duration) time.Duration {
	switch {
	case timeout <= 1*time.Second:
		return 10 * time.Millisecond // 短超时：10ms 检查
	case timeout <= 5*time.Second:
		return 50 * time.Millisecond // 中等超时：50ms 检查
	default:
		return 100 * time.Millisecond // 长超时：100ms 检查
	}
}

// GetActiveOpsCount 获取当前活跃操作数量
func (x *SharedNode[T]) GetActiveOpsCount() int64 {
	return atomic.LoadInt64(&x.activeOps)
}

// zeroValue 函数用于返回 T 类型的零值
func zeroValue[T any]() T {
	var zero T
	return zero
}

// 使用示例：
//
// 在继承SharedNode的组件中：
//
// type MyNode struct {
//     base.SharedNode[MyClient]
//     // 其他字段...
// }
//
// func (x *MyNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
//     defer x.SharedNode.EndOp()
//     x.SharedNode.BeginOp()
//
//     if x.SharedNode.IsShuttingDown() {
//         ctx.TellFailure(msg, errors.New("component is shutting down"))
//         return
//     }
//
//     client, err := x.SharedNode.Get()
//     if err != nil {
//         ctx.TellFailure(msg, err)
//         return
//     }
//     // 使用client处理消息...
// }
//
// func (x *MyNode) Destroy() {
//     x.SharedNode.GracefulShutdown(0, func() {
//         // 只在非资源池模式下关闭本地资源
//         if x.localClient != nil {
//             x.localClient.Close()
//         }
//     })
// }
