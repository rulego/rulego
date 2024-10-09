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
type RelationCache struct {
	inNodeId     types.RuleNodeId // Identifier of the incoming node
	relationType string           // Type of relationship with the outgoing node
}

// RuleChainCtx defines an instance of a rule chain.
// It initializes all nodes and records the routing relationships between all nodes in the rule chain.
type RuleChainCtx struct {
	Id                 types.RuleNodeId                              // Identifier of the node
	SelfDefinition     *types.RuleChain                              // Definition of the rule chain
	config             types.Config                                  // Configuration of the rule engine
	initialized        bool                                          // Indicates whether the rule chain context has been initialized
	componentsRegistry types.ComponentRegistry                       // Registry of components
	nodeIds            []types.RuleNodeId                            // List of node identifiers
	nodes              map[types.RuleNodeId]types.NodeCtx            // Map of node contexts
	nodeRoutes         map[types.RuleNodeId][]types.RuleNodeRelation // Map of node routing relationships
	parentNodeIds      map[types.RuleNodeId][]types.RuleNodeId       // Map of parent node identifiers
	relationCache      map[RelationCache][]types.NodeCtx             // Cache of outgoing node lists based on incoming node and relationship
	rootRuleContext    types.RuleContext                             // Root context of the rule chain
	ruleChainPool      types.RuleEnginePool                          // Pool of sub-rule chains
	aspects            types.AspectList                              // List of AOP (Aspect-Oriented Programming) aspects
	afterReloadAspects []types.OnReloadAspect                        // List of aspects triggered after reload
	destroyAspects     []types.OnDestroyAspect                       // List of aspects triggered on destruction
	vars               map[string]string                             // Map of variables
	decryptSecrets     map[string]string                             // Map of decrypted secrets
	isEmpty            bool                                          // Indicates whether the rule chain has no nodes
	sync.RWMutex                                                     // Read/write mutex lock
}

// InitRuleChainCtx initializes a RuleChainCtx with the given configuration, aspects, and rule chain definition.
func InitRuleChainCtx(config types.Config, aspects types.AspectList, ruleChainDef *types.RuleChain) (*RuleChainCtx, error) {
	// Retrieve aspects for the engine
	chainBeforeInitAspects, _, _, afterReloadAspects, destroyAspects := aspects.GetEngineAspects()
	for _, aspect := range chainBeforeInitAspects {
		if err := aspect.OnChainBeforeInit(ruleChainDef); err != nil {
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
	if index >= len(rc.nodeIds) {
		return &RuleNodeCtx{}, false
	}
	return rc.GetNodeById(rc.nodeIds[index])
}

// GetFirstNode retrieves the first node, where the message starts flowing. By default, it's the node with index 0
func (rc *RuleChainCtx) GetFirstNode() (types.NodeCtx, bool) {
	var firstNodeIndex = rc.SelfDefinition.Metadata.FirstNodeIndex
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
	panic("not support this func")
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
	ctx.TellFlow(context.Background(), rc.Id.Id, msg, nil, nil)
}

// Destroy cleans up resources and executes destroy aspects
func (rc *RuleChainCtx) Destroy() {
	rc.RLock()
	defer rc.RUnlock()
	for _, v := range rc.nodes {
		temp := v
		temp.Destroy()
	}
	// Execute destroy aspects
	for _, aop := range rc.destroyAspects {
		aop.OnDestroy(rc)
	}
}

// IsDebugMode checks if debug mode is enabled
func (rc *RuleChainCtx) IsDebugMode() bool {
	return rc.SelfDefinition.RuleChain.DebugMode
}

// GetNodeId returns the node ID
func (rc *RuleChainCtx) GetNodeId() types.RuleNodeId {
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
func (rc *RuleChainCtx) ReloadSelfFromDef(def types.RuleChain) error {
	if ctx, err := InitRuleChainCtx(rc.config, rc.aspects, &def); err == nil {
		rc.Destroy()
		rc.Copy(ctx)
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
	v, _ := rc.config.Parser.EncodeRuleChain(rc.SelfDefinition)
	return v
}

// Definition returns the rule chain definition
func (rc *RuleChainCtx) Definition() *types.RuleChain {
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
	rc.aspects = aspects
	_, _, _, afterReloadAspects, destroyAspects := aspects.GetEngineAspects()
	rc.afterReloadAspects = afterReloadAspects
	rc.destroyAspects = destroyAspects
}

// GetAspects retrieves the aspects of the rule chain
func (rc *RuleChainCtx) GetAspects() types.AspectList {
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
