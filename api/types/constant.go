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

package types

import "errors"

const (
	CallbackFuncOnRuleChainCompleted = "onRuleChainCompleted"
	CallbackFuncOnNodeCompleted      = "onNodeCompleted"
	CallbackFuncDebug                = "onDebug"
)

const (
	Global = "global"
	// Vars ruleChain dsl additionalInfo vars key
	Vars = "vars"
	// Secrets ruleChain dsl additionalInfo secrets key
	Secrets = "secrets"
	// RuleChainKey ruleChain dsl key for accessing rule chain properties
	// RuleChainKey DSL key, used to access the rule chain properties
	RuleChainKey = "ruleChain"
)

const (
	EndpointTypePrefix                = "endpoint/"
	NodeConfigurationPrefixInstanceId = "ref://"
	// NamespaceSeparator defines the separator for namespace prefixes
	NamespaceSeparator = ":"
)

// Node type constants define the standard node types used in rule chains.
// Node Type Constants define the standard node types used in the rule chain.
const (
	// NodeTypeEnd represents the end node type that triggers rule chain completion callbacks
	// NodeTypeEnd indicates the type of node that triggers the rule chain completion callback
	NodeTypeEnd = "end"
)

const (
	//NodeConfigurationKeyIsInitNetResource component configuration key is used to initialize network resources, used for parameter validation differentiation in node component initialization
	NodeConfigurationKeyIsInitNetResource = "$initNetResource"
	// NodeConfigurationKeyChainCtx obtains the context of the rule chain, Key, value type: ChainCtx
	NodeConfigurationKeyChainCtx = "$chainCtx"
	//NodeConfigurationKeySelfDefinition Gets the node definition, value type: RuleNode
	NodeConfigurationKeySelfDefinition = "$selfDefinition"
	//NodeConfigurationKeyRuleChainDefinition obtains the rule chain definition and is used for the initialization of dynamic endpoints. value type: *RuleChain
	NodeConfigurationKeyRuleChainDefinition = "$ruleChainDefinition"
	//NodeConfigurationKeySessionKey Server-type endpoint sessionKey configuration key for session addressing (values support ${} expressions or array multi-candidate configuration)
	NodeConfigurationKeySessionKey = "sessionKey"
	//NodeConfigurationKeySessionTTL Server-type endpoint session idle TTL (seconds, <=0 uses default 1800)
	NodeConfigurationKeySessionTTL = "sessionTTL"
)

var (
	// ErrConcurrencyLimitReached is the error returned when the concurrency limit has been reached
	ErrConcurrencyLimitReached = errors.New("concurrency limit reached")
	ErrCacheNotInitialized     = errors.New("cache not initialized")
	// ErrEngineShuttingDown is the error returned when the engine is shutting down and cannot accept new messages
	ErrEngineShuttingDown = errors.New("engine is shutting down")
	// ErrEngineNotInitialized is the error returned when the rule engine is not initialized
	ErrEngineNotInitialized = errors.New("rule engine not initialized")
	// ErrEngineReloadTimeout is the error returned when engine reload operation times out
	ErrEngineReloadTimeout = errors.New("engine reload timeout")
	// ErrEngineReloadBackpressureLimit is the error returned when reload backpressure limit is reached
	// to prevent memory overflow during high-traffic reload operations
	ErrEngineReloadBackpressureLimit = errors.New("engine reload backpressure limit reached - rejecting message to prevent memory overflow")
	// ErrRuleChainHasNoNodes is the error returned when the rule chain has no nodes
	ErrRuleChainHasNoNodes = errors.New("the rule chain has no nodes")
	// ErrEngineDisabled is returned when attempting to use a disabled rule chain.
	ErrEngineDisabled = errors.New("the rule chain has been disabled")
	// ErrEngineDslEmpty is returned when the rule chain dsl is empty.
	ErrEngineDslEmpty = errors.New("dsl can not empty")
)

const (
	// DefaultRelationType The default relationship name used when a matching node cannot be found
	// DefaultRelationType is the default relation name used when no matching node is found.
	DefaultRelationType = "Default"

	// DefaultRelationTypeKey is used to customize the configuration attribute key for the default relationship type
	// DefaultRelationTypeKey is the configuration property key for customizing the default relation type.
	DefaultRelationTypeKey = "defaultRelationType"
)

const (
	// KeyStreamCompleted key
	KeyStreamCompleted = "stream_completed"
	// KeyStreamStart key
	KeyStreamStart = "stream_start"
	// ValueTrue truth string
	ValueTrue = "true"
)

const (
	// KeyDebugMode per-message debug mode metadata key
	// KeyDebugMode per-message Debug mode metadata key
	KeyDebugMode = "_debugMode"
	// KeySkipTellNext per-message skip tell next metadata key
	// KeySkipTellNext per-message skips the next node metadata key for notification
	KeySkipTellNext = "_skipTellNext"
)
