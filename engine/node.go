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
	defaultNodeIdPrefix = "node"
)

// RuleNodeCtx represents an instance of a node component within the rule engine.
type RuleNodeCtx struct {
	node              types.Node       // Instance of the component (now private to force use of safe methods)
	ChainCtx          *RuleChainCtx    // Context of the rule chain configuration
	SelfDefinition    *types.RuleNode  // Configuration of the component itself
	config            types.Config     // Configuration of the rule engine
	aspects           types.AspectList // List of AOP (Aspect-Oriented Programming) aspects
	isInitNetResource bool             // Indicates if network resources should be initialized
	sync.RWMutex                       // Add mutex for thread safety
}

// InitRuleNodeCtx initializes a RuleNodeCtx with the given parameters.
func InitRuleNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	return initRuleNodeCtx(config, chainCtx, aspects, selfDefinition, false)
}

// InitNetResourceNodeCtx initializes a RuleNodeCtx with network resources.
func InitNetResourceNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	return initRuleNodeCtx(config, chainCtx, aspects, selfDefinition, true)
}

// initRuleNodeCtx is the core initialization function for RuleNodeCtx.
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
				node:              node,
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
	config := rn.config
	rn.RUnlock()

	if node, err := config.Parser.DecodeRuleNode(def); err == nil {
		return rn.ReloadSelfFromDef(node)
	} else {
		return err
	}
}

// ReloadSelfFromDef reloads the node from a RuleNode definition.
func (rn *RuleNodeCtx) ReloadSelfFromDef(def types.RuleNode) error {
	// Read current values with lock protection
	rn.RLock()
	chainCtx := rn.ChainCtx
	config := rn.config
	isInitNetResource := rn.isInitNetResource
	rn.RUnlock()

	var ctx *RuleNodeCtx
	var err error
	if chainCtx == nil {
		ctx, err = initRuleNodeCtx(config, nil, nil, &def, isInitNetResource)
	} else {
		ctx, err = initRuleNodeCtx(config, chainCtx, chainCtx.aspects, &def, isInitNetResource)
	}
	if err == nil {
		rn.Lock()
		// Store old node for destruction after unlocking
		oldNode := rn.node
		// Copy the new context (direct assignment to avoid additional Copy() call)
		rn.node = ctx.node
		rn.config = ctx.config
		rn.aspects = ctx.aspects
		rn.SelfDefinition = ctx.SelfDefinition
		rn.Unlock()

		// Destroy the old node after releasing the lock to avoid race conditions
		if oldNode != nil {
			oldNode.Destroy()
		}
		return nil
	} else {
		return err
	}
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
	config := rn.config
	selfDefinition := rn.SelfDefinition
	rn.RUnlock()

	v, _ := config.Parser.EncodeRuleNode(selfDefinition)
	return v
}

// Copy copies the contents of a new RuleNodeCtx into this one.
func (rn *RuleNodeCtx) Copy(newCtx *RuleNodeCtx) {
	rn.Lock()
	defer rn.Unlock()
	rn.node = newCtx.node
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

// Thread-safe Node interface implementation
// OnMsg safely handles message processing
func (rn *RuleNodeCtx) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	rn.RLock()
	node := rn.node
	rn.RUnlock()
	if node != nil {
		node.OnMsg(ctx, msg)
	}
}

// Type safely returns the component type
func (rn *RuleNodeCtx) Type() string {
	rn.RLock()
	node := rn.node
	rn.RUnlock()
	if node != nil {
		return node.Type()
	}
	return ""
}

// New safely creates a new instance
func (rn *RuleNodeCtx) New() types.Node {
	rn.RLock()
	node := rn.node
	rn.RUnlock()
	if node != nil {
		return node.New()
	}
	return nil
}

// Init safely initializes the node
func (rn *RuleNodeCtx) Init(config types.Config, configuration types.Configuration) error {
	rn.RLock()
	node := rn.node
	rn.RUnlock()
	if node != nil {
		return node.Init(config, configuration)
	}
	return nil
}

// Destroy safely destroys the node
func (rn *RuleNodeCtx) Destroy() {
	rn.RLock()
	node := rn.node
	rn.RUnlock()
	if node != nil {
		node.Destroy()
	}
}

// GetNode safely returns the underlying node (mainly for testing and debugging)
func (rn *RuleNodeCtx) GetNode() types.Node {
	rn.RLock()
	defer rn.RUnlock()
	return rn.node
}
