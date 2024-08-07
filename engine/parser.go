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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
)

// JsonParser Json
type JsonParser struct {
}

// DecodeRuleChain 通过json解析规则链结构体
func (p *JsonParser) DecodeRuleChain(rootRuleChain []byte) (types.RuleChain, error) {
	var def types.RuleChain
	err := json.Unmarshal(rootRuleChain, &def)
	return def, err
}

// DecodeRuleNode 通过json解析节点结构体
func (p *JsonParser) DecodeRuleNode(rootRuleChain []byte) (types.RuleNode, error) {
	var def types.RuleNode
	err := json.Unmarshal(rootRuleChain, &def)
	return def, err
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
