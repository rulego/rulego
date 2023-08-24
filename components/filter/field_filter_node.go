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

package filter

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"strings"
)

func init() {
	Registry.Add(&FieldFilterNode{})
}

//FieldFilterNodeConfiguration 节点配置
type FieldFilterNodeConfiguration struct {
	//是否是满足所有field key存在
	CheckAllKeys bool
	//msg data字段key多个与逗号隔开
	DataNames string
	//metadata字段key多个与逗号隔开
	MetadataNames string
}

//FieldFilterNode 过滤满足是否存在某个msg字段/metadata字段 消息
//如果 `True`发送信息到`True`链, `False`发到`False`链。
type FieldFilterNode struct {
	config            FieldFilterNodeConfiguration
	DataNamesList     []string
	MetadataNamesList []string
}

//Type 组件类型
func (x *FieldFilterNode) Type() string {
	return "fieldFilter"
}

func (x *FieldFilterNode) New() types.Node {
	return &FieldFilterNode{}
}

//Init 初始化
func (x *FieldFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.config)
	x.DataNamesList = strings.Split(x.config.DataNames, ",")
	x.MetadataNamesList = strings.Split(x.config.MetadataNames, ",")
	return err
}

//OnMsg 处理消息
func (x *FieldFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	var dataMap = make(map[string]interface{})
	if msg.DataType == types.JSON {
		if err := json.Unmarshal([]byte(msg.Data), &dataMap); err != nil {
			ctx.TellFailure(msg, err)
			return err
		}
	}

	if x.config.CheckAllKeys {
		if x.checkAllKeysMetadata(msg.Metadata) && x.checkAllKeysData(dataMap) {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	} else {
		if x.checkAtLeastOneMetadata(msg.Metadata) || x.checkAtLeastOneData(dataMap) {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
	return nil
}

//Destroy 销毁
func (x *FieldFilterNode) Destroy() {
}

func (x *FieldFilterNode) checkAllKeysMetadata(metadata types.Metadata) bool {
	for _, item := range x.MetadataNamesList {
		if !metadata.Has(item) {
			return false
		}
	}
	return true
}

func (x *FieldFilterNode) checkAllKeysData(data map[string]interface{}) bool {
	for _, item := range x.DataNamesList {
		if data == nil {
			return false
		}
		if _, ok := data[item]; !ok {
			return false
		}
	}
	return true
}

func (x *FieldFilterNode) checkAtLeastOneMetadata(metadata types.Metadata) bool {
	for _, item := range x.MetadataNamesList {
		if metadata.Has(item) {
			return true
		}
	}
	return false
}

func (x *FieldFilterNode) checkAtLeastOneData(data map[string]interface{}) bool {
	for _, item := range x.DataNamesList {
		if data == nil {
			return false
		}
		if _, ok := data[item]; ok {
			return true
		}
	}
	return false
}
