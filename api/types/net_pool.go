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

// NetNode represents a network resource node component such as a client or server connection.
type NetNode interface {
	Node
	// GetNetResource retrieves the underlying net client or server connection.
	// Used for connection pool reuse
	GetNetResource() (interface{}, error)
}

type NetNodeCtx interface {
	NodeCtx
	GetNetResource() (interface{}, error)
}

type NetPool interface {
	// New creates a new NetNode instance with the given ID and DSL.
	New(nodeType, id string, dsl []byte) (NetNodeCtx, error)
	// NewFromDef creates a new NetNode instance from a RuleNode definition.
	NewFromDef(def RuleNode) (NetNodeCtx, error)
	// Get retrieves a NetNode instance by its ID.
	Get(nodeType string, id string) (NetNodeCtx, bool)
	// GetNetResource retrieves a net client or server connection by its nodeTye and ID.
	GetNetResource(nodeType string, id string) (interface{}, error)
	// Del deletes a NetNode instance by its nodeTye and ID.
	Del(nodeType string, id string)
	// Stop stops and releases all NetNode instances.
	Stop()
	// GetAll get all NetNode instances
	GetAll() map[string][]NetNodeCtx
}
