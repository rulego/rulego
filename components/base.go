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

package components

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
)

type BaseNode struct {
}

var NodeUtils = &nodeUtils{}

type nodeUtils struct {
}

func (n *nodeUtils) GetVars(configuration types.Configuration) map[string]interface{} {
	if v, ok := configuration[types.Vars]; ok {
		fromVars := make(map[string]interface{})
		fromVars[types.Vars] = v
		return fromVars
	} else {
		return nil
	}
}

func (n *nodeUtils) GetEvn(_ types.RuleContext, msg types.RuleMsg) (map[string]interface{}, error) {
	var data interface{}
	if msg.DataType == types.JSON {
		// 解析 JSON 字符串到 map
		if err := json.Unmarshal([]byte(msg.Data), &data); err != nil {
			// 解析失败，使用原始数据
			data = msg.Data
		}
	} else {
		// 如果不是 JSON 类型，直接使用原始数据
		data = msg.Data
	}
	var evn = make(map[string]interface{})
	evn[types.IdKey] = msg.Id
	evn[types.TsKey] = msg.Ts
	evn[types.DataKey] = msg.Data
	evn[types.MsgKey] = data
	evn[types.MetadataKey] = msg.Metadata
	evn[types.MsgTypeKey] = msg.Type
	evn[types.TypeKey] = msg.Type
	evn[types.DataTypeKey] = msg.DataType
	return evn, nil
}
