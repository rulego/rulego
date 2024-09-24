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

package types

import (
	"context"
)

// Relation types define the connections between nodes. These are common relations that can be customized.
const (
	Success = "Success"
	Failure = "Failure"
	True    = "True"
	False   = "False"
)

// Flow direction types indicate the direction of message flow into and out of nodes.
const (
	In  = "IN"  // Represents a message flowing into a node.
	Out = "OUT" // Represents a message flowing out of a node.
	Log = "Log" // Used for logging purposes.
)

// Script types define the scripting languages supported for script execution within nodes.
const (
	Js     = "Js"     // Represents JavaScript scripting language.
	Lua    = "Lua"    // Represents Lua scripting language.
	Python = "Python" // Represents Python scripting language.
)

// OnEndFunc is a callback function type that is executed when a branch of the rule chain completes.
type OnEndFunc = func(ctx RuleContext, msg RuleMsg, err error, relationType string)

// Configuration is a type for component configurations, represented as a map with string keys and interface{} values.
type Configuration map[string]interface{}

// ComponentType is an enum for component types: rule nodes or sub-rule chains.
type ComponentType int

const (
	NODE  ComponentType = iota // NODE represents a rule node component.
	CHAIN                      // CHAIN represents a sub-rule chain component.
	ENDPOINT
)

// PluginRegistry is an interface for providing node components via Go plugins.
// Example:
// package main
// var Plugins MyPlugins // Plugin entry point
// type MyPlugins struct{}
//
//	func (p *MyPlugins) Init() error {
//		return nil // Initialization logic for the plugin
//	}
//
//	func (p *MyPlugins) Components() []types.Node {
//		return []types.Node{&UpperNode{}, &TimeNode{}, &FilterNode{}} // A plugin can provide multiple components
//	}
//
// go build -buildmode=plugin -o plugin.so plugin.go # Compile the plugin to generate a plugin.so file
// rulego.Registry.RegisterPlugin("test", "./plugin.so") // Register the plugin with the default RuleGo registry
type PluginRegistry interface {
	// Init initializes the plugin.
	Init() error
	// Components returns a list of components provided by the plugin.
	Components() []Node
}

// ComponentRegistry is an interface for registering node components.
type ComponentRegistry interface {
	// Register adds a new component. If `node.Type()` already exists, it returns an 'already exists' error.
	Register(node Node) error
	// RegisterPlugin loads and registers a component from an external .so file using the plugin mechanism.
	// If `name` already exists or the component list provided by the plugin `node.Type()` exists, it returns an 'already exists' error.
	RegisterPlugin(name string, file string) error
	// Unregister removes a component or a batch of components by plugin name.
	Unregister(componentType string) error
	// NewNode creates a new instance of a node by nodeType.
	NewNode(nodeType string) (Node, error)
	// GetComponents retrieves a list of all registered components.
	GetComponents() map[string]Node
	// GetComponentForms retrieves configuration forms for all registered components, used for visual configuration.
	GetComponentForms() ComponentFormList
}

// Node is an interface for rule engine node components.
// Business or common logic is encapsulated into components, which are then invoked through rule chain configurations.
// Implementation reference can be found in the `components` package.
// Then register to the default `RuleGo` registry.
// rulego.Registry.Register(&MyNode{})
type Node interface {
	// New creates a new instance of a component.
	// A new instance is created for each rule node in the rule chain, with independent data.
	New() Node
	// Type returns the component type, which must be unique.
	// Used for rule chain configuration to initialize the corresponding component.
	// It is recommended to use `/` to distinguish namespaces and prevent conflicts, e.g., x/httpClient.
	Type() string
	// Init initializes the component, typically for component parameter configuration or client initialization.
	// This is called once during the initialization of rule nodes in the rule chain.
	Init(ruleConfig Config, configuration Configuration) error
	// OnMsg processes messages. Each message flowing into the component is processed by this function.
	// ctx: Context for message processing by the rule engine.
	// msg: The message to be processed.
	// After executing the logic, call ctx.TellSuccess/ctx.TellFailure/ctx.TellNext to notify the next node, otherwise, the rule chain cannot end.
	OnMsg(ctx RuleContext, msg RuleMsg)
	// Destroy releases resources when the component is no longer needed.
	Destroy()
}

// NodeCtx is the context for instantiating rule nodes.
type NodeCtx interface {
	Node
	Config() Config
	// IsDebugMode checks if the node is in debug mode.
	// True: When messages flow in and out of the node, the config.OnDebug callback function is called; otherwise, it is not.
	IsDebugMode() bool
	// GetNodeId retrieves the component ID.
	GetNodeId() RuleNodeId
	// ReloadSelf refreshes the configuration of the component.
	ReloadSelf(def []byte) error
	// GetNodeById retrieves the configuration of a specified ID component in a sub-rule chain.
	// If it is a node type, this method is not supported.
	GetNodeById(nodeId RuleNodeId) (NodeCtx, bool)
	// DSL returns the configuration DSL of the node.
	DSL() []byte
}

type ChainCtx interface {
	NodeCtx
	// ReloadChild refreshes the configuration of a specified ID component in a sub-rule chain.
	// If it is a node type, this method is not supported.
	ReloadChild(nodeId RuleNodeId, def []byte) error
	// Definition returns the definition of the rule chain.
	Definition() *RuleChain
	// GetRuleEnginePool retrieves the rule engine pool.
	GetRuleEnginePool() RuleEnginePool
}

// RuleContext is the interface for message processing context within the rule engine.
// It handles the transfer of messages to the next or multiple nodes and triggers their business logic.
// It also controls and orchestrates the node flow of the current execution instance.
type RuleContext interface {
	// TellSuccess notifies the rule engine that the current message has been successfully processed and sends the message to the next node via the 'Success' relationship.
	TellSuccess(msg RuleMsg)
	// TellFailure notifies the rule engine that the current message has failed to process and sends the message to the next node via the 'Failure' relationship.
	TellFailure(msg RuleMsg, err error)
	// TellNext sends the message to the next node using the specified relationTypes.
	TellNext(msg RuleMsg, relationTypes ...string)
	// TellSelf sends a message to the current node after a specified delay (in milliseconds).
	TellSelf(msg RuleMsg, delayMs int64)
	// TellNextOrElse sends the message to the next node using the specified relationTypes. If the corresponding relationType does not find the next node, it uses defaultRelationType to search.
	TellNextOrElse(msg RuleMsg, defaultRelationType string, relationTypes ...string)
	// TellFlow executes a sub-rule chain.
	// ruleChainId: The ID of the rule chain.
	// onEndFunc: Callback for when a branch of the sub-rule chain completes, returning the result of that chain. If multiple branches are triggered, it will be called multiple times.
	// onAllNodeCompleted: Callback for when all nodes have completed, with no result returned.
	// If the rule chain is not found, the message is sent to the next node via the 'Failure' relationship.
	TellFlow(ctx context.Context, ruleChainId string, msg RuleMsg, endFunc OnEndFunc, onAllNodeCompleted func())
	// TellNode starts execution from a specified node. If skipTellNext=true, only the current node is executed without notifying the next node.
	// onEnd is used to view the final execution result.
	TellNode(ctx context.Context, nodeId string, msg RuleMsg, skipTellNext bool, onEnd OnEndFunc, onAllNodeCompleted func())
	// TellChainNode executes the specified node in the specified rule chain.
	// If skipTellNext=true, only the current node is executed, and no message is sent to the next node.
	TellChainNode(ctx context.Context, ruleChainId, nodeId string, msg RuleMsg, skipTellNext bool, onEnd OnEndFunc, onAllNodeCompleted func())
	// NewMsg creates a new message instance.
	NewMsg(msgType string, metaData Metadata, data string) RuleMsg
	// GetSelfId retrieves the current node ID.
	GetSelfId() string
	// Self retrieves the current node instance.
	Self() NodeCtx
	// From retrieves the node instance from which the message entered the current node.
	From() NodeCtx
	// RuleChain retrieves the rule chain instance where the current node resides.
	RuleChain() NodeCtx
	// Config retrieves the configuration of the rule engine.
	Config() Config
	// SubmitTack submits an asynchronous task for execution.
	SubmitTack(task func())
	// SetEndFunc sets the callback function for when the current message processing ends.
	SetEndFunc(f OnEndFunc) RuleContext
	// GetEndFunc retrieves the callback function for when the current message processing ends.
	GetEndFunc() OnEndFunc
	// SetContext sets a context for sharing semaphores or data across different component instances.
	SetContext(c context.Context) RuleContext
	// GetContext retrieves the context for sharing semaphores or data across different component instances.
	GetContext() context.Context
	// SetOnAllNodeCompleted sets the callback for when all nodes have completed execution.
	SetOnAllNodeCompleted(onAllNodeCompleted func())
	// DoOnEnd triggers the OnEnd callback function.
	DoOnEnd(msg RuleMsg, err error, relationType string)
	// SetCallbackFunc sets a callback function.
	SetCallbackFunc(functionName string, f interface{})
	// GetCallbackFunc retrieves a callback function.
	GetCallbackFunc(functionName string) interface{}
	// OnDebug calls the configured OnDebug callback function.
	OnDebug(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	// SetExecuteNode sets the current node.
	// If relationTypes is empty, execute the current node; otherwise,
	// find and execute the child nodes of the current node.
	SetExecuteNode(nodeId string, relationTypes ...string)
	// TellCollect gathers the execution results from multiple nodes and registers a callback function to collect the result list.
	// If it is the first time to register, it returns true; otherwise, it returns false.
	TellCollect(msg RuleMsg, callback func(msgList []WrapperMsg)) bool
}

// RuleContextOption is a function type for modifying RuleContext options.
type RuleContextOption func(RuleContext)

// WithEndFunc is a callback function for when a branch of the rule chain completes.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Use `types.WithOnEnd` instead.
func WithEndFunc(endFunc func(ctx RuleContext, msg RuleMsg, err error)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetEndFunc(func(ctx RuleContext, msg RuleMsg, err error, relationType string) {
			endFunc(ctx, msg, err)
		})
	}
}

// WithOnEnd is a callback function for when a branch of the rule chain completes.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
func WithOnEnd(endFunc func(ctx RuleContext, msg RuleMsg, err error, relationType string)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetEndFunc(endFunc)
	}
}

// WithContext sets a context for sharing data or semaphores between different component instances.
// It is also used for timeout cancellation.
func WithContext(c context.Context) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetContext(c)
	}
}

// WithOnAllNodeCompleted is a callback function for when the rule chain execution completes.
func WithOnAllNodeCompleted(onAllNodeCompleted func()) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetOnAllNodeCompleted(onAllNodeCompleted)
	}
}

// WithOnRuleChainCompleted is a callback function for when the rule chain execution completes and collects the runtime logs of each node.
func WithOnRuleChainCompleted(onCallback func(ctx RuleContext, snapshot RuleChainRunSnapshot)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncOnRuleChainCompleted, onCallback)
	}
}

// WithOnNodeCompleted is a callback function for when a node execution completes and collects the node's runtime log.
func WithOnNodeCompleted(onCallback func(ctx RuleContext, nodeRunLog RuleNodeRunLog)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncOnNodeCompleted, onCallback)
	}
}

// WithOnNodeDebug is a callback function for node debug logs, called in real-time asynchronously. It is triggered only if the node is configured with debugMode.
func WithOnNodeDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncDebug, onDebug)
	}
}

// WithStartNode 设置第一个开始执行节点
// WithStartNode sets the first node to start execution.
func WithStartNode(nodeId string) RuleContextOption {
	return func(rc RuleContext) {
		if nodeId == "" {
			return
		}
		rc.SetExecuteNode(nodeId)
	}
}

// WithTellNext 设置通过指定节点Id，查找下一个或者多个执行节点。用于恢复规则链执行链路
// WithTellNext sets the next or multiple execution nodes by specifying the node ID.
// It is used to restore the execution path of the rule chain.
func WithTellNext(fromNodeId string, relationTypes ...string) RuleContextOption {
	return func(rc RuleContext) {
		if fromNodeId == "" {
			return
		}
		rc.SetExecuteNode(fromNodeId, relationTypes...)
	}
}

// JsEngine is a JavaScript script engine interface.
type JsEngine interface {
	// Execute runs a specified function in the JS script, which is initialized when the JsEngine instance is created.
	// functionName is the name of the function to execute.
	// argumentList is the list of arguments for the function.
	Execute(functionName string, argumentList ...interface{}) (interface{}, error)
	// Stop releases the resources of the JS engine.
	Stop()
}

// Parser is an interface for parsing rule chain definition files (DSL).
// The default implementation uses JSON. If other formats are used to define rule chains, this interface can be implemented.
// Then register it with the rule engine like this: `rulego.NewConfig(WithParser(&MyParser{})`
type Parser interface {
	// DecodeRuleChain parses a rule chain structure from a description file.
	DecodeRuleChain(rootRuleChain []byte) (RuleChain, error)
	// DecodeRuleNode parses a rule node structure from a description file.
	DecodeRuleNode(rootRuleChain []byte) (RuleNode, error)
	// EncodeRuleChain converts a rule chain structure into a description file.
	EncodeRuleChain(def interface{}) ([]byte, error)
	// EncodeRuleNode converts a rule node structure into a description file.
	EncodeRuleNode(def interface{}) ([]byte, error)
}

// Pool is an interface for a coroutine pool.
type Pool interface {
	// Submit submits a task to the coroutine pool.
	// Returns an error if the coroutine pool is full.
	Submit(task func()) error
	// Release releases the resources of the pool.
	Release()
}

// EmptyRuleNodeId is an empty node ID.
var EmptyRuleNodeId = RuleNodeId{}

// RuleNodeId is a type definition for component IDs.
type RuleNodeId struct {
	// Id is the node ID.
	Id string
	// Type is the component type, either a node or a sub-rule chain.
	Type ComponentType
}

// RuleNodeRelation defines the relationship between nodes.
type RuleNodeRelation struct {
	// InId is the incoming component ID.
	InId RuleNodeId
	// OutId is the outgoing component ID.
	OutId RuleNodeId
	// RelationType is the type of relationship, such as True, False, Success, Failure, or other custom types.
	RelationType string
}

// ScriptFuncSeparator is the delimiter for script function names.
const ScriptFuncSeparator = "#"

// Script is used to register native functions or custom functions defined in Go.
type Script struct {
	// Type is the script type, default is Js.
	Type string
	// Content is the script content or custom function.
	Content interface{}
}
