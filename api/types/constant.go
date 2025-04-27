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

import "errors"

const (
	CallbackFuncOnRuleChainCompleted = "onRuleChainCompleted"
	CallbackFuncOnNodeCompleted      = "onNodeCompleted"
	CallbackFuncDebug                = "onDebug"
)

const (
	Global = "global"
	// Vars ruleChain dsl additionalInfo vars key
	Vars = "vars"
	// Secrets ruleChain dsl additionalInfo secrets key
	Secrets = "secrets"
)

const (
	EndpointTypePrefix                = "endpoint/"
	NodeConfigurationPrefixInstanceId = "ref://"
)

const (
	//NodeConfigurationKeyIsInitNetResource 组件配置key是否是初始化网络资源，用于节点组件初始化参数校验区分
	NodeConfigurationKeyIsInitNetResource = "$initNetResource"
	// NodeConfigurationKeyChainCtx 获取规则链上下文Key, value类型: ChainCtx
	NodeConfigurationKeyChainCtx = "$chainCtx"
	//NodeConfigurationKeySelfDefinition 获取节点定义，value类型: RuleNode
	NodeConfigurationKeySelfDefinition = "$selfDefinition"
	//NodeConfigurationKeyRuleChainDefinition 获取规则链定义，应用于动态endpoint的初始化。value类型: *RuleChain
	NodeConfigurationKeyRuleChainDefinition = "$ruleChainDefinition"
)

var (
	// ErrConcurrencyLimitReached is the error returned when the concurrency limit has been reached
	ErrConcurrencyLimitReached = errors.New("concurrency limit reached")
)
