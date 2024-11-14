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

// RuleChain defines a rule chain.
type RuleChain struct {
	// RuleChain contains the basic information of the rule chain.
	RuleChain RuleChainBaseInfo `json:"ruleChain"`
	// Metadata includes information about the nodes and connections within the rule chain.
	Metadata RuleMetadata `json:"metadata"`
}

// RuleChainBaseInfo defines the basic information of a rule chain.
type RuleChainBaseInfo struct {
	// ID is the unique identifier of the rule chain.
	ID string `json:"id"`
	// Name is the name of the rule chain.
	Name string `json:"name"`
	// DebugMode indicates whether the node is in debug mode. If true, a debug callback function is triggered when the node processes messages.
	// This setting overrides the `DebugMode` configuration of the node.
	DebugMode bool `json:"debugMode"`
	// Root indicates whether this rule chain is a root or a sub-rule chain. (Used only as a marker, not applied in actual logic)
	Root bool `json:"root"`
	// Configuration contains the configuration information of the rule chain.
	Configuration Configuration `json:"configuration,omitempty"`
	// AdditionalInfo is an extension field.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// GetAdditionalInfo retrieves additional information by key.
func (r RuleChainBaseInfo) GetAdditionalInfo(key string) (interface{}, bool) {
	if r.AdditionalInfo == nil {
		return "", false
	}
	v, ok := r.AdditionalInfo[key]
	return v, ok
}

// PutAdditionalInfo adds additional information by key and value.
func (r RuleChainBaseInfo) PutAdditionalInfo(key string, value interface{}) {
	if r.AdditionalInfo == nil {
		r.AdditionalInfo = make(map[string]interface{})
	}
	r.AdditionalInfo[key] = value
}

// RuleMetadata defines the metadata of a rule chain, including information about nodes and connections.
type RuleMetadata struct {
	// FirstNodeIndex is the index of the first node in data flow, default is 0.
	FirstNodeIndex int `json:"firstNodeIndex"`
	// Nodes are the component definitions of the nodes.
	Endpoints []*EndpointDsl `json:"endpoints,omitempty"`
	// Nodes are the component definitions of the nodes.
	// Each object represents a rule node within the rule chain.
	Nodes []*RuleNode `json:"nodes"`
	// Connections define the connections between two nodes in the rule chain.
	Connections []NodeConnection `json:"connections"`
	// Deprecated: Use Flow Node instead.
	// RuleChainConnections are the connections between a node and a sub-rule chain.
	RuleChainConnections []RuleChainConnection `json:"ruleChainConnections,omitempty"`
}

// RuleNode defines the information of a rule chain node.
type RuleNode struct {
	// Id is the unique identifier of the node, which can be any string.
	Id string `json:"id"`
	// AdditionalInfo is an extension field for visualization position information (reserved field).
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
	// Type is the type of the node, which determines the logic and behavior of the node. It should match one of the node types registered in the rule engine.
	Type string `json:"type"`
	// Name is the name of the node, which can be any string.
	Name string `json:"name"`
	// DebugMode indicates whether the node is in debug mode. If true, a debug callback function is triggered when the node processes messages.
	// This setting can be overridden by the RuleChain `DebugMode` configuration.
	DebugMode bool `json:"debugMode"`
	// Configuration contains the configuration parameters of the node, which vary depending on the node type.
	// For example, a JS filter node might have a `jsScript` field defining the filtering logic,
	// while a REST API call node might have a `restEndpointUrlPattern` field defining the URL to call.
	Configuration Configuration `json:"configuration"`
}

// NodeAdditionalInfo is used for visualization position information (reserved field).
type NodeAdditionalInfo struct {
	Description string `json:"description"`
	LayoutX     int    `json:"layoutX"`
	LayoutY     int    `json:"layoutY"`
}

// NodeConnection defines the connection between two nodes in a rule chain.
type NodeConnection struct {
	// FromId is the id of the source node, which should match the id of a node in the nodes array.
	FromId string `json:"fromId"`
	// ToId is the id of the target node, which should match the id of a node in the nodes array.
	ToId string `json:"toId"`
	// Type is the type of connection, which determines when and how messages are sent from one node to another. It should match one of the connection types supported by the source node type.
	// For example, a JS filter node might support two connection types: "True" and "False," indicating whether the message passes or fails the filter condition.
	Type string `json:"type"`
}

// RuleChainConnection defines the connection between a node and a sub-rule chain.
type RuleChainConnection struct {
	// FromId is the id of the source node, which should match the id of a node in the nodes array.
	FromId string `json:"fromId"`
	// ToId is the id of the target sub-rule chain, which should match one of the sub-rule chains registered in the rule engine.
	ToId string `json:"toId"`
	// Type is the type of connection, which determines when and how messages are sent from one node to another. It should match one of the connection types supported by the source node type.
	Type string `json:"type"`
}

// RuleChainRunSnapshot is a snapshot of the rule chain execution log.
type RuleChainRunSnapshot struct {
	RuleChain
	// Id is the execution ID.
	Id string `json:"id"`
	// StartTs is the start time of execution.
	StartTs int64 `json:"startTs"`
	// EndTs is the end time of execution.
	EndTs int64 `json:"endTs"`
	// Logs are the logs for each node.
	Logs []RuleNodeRunLog `json:"logs"`
	// AdditionalInfo is an extension field.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// RuleNodeRunLog is the log for a node.
type RuleNodeRunLog struct {
	// Id is the node ID.
	Id string `json:"nodeId"`
	// InMsg is the input message.
	InMsg RuleMsg `json:"inMsg"`
	// OutMsg is the output message.
	OutMsg RuleMsg `json:"outMsg"`
	// RelationType is the connection type with the next node.
	RelationType string `json:"relationType"`
	// Err is the error information.
	Err string `json:"err"`
	// LogItems are the logs during execution.
	LogItems []string `json:"logItems"`
	// StartTs is the start time of execution.
	StartTs int64 `json:"startTs"`
	// EndTs is the end time of execution.
	EndTs int64 `json:"endTs"`
}

// EndpointDsl defines the DSL for an endpoint.
type EndpointDsl struct {
	RuleNode
	// Processors is the list of global processors for the endpoint.
	// Using processors registered in builtin/processor#Builtins xx by name.
	Processors []string `json:"processors"`
	// Routers is the list of routers.
	Routers []*RouterDsl `json:"routers"`
}

// RouterDsl defines a router for an endpoint.
type RouterDsl struct {
	// Id is the router ID, optional and by default uses From.Path.
	Id string `json:"id"`
	// Params is the parameters for the router.
	// HTTP Endpoint router params is POST/GET/PUT...
	Params []interface{} `json:"params"`
	// From is the source for the router.
	From FromDsl `json:"from"`
	// To is the destination for the router.
	To ToDsl `json:"to"`
	// AdditionalInfo is an extension field.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// FromDsl defines the source for an endpoint router.
type FromDsl struct {
	// Path is the path of the source.
	Path string `json:"path"`
	// Configuration is the configuration for the source.
	Configuration Configuration `json:"configuration"`
	// Processors is the list of processors for the source.
	// Using processors registered in builtin/processor#Builtins xx by name.
	Processors []string `json:"processors"`
}

// ToDsl defines the destination for an endpoint router.
type ToDsl struct {
	// Path is the path of the executor for the destination.
	// For example, "chain:default" to execute by a rule chain for `default`, "component:jsTransform" to execute a JS transform component.
	Path string `json:"path"`
	// Configuration is the configuration for the destination.
	Configuration Configuration `json:"configuration"`
	// Wait indicates whether to wait for the 'To' executor to finish before proceeding.
	Wait bool `json:"wait"`
	// Processors is the list of processors for the destination.
	// Using processors registered in builtin/processor#Builtins xx by name.
	Processors []string `json:"processors"`
}
