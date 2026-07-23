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
// This package implements various aspects that can be applied to rule nodes and chains,
// allowing for cross-cutting concerns to be addressed separately from the main business logic.
//
// Package aspect provides built-in face-to-face programming (AOP) functionality for the RuleGo rule engine.
// This package implements aspects for rule nodes and chains, allowing cross-cutting concerns to be handled separately from the main business logic.
//
// Available Built-in Aspects:
// Available built-in aspects:
//
//   - Debug: Logging aspect for debug information before and after node execution
//     Debug: A log section that records debugging information before and after node execution
//
//   - EndpointAspect: Management aspect for rule chain endpoints lifecycle
//     EndpointAspect: Rule chain endpoint lifecycle management face-to-face
//
//   - ConcurrencyLimiterAspect: Limits concurrent execution of rule engine
//     ConcurrencyLimiterAspect: The aspect of concurrent execution by the restriction rule engine
//
//   - MetricsAspect: Collects and maintains rule engine execution metrics
//     MetricsAspect: Collects and maintains rule engine execution metrics
//
//   - SkipFallbackAspect: Implements circuit breaker pattern for node failure handling
//     SkipFallbackAspect: Fuse mode cross-section for node fault handling
//
//   - Validator: Validation aspect for rule chain initialization
//     Validator: Validates rule chain initialization
//
// Aspect Execution Order:
// Aspect Execution Order:
//
// Aspects are executed in order based on their Order() method:
// The faces are executed sequentially according to their Order() method:
//  1. ConcurrencyLimiterAspect (order: 10)
//  2. SkipFallbackAspect (order: 10)
//  3. Validator (order: 10)
//  4. MetricsAspect (order: 20)
//  5. Debug (order: 900)
//  6. EndpointAspect (order: 900)
//
// Usage Examples:
// Example:
//
//	// Apply debug aspect to rule engine
//	Debug aspects for rule engine applications
//	engine := rulego.NewRuleEngine(types.NewConfig().WithAspects(&Debug{}))
//
//	// Apply multiple aspects with custom configuration
//	Apply multiple aspects and customize their configuration
//	engine := rulego.NewRuleEngine(types.NewConfig().WithAspects(
//		&Debug{},
//		NewConcurrencyLimiterAspect(100),
//		NewMetricsAspect(nil),
//		&SkipFallbackAspect{ErrorCountLimit: 5, LimitDuration: time.Minute},
//	))
//
// Custom Aspect Development:
// Custom Facet Development:
//
// To create custom aspects, implement one or more aspect interfaces:
// To create custom aspects, implement one or more aspect interfaces:
//
//	type CustomAspect struct{}
//
//	func (a *CustomAspect) Order() int { return 100 }
//	func (a *CustomAspect) New() types.Aspect { return &CustomAspect{} }
//	func (a *CustomAspect) Type() string { return "custom" }
//	func (a *CustomAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
//		return true // Apply to all nodes
//	}
//
// For detailed documentation on individual aspects, see their respective source files.
// For detailed documentation on each aspect, please refer to their respective source files.
package aspect
