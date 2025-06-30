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
	"context"
	"fmt"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/aes"
	"github.com/rulego/rulego/utils/str"
)

// RelationCache caches the outgoing node relationships based on the incoming node.
// This structure is used as a key for caching node relationships to improve performance
// by avoiding repeated lookups of node routing information.
//
// RelationCache 基于传入节点缓存传出节点关系。
// 此结构用作缓存节点关系的键，通过避免重复查找节点路由信息来提高性能。
//
// Cache Key Structure:
// 缓存键结构：
//   - inNodeId: The source node from which the relationship originates
//     关系源头的源节点
//   - relationType: The type of relationship (e.g., "Success", "Failure", "True", "False")
//     关系类型（例如，"Success"、"Failure"、"True"、"False"）
//
// Usage:
// 用法：
//
//	This cache significantly improves performance in rule chains with complex
//	routing by avoiding repeated traversal of the node relationship map.
//	该缓存通过避免重复遍历节点关系映射，显著提高了具有复杂路由的规则链的性能。
type RelationCache struct {
	inNodeId     types.RuleNodeId // Identifier of the incoming node  传入节点的标识符
	relationType string           // Type of relationship with the outgoing node  与传出节点的关系类型
}

// RuleChainCtx defines an instance of a rule chain.
// It initializes all nodes and records the routing relationships between all nodes in the rule chain.
// This is the core context that manages the execution environment for a complete rule chain.
//
// RuleChainCtx 定义规则链的实例。
// 它初始化所有节点并记录规则链中所有节点之间的路由关系。
// 这是管理完整规则链执行环境的核心上下文。
//
// Core Responsibilities:
// 核心职责：
//   - Node lifecycle management  节点生命周期管理
//   - Routing relationship management  路由关系管理
//   - Configuration and variable handling  配置和变量处理
//   - Aspect-oriented programming integration  面向切面编程集成
//   - Sub-rule chain orchestration  子规则链编排
//   - Thread-safe operations  线程安全操作
//
// Architecture:
// 架构：
//
//	Each RuleChainCtx represents a complete rule chain with:
//	每个 RuleChainCtx 代表一个完整的规则链，包含：
//	- Multiple RuleNodeCtx instances (individual nodes)  多个 RuleNodeCtx 实例（单个节点）
//	- Routing matrix defining node connections  定义节点连接的路由矩阵
//	- Shared configuration and variables  共享配置和变量
//	- Root context for message processing  消息处理的根上下文
//
// Performance Features:
// 性能特性：
//   - Relationship caching for fast routing lookups  用于快速路由查找的关系缓存
//   - Efficient parent-child node tracking  高效的父子节点跟踪
//   - Lock-optimized concurrent access  锁优化的并发访问
//   - Variable preprocessing and secret decryption  变量预处理和密钥解密
type RuleChainCtx struct {
	// Id is the unique identifier of the rule chain node
	// Id 是规则链节点的唯一标识符
	Id types.RuleNodeId

	// SelfDefinition contains the complete rule chain definition including
	// metadata, nodes, connections, and configuration
	// SelfDefinition 包含完整的规则链定义，包括元数据、节点、连接和配置
	SelfDefinition *types.RuleChain

	// config contains the rule engine configuration including
	// component registry, parser, and global settings
	// config 包含规则引擎配置，包括组件注册表、解析器和全局设置
	config types.Config

	// initialized indicates whether the rule chain context has been
	// properly initialized and is ready for message processing
	// initialized 指示规则链上下文是否已正确初始化并准备好进行消息处理
	initialized bool

	// componentsRegistry provides access to available node components
	// and is used for creating new node instances during initialization
	// componentsRegistry 提供对可用节点组件的访问，用于在初始化期间创建新节点实例
	componentsRegistry types.ComponentRegistry

	// nodeIds maintains an ordered list of node identifiers for iteration
	// and access by index, preserving the original definition order
	// nodeIds 维护节点标识符的有序列表，用于迭代和按索引访问，保留原始定义顺序
	nodeIds []types.RuleNodeId

	// nodes maps node identifiers to their corresponding node contexts,
	// providing O(1) lookup time for node access operations
	// nodes 将节点标识符映射到其对应的节点上下文，为节点访问操作提供 O(1) 查找时间
	nodes map[types.RuleNodeId]types.NodeCtx

	// nodeRoutes maps each node to its outgoing relationships,
	// defining the flow of messages through the rule chain
	// nodeRoutes 将每个节点映射到其传出关系，定义消息通过规则链的流动
	nodeRoutes map[types.RuleNodeId][]types.RuleNodeRelation

	// parentNodeIds maps each node to its incoming node identifiers,
	// enabling reverse traversal and dependency analysis
	// parentNodeIds 将每个节点映射到其传入节点标识符，支持反向遍历和依赖分析
	parentNodeIds map[types.RuleNodeId][]types.RuleNodeId

	// relationCache caches outgoing node lists based on incoming node and relationship type,
	// significantly improving routing performance for frequently accessed paths
	// relationCache 基于传入节点和关系类型缓存传出节点列表，显著提高频繁访问路径的路由性能
	relationCache map[RelationCache][]types.NodeCtx

	// rootRuleContext is the root context for message processing within this rule chain,
	// providing the entry point for message flow and execution coordination
	// rootRuleContext 是此规则链内消息处理的根上下文，为消息流和执行协调提供入口点
	rootRuleContext types.RuleContext

	// ruleChainPool manages sub-rule chains and nested rule execution,
	// enabling complex rule orchestration and modular rule design
	// ruleChainPool 管理子规则链和嵌套规则执行，支持复杂的规则编排和模块化规则设计
	ruleChainPool types.RuleEnginePool

	// aspects contains the list of AOP aspects applied to this rule chain,
	// providing cross-cutting concerns like logging, validation, and metrics
	// aspects 包含应用于此规则链的 AOP 切面列表，提供如日志、验证和指标等横切关注点
	aspects types.AspectList

	// afterReloadAspects contains aspects triggered after rule chain reload,
	// enabling post-reload processing and validation
	// afterReloadAspects 包含规则链重载后触发的切面，支持重载后处理和验证
	afterReloadAspects []types.OnReloadAspect

	// destroyAspects contains aspects triggered when the rule chain is destroyed,
	// enabling proper cleanup and resource deallocation
	// destroyAspects 包含规则链销毁时触发的切面，支持正确的清理和资源释放
	destroyAspects []types.OnDestroyAspect

	// vars contains user-defined variables available throughout the rule chain,
	// supporting dynamic configuration and parameterized rule execution
	// vars 包含在整个规则链中可用的用户定义变量，支持动态配置和参数化规则执行
	vars map[string]string

	// decryptSecrets contains decrypted secret values accessible to nodes,
	// providing secure access to sensitive configuration data
	// decryptSecrets 包含节点可访问的解密秘密值，为敏感配置数据提供安全访问
	decryptSecrets map[string]string

	// isEmpty indicates whether the rule chain has no nodes,
	// used for optimization and error handling in empty chains
	// isEmpty 指示规则链是否没有节点，用于空链的优化和错误处理
	isEmpty bool

	// RWMutex provides thread-safe access to the rule chain context,
	// allowing concurrent reads while ensuring exclusive writes
	// RWMutex 为规则链上下文提供线程安全访问，允许并发读取同时确保独占写入
	sync.RWMutex
}

// InitRuleChainCtx initializes a RuleChainCtx with the given configuration, aspects, and rule chain definition.
// This function performs the complete initialization of a rule chain context, including node creation,
// relationship mapping, variable processing, and aspect integration.
//
// InitRuleChainCtx 使用给定的配置、切面和规则链定义初始化 RuleChainCtx。
// 此函数执行规则链上下文的完整初始化，包括节点创建、关系映射、变量处理和切面集成。
//
// Parameters:
// 参数：
//   - config: Rule engine configuration containing component registry and global settings
//     包含组件注册表和全局设置的规则引擎配置
//   - aspects: List of AOP aspects to be applied to the rule chain
//     要应用于规则链的 AOP 切面列表
//   - ruleChainDef: Complete rule chain definition with nodes and connections
//     包含节点和连接的完整规则链定义
//
// Returns:
// 返回：
//   - *RuleChainCtx: Fully initialized rule chain context  完全初始化的规则链上下文
//   - error: Initialization error if any  如果有的话，初始化错误
//
// Initialization Process:
// 初始化过程：
//  1. Execute before-init aspects  执行初始化前切面
//  2. Create and configure RuleChainCtx structure  创建和配置 RuleChainCtx 结构
//  3. Process variables and secrets  处理变量和密钥
//  4. Initialize all node components  初始化所有节点组件
//  5. Build node relationship mappings  构建节点关系映射
//  6. Set up sub-rule chain connections  设置子规则链连接
//  7. Create root rule context  创建根规则上下文
//  8. Handle empty rule chain cases  处理空规则链情况
//
// Error Handling:
// 错误处理：
//   - Aspect initialization failures  切面初始化失败
//   - Node component creation errors  节点组件创建错误
//   - Variable processing failures  变量处理失败
//   - Invalid rule chain definitions  无效的规则链定义
func InitRuleChainCtx(config types.Config, aspects types.AspectList, ruleChainDef *types.RuleChain) (*RuleChainCtx, error) {
	// Retrieve aspects for the engine
	chainBeforeInitAspects, _, _, afterReloadAspects, destroyAspects := aspects.GetEngineAspects()
	for _, aspect := range chainBeforeInitAspects {
		if err := aspect.OnChainBeforeInit(config, ruleChainDef); err != nil {
			return nil, err
		}
	}

	// Initialize a new RuleChainCtx with the provided configuration and aspects
	var ruleChainCtx = &RuleChainCtx{
		config:             config,
		SelfDefinition:     ruleChainDef,
		nodes:              make(map[types.RuleNodeId]types.NodeCtx),
		nodeRoutes:         make(map[types.RuleNodeId][]types.RuleNodeRelation),
		relationCache:      make(map[RelationCache][]types.NodeCtx),
		parentNodeIds:      make(map[types.RuleNodeId][]types.RuleNodeId),
		componentsRegistry: config.ComponentsRegistry,
		initialized:        true,
		aspects:            aspects,
		afterReloadAspects: afterReloadAspects,
		destroyAspects:     destroyAspects,
	}
	// Set the ID of the rule chain context if provided in the definition
	if ruleChainDef.RuleChain.ID != "" {
		ruleChainCtx.Id = types.RuleNodeId{Id: ruleChainDef.RuleChain.ID, Type: types.CHAIN}
	}
	// Process the rule chain configuration's vars and secrets
	if ruleChainDef != nil && ruleChainDef.RuleChain.Configuration != nil {
		varsConfig := ruleChainDef.RuleChain.Configuration[types.Vars]
		ruleChainCtx.vars = str.ToStringMapString(varsConfig)
		envConfig := ruleChainDef.RuleChain.Configuration[types.Secrets]
		secrets := str.ToStringMapString(envConfig)
		ruleChainCtx.decryptSecrets = decryptSecret(secrets, []byte(config.SecretKey))
	}
	nodeLen := len(ruleChainDef.Metadata.Nodes)
	ruleChainCtx.nodeIds = make([]types.RuleNodeId, nodeLen)
	// Load all node information
	for index, item := range ruleChainDef.Metadata.Nodes {
		if item.Id == "" {
			item.Id = fmt.Sprintf(defaultNodeIdPrefix+"%d", index)
		}
		ruleNodeId := types.RuleNodeId{Id: item.Id, Type: types.NODE}
		ruleChainCtx.nodeIds[index] = ruleNodeId
		ruleNodeCtx, err := InitRuleNodeCtx(config, ruleChainCtx, aspects, item)
		if err != nil {
			return nil, err
		}
		ruleChainCtx.nodes[ruleNodeId] = ruleNodeCtx
	}
	// Load node relationship information
	for _, item := range ruleChainDef.Metadata.Connections {
		inNodeId := types.RuleNodeId{Id: item.FromId, Type: types.NODE}
		outNodeId := types.RuleNodeId{Id: item.ToId, Type: types.NODE}
		ruleNodeRelation := types.RuleNodeRelation{
			InId:         inNodeId,
			OutId:        outNodeId,
			RelationType: item.Type,
		}
		nodeRelations, ok := ruleChainCtx.nodeRoutes[inNodeId]

		if ok {
			nodeRelations = append(nodeRelations, ruleNodeRelation)
		} else {
			nodeRelations = []types.RuleNodeRelation{ruleNodeRelation}
		}
		ruleChainCtx.nodeRoutes[inNodeId] = nodeRelations

		// Record parent nodes
		parentNodeIds, ok := ruleChainCtx.parentNodeIds[outNodeId]
		if ok {
			parentNodeIds = append(parentNodeIds, inNodeId)
		} else {
			parentNodeIds = []types.RuleNodeId{inNodeId}
		}
		ruleChainCtx.parentNodeIds[outNodeId] = parentNodeIds
	}
	// Load sub-rule chains
	for _, item := range ruleChainDef.Metadata.RuleChainConnections {
		inNodeId := types.RuleNodeId{Id: item.FromId, Type: types.NODE}
		outNodeId := types.RuleNodeId{Id: item.ToId, Type: types.CHAIN}
		ruleChainRelation := types.RuleNodeRelation{
			InId:         inNodeId,
			OutId:        outNodeId,
			RelationType: item.Type,
		}

		nodeRelations, ok := ruleChainCtx.nodeRoutes[inNodeId]
		if ok {
			nodeRelations = append(nodeRelations, ruleChainRelation)
		} else {
			nodeRelations = []types.RuleNodeRelation{ruleChainRelation}
		}
		ruleChainCtx.nodeRoutes[inNodeId] = nodeRelations

		// Record parent nodes
		parentNodeIds, ok := ruleChainCtx.parentNodeIds[outNodeId]
		if ok {
			parentNodeIds = append(parentNodeIds, inNodeId)
		} else {
			parentNodeIds = []types.RuleNodeId{inNodeId}
		}
		ruleChainCtx.parentNodeIds[outNodeId] = parentNodeIds
	}
	// Initialize the root rule context
	if firstNode, ok := ruleChainCtx.GetFirstNode(); ok {
		ruleChainCtx.rootRuleContext = NewRuleContext(context.TODO(), ruleChainCtx.config, ruleChainCtx, nil,
			firstNode, config.Pool, nil, nil)
	} else {
		// If there are no nodes, initialize an empty node context
		ruleNodeCtx, _ := InitRuleNodeCtx(config, ruleChainCtx, aspects, &types.RuleNode{})
		ruleChainCtx.rootRuleContext = NewRuleContext(context.TODO(), ruleChainCtx.config, ruleChainCtx, nil,
			ruleNodeCtx, config.Pool, nil, nil)
		ruleChainCtx.isEmpty = true
	}

	return ruleChainCtx, nil
}

// Config returns the configuration of the rule chain context
func (rc *RuleChainCtx) Config() types.Config {
	rc.RLock()
	defer rc.RUnlock()
	return rc.config
}

// GetNodeById retrieves a node context by its ID
// This method supports both regular nodes and sub-rule chains,
// providing unified access to all types of nodes in the rule chain.
//
// GetNodeById 通过 ID 检索节点上下文
// 此方法支持常规节点和子规则链，为规则链中所有类型的节点提供统一访问。
//
// Parameters:
// 参数：
//   - id: Node identifier with type information  带有类型信息的节点标识符
//
// Returns:
// 返回：
//   - types.NodeCtx: Node context if found  如果找到，节点上下文
//   - bool: true if node exists, false otherwise  如果节点存在则为 true，否则为 false
//
// Behavior:
// 行为：
//   - For CHAIN type: searches in the rule engine pool  对于 CHAIN 类型：在规则引擎池中搜索
//   - For NODE type: searches in the local nodes map  对于 NODE 类型：在本地节点映射中搜索
//   - Thread-safe: uses read lock for concurrent access  线程安全：使用读锁进行并发访问
func (rc *RuleChainCtx) GetNodeById(id types.RuleNodeId) (types.NodeCtx, bool) {
	rc.RLock()
	defer rc.RUnlock()
	if id.Type == types.CHAIN {
		// For sub-rule chains, search through the rule chain pool
		if subRuleEngine, ok := rc.GetRuleEnginePool().Get(id.Id); ok && subRuleEngine.RootRuleChainCtx() != nil {
			return subRuleEngine.RootRuleChainCtx(), true
		} else {
			return nil, false
		}
	} else {
		ruleNodeCtx, ok := rc.nodes[id]
		return ruleNodeCtx, ok
	}
}

// GetNodeByIndex retrieves a node context by its index
func (rc *RuleChainCtx) GetNodeByIndex(index int) (types.NodeCtx, bool) {
	rc.RLock()
	if index >= len(rc.nodeIds) {
		rc.RUnlock()
		return &RuleNodeCtx{}, false
	}
	nodeId := rc.nodeIds[index]
	rc.RUnlock()
	return rc.GetNodeById(nodeId)
}

// GetFirstNode retrieves the first node, where the message starts flowing. By default, it's the node with index 0
func (rc *RuleChainCtx) GetFirstNode() (types.NodeCtx, bool) {
	rc.RLock()
	firstNodeIndex := rc.SelfDefinition.Metadata.FirstNodeIndex
	rc.RUnlock()
	return rc.GetNodeByIndex(firstNodeIndex)
}

// GetNodeRoutes retrieves the routes for a given node ID
func (rc *RuleChainCtx) GetNodeRoutes(id types.RuleNodeId) ([]types.RuleNodeRelation, bool) {
	rc.RLock()
	defer rc.RUnlock()
	relations, ok := rc.nodeRoutes[id]
	return relations, ok
}

// GetParentNodeIds retrieves the parent node IDs for a given node ID
func (rc *RuleChainCtx) GetParentNodeIds(id types.RuleNodeId) ([]types.RuleNodeId, bool) {
	rc.RLock()
	defer rc.RUnlock()
	nodeIds, ok := rc.parentNodeIds[id]
	return nodeIds, ok
}

// GetNextNodes retrieves the child nodes of the current node with the specified relationship
// This method implements a two-level caching strategy: first checking the relationCache,
// then building the cache if needed, providing high-performance routing for message flow.
//
// GetNextNodes 检索具有指定关系的当前节点的子节点
// 此方法实现两级缓存策略：首先检查 relationCache，然后在需要时构建缓存，
// 为消息流提供高性能路由。
//
// Parameters:
// 参数：
//   - id: Source node identifier  源节点标识符
//   - relationType: Type of relationship to follow (e.g., "Success", "Failure", "True", "False")
//     要遵循的关系类型（例如，"Success"、"Failure"、"True"、"False"）
//
// Returns:
// 返回：
//   - []types.NodeCtx: List of child node contexts  子节点上下文列表
//   - bool: true if any child nodes found, false otherwise  如果找到任何子节点则为 true，否则为 false
//
// Performance Features:
// 性能特性：
//   - Relationship caching: O(1) lookup time for cached relationships  关系缓存：缓存关系的 O(1) 查找时间
//   - Lazy cache building: cache is built only when needed  延迟缓存构建：仅在需要时构建缓存
//   - Thread-safe: proper locking for concurrent access  线程安全：适当的锁定以进行并发访问
//
// Cache Strategy:
// 缓存策略：
//  1. Check relationCache for existing entry  检查 relationCache 中的现有条目
//  2. If not found, traverse nodeRoutes to find matches  如果未找到，遍历 nodeRoutes 查找匹配项
//  3. Build node context list from matching relationships  从匹配的关系构建节点上下文列表
//  4. Store result in relationCache for future use  将结果存储在 relationCache 中以供将来使用
func (rc *RuleChainCtx) GetNextNodes(id types.RuleNodeId, relationType string) ([]types.NodeCtx, bool) {
	var nodeCtxList []types.NodeCtx
	cacheKey := RelationCache{inNodeId: id, relationType: relationType}
	rc.RLock()
	// Get from cache
	nodeCtxList, ok := rc.relationCache[cacheKey]
	rc.RUnlock()
	if ok {
		return nodeCtxList, nodeCtxList != nil
	}

	// Get from the Routes
	relations, ok := rc.GetNodeRoutes(id)
	hasNextComponents := false
	if ok {
		for _, item := range relations {
			if item.RelationType == relationType {
				if nodeCtx, nodeCtxOk := rc.GetNodeById(item.OutId); nodeCtxOk {
					nodeCtxList = append(nodeCtxList, nodeCtx)
					hasNextComponents = true
				}
			}
		}
	}
	rc.Lock()
	// Add to the cache
	rc.relationCache[cacheKey] = nodeCtxList
	rc.Unlock()
	return nodeCtxList, hasNextComponents
}

// Type returns the component type
func (rc *RuleChainCtx) Type() string {
	return "ruleChain"
}

// New creates a new instance (not supported for RuleChainCtx)
func (rc *RuleChainCtx) New() types.Node {
	panic("not support this method")
}

// Init initializes the rule chain context
func (rc *RuleChainCtx) Init(_ types.Config, configuration types.Configuration) error {
	if rootRuleChainDef, ok := configuration["selfDefinition"]; ok {
		if v, ok := rootRuleChainDef.(*types.RuleChain); ok {
			if ruleChainCtx, err := InitRuleChainCtx(rc.config, rc.aspects, v); err == nil {
				rc.Copy(ruleChainCtx)
			} else {
				return err
			}
		}
	}
	return nil
}

// OnMsg processes incoming messages
func (rc *RuleChainCtx) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	rc.RLock()
	id := rc.Id.Id
	rc.RUnlock()
	ctx.TellFlow(context.Background(), id, msg, nil, nil)
}

// Destroy cleans up resources and executes destroy aspects
func (rc *RuleChainCtx) Destroy() {
	// Get copies of what we need to destroy without holding locks for too long
	rc.RLock()
	nodesToDestroy := make([]types.NodeCtx, 0, len(rc.nodes))
	for _, v := range rc.nodes {
		nodesToDestroy = append(nodesToDestroy, v)
	}
	destroyAspects := make([]types.OnDestroyAspect, len(rc.destroyAspects))
	copy(destroyAspects, rc.destroyAspects)
	// Pre-fetch the node ID to avoid calling GetNodeId() in OnDestroy which needs a lock
	nodeId := rc.getNodeIdUnsafe()
	rc.RUnlock()

	// Destroy nodes without holding any locks
	for _, v := range nodesToDestroy {
		v.Destroy()
	}

	// Create a wrapper to avoid GetNodeId() calls in OnDestroy
	wrapper := &nodeCtxWrapper{
		nodeId:   nodeId,
		original: rc,
	}

	// Execute destroy aspects without holding locks
	// Note: We avoid calling methods that need locks within OnDestroy by pre-fetching data
	for _, aop := range destroyAspects {
		aop.OnDestroy(wrapper)
	}
}

// IsDebugMode checks if debug mode is enabled
func (rc *RuleChainCtx) IsDebugMode() bool {
	rc.RLock()
	defer rc.RUnlock()
	return rc.SelfDefinition.RuleChain.DebugMode
}

// GetNodeId returns the node ID
func (rc *RuleChainCtx) GetNodeId() types.RuleNodeId {
	rc.RLock()
	defer rc.RUnlock()
	return rc.getNodeIdUnsafe()
}

// getNodeIdUnsafe returns the node ID without locking (for internal use)
func (rc *RuleChainCtx) getNodeIdUnsafe() types.RuleNodeId {
	return rc.Id
}

// ReloadSelf reloads the rule chain from a byte slice definition
func (rc *RuleChainCtx) ReloadSelf(def []byte) error {
	if rootRuleChainDef, err := rc.config.Parser.DecodeRuleChain(def); err == nil {
		return rc.ReloadSelfFromDef(rootRuleChainDef)
	} else {
		return err
	}
}

// ReloadSelfFromDef reloads the rule chain from a RuleChain definition
// This method performs hot reloading of rule chain configuration, supporting
// dynamic updates without stopping the rule engine.
//
// ReloadSelfFromDef 从 RuleChain 定义重新加载规则链
// 此方法执行规则链配置的热重载，支持在不停止规则引擎的情况下进行动态更新。
//
// Parameters:
// 参数：
//   - def: New rule chain definition  新的规则链定义
//
// Returns:
// 返回：
//   - error: Reload error if any  如果有的话，重载错误
//
// Hot Reload Process:
// 热重载过程：
//  1. Check if rule chain is disabled  检查规则链是否被禁用
//  2. Initialize new rule chain context  初始化新的规则链上下文
//  3. Safely destroy old nodes without holding locks  在不持有锁的情况下安全销毁旧节点
//  4. Execute destroy aspects for cleanup  执行销毁切面进行清理
//  5. Atomically replace old context with new one  原子性地用新上下文替换旧上下文
//  6. Execute reload aspects for post-reload processing  执行重载切面进行重载后处理
//
// Error Handling:
// 错误处理：
//   - Disabled rule chain detection  禁用规则链检测
//   - Context initialization failures  上下文初始化失败
//   - Aspect execution errors  切面执行错误
func (rc *RuleChainCtx) ReloadSelfFromDef(def types.RuleChain) error {
	if def.RuleChain.Disabled {
		return ErrDisabled
	}
	if ctx, err := InitRuleChainCtx(rc.config, rc.aspects, &def); err == nil {
		// First, execute destroy operations without holding locks to avoid deadlock
		rc.RLock()
		oldNodes := make(map[types.RuleNodeId]types.NodeCtx)
		for k, v := range rc.nodes {
			oldNodes[k] = v
		}
		destroyAspects := make([]types.OnDestroyAspect, len(rc.destroyAspects))
		copy(destroyAspects, rc.destroyAspects)
		// Pre-fetch the node ID to avoid deadlock in OnDestroy
		nodeId := rc.getNodeIdUnsafe()
		rc.RUnlock()

		// Destroy old nodes without holding any locks
		for _, v := range oldNodes {
			v.Destroy()
		}

		// Create a wrapper to avoid GetNodeId() calls in OnDestroy
		wrapper := &nodeCtxWrapper{
			nodeId:   nodeId,
			original: rc,
		}

		// Execute destroy aspects without holding locks
		for _, aop := range destroyAspects {
			aop.OnDestroy(wrapper)
		}

		// Now lock and copy the new context
		rc.Lock()
		rc.copyUnsafe(ctx)
		rc.Unlock()

		// Execute reload aspects
		for _, aop := range rc.afterReloadAspects {
			if err := aop.OnReload(rc, rc); err != nil {
				return err
			}
		}
		return nil
	} else {
		return err
	}
}

// copyUnsafe copies the content from another RuleChainCtx without locking
// This method should only be called when the caller already holds the lock
func (rc *RuleChainCtx) copyUnsafe(newCtx *RuleChainCtx) {
	rc.Id = newCtx.Id
	rc.config = newCtx.config
	rc.initialized = newCtx.initialized
	rc.componentsRegistry = newCtx.componentsRegistry
	rc.SelfDefinition = newCtx.SelfDefinition
	rc.nodeIds = newCtx.nodeIds
	rc.nodes = newCtx.nodes
	rc.nodeRoutes = newCtx.nodeRoutes
	rc.rootRuleContext = newCtx.rootRuleContext
	rc.aspects = newCtx.aspects
	rc.afterReloadAspects = newCtx.afterReloadAspects
	rc.destroyAspects = newCtx.destroyAspects
	rc.vars = newCtx.vars
	rc.decryptSecrets = newCtx.decryptSecrets
	// Clear cache
	rc.relationCache = make(map[RelationCache][]types.NodeCtx)
}

// ReloadChild reloads a child node
func (rc *RuleChainCtx) ReloadChild(ruleNodeId types.RuleNodeId, def []byte) error {
	if node, ok := rc.GetNodeById(ruleNodeId); ok {
		// Update child node
		err := node.ReloadSelf(def)
		// Execute reload aspects
		for _, aop := range rc.afterReloadAspects {
			if err := aop.OnReload(rc, node); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}

// DSL returns the rule chain definition as a byte slice
func (rc *RuleChainCtx) DSL() []byte {
	rc.RLock()
	defer rc.RUnlock()
	v, _ := rc.config.Parser.EncodeRuleChain(rc.SelfDefinition)
	return v
}

// Definition returns the rule chain definition
func (rc *RuleChainCtx) Definition() *types.RuleChain {
	rc.RLock()
	defer rc.RUnlock()
	return rc.SelfDefinition
}

// Copy copies the content from another RuleChainCtx
func (rc *RuleChainCtx) Copy(newCtx *RuleChainCtx) {
	rc.Lock()
	defer rc.Unlock()
	rc.Id = newCtx.Id
	rc.config = newCtx.config
	rc.initialized = newCtx.initialized
	rc.componentsRegistry = newCtx.componentsRegistry
	rc.SelfDefinition = newCtx.SelfDefinition
	rc.nodeIds = newCtx.nodeIds
	rc.nodes = newCtx.nodes
	rc.nodeRoutes = newCtx.nodeRoutes
	rc.rootRuleContext = newCtx.rootRuleContext
	rc.aspects = newCtx.aspects
	rc.afterReloadAspects = newCtx.afterReloadAspects
	rc.destroyAspects = newCtx.destroyAspects
	rc.vars = newCtx.vars
	rc.decryptSecrets = newCtx.decryptSecrets
	// Clear cache
	rc.relationCache = make(map[RelationCache][]types.NodeCtx)
}

// SetRuleEnginePool sets the sub-rule chain pool
func (rc *RuleChainCtx) SetRuleEnginePool(ruleChainPool types.RuleEnginePool) {
	rc.ruleChainPool = ruleChainPool
}

// GetRuleEnginePool retrieves the sub-rule chain pool
func (rc *RuleChainCtx) GetRuleEnginePool() types.RuleEnginePool {
	if rc.ruleChainPool == nil {
		return DefaultPool
	} else {
		return rc.ruleChainPool
	}
}

// SetAspects sets the aspects for the rule chain
func (rc *RuleChainCtx) SetAspects(aspects types.AspectList) {
	rc.Lock()
	defer rc.Unlock()
	rc.aspects = aspects
	_, _, _, afterReloadAspects, destroyAspects := aspects.GetEngineAspects()
	rc.afterReloadAspects = afterReloadAspects
	rc.destroyAspects = destroyAspects
}

// GetAspects retrieves the aspects of the rule chain
func (rc *RuleChainCtx) GetAspects() types.AspectList {
	rc.RLock()
	defer rc.RUnlock()
	return rc.aspects
}

// decryptSecret decrypts the secrets in the input map using the provided secret key
func decryptSecret(inputMap map[string]string, secretKey []byte) map[string]string {
	result := make(map[string]string)
	for key, value := range inputMap {
		if plaintext, err := aes.Decrypt(value, secretKey); err == nil {
			result[key] = plaintext
		} else {
			result[key] = value
		}
	}
	return result
}

// nodeCtxWrapper wraps RuleChainCtx to provide a cached node ID, avoiding lock calls in OnDestroy
type nodeCtxWrapper struct {
	nodeId   types.RuleNodeId
	original *RuleChainCtx
}

func (w *nodeCtxWrapper) GetNodeId() types.RuleNodeId {
	return w.nodeId // Return cached value without locking
}

// Delegate all other methods to the original context
func (w *nodeCtxWrapper) Config() types.Config        { return w.original.Config() }
func (w *nodeCtxWrapper) IsDebugMode() bool           { return w.original.IsDebugMode() }
func (w *nodeCtxWrapper) ReloadSelf(def []byte) error { return w.original.ReloadSelf(def) }
func (w *nodeCtxWrapper) ReloadSelfFromDef(def types.RuleChain) error {
	return w.original.ReloadSelfFromDef(def)
}
func (w *nodeCtxWrapper) ReloadChild(ruleNodeId types.RuleNodeId, def []byte) error {
	return w.original.ReloadChild(ruleNodeId, def)
}
func (w *nodeCtxWrapper) GetNodeById(id types.RuleNodeId) (types.NodeCtx, bool) {
	return w.original.GetNodeById(id)
}
func (w *nodeCtxWrapper) DSL() []byte     { return w.original.DSL() }
func (w *nodeCtxWrapper) Type() string    { return w.original.Type() }
func (w *nodeCtxWrapper) New() types.Node { return w.original.New() }
func (w *nodeCtxWrapper) Init(config types.Config, configuration types.Configuration) error {
	return w.original.Init(config, configuration)
}
func (w *nodeCtxWrapper) OnMsg(ctx types.RuleContext, msg types.RuleMsg) { w.original.OnMsg(ctx, msg) }
func (w *nodeCtxWrapper) Destroy()                                       { w.original.Destroy() }
