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

package rulego

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
	//组件配置
	SelfDefinition *RuleNode
	//规则引擎配置
	Config types.Config
}

//InitRuleNodeCtx 初始化RuleNodeCtx
func InitRuleNodeCtx(config types.Config, selfDefinition *RuleNode) (*RuleNodeCtx, error) {
	node, err := config.ComponentsRegistry.NewNode(selfDefinition.Type)
	if err != nil {
		return &RuleNodeCtx{}, err
	} else {
		if selfDefinition.Configuration == nil {
			selfDefinition.Configuration = make(types.Configuration)
		}
		if err = node.Init(config, processGlobalPlaceholders(config, selfDefinition.Configuration)); err != nil {
			return &RuleNodeCtx{}, err
		} else {
			return &RuleNodeCtx{
				Node:           node,
				SelfDefinition: selfDefinition,
				Config:         config,
			}, nil
		}
	}

}

func (rn *RuleNodeCtx) IsDebugMode() bool {
	return rn.SelfDefinition.DebugMode
}

func (rn *RuleNodeCtx) GetNodeId() types.RuleNodeId {
	return types.RuleNodeId{Id: rn.SelfDefinition.Id, Type: types.NODE}
}

func (rn *RuleNodeCtx) ReloadSelf(def []byte) error {
	if ruleNodeCtx, err := rn.Config.Parser.DecodeRuleNode(rn.Config, def); err == nil {
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
	v, _ := rn.Config.Parser.EncodeRuleNode(rn.SelfDefinition)
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
func processGlobalPlaceholders(config types.Config, configuration types.Configuration) types.Configuration {
	if config.Properties.Values() != nil {
		var result = make(types.Configuration)
		for key, value := range configuration {
			if strV, ok := value.(string); ok {
				result[key] = str.SprintfVar(strV, "global.", config.Properties.Values())
			} else {
				result[key] = value
			}
		}
		return result
	}
	return configuration
}
