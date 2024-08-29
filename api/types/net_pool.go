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

// NetResource represents a network resource node component such as a client or server connection.
type NetResource interface {
	Node
	// GetNetResource retrieves the underlying net client or server connection.
	// Used for connection pool reuse
	GetNetResource() (interface{}, error)
}

type NetResourceCtx interface {
	NodeCtx
	GetNetResource() (interface{}, error)
}

type NetPool interface {
	// New creates a new NetResource instance with the given ID and DSL.
	New(nodeType, id string, dsl []byte) (NetResourceCtx, error)
	// NewFromDef creates a new NetResource instance from a RuleNode definition.
	NewFromDef(def RuleNode) (NetResourceCtx, error)
	// Get retrieves a NetResource instance by its ID.
	Get(nodeType string, id string) (NetResourceCtx, bool)
	// GetNetResource retrieves a net client or server connection by its nodeTye and ID.
	GetNetResource(nodeType string, id string) (interface{}, error)
	// Del deletes a NetResource instance by its nodeTye and ID.
	Del(nodeType string, id string)
	// Stop stops and releases all NetResource instances.
	Stop()
	// GetAll get all NetResource instances
	GetAll() map[string][]NetResourceCtx
}
