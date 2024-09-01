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

// SharedNode represents a network resource node component such as a client or server connection.
type SharedNode interface {
	Node
	// GetInstance retrieves the underlying net client or server connection.
	// Used for connection pool reuse
	GetInstance() (interface{}, error)
}

type SharedNodeCtx interface {
	NodeCtx
	// GetInstance Obtain shared component resource instance
	GetInstance() (interface{}, error)
}

type NodePool interface {
	// Load loads sharedNode list from a ruleChain DSL definition.
	Load(dsl []byte) (NodePool, error)
	// LoadFromRuleChain loads sharedNode list from a ruleChain definition.
	LoadFromRuleChain(def RuleChain) (NodePool, error)
	//NewFromEndpoint new an endpoint sharedNode
	NewFromEndpoint(def EndpointDsl) (SharedNodeCtx, error)
	//NewFromRuleNode new a rule node sharedNode
	NewFromRuleNode(def RuleNode) (SharedNodeCtx, error)
	// Get retrieves a SharedNode instance by its ID.
	Get(id string) (SharedNodeCtx, bool)
	// GetInstance retrieves a net client or server connection by its nodeTye and ID.
	GetInstance(id string) (interface{}, error)
	// Del deletes a SharedNode instance by its nodeTye and ID.
	Del(id string)
	// Stop stops and releases all SharedNode instances.
	Stop()
	// GetAll get all SharedNode instances
	GetAll() []SharedNodeCtx
	// GetAllDef get all SharedNode instances definition
	GetAllDef() (map[string][]*RuleNode, error)
}
