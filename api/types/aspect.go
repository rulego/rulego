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
	"sort"
)

// The interface provides AOP (Aspect Oriented Programming) mechanism, Which is similar to interceptor or hook mechanism, but more powerful and flexible.
//
//   - It allows adding extra behavior to the rule chain execution without modifying the original logic of the rule chain or nodes.
//   - It allows separating some common behaviors (such as logging, security, rule chain execution tracking, component degradation, component retry, component caching) from the business logic.
//
// 该接口提供 AOP(面向切面编程，Aspect Oriented Programming)机制，它类似拦截器或者hook机制，但是功能更加强大和灵活。
//
//   - 它允许在不修改规则链或节点的原有逻辑的情况下，对规则链的执行添加额外的行为，或者直接替换原规则链或者节点逻辑。
//   - 它允许把一些公共的行为（例如：日志、安全、规则链执行跟踪、组件降级、组件重试、组件缓存）从业务逻辑中分离出来。

// Aspect is the base interface for advice
// Aspect 增强点接口的基类
type Aspect interface {
	// Order returns the execution order, the smaller the value, the higher the priority
	// Order 返回执行顺序，值越小，优先级越高
	Order() int
	// New returns a new instance of the aspect
	// The method will be called to create a new instance during the initialization of the rule chain.
	// If the field value needs to be inherited, it must be handled here.
	// 规则链初始化时候会调用该方法创建新的实例，如果字段值需要继承，必须在这里处理
	New() Aspect
}

// NodeAspect is the base interface for node advice
// NodeAspect 节点增强点接口的基类
type NodeAspect interface {
	Aspect
	//PointCut declares a cut-in point, used to determine whether to execute the advice if OnMsg
	//PointCut 声明一个切入点，用于判断是否需要执行节点 OnMsg 相关增强点
	//For example: specify some component types or relationType to execute the aspect logic;return ctx.Self().Type()=="mqttClient"
	//例如：指定某些组件类型或者relationType才执行切面逻辑;return ctx.Self().Type()=="mqttClient"
	PointCut(ctx RuleContext, msg RuleMsg, relationType string) bool
}

// BeforeAspect is the interface for node pre-execution advice
// BeforeAspect 节点 OnMsg 方法执行之前的增强点接口
type BeforeAspect interface {
	NodeAspect
	// Before is the advice that executes before the node OnMsg method. The returned Msg will be used as the input for the next advice and the node OnMsg method.
	// Before 节点 OnMsg 方法执行之前的增强点。返回的Msg将作为下一个增强点和节点 OnMsg 方法的入参。
	Before(ctx RuleContext, msg RuleMsg, relationType string) RuleMsg
}

// AfterAspect is the interface for node post-execution advice
// AfterAspect 节点 OnMsg 方法执行后置增强点接口
type AfterAspect interface {
	NodeAspect
	//After is the advice that executes after the node OnMsg method. The returned Msg will be used as the input for the next advice and the next node OnMsg method.
	//After 节点 OnMsg 方法执行之后的增强点。返回的Msg将作为下一个增强点和下一个节点 OnMsg 方法的入参。
	After(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// AroundAspect is the interface for node around-execution advice
// AroundAspect 节点 OnMsg 方法执行环绕增强点接口
type AroundAspect interface {
	NodeAspect
	//Around is the advice that executes around the node OnMsg method. The returned Msg will be used as the input for the next advice and the next node OnMsg method.
	//Around 节点 OnMsg 方法执行环绕的增强点。返回的Msg将作为下一个增强点和下一个节点 OnMsg 方法的入参。
	//If the return is false: the engine will not call the current node's OnMsg method,
	//it needs to be manually triggered by Aspect, such as: ctx.Self().OnMsg (ctx, msg), or skip the current node's logic: ctx.TellNext .
	//如果返回false:引擎不会调用当前节点的OnMsg方法，需要Aspect手动触发，如：ctx.Self().OnMsg(ctx, msg)，或者跳过当前节点逻辑,如:ctx.TellNext。
	//If it returns true: the engine will call the current node's OnMsg method.
	//如果返回true：引擎执行当前节点逻辑。
	Around(ctx RuleContext, msg RuleMsg, relationType string) (RuleMsg, bool)
}

// StartAspect is the interface for rule engine pre-execution advice
// StartAspect 规则引擎 OnMsg 方法执行之前的增强点接口
type StartAspect interface {
	NodeAspect
	//Start is the advice that executes before the rule engine OnMsg method. The returned Msg will be used as the input for the next advice and the next node OnMsg method.
	//Start 规则引擎 OnMsg 方法执行之前的增强点。返回的Msg将作为下一个增强点和下一个节点 OnMsg 方法的入参。
	Start(ctx RuleContext, msg RuleMsg) RuleMsg
}

// EndAspect is the interface for rule engine post-execution advice
// EndAspect 规则引擎 OnMsg 方法执行之后，分支链执行结束的增强点接口
type EndAspect interface {
	NodeAspect
	// End is the advice that executes after the rule engine OnMsg method and the branch chain execution ends. The returned Msg will be used as the input for the next advice.
	// End 规则引擎 OnMsg 方法执行之后，分支链执行结束的增强点。返回的Msg将作为下一个增强点的入参。
	End(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// CompletedAspect is the interface for rule engine all branch execution end advice
// CompletedAspect 规则引擎 OnMsg 方法执行之后，所有分支链执行结束的增强点接口
type CompletedAspect interface {
	NodeAspect
	// Completed is the advice that executes after the rule engine OnMsg method and all branch chain execution ends. The returned Msg will be used as the input for the next advice.
	// Completed 规则引擎 OnMsg 方法执行之后，所有分支链执行结束的增强点。返回的Msg将作为下一个增强点的入参。
	Completed(ctx RuleContext, msg RuleMsg) RuleMsg
}

// OnChainBeforeInitAspect is the interface for rule engine initialization before advice
// OnChainBeforeInitAspect 规则引擎初始化之前的增强点，如果返回错误，则创建失败
type OnChainBeforeInitAspect interface {
	Aspect
	// OnChainBeforeInit is the advice that executes before the rule engine initialization.
	// OnChainBeforeInit 规则引擎初始化之前的增强点，如果返回错误，则创建失败
	OnChainBeforeInit(def *RuleChain) error
}

// OnNodeBeforeInitAspect is the interface for rule node initialization before advice
// OnNodeBeforeInitAspect 规则节点初始化之前的增强点，如果返回错误，则创建失败
type OnNodeBeforeInitAspect interface {
	Aspect
	// OnNodeBeforeInit is the advice that executes before the rule node initialization.
	// OnNodeBeforeInit 规则节点初始化之前的增强点，如果返回错误，则创建失败
	OnNodeBeforeInit(def *RuleNode) error
}

// OnCreatedAspect is the interface for rule engine creation success advice
// OnCreatedAspect 规则引擎成功创建之后增强点接口
type OnCreatedAspect interface {
	Aspect
	// OnCreated is the advice that executes after the rule engine is successfully created.
	// OnCreated 规则引擎成功创建之后的增强点
	OnCreated(chainCtx NodeCtx) error
}

// OnReloadAspect is the interface for rule engine reload rule chain or child node configuration advice
// OnReloadAspect 规则引擎重新加载规则链或者子节点配置之后增强点接口
type OnReloadAspect interface {
	Aspect
	// OnReload is the advice that executes after the rule engine reloads the rule chain or child node configuration.
	// OnReload 规则引擎重新加载规则链或者子节点配置之后的增强点。规则链更新会同时触发OnDestroy、OnBeforeReload、OnReload
	// If the rule chain is updated, then chainCtx=ctx
	// 如果更新规则链，则chainCtx=ctx
	OnReload(chainCtx NodeCtx, ctx NodeCtx) error
}

// OnDestroyAspect is the interface for rule engine instance destruction advice
// OnDestroyAspect 规则引擎实例销毁执行之后增强点接口
type OnDestroyAspect interface {
	Aspect
	// OnDestroy is the advice that executes after the rule engine instance is destroyed.
	// OnDestroy 规则引擎实例销毁执行之后增强点
	OnDestroy(chainCtx NodeCtx)
}

type AspectList []Aspect

// GetNodeAspects 获取节点执行类型增强点切面列表
func (list AspectList) GetNodeAspects() ([]AroundAspect, []BeforeAspect, []AfterAspect) {

	//从小到大排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Order() < list[j].Order()
	})

	var aroundAspects []AroundAspect
	var beforeAspects []BeforeAspect
	var afterAspects []AfterAspect

	for _, item := range list {
		if a, ok := item.(AroundAspect); ok {
			aroundAspects = append(aroundAspects, a)
		}
		if a, ok := item.(BeforeAspect); ok {
			beforeAspects = append(beforeAspects, a)
		}
		if a, ok := item.(AfterAspect); ok {
			afterAspects = append(afterAspects, a)
		}
	}

	return aroundAspects, beforeAspects, afterAspects
}

// GetChainAspects 获取规则链执行类型增强点切面列表
func (list AspectList) GetChainAspects() ([]StartAspect, []EndAspect, []CompletedAspect) {

	//从小到大排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Order() < list[j].Order()
	})

	var startAspects []StartAspect
	var endAspects []EndAspect
	var completedAspects []CompletedAspect
	for _, item := range list {
		if a, ok := item.(StartAspect); ok {
			startAspects = append(startAspects, a)
		}
		if a, ok := item.(EndAspect); ok {
			endAspects = append(endAspects, a)
		}
		if a, ok := item.(CompletedAspect); ok {
			completedAspects = append(completedAspects, a)
		}
	}

	return startAspects, endAspects, completedAspects
}

// GetEngineAspects 获取规则引擎类型增强点切面列表
func (list AspectList) GetEngineAspects() ([]OnChainBeforeInitAspect, []OnNodeBeforeInitAspect, []OnCreatedAspect, []OnReloadAspect, []OnDestroyAspect) {

	//从小到大排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Order() < list[j].Order()
	})

	var chainBeforeInitAspects []OnChainBeforeInitAspect
	var nodeBeforeInitAspects []OnNodeBeforeInitAspect
	var createdAspects []OnCreatedAspect
	var afterReloadAspects []OnReloadAspect
	var destroyAspects []OnDestroyAspect

	for _, item := range list {
		if a, ok := item.(OnChainBeforeInitAspect); ok {
			chainBeforeInitAspects = append(chainBeforeInitAspects, a)
		}
		if a, ok := item.(OnNodeBeforeInitAspect); ok {
			nodeBeforeInitAspects = append(nodeBeforeInitAspects, a)
		}
		if a, ok := item.(OnCreatedAspect); ok {
			createdAspects = append(createdAspects, a)
		}
		if a, ok := item.(OnReloadAspect); ok {
			afterReloadAspects = append(afterReloadAspects, a)
		}
		if a, ok := item.(OnDestroyAspect); ok {
			destroyAspects = append(destroyAspects, a)
		}
	}

	return chainBeforeInitAspects, nodeBeforeInitAspects, createdAspects, afterReloadAspects, destroyAspects
}
