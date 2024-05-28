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

type RuleEngineOption func(RuleEngine) error

func WithConfig(config Config) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetConfig(config)
		return nil
	}
}

type RuleEngine interface {
	Id() string
	SetConfig(config Config)
	Reload(opts ...RuleEngineOption) error
	ReloadSelf(def []byte, opts ...RuleEngineOption) error
	ReloadChild(ruleNodeId string, dsl []byte) error
	DSL() []byte
	Definition() RuleChain
	RootRuleChainCtx() ChainCtx
	NodeDSL(chainId RuleNodeId, childNodeId RuleNodeId) []byte
	Initialized() bool
	Stop()
	OnMsg(msg RuleMsg, opts ...RuleContextOption)
	OnMsgAndWait(msg RuleMsg, opts ...RuleContextOption)
}

type RuleEnginePool interface {
	// Load 加载指定文件夹及其子文件夹所有规则链配置（与.json结尾文件），到规则引擎实例池
	// 规则链ID，使用文件配置的 ruleChain.id
	Load(folderPath string, opts ...RuleEngineOption) error
	// New 创建一个新的RuleEngine并将其存储在RuleGo规则链池中
	New(id string, rootRuleChainSrc []byte, opts ...RuleEngineOption) (RuleEngine, error)
	Get(id string) (RuleEngine, bool)
	// Del 删除指定ID规则引擎实例
	Del(id string)
	// Stop 释放所有规则引擎实例
	Stop()
	// OnMsg 调用所有规则引擎实例处理消息
	// 规则引擎实例池所有规则链都会去尝试处理该消息
	OnMsg(msg RuleMsg)
	// Reload 重新加载所有规则引擎实例
	Reload(opts ...RuleEngineOption)
	// Range 遍历所有规则引擎实例
	Range(f func(key, value any) bool)
}
