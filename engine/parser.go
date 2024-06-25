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
	"github.com/rulego/rulego/utils/json"
)

// JsonParser Json
type JsonParser struct {
}

func (p *JsonParser) DecodeRuleChain(config types.Config, aspects types.AspectList, dsl []byte) (types.Node, error) {
	if rootRuleChainDef, err := ParserRuleChain(dsl); err == nil {
		//初始化
		return InitRuleChainCtx(config, aspects, &rootRuleChainDef)
	} else {
		return nil, err
	}
}

func (p *JsonParser) DecodeRuleNode(config types.Config, dsl []byte, chainCtx types.Node) (types.Node, error) {
	if node, err := ParserRuleNode(dsl); err == nil {
		if chainCtx == nil {
			return InitRuleNodeCtx(config, nil, nil, &node)
		} else if ruleChainCtx, ok := chainCtx.(*RuleChainCtx); !ok {
			return nil, errors.New("ruleChainCtx needs to be provided")
		} else {
			return InitRuleNodeCtx(config, ruleChainCtx, ruleChainCtx.aspects, &node)
		}
	} else {
		return nil, err
	}
}

func (p *JsonParser) EncodeRuleChain(def interface{}) ([]byte, error) {
	if v, err := json.Marshal(def); err != nil {
		return nil, err
	} else {
		//格式化Json
		return json.Format(v)
	}
}

func (p *JsonParser) EncodeRuleNode(def interface{}) ([]byte, error) {
	if v, err := json.Marshal(def); err != nil {
		return nil, err
	} else {
		//格式化Json
		return json.Format(v)
	}
}

// ParserRuleChain 通过json解析规则链结构体
func ParserRuleChain(rootRuleChain []byte) (types.RuleChain, error) {
	var def types.RuleChain
	err := json.Unmarshal(rootRuleChain, &def)
	return def, err
}

// ParserRuleNode 通过json解析节点结构体
func ParserRuleNode(rootRuleChain []byte) (types.RuleNode, error) {
	var def types.RuleNode
	err := json.Unmarshal(rootRuleChain, &def)
	return def, err
}
