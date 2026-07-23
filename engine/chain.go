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
	"github.com/rulego/rulego/utils/lca"
	"github.com/rulego/rulego/utils/str"
)

// RelationCache caches the outgoing node relationships based on the incoming node.
// This structure is used as a key for caching node relationships to improve performance
// by avoiding repeated lookups of node routing information.
//
// RelationCache is based on the relationship between the incoming node cache and the outgoing node.
// This structure serves as a key for caching node relationships, improving performance by avoiding repeated searches for node routing information.
//
// Cache Key Structure:
// Cache key structure:
//   - inNodeId: The source node from which the relationship originates
//     Source nodes of the relationship source
//   - relationType: The type of relationship (e.g., "Success", "Failure", "True", "False")
//     Relationship types (e.g., "Success", "Failure", "True", "False")
//
// Usage:
// Usage:
//
//	This cache significantly improves performance in rule chains with complex
//	routing by avoiding repeated traversal of the node relationship map.
//	This cache significantly improves the performance of rule chains with complex routing by avoiding repeated traversal of node relationship mappings.
type RelationCache struct {
	inNodeId     types.RuleNodeId // Identifier of the incoming node
	relationType string           // Type of relationship with the outgoing node
}

// RuleChainCtx defines an instance of a rule chain.
// It initializes all nodes and records the routing relationships between all nodes in the rule chain.
// This is the core context that manages the execution environment for a complete rule chain.
//
// RuleChainCtx defines an instance of the rule chain.
// It initializes all nodes and records routing relationships among all nodes in the rule chain.
// This is the core context for managing the complete rule chain execution environment.
//
// Core Responsibilities:
// Core Responsibilities:
//   - Node lifecycle management
//   - Routing relationship management
//   - Configuration and variable handling
//   - Aspect-oriented programming integration
//   - Sub-rule chain orchestration
//   - Thread-safe operations
//
// Architecture:
// Structure:
//
//	Each RuleChainCtx represents a complete rule chain with:
//	Each RuleChainCtx represents a complete rule chain, including:
//	- Multiple RuleNodeCtx instances (individual nodes)
//	- Routing matrix defining node connections
//	- Shared configuration and variables
//	- Root context for message processing
//
// Performance Features:
// Performance Features:
//   - Relationship caching for fast routing lookups
//   - Efficient parent-child node tracking
//   - Lock-optimized concurrent access
//   - Variable preprocessing and secret decryption
type RuleChainCtx struct {
	// Id is the unique identifier of the rule chain node
	// The Id is the unique identifier of the rule chain node
	Id types.RuleNodeId

	// SelfDefinition contains the complete rule chain definition including
	// metadata, nodes, connections, and configuration
	// SelfDefinition contains a complete rule chain definition, including metadata, nodes, connections, and configurations
	SelfDefinition *types.RuleChain

	// config contains the rule engine configuration including
	// component registry, parser, and global settings
	// config contains the rule engine configuration, including the component registry, parser, and global settings
	config types.Config

	// initialized indicates whether the rule chain context has been
	// properly initialized and is ready for message processing
	// initialized indicates whether the rule chain context has been properly initialized and ready for message processing
	initialized bool

	// componentsRegistry provides access to available node components
	// and is used for creating new node instances during initialization
	// componentsRegistry provides access to available node components for creating new node instances during initialization
	componentsRegistry types.ComponentRegistry

	// nodeIds maintains an ordered list of node identifiers for iteration
	// and access by index, preserving the original definition order
	// nodeIds maintains an ordered list of node identifiers for iteration and indexed access, preserving the original defined order
	nodeIds []types.RuleNodeId

	// nodes maps node identifiers to their corresponding node contexts,
	// providing O(1) lookup time for node access operations
	// nodes map node identifiers to their corresponding node context, providing O(1) lookup time for node access operations
	nodes map[types.RuleNodeId]types.NodeCtx

	// nodeRoutes maps each node to its outgoing relationships,
	// defining the flow of messages through the rule chain
	// nodeRoutes maps each node to its outgoing relationship and defines the flow of messages through the rule chain
	nodeRoutes map[types.RuleNodeId][]types.RuleNodeRelation

	// parentNodeIds maps each node to its incoming node identifiers,
	// enabling reverse traversal and dependency analysis
	// parentNodeIds maps each node to its incoming node identifier, supporting reverse traversal and dependency analysis
	parentNodeIds map[types.RuleNodeId][]types.RuleNodeId

	// relationCache caches outgoing node lists based on incoming node and relationship type,
	// significantly improving routing performance for frequently accessed paths
	// relationCache significantly improves routing performance for frequently accessed paths based on incoming nodes and relational type caches
	relationCache map[RelationCache][]types.NodeCtx

	// lcaCalculator provides LCA calculation functionality
	// lcaCalculator provides LCA calculation functions
	lcaCalculator *lca.LCACalculator

	// rootRuleContext is the root context for message processing within this rule chain,
	// providing the entry point for message flow and execution coordination
	// rootRuleContext is the root context for message processing within the chain of this rule, providing an entry point for message flow and execution coordination
	rootRuleContext types.RuleContext

	// ruleChainPool manages sub-rule chains and nested rule execution,
	// enabling complex rule orchestration and modular rule design
	// ruleChainPool manages sub-rule chains and nested rule execution, supporting complex rule orchestration and modular rule design
	ruleChainPool types.RuleEnginePool

	// aspects contains the list of AOP aspects applied to this rule chain,
	// providing cross-cutting concerns like logging, validation, and metrics
	// aspects contains the AOP aspects applied to this rule chain, providing cross-cutting concerns such as logging, validation, and metrics
	aspects types.AspectList

	// afterReloadAspects contains aspects triggered after rule chain reload,
	// enabling post-reload processing and validation
	// afterReloadAspects contains aspects triggered after a rule chain reload, supporting post-reload processing and validation
	afterReloadAspects []types.OnReloadAspect

	// destroyAspects contains aspects triggered when the rule chain is destroyed,
	// enabling proper cleanup and resource deallocation
	// destroyAspects contains aspects triggered when the rule chain is destroyed, supporting proper cleanup and resource release
	destroyAspects []types.OnDestroyAspect

	// vars contains user-defined variables available throughout the rule chain,
	// supporting dynamic configuration and parameterized rule execution
	// VARS contains user-defined variables available throughout the rule chain, supporting dynamic configuration and parameterized rule execution
	vars map[string]string

	// decryptSecrets contains decrypted secret values accessible to nodes,
	// providing secure access to sensitive configuration data
	// decryptSecrets contains decryption secret values accessible to nodes, providing secure access to sensitive configuration data
	decryptSecrets map[string]string

	// isEmpty indicates whether the rule chain has no nodes,
	// used for optimization and error handling in empty chains
	// isEmpty indicates whether the rule chain has no nodes, used for optimizing and handling errors in the empty chain
	isEmpty bool

	// hasEndNode indicates whether the rule chain has configured end nodes,
	// cached during initialization to avoid repeated traversal for performance
	// hasEndNode indicates whether the rule chain has configured termination nodes, caching at initialization to avoid repeated traversals and improve performance
	hasEndNode bool

	// referencedNodes contains the list of nodes that are referenced by other nodes
	// referencedNodes contains a list of nodes referenced by other nodes
	referencedNodes []string

	// nodeDependencies stores the dependency mapping for each node
	// nodeDependencies stores the dependency mapping for each node
	nodeDependencies map[string][]string

	// resources: Resource directory on the current chain (ref:// for same-chain parsing), lazy initialization. Pointers are replaced as a whole during reload.
	resources *resourceRegistry

	// RWMutex provides thread-safe access to the rule chain context,
	// allowing concurrent reads while ensuring exclusive writes
	// RWMutex provides thread-safe access to the Rule Chain context, allowing concurrent reads while ensuring exclusive writes
	sync.RWMutex
}

// InitRuleChainCtx initializes a RuleChainCtx with the given configuration, aspects, and rule chain definition.
// This function performs the complete initialization of a rule chain context, including node creation,
// relationship mapping, variable processing, and aspect integration.
//
// InitRuleChainCtx initializes RuleChainCtx using the given configuration, aspect, and rule chain definition.
// This function performs complete initialization of the rule chain context, including node creation, relational mapping, variable handling, and facet integration.
//
// Parameters:
// Parameters:
//   - config: Rule engine configuration containing component registry and global settings
//     Includes a component registry and a global rule engine configuration
//   - aspects: List of AOP aspects to be applied to the rule chain
//     The list of AOP aspects to apply to the rule chain
//   - ruleChainDef: Complete rule chain definition with nodes and connections
//     A complete rule chain definition containing nodes and connections
//
// Returns:
// Returns:
//   - *RuleChainCtx: Fully initialized rule chain context
//   - error: Initialization error if any
//
// Initialization Process:
// Initialization process:
//  1. Execute before-init aspects
//  2. Create and configure RuleChainCtx structure
//  3. Process variables and secrets
//  4. Initialize all node components
//  5. Build node relationship mappings
//  6. Set up sub-rule chain connections
//  7. Create root rule context
//  8. Handle empty rule chain cases
//
// Error Handling:
// Error handling:
//   - Aspect initialization failures
//   - Node component creation errors
//   - Variable processing failures
//   - Invalid rule chain definitions
func InitRuleChainCtx(config types.Config, aspects types.AspectList, ruleChainDef *types.RuleChain, ruleChainPool types.RuleEnginePool) (*RuleChainCtx, error) {
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
		ruleChainPool:      ruleChainPool,
		referencedNodes:    make([]string, 0),
	}
	// Initialize LCA calculator
	ruleChainCtx.lcaCalculator = lca.NewLCACalculator(ruleChainCtx)
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

	// Check if there are any end nodes and cache the result
	// Check if there are termination nodes and cache results
	for _, nodeCtx := range ruleChainCtx.nodes {
		if nodeCtx.Type() == types.NodeTypeEnd {
			ruleChainCtx.hasEndNode = true
			break
		}
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
		ruleChainCtx.rootRuleContext = NewRuleContext(context.Background(), ruleChainCtx.config, ruleChainCtx, nil,
			firstNode, config.Pool, nil, ruleChainPool)
	} else {
		// If there are no nodes, initialize an empty node context
		ruleNodeCtx, _ := InitRuleNodeCtx(config, ruleChainCtx, aspects, &types.RuleNode{})
		ruleChainCtx.rootRuleContext = NewRuleContext(context.Background(), ruleChainCtx.config, ruleChainCtx, nil,
			ruleNodeCtx, config.Pool, nil, ruleChainPool)
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

// GetLCA finds the lowest common ancestor of a node's parent nodes using optimized algorithm
// GetLCA uses an optimization algorithm to find the lowest common ancestor of all parent nodes
func (rc *RuleChainCtx) GetLCA(id types.RuleNodeId) (types.RuleNodeId, bool) {
	return rc.lcaCalculator.GetLCA(id)
}

// GetLCAOfNodes finds the lowest common ancestor of multiple nodes.
func (rc *RuleChainCtx) GetLCAOfNodes(nodeIds []types.RuleNodeId) (types.RuleNodeId, bool) {
	return rc.lcaCalculator.GetLCAOfNodes(nodeIds)
}

// GetNextNodes retrieves the child nodes of the current node with the specified relationship
// This method implements a two-level caching strategy: first checking the relationCache,
// then building the cache if needed, providing high-performance routing for message flow.
//
// GetNextNodes retrieves the child node of the current node with the specified relationship
// This method implements a two-level caching strategy: first check the relationCache, then build the cache when needed,
// Providing high-performance routing for message streams.
//
// Parameters:
// Parameters:
//   - id: Source node identifier
//   - relationType: Type of relationship to follow (e.g., "Success", "Failure", "True", "False")
//     The types of relationships to follow (e.g., "Success"," "Failure"," "True"," "False")
//
// Returns:
// Returns:
//   - []types.NodeCtx: List of child node contexts
//   - bool: true if any child nodes found, false otherwise
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
			if ruleChainCtx, err := InitRuleChainCtx(rc.config, rc.aspects, v, nil); err == nil {
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
	ctx.TellFlow(id, msg)
}

// Destroy cleans up resources and executes destroy aspects
func (rc *RuleChainCtx) Destroy() {
	defer func() {
		if r := recover(); r != nil {
			if rc.config.Logger != nil {
				rc.config.Logger.Printf("RuleChainCtx.Destroy() panic recovered: %v", r)
			}
		}
	}()

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
	config := rc.config
	rc.RUnlock()

	// Destroy nodes without holding any locks
	for _, v := range nodesToDestroy {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if config.Logger != nil {
						config.Logger.Printf("Node.Destroy() panic recovered: %v", r)
					}
				}
			}()
			v.Destroy()
		}()
	}

	// Create a wrapper to avoid GetNodeId() calls in OnDestroy
	wrapper := &nodeCtxWrapper{
		nodeId:   nodeId,
		original: rc,
	}

	// Execute destroy aspects without holding locks
	// Note: We avoid calling methods that need locks within OnDestroy by pre-fetching data
	for _, aop := range destroyAspects {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if config.Logger != nil {
						config.Logger.Printf("OnDestroy aspect panic recovered: %v", r)
					}
				}
			}()
			aop.OnDestroy(wrapper)
		}()
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
// ReloadSelfFromDef Reloads the rule chain from the RuleChain definition
// This method performs hot reload of rule chain configuration and supports dynamic updates without stopping the rule engine.
//
// Parameters:
// Parameters:
//   - def: New rule chain definition
//
// Returns:
// Returns:
//   - error: Reload error if any
//
// Hot Reload Process:
// Thermal Heavy-Loading Process:
//  1. Check if rule chain is disabled
//  2. Initialize new rule chain context
//  3. Safely destroy old nodes without holding locks
//  4. Execute destroy aspects for cleanup
//  5. Atomically replace old context with new one
//  6. Execute reload aspects for post-reload processing
//
// Error Handling:
// Error handling:
//   - Disabled rule chain detection
//   - Context initialization failures
//   - Aspect execution errors
func (rc *RuleChainCtx) ReloadSelfFromDef(def types.RuleChain) error {
	defer func() {
		if r := recover(); r != nil {
			if rc.config.Logger != nil {
				rc.config.Logger.Printf("ReloadSelfFromDef panic recovered: %v", r)
			}
		}
	}()

	if def.RuleChain.Disabled {
		return types.ErrEngineDisabled
	}
	if ctx, err := InitRuleChainCtx(rc.config, rc.aspects, &def, rc.ruleChainPool); err == nil {
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
		config := rc.config
		rc.RUnlock()

		// Destroy old nodes without holding any locks
		for _, v := range oldNodes {
			func() {
				defer func() {
					if r := recover(); r != nil {
						if config.Logger != nil {
							config.Logger.Printf("Node destroy in reload panic recovered: %v", r)
						}
					}
				}()
				v.Destroy()
			}()
		}

		// Create a wrapper to avoid GetNodeId() calls in OnDestroy
		wrapper := &nodeCtxWrapper{
			nodeId:   nodeId,
			original: rc,
		}

		// Execute destroy aspects without holding locks
		for _, aop := range destroyAspects {
			func() {
				defer func() {
					if r := recover(); r != nil {
						if config.Logger != nil {
							config.Logger.Printf("OnDestroy aspect in reload panic recovered: %v", r)
						}
					}
				}()
				aop.OnDestroy(wrapper)
			}()
		}

		// Now lock and copy the new context
		rc.Lock()
		rc.copyUnsafe(ctx)
		rc.Unlock()

		// Execute reload aspects
		for _, aop := range rc.afterReloadAspects {
			func() {
				defer func() {
					if r := recover(); r != nil {
						if config.Logger != nil {
							config.Logger.Printf("OnReload aspect panic recovered: %v", r)
						}
					}
				}()
				if err := aop.OnReload(rc, rc); err != nil {
					if config.Logger != nil {
						config.Logger.Printf("OnReload aspect error: %v", err)
					}
				}
			}()
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
	// Takes over the resource directory of newCtx, so that after reloading, ref:// parses to the new endpoint.
	rc.resources = newCtx.resources
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

// Resources: Returns the read-only view of the resource directory on the current chain (ref:// for same-chain resolution, for the consumer's read-only lookup).
func (rc *RuleChainCtx) Resources() types.ResourceLookup {
	return rc.ensureResources()
}

// EndpointRegistry returns a writable resource directory (only used by producer Register/Unregister such as EndpointAspect for production).
func (rc *RuleChainCtx) EndpointRegistry() types.ResourceRegistry {
	return rc.ensureResources()
}

// ensureResources lazy initializes the resource directory (double-check lock).
func (rc *RuleChainCtx) ensureResources() *resourceRegistry {
	rc.RLock()
	r := rc.resources
	rc.RUnlock()
	if r != nil {
		return r
	}
	rc.Lock()
	defer rc.Unlock()
	if rc.resources == nil {
		rc.resources = &resourceRegistry{}
	}
	return rc.resources
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

// HasEndNode checks whether the rule chain has configured termination nodes
// HasEndNode checks if the rule chain has configured end nodes
func (rc *RuleChainCtx) HasEndNode() bool {
	rc.RLock()
	defer rc.RUnlock()
	return rc.hasEndNode
}

// HasEndDescendant starts from a specified node and checks whether there are descendants of the "end node."
// HasEndDescendant determines whether there exists a descendant end node starting from the given node
//
// Parameters:
//   - startId: The identifier of the starting node
//
// Back:
//   - bool: Returns true if any termination node is reached; otherwise, returns false
//
// Explanation:
//   - Routing along the current rule chain through breadth-first traversal; When encountering a sub-rule chain connection, if the sub-rule chain has already been configured with a termination node, it is considered that there are descendants of the termination node
func (rc *RuleChainCtx) HasEndDescendant(startId types.RuleNodeId) bool {
	visited := make(map[types.RuleNodeId]struct{})
	queue := []types.RuleNodeId{startId}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		relations, ok := rc.GetNodeRoutes(current)
		if !ok {
			continue
		}

		for _, rel := range relations {
			nextId := rel.OutId
			if _, seen := visited[nextId]; seen {
				continue
			}
			visited[nextId] = struct{}{}

			if nodeCtx, ok := rc.GetNodeById(nextId); ok {
				if nodeCtx.Type() == types.NodeTypeEnd {
					return true
				} else if nextId.Type == types.CHAIN {
					if subChain, _ := nodeCtx.(*RuleChainCtx); subChain != nil {
						if subChain.HasEndNode() {
							return true
						}
					}
				}
			}

			queue = append(queue, nextId)
		}
	}

	return false
}

// GetReferencedNodes retrieves the list of nodes referenced by other nodes
// GetReferencedNodes gets the list of nodes that are referenced by other nodes
func (rc *RuleChainCtx) GetReferencedNodes() []string {
	rc.RLock()
	defer rc.RUnlock()
	return rc.referencedNodes
}

// GetNodeDependencies retrieves the list of dependency node IDs for the specified node
// GetNodeDependencies gets the dependent node IDs for the specified node
func (rc *RuleChainCtx) GetNodeDependencies(nodeId string) []string {
	rc.RLock()
	defer rc.RUnlock()
	if dependencies, exists := rc.nodeDependencies[nodeId]; exists {
		return dependencies
	}
	return nil
}

// AddNodeDependency Adds dependencies between nodes
// AddNodeDependency adds a dependency relationship between nodes
func (rc *RuleChainCtx) AddNodeDependency(nodeId string, dependentNodeId string) {
	rc.Lock()
	defer rc.Unlock()

	// Initialize nodeDependencies map if it doesn't exist
	if rc.nodeDependencies == nil {
		rc.nodeDependencies = make(map[string][]string)
	}

	// Get existing dependencies for the node
	dependencies, exists := rc.nodeDependencies[nodeId]
	if !exists {
		dependencies = make([]string, 0)
	}

	// Check if dependency already exists to avoid duplicates
	for _, existingDep := range dependencies {
		if existingDep == dependentNodeId {
			return // Dependency already exists
		}
	}

	// Add the new dependency
	dependencies = append(dependencies, dependentNodeId)
	rc.nodeDependencies[nodeId] = dependencies

	// Add to referencedNodes if not already present
	alreadyReferenced := false
	for _, referencedNode := range rc.referencedNodes {
		if referencedNode == dependentNodeId {
			alreadyReferenced = true
			break
		}
	}
	if !alreadyReferenced {
		rc.referencedNodes = append(rc.referencedNodes, dependentNodeId)
	}
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
