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
	"github.com/rulego/rulego/api/types"
	string2 "github.com/rulego/rulego/utils/json"
)

// JsonParser Json
type JsonParser struct {
}

func (p *JsonParser) DecodeRuleChain(config types.Config, dsl []byte) (types.Node, error) {
	if rootRuleChainDef, err := ParserRuleChain(dsl); err == nil {
		//初始化
		return InitRuleChainCtx(config, &rootRuleChainDef)
	} else {
		return nil, err
	}
}
func (p *JsonParser) DecodeRuleNode(config types.Config, dsl []byte) (types.Node, error) {
	if node, err := ParserRuleNode(dsl); err == nil {
		return InitRuleNodeCtx(config, &node)
	} else {
		return nil, err
	}
}
func (p *JsonParser) EncodeRuleChain(def interface{}) ([]byte, error) {
	//缩进符为两个空格
	return string2.MarshalIndent(def, "", "  ")
}
func (p *JsonParser) EncodeRuleNode(def interface{}) ([]byte, error) {
	//缩进符为两个空格
	return string2.MarshalIndent(def, "", "  ")
}
