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

package engine

import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
)

const (
	defaultNodeIdPrefix = "node"
)

// RuleNodeCtx 节点组件实例定义
type RuleNodeCtx struct {
	//组件实例
	types.Node
	//规则链配置上下文
	ChainCtx *RuleChainCtx
	//组件配置
	SelfDefinition *types.RuleNode
	//规则引擎配置
	config types.Config
}

// InitRuleNodeCtx 初始化RuleNodeCtx
func InitRuleNodeCtx(config types.Config, chainCtx *RuleChainCtx, selfDefinition *types.RuleNode) (*RuleNodeCtx, error) {
	node, err := config.ComponentsRegistry.NewNode(selfDefinition.Type)
	if err != nil {
		return &RuleNodeCtx{
			ChainCtx:       chainCtx,
			SelfDefinition: selfDefinition,
			config:         config,
		}, err
	} else {
		if selfDefinition.Configuration == nil {
			selfDefinition.Configuration = make(types.Configuration)
		}
		configuration, err := processVariables(config, chainCtx, selfDefinition.Configuration)
		if err != nil {
			return &RuleNodeCtx{}, err
		}
		if err = node.Init(config, configuration); err != nil {
			return &RuleNodeCtx{}, err
		} else {
			return &RuleNodeCtx{
				Node:           node,
				ChainCtx:       chainCtx,
				SelfDefinition: selfDefinition,
				config:         config,
			}, nil
		}
	}

}

func (rn *RuleNodeCtx) Config() types.Config {
	return rn.config
}

func (rn *RuleNodeCtx) IsDebugMode() bool {
	return rn.SelfDefinition.DebugMode
}

func (rn *RuleNodeCtx) GetNodeId() types.RuleNodeId {
	return types.RuleNodeId{Id: rn.SelfDefinition.Id, Type: types.NODE}
}

func (rn *RuleNodeCtx) ReloadSelf(def []byte) error {
	if ruleNodeCtx, err := rn.config.Parser.DecodeRuleNode(rn.config, def, rn.ChainCtx); err == nil {
		//先销毁
		rn.Destroy()
		//重新加载
		rn.Copy(ruleNodeCtx.(*RuleNodeCtx))
		return nil
	} else {
		return err
	}
}

func (rn *RuleNodeCtx) ReloadChild(_ types.RuleNodeId, _ []byte) error {
	return errors.New("not support this func")
}

func (rn *RuleNodeCtx) GetNodeById(_ types.RuleNodeId) (types.NodeCtx, bool) {
	return nil, false
}

func (rn *RuleNodeCtx) DSL() []byte {
	v, _ := rn.config.Parser.EncodeRuleNode(rn.SelfDefinition)
	return v
}

// Copy 复制
func (rn *RuleNodeCtx) Copy(newCtx *RuleNodeCtx) {
	rn.Node = newCtx.Node

	rn.SelfDefinition.AdditionalInfo = newCtx.SelfDefinition.AdditionalInfo
	rn.SelfDefinition.Name = newCtx.SelfDefinition.Name
	rn.SelfDefinition.Type = newCtx.SelfDefinition.Type
	rn.SelfDefinition.DebugMode = newCtx.SelfDefinition.DebugMode
	rn.SelfDefinition.Configuration = newCtx.SelfDefinition.Configuration
}

// 使用全局配置替换节点占位符配置，例如：${global.propertyKey}
func processVariables(config types.Config, chainCtx *RuleChainCtx, configuration types.Configuration) (types.Configuration, error) {
	var result = make(types.Configuration)
	globalEnv := make(map[string]string)

	if config.Properties != nil {
		globalEnv = config.Properties.Values()
	}

	var varsEnv map[string]string
	var decryptSecrets map[string]string

	if chainCtx != nil {
		varsEnv = copyMap(chainCtx.vars)
		//解密Secrets
		decryptSecrets = copyMap(chainCtx.decryptSecrets)
	}
	for key, value := range configuration {
		if strV, ok := value.(string); ok {
			v := str.SprintfVar(strV, types.Global+".", globalEnv)
			v = str.SprintfVar(v, types.Vars+".", varsEnv)
			result[key] = v
		} else {
			result[key] = value
		}
	}
	if varsEnv != nil {
		result[types.Vars] = varsEnv
	}
	if decryptSecrets != nil {
		result[types.Secrets] = decryptSecrets
	}
	return result, nil
}

func copyMap(inputMap map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range inputMap {
		result[key] = value
	}
	return result
}
