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

package engine

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
)

const (
	// defaultNodeIdPrefix is the prefix used for auto-generated node IDs
	// when no explicit ID is provided in the node definition.
	// defaultNodeIdPrefix 是当节点定义中没有提供明确 ID 时用于自动生成节点 ID 的前缀。
	defaultNodeIdPrefix = "node"
)

// RuleNodeCtx represents an instance of a node component within the rule engine.
// It acts as a wrapper around the actual node implementation, providing additional
// context and metadata required for rule chain execution.
//
// RuleNodeCtx 表示规则引擎中节点组件的实例。
// 它充当实际节点实现的包装器，提供规则链执行所需的额外上下文和元数据。
//
// Architecture:
// 架构：
//
//	RuleNodeCtx embeds the types.Node interface, allowing it to act as both
//	a node wrapper and a node implementation. This design provides:
//	RuleNodeCtx 嵌入 types.Node 接口，允许它既充当节点包装器又充当节点实现。
//	此设计提供：
//	- Direct access to node methods through interface embedding  通过接口嵌入直接访问节点方法
//	- Additional context and configuration management  额外的上下文和配置管理
//	- Thread-safe operations with mutex protection  使用互斥锁保护的线程安全操作
//	- Hot reloading capabilities  热重载功能
type RuleNodeCtx struct {
	// types.Node is the embedded node implementation providing the core functionality.
	// This embedding allows RuleNodeCtx to act as a node while adding wrapper capabilities.
	// types.Node 是嵌入的节点实现，提供核心功能。
	// 这种嵌入允许 RuleNodeCtx 在添加包装器功能的同时充当节点。
	types.Node

	// ChainCtx provides access to the parent rule chain context,
	// enabling node-to-chain communication and access to shared resources.
	// ChainCtx 提供对父规则链上下文的访问，支持节点到链的通信和对共享资源的访问。
	ChainCtx *RuleChainCtx

	// SelfDefinition contains the configuration and metadata for this specific node,
	// including its type, ID, configuration parameters, and behavioral settings.
	// SelfDefinition 包含此特定节点的配置和元数据，
	// 包括其类型、ID、配置参数和行为设置。
	SelfDefinition *types.RuleNode

	// config holds the global rule engine configuration,
	// providing access to component registry, parsers, and global settings.
	// config 保存全局规则引擎配置，提供对组件注册表、解析器和全局设置的访问。
	config types.Config

	// aspects contains the list of AOP aspects applied to this node,
	// enabling cross-cutting concerns like logging, validation, and metrics.
	// aspects 包含应用于此节点的 AOP 切面列表，
	// 支持如日志、验证和指标等横切关注点。
	aspects types.AspectList

	// isInitNetResource indicates whether network resources should be initialized
	// for this node. This flag is used for nodes that require network connectivity.
	// isInitNetResource 指示是否应为此节点初始化网络资源。
	// 此标志用于需要网络连接的节点。
	isInitNetResource bool

	// sync.RWMutex provides thread-safe access to the node context,
	// ensuring concurrent safety during hot reloads and message processing.
	// sync.RWMutex 为节点上下文提供线程安全访问，
	// 确保在热重载和消息处理期间的并发安全。
	sync.RWMutex
}

// InitRuleNodeCtx initializes a RuleNodeCtx with the given parameters.
// This is the standard initialization function for regular nodes without network resources.
//
// InitRuleNodeCtx 使用给定参数初始化 RuleNodeCtx。
// 这是不带网络资源的常规节点的标准初始化函数。
//
// Parameters:
// 参数：
//   - config: Global rule engine configuration  全局规则引擎配置
//   - chainCtx: Parent rule chain context  父规则链上下文
//   - aspects: List of AOP aspects to apply  要应用的 AOP 切面列表
//   - selfDefinition: Node definition and configuration  节点定义和配置
//
// Returns:
// 返回：
//   - *RuleNodeCtx: Initialized node context  已初始化的节点上下文
//   - error: Initialization error if any  如果有的话，初始化错误
func InitRuleNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	return initRuleNodeCtx(config, chainCtx, aspects, selfDefinition, false)
}

// InitNetResourceNodeCtx initializes a RuleNodeCtx with network resources.
// This function is used for nodes that require network connectivity and resources.
//
// InitNetResourceNodeCtx 初始化带有网络资源的 RuleNodeCtx。
// 此函数用于需要网络连接和资源的节点。
//
// Parameters:
// 参数：
//   - config: Global rule engine configuration  全局规则引擎配置
//   - chainCtx: Parent rule chain context  父规则链上下文
//   - aspects: List of AOP aspects to apply  要应用的 AOP 切面列表
//   - selfDefinition: Node definition and configuration  节点定义和配置
//
// Returns:
// 返回：
//   - *RuleNodeCtx: Initialized node context with network resources  已初始化的带网络资源的节点上下文
//   - error: Initialization error if any  如果有的话，初始化错误
func InitNetResourceNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	return initRuleNodeCtx(config, chainCtx, aspects, selfDefinition, true)
}

// initRuleNodeCtx is the core initialization function for RuleNodeCtx.
// It handles the complete node initialization process including component creation,
// configuration processing, and aspect integration.
//
// initRuleNodeCtx 是 RuleNodeCtx 的核心初始化函数。
// 它处理完整的节点初始化过程，包括组件创建、配置处理和切面集成。
//
// Parameters:
// 参数：
//   - config: Global rule engine configuration  全局规则引擎配置
//   - chainCtx: Parent rule chain context  父规则链上下文
//   - aspects: List of AOP aspects to apply  要应用的 AOP 切面列表
//   - selfDefinition: Node definition and configuration  节点定义和配置
//   - isInitNetResource: Whether to initialize network resources  是否初始化网络资源
//
// Returns:
// 返回：
//   - *RuleNodeCtx: Initialized node context  已初始化的节点上下文
//   - error: Initialization error if any  如果有的话，初始化错误
//
// Initialization Process:
// 初始化过程：
//  1. Execute before-init aspects  执行初始化前切面
//  2. Create node instance from component registry  从组件注册表创建节点实例
//  3. Process configuration variables and templates  处理配置变量和模板
//  4. Inject chain context and node definition  注入链上下文和节点定义
//  5. Initialize the node with processed configuration  使用处理过的配置初始化节点
//  6. Return wrapped node context  返回包装的节点上下文
//
// Error Handling:
// 错误处理：
//   - Aspect execution failures  切面执行失败
//   - Component creation errors  组件创建错误
//   - Configuration processing failures  配置处理失败
//   - Node initialization errors  节点初始化错误
func initRuleNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode, isInitNetResource bool) (*RuleNodeCtx, error) {
	// Retrieve aspects for the engine.
	_, nodeBeforeInitAspects, _, _, _ := aspects.GetEngineAspects()
	for _, aspect := range nodeBeforeInitAspects {
		if err := aspect.OnNodeBeforeInit(config, selfDefinition); err != nil {
			return nil, fmt.Errorf("nodeType:%s for id:%s OnNodeBeforeInit error:%s", selfDefinition.Type, selfDefinition.Id, err.Error())
		}
	}

	node, err := config.ComponentsRegistry.NewNode(selfDefinition.Type)
	if err != nil {
		return &RuleNodeCtx{
			ChainCtx:          chainCtx,
			SelfDefinition:    selfDefinition,
			config:            config,
			aspects:           aspects,
			isInitNetResource: isInitNetResource,
		}, fmt.Errorf("nodeType:%s for id:%s new error:%s", selfDefinition.Type, selfDefinition.Id, err.Error())
	} else {
		// If selfDefinition.Configuration is nil, initialize it as an empty configuration.
		if selfDefinition.Configuration == nil {
			selfDefinition.Configuration = make(types.Configuration)
		}
		// Process variables within the configuration.
		configuration, err := processVariables(config, chainCtx, selfDefinition.Configuration)
		if err != nil {
			return &RuleNodeCtx{}, fmt.Errorf("nodeType:%s for id:%s process variables error:%s", selfDefinition.Type, selfDefinition.Id, err.Error())
		}
		if isInitNetResource {
			configuration[types.NodeConfigurationKeyIsInitNetResource] = true
		}
		// Add the chain context to the configuration.
		configuration[types.NodeConfigurationKeyChainCtx] = chainCtx
		configuration[types.NodeConfigurationKeySelfDefinition] = *selfDefinition
		// Initialize the node with the processed configuration.
		if err = node.Init(config, configuration); err != nil {
			return &RuleNodeCtx{}, fmt.Errorf("nodeType:%s for id:%s init error:%s", selfDefinition.Type, selfDefinition.Id, err.Error())
		} else {
			// Return a RuleNodeCtx with the initialized node and provided context and definition.
			return &RuleNodeCtx{
				Node:              node,
				ChainCtx:          chainCtx,
				SelfDefinition:    selfDefinition,
				config:            config,
				aspects:           aspects,
				isInitNetResource: isInitNetResource,
			}, nil
		}
	}
}

// Config returns the configuration of the rule engine.
func (rn *RuleNodeCtx) Config() types.Config {
	rn.RLock()
	defer rn.RUnlock()
	return rn.config
}

// IsDebugMode returns whether the node is in debug mode.
func (rn *RuleNodeCtx) IsDebugMode() bool {
	rn.RLock()
	defer rn.RUnlock()
	return rn.SelfDefinition.DebugMode
}

// GetNodeId returns the ID of the node.
func (rn *RuleNodeCtx) GetNodeId() types.RuleNodeId {
	rn.RLock()
	defer rn.RUnlock()
	return types.RuleNodeId{Id: rn.SelfDefinition.Id, Type: types.NODE}
}

// ReloadSelf reloads the node from a byte slice definition.
func (rn *RuleNodeCtx) ReloadSelf(def []byte) error {
	rn.RLock()
	parser := rn.config.Parser
	rn.RUnlock()

	node, err := parser.DecodeRuleNode(def)
	if err != nil {
		return err
	}

	return rn.ReloadSelfFromDef(node)
}

// ReloadSelfFromDef reloads the node from a RuleNode definition.
// This method implements hot reloading for individual nodes, allowing dynamic
// updates without stopping the entire rule chain.
//
// ReloadSelfFromDef 从 RuleNode 定义重新加载节点。
// 此方法为单个节点实现热重载，允许在不停止整个规则链的情况下进行动态更新。
//
// Parameters:
// 参数：
//   - def: New node definition  新的节点定义
//
// Returns:
// 返回：
//   - error: Reload error if any  如果有的话，重载错误
func (rn *RuleNodeCtx) ReloadSelfFromDef(def types.RuleNode) error {
	// 阶段1：快速读取当前配置（最小读锁时间）
	rn.RLock()
	chainCtx := rn.ChainCtx
	config := rn.config
	isInitNetResource := rn.isInitNetResource
	rn.RUnlock()

	// 阶段2：在锁外执行耗时的新节点创建和初始化
	var newNodeCtx *RuleNodeCtx
	var err error
	if chainCtx == nil {
		newNodeCtx, err = initRuleNodeCtx(config, nil, nil, &def, isInitNetResource)
	} else {
		newNodeCtx, err = initRuleNodeCtx(config, chainCtx, chainCtx.aspects, &def, isInitNetResource)
	}

	if err != nil {
		return err
	}

	// 阶段3：快速原子替换（最小写锁时间）
	rn.Lock()
	oldNode := rn.Node                            // 保存旧节点引用，锁外销毁
	rn.Node = newNodeCtx.Node                     // 原子替换最关键的Node字段
	rn.config = newNodeCtx.config                 // 更新配置
	rn.aspects = newNodeCtx.aspects               // 更新切面
	rn.SelfDefinition = newNodeCtx.SelfDefinition // 更新节点定义
	rn.Unlock()

	// 阶段4：锁外清理旧资源（避免在锁内执行耗时的清理操作）
	if oldNode != nil {
		oldNode.Destroy()
	}

	return nil
}

// ReloadChild is not supported for RuleNodeCtx.
func (rn *RuleNodeCtx) ReloadChild(_ types.RuleNodeId, _ []byte) error {
	return errors.New("not support this func")
}

// GetNodeById is not supported for RuleNodeCtx.
func (rn *RuleNodeCtx) GetNodeById(_ types.RuleNodeId) (types.NodeCtx, bool) {
	return nil, false
}

// DSL returns the DSL representation of the node.
func (rn *RuleNodeCtx) DSL() []byte {
	rn.RLock()
	parser := rn.config.Parser
	selfDefinition := rn.SelfDefinition
	rn.RUnlock()

	result, _ := parser.EncodeRuleNode(selfDefinition)
	return result
}

// OnMsg 提供并发安全的消息处理，保护内嵌Node访问
// OnMsg provides concurrent-safe message processing with protected access to the embedded Node.
// This method ensures thread safety during message processing by using read locks to protect
// against concurrent modifications during hot reloads.
//
// OnMsg 提供并发安全的消息处理，通过使用读锁保护嵌入的 Node 访问。
// 此方法通过使用读锁防止热重载期间的并发修改，确保消息处理期间的线程安全。
//
// Parameters:
// 参数：
//   - ctx: Rule context for message processing  用于消息处理的规则上下文
//   - msg: Message to be processed  要处理的消息
func (rn *RuleNodeCtx) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 使用读锁保护Node字段的访问，与ReloadSelfFromDef的写锁互斥
	rn.RLock()
	node := rn.Node
	rn.RUnlock()

	if node != nil {
		node.OnMsg(ctx, msg)
	}
}

// Copy copies the contents of a new RuleNodeCtx into this one.
// This method is used for updating node configuration during reloads.
//
// Copy 将新 RuleNodeCtx 的内容复制到当前实例中。
// 此方法用于在重载期间更新节点配置。
//
// Parameters:
// 参数：
//   - newCtx: New node context to copy from  要复制的新节点上下文
func (rn *RuleNodeCtx) Copy(newCtx *RuleNodeCtx) {
	rn.Lock()
	defer rn.Unlock()
	rn.Node = newCtx.Node
	rn.config = newCtx.config
	rn.aspects = newCtx.aspects
	rn.SelfDefinition = newCtx.SelfDefinition
}

// processVariables replaces placeholders in the node configuration with global and chain-specific variables.
func processVariables(config types.Config, chainCtx *RuleChainCtx, configuration types.Configuration) (types.Configuration, error) {
	result := make(types.Configuration)
	globalEnv := make(map[string]string)

	if config.Properties != nil {
		globalEnv = config.Properties.Values()
	}

	var varsEnv, decryptSecrets map[string]string

	if chainCtx != nil {
		varsEnv = copyMap(chainCtx.vars)
		decryptSecrets = copyMap(chainCtx.decryptSecrets)
	}

	env := map[string]interface{}{
		types.Global: globalEnv,
		types.Vars:   varsEnv,
	}

	for key, value := range configuration {
		if strV, ok := value.(string); ok {
			result[key] = str.ExecuteTemplate(strV, env)
		} else {
			result[key] = value
		}
	}

	if varsEnv != nil {
		result[types.Vars] = varsEnv
	}
	if decryptSecrets != nil {
		result[types.Secrets] = decryptSecrets
	}

	return result, nil
}

// copyMap creates a shallow copy of a string map.
func copyMap(inputMap map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range inputMap {
		result[key] = value
	}
	return result
}

// Destroy safely destroys the embedded node
func (rn *RuleNodeCtx) Destroy() {
	rn.RLock()
	node := rn.Node
	rn.RUnlock()

	if node != nil {
		node.Destroy()
	}
}
