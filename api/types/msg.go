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
	"github.com/gofrs/uuid/v5"
	"time"
)

// DataType 消息数据类型
type DataType string

const (
	JSON   = DataType("JSON")
	TEXT   = DataType("TEXT")
	BINARY = DataType("BINARY")
)

const (
	MsgKey      = "msg"
	MetadataKey = "metadata"
	MsgTypeKey  = "msgType"
)

// Metadata 规则引擎消息元数据
type Metadata map[string]string

// NewMetadata 创建一个新的规则引擎消息元数据实例
func NewMetadata() Metadata {
	return make(Metadata)
}

// BuildMetadata 通过map，创建一个新的规则引擎消息元数据实例
func BuildMetadata(data Metadata) Metadata {
	metadata := make(Metadata)
	for k, v := range data {
		metadata[k] = v
	}
	return metadata
}

// Copy 复制
func (md Metadata) Copy() Metadata {
	return BuildMetadata(md)
}

// Has 是否存在某个key
func (md Metadata) Has(key string) bool {
	_, ok := md[key]
	return ok
}

// GetValue 通过key获取值
func (md Metadata) GetValue(key string) string {
	v, _ := md[key]
	return v
}

// PutValue 设置值
func (md Metadata) PutValue(key, value string) {
	if key != "" {
		md[key] = value
	}
}

// Values 获取所有值
func (md Metadata) Values() map[string]string {
	return md
}

// RuleMsg 规则引擎消息
type RuleMsg struct {
	// 消息时间戳
	Ts int64 `json:"ts"`
	// 消息ID，同一条消息再规则引擎流转，整个过程是唯一的
	Id string `json:"id"`
	//数据类型
	DataType DataType `json:"dataType"`
	//消息类型，规则引擎分发数据的重要字段
	//一般把消息交给规则引擎处理`ruleEngine.OnMsg(msg)`,需要把消息分类并指定其Type
	//例如：POST_TELEMETRY、ACTIVITY_EVENT、INACTIVITY_EVENT、CONNECT_EVENT、DISCONNECT_EVENT
	//ENTITY_CREATED、ENTITY_UPDATED、ENTITY_DELETED、DEVICE_ALARM、POST_DEVICE_DATA
	Type string `json:"type"`
	//消息内容
	Data string `json:"data"`
	//消息元数据
	Metadata Metadata `json:"metadata"`
}

// NewMsg 创建一个新的消息实例，并通过uuid生成消息ID
func NewMsg(ts int64, msgType string, dataType DataType, metaData Metadata, data string) RuleMsg {
	uuId, _ := uuid.NewV4()
	return newMsg(uuId.String(), ts, msgType, dataType, metaData, data)
}

func newMsg(id string, ts int64, msgType string, dataType DataType, metaData Metadata, data string) RuleMsg {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	if id == "" {
		uuId, _ := uuid.NewV4()
		id = uuId.String()
	}
	return RuleMsg{
		Ts:       ts,
		Id:       id,
		Type:     msgType,
		DataType: dataType,
		Data:     data,
		Metadata: metaData,
	}
}

// Copy 复制
func (m *RuleMsg) Copy() RuleMsg {
	return newMsg(m.Id, m.Ts, m.Type, m.DataType, m.Metadata.Copy(), m.Data)
}

// WrapperMsg 节点执行结果封装，用于封装多个节点执行结果
type WrapperMsg struct {
	//Msg 消息
	Msg RuleMsg `json:"msg"`
	//Err 错误
	Err string `json:"err"`
	//NodeId 结束节点ID
	NodeId string `json:"nodeId"`
}
