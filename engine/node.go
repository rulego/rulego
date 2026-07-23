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
	"github.com/rulego/rulego/utils/dsl"
	"github.com/rulego/rulego/utils/str"
)

const (
	// defaultNodeIdPrefix is the prefix used for auto-generated node IDs
	// when no explicit ID is provided in the node definition.
	// defaultNodeIdPrefix is a prefix used to automatically generate node IDs when no explicit ID is provided in the node definition.
	defaultNodeIdPrefix = "node"
)

// RuleNodeCtx represents an instance of a node component within the rule engine.
// It acts as a wrapper around the actual node implementation, providing additional
// context and metadata required for rule chain execution.
//
// RuleNodeCtx represents an instance of a node component in the rule engine.
// It acts as a wrapper for the actual node implementation, providing the additional context and metadata needed for the execution of the rule chain.
//
// Architecture:
// Structure:
//
//	RuleNodeCtx embeds the types.Node interface, allowing it to act as both
//	a node wrapper and a node implementation. This design provides:
//	RuleNodeCtx embed types.Node interface, allowing it to act both as a node wrapper and as a node implementation.
//	This design provides:
//	- Direct access to node methods through interface embedding
//	- Additional context and configuration management
//	- Thread-safe operations with mutex protection
//	- Hot reloading capabilities
type RuleNodeCtx struct {
	// types.Node is the embedded node implementation providing the core functionality.
	// This embedding allows RuleNodeCtx to act as a node while adding wrapper capabilities.
	// types.Node is an embedded node implementation that provides core functionality.
	// This embedding allows RuleNodeCtx to act as a node while adding wrapper functionality.
	types.Node

	// ChainCtx provides access to the parent rule chain context,
	// enabling node-to-chain communication and access to shared resources.
	// ChainCtx provides access to the parent rule chain context, supports node-to-chain communication, and access to shared resources.
	ChainCtx *RuleChainCtx

	// SelfDefinition contains the configuration and metadata for this specific node,
	// including its type, ID, configuration parameters, and behavioral settings.
	// SelfDefinition contains the configuration and metadata of this specific node,
	// Including its type, ID, configuration parameters, and behavior settings.
	SelfDefinition *types.RuleNode

	// config holds the global rule engine configuration,
	// providing access to component registry, parsers, and global settings.
	// config stores global rule engine configurations and provides access to component registry, parsers, and global settings.
	config types.Config

	// aspects contains the list of AOP aspects applied to this node,
	// enabling cross-cutting concerns like logging, validation, and metrics.
	// aspects contains the AOP aspects applied to this node,
	// Supports cross-cutting concerns such as logs, validation, and metrics.
	aspects types.AspectList

	// isInitNetResource indicates whether network resources should be initialized
	// for this node. This flag is used for nodes that require network connectivity.
	// isInitNetResource indicates whether network resources should be initialized for this node.
	// This flag is used for nodes that require network connectivity.
	isInitNetResource bool

	// sync.RWMutex provides thread-safe access to the node context,
	// ensuring concurrent safety during hot reloads and message processing.
	// sync.RWMutex provides thread-safe access to the node context,
	// Ensure concurrency security during hot overloads and message processing.
	sync.RWMutex
}

// InitRuleNodeCtx initializes a RuleNodeCtx with the given parameters.
// This is the standard initialization function for regular nodes without network resources.
//
// InitRuleNodeCtx initializes RuleNodeCtx using a given parameter.
// This is the standard initialization function for regular nodes without network resources.
//
// Parameters:
// Parameters:
//   - config: Global rule engine configuration
//   - chainCtx: Parent rule chain context
//   - aspects: List of AOP aspects to apply
//   - selfDefinition: Node definition and configuration
//
// Returns:
// Returns:
//   - *RuleNodeCtx: Initialized node context
//   - error: Initialization error if any
func InitRuleNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	return initRuleNodeCtx(config, chainCtx, aspects, selfDefinition, false)
}

// InitNetResourceNodeCtx initializes a RuleNodeCtx with network resources.
// This function is used for nodes that require network connectivity and resources.
//
// InitNetResourceNodeCtx initializes RuleNodeCtx with network resources.
// This function is used for nodes that require network connectivity and resources.
//
// Parameters:
// Parameters:
//   - config: Global rule engine configuration
//   - chainCtx: Parent rule chain context
//   - aspects: List of AOP aspects to apply
//   - selfDefinition: Node definition and configuration
//
// Returns:
// Returns:
//   - *RuleNodeCtx: Initialized node context with network resources
//   - error: Initialization error if any
func InitNetResourceNodeCtx(config types.Config, chainCtx *RuleChainCtx, aspects types.AspectList, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	return initRuleNodeCtx(config, chainCtx, aspects, selfDefinition, true)
}

// initRuleNodeCtx is the core initialization function for RuleNodeCtx.
// It handles the complete node initialization process including component creation,
// configuration processing, and aspect integration.
//
// initRuleNodeCtx is the core initialization function of RuleNodeCtx.
// It handles the complete node initialization process, including component creation, configuration processing, and facet integration.
//
// Parameters:
// Parameters:
//   - config: Global rule engine configuration
//   - chainCtx: Parent rule chain context
//   - aspects: List of AOP aspects to apply
//   - selfDefinition: Node definition and configuration
//   - isInitNetResource: Whether to initialize network resources
//
// Returns:
// Returns:
//   - *RuleNodeCtx: Initialized node context
//   - error: Initialization error if any
//
// Initialization Process:
// Initialization process:
//  1. Execute before-init aspects
//  2. Create node instance from component registry
//  3. Process configuration variables and templates
//  4. Inject chain context and node definition
//  5. Initialize the node with processed configuration
//  6. Return wrapped node context
//
// Error Handling:
// Error handling:
//   - Aspect execution failures
//   - Component creation errors
//   - Configuration processing failures
//   - Node initialization errors
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
			// Parse and add node dependencies during initialization
			// Parses and adds node dependencies during initialization
			if chainCtx != nil {
				referencedNodeIds := dsl.ExtractReferencedNodeIds(selfDefinition.Configuration)
				for _, dependentNodeId := range referencedNodeIds {
					// Only add dependencies for nodes that exist in the rule chain
					// Only adds dependencies for nodes present in the rule chain
					if dsl.IsNodeIdDefined(*chainCtx.SelfDefinition, dependentNodeId) {
						chainCtx.AddNodeDependency(selfDefinition.Id, dependentNodeId)
					}
				}
			}

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
// ReloadSelfFromDef Reloads the node from the RuleNode definition.
// This method enables hot overloading for a single node, allowing dynamic updates without stopping the entire rule chain.
//
// Parameters:
// Parameters:
//   - def: New node definition
//
// Returns:
// Returns:
//   - error: Reload error if any
func (rn *RuleNodeCtx) ReloadSelfFromDef(def types.RuleNode) error {
	// Stage 1: Quickly read the current configuration (minimum lock read time)
	rn.RLock()
	chainCtx := rn.ChainCtx
	config := rn.config
	isInitNetResource := rn.isInitNetResource
	rn.RUnlock()

	// Stage 2: Create and initialize new nodes outside the lock, which is time-consuming
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

	// Stage 3: Fast atomic replacement (minimum write-lock time)
	rn.Lock()
	oldNode := rn.Node                            // Save old node references and destroy them out of lock
	rn.Node = newNodeCtx.Node                     // Atoms replace the most critical Node fields
	rn.config = newNodeCtx.config                 // Update the configuration
	rn.aspects = newNodeCtx.aspects               // Update the facet
	rn.SelfDefinition = newNodeCtx.SelfDefinition // Update node definitions
	rn.Unlock()

	// Stage 4: Cleaning old resources outside the lock (avoiding time-consuming cleaning operations inside the lock)
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

// OnMsg provides concurrent secure message processing to protect embedded Node access
// OnMsg provides concurrent-safe message processing with protected access to the embedded Node.
// This method ensures thread safety during message processing by using read locks to protect
// against concurrent modifications during hot reloads.
//
// OnMsg provides concurrent secure message processing by using lock reads to protect embedded node access.
// This method uses read locks to prevent concurrent modifications during hot overload, ensuring thread safety during message processing.
//
// Parameters:
// Parameters:
//   - ctx: Rule context for message processing
//   - msg: Message to be processed
func (rn *RuleNodeCtx) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// Uses read lock to protect access to Node fields, mutually exclusively with write locks in ReloadSelfFromDef
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
// Copy copies the contents of the new RuleNodeCtx to the current instance.
// This method is used to update node configurations during overload.
//
// Parameters:
// Parameters:
//   - newCtx: New node context to copy from
func (rn *RuleNodeCtx) Copy(newCtx *RuleNodeCtx) {
	rn.Lock()
	defer rn.Unlock()
	rn.Node = newCtx.Node
	rn.config = newCtx.config
	rn.aspects = newCtx.aspects
	rn.SelfDefinition = newCtx.SelfDefinition
}

// processVariables replaces placeholders in the node configuration with global and chain-specific variables.
// It now recursively processes nested maps and slices.
func processVariables(config types.Config, chainCtx *RuleChainCtx, configuration types.Configuration) (types.Configuration, error) {
	result := make(types.Configuration)
	globalEnv := make(map[string]string)

	if config.Properties != nil {
		globalEnv = config.Properties.Values()
	}

	var varsEnv, decryptSecrets map[string]string
	var ruleChainEnv *types.RuleChainBaseInfo

	if chainCtx != nil {
		varsEnv = copyMap(chainCtx.vars)
		decryptSecrets = copyMap(chainCtx.decryptSecrets)
		// Injecting rule chain definitions, supporting access to rule chain properties via ${ruleChain.id} and other methods
		if chainCtx.SelfDefinition != nil {
			ruleChainEnv = &chainCtx.SelfDefinition.RuleChain
		}
	}

	env := map[string]interface{}{
		types.Global:       globalEnv,
		types.Vars:         varsEnv,
		types.RuleChainKey: ruleChainEnv,
	}

	// Recurrent processing of all configuration values
	for key, value := range configuration {
		result[key] = processValueRecursive(env, value)
	}

	if varsEnv != nil {
		result[types.Vars] = varsEnv
	}
	if decryptSecrets != nil {
		result[types.Secrets] = decryptSecrets
	}

	return result, nil
}

// processValueRecursive reverses configuration values and replaces template variables
func processValueRecursive(env map[string]interface{}, value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return str.ExecuteTemplate(v, env)
	case map[string]interface{}:
		// Recurrent processing map
		subResult := make(map[string]interface{})
		for k, subV := range v {
			subResult[k] = processValueRecursive(env, subV)
		}
		return subResult
	case []interface{}:
		// Recurrent processing slice
		resultList := make([]interface{}, len(v))
		for i, item := range v {
			resultList[i] = processValueRecursive(env, item)
		}
		return resultList
	default:
		return value
	}
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
