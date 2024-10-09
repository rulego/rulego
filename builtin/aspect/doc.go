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

// Package aspect provides built-in aspect-oriented programming (AOP) functionality for the RuleGo rule engine.
//
// This package implements various aspects that can be applied to rule nodes,
// allowing for cross-cutting concerns to be addressed separately from the main
// business logic. It includes pre-defined aspects such as debugging and metrics
// collection, as well as the ability to create custom aspects.
//
// Key components:
// - Debug: An aspect for logging debug information before and after node execution.
// - EndpointAspect: An aspect for rule chain endpoint.
//
// The package supports features such as:
// - Before and After execution hooks
// - Conditional aspect application based on node types or custom criteria
// - Ordering of multiple aspects
// - Easy integration with the RuleGo rule engine
//
// This package is crucial for adding observability, performance monitoring,
// and custom cross-cutting functionality to rule chains without modifying
// the core node implementations.
package aspect
