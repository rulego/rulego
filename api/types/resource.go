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

// resource.go defines a general abstraction of the ref:// reference mechanism (underlying library, performance-sensitive):
//   - ResourceLookup / ResourceRegistry: Resource directory (parse by ID/register instances)
//   - TargetSender: Addressing and sending capability interface
//
// Design Objectives:
//   - Read paths without locks and zero allocation (implemented using sync.Map, etc.)
//   - No specific component type: Any "container" implementing ResourceLookup can become a ref:// parsing source;
//     Any addressable component implementing TargetSender can be addressed and pushed by nodes such as net (open/close principle).
//   - Avoid types main package ↔ endpoint subpacket loop dependencies: interface parameters use any, and the consumer makes capability assertions

// ResourceLookup is a read-only parsed view referenced by ref://.
// The implementer must ensure that lookup reads paths without locks and zero allocation.
//
// Known implementation: *node_pool.NodePool (shared pool), *engine.RuleChainCtx (same-chain endpoints, etc.).
type ResourceLookup interface {
	// Lookup searches for resource instances by ID. Return not found (nil, false).
	Lookup(id string) (resource any, found bool)
}

// ResourceRegistry is a writable resource directory with embedded ResourceLookup.
// Write (Register/Unregister) low frequency (when component deployment/overload); Lookup (ref:// Analysis).
type ResourceRegistry interface {
	ResourceLookup
	// Register: Register/override a resource instance.
	Register(id string, resource any)
	// Unregister logs out a resource instance.
	Unregister(id string)
}

// TargetSender is an interface capable of "sending by target address."
// Addressable resources (such as endpoint/net) implementing this interface can be pushed and reused by the net node's ref:// addressing.
//
// The implementer must guarantee: zero allocation (reuse of parameter data) and concurrency security.
type TargetSender interface {
	// SendToTarget sends data by target address.
	//   - target:IP / deviceId / *(broadcast) / empty (broadcast)
	// Returns sent (number of successes), failed (number of failures), err (first error; If all failed, it would also return).
	SendToTarget(target string, data []byte) (sent, failed int, err error)
}
