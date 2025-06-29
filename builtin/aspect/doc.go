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
// Package aspect 为 RuleGo 规则引擎提供内置的面向切面编程（AOP）功能。
// 该包实现了可应用于规则节点和链的各种切面，允许横切关注点与主要业务逻辑分离处理。
//
// Available Built-in Aspects:
// 可用的内置切面：
//
//   - Debug: Logging aspect for debug information before and after node execution
//     Debug：在节点执行前后记录调试信息的日志切面
//
//   - EndpointAspect: Management aspect for rule chain endpoints lifecycle
//     EndpointAspect：规则链端点生命周期管理切面
//
//   - ConcurrencyLimiterAspect: Limits concurrent execution of rule engine
//     ConcurrencyLimiterAspect：限制规则引擎并发执行的切面
//
//   - MetricsAspect: Collects and maintains rule engine execution metrics
//     MetricsAspect：收集和维护规则引擎执行指标的切面
//
//   - SkipFallbackAspect: Implements circuit breaker pattern for node failure handling
//     SkipFallbackAspect：实现节点故障处理的熔断器模式切面
//
//   - Validator: Validation aspect for rule chain initialization
//     Validator：规则链初始化验证切面
//
// Aspect Execution Order:
// 切面执行顺序：
//
// Aspects are executed in order based on their Order() method:
// 切面根据其 Order() 方法按顺序执行：
//  1. ConcurrencyLimiterAspect (order: 10)
//  2. SkipFallbackAspect (order: 10)
//  3. Validator (order: 10)
//  4. MetricsAspect (order: 20)
//  5. Debug (order: 900)
//  6. EndpointAspect (order: 900)
//
// Usage Examples:
// 使用示例：
//
//	// Apply debug aspect to rule engine
//	// 为规则引擎应用调试切面
//	engine := rulego.NewRuleEngine(types.NewConfig().WithAspects(&Debug{}))
//
//	// Apply multiple aspects with custom configuration
//	// 应用多个切面并自定义配置
//	engine := rulego.NewRuleEngine(types.NewConfig().WithAspects(
//		&Debug{},
//		NewConcurrencyLimiterAspect(100),
//		NewMetricsAspect(nil),
//		&SkipFallbackAspect{ErrorCountLimit: 5, LimitDuration: time.Minute},
//	))
//
// Custom Aspect Development:
// 自定义切面开发：
//
// To create custom aspects, implement one or more aspect interfaces:
// 要创建自定义切面，请实现一个或多个切面接口：
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
// 有关各个切面的详细文档，请参见其各自的源文件。
package aspect
